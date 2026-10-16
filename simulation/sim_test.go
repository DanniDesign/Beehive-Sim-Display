package simulation

import (
	"fmt"
	"math"
	"sync"
	"testing"
)

// checkInvariants verifies that every incrementally maintained index agrees
// with a from-scratch recount of the comb and the bees.
func checkInvariants(h *Hive) error {
	occ := make([]uint8, len(h.occ))
	queens := 0
	for _, b := range h.bees {
		if b.dead {
			return fmt.Errorf("dead bee %d still in list", b.ID)
		}
		if b.Role == Queen && !b.leaving {
			queens++
			if h.queen != b {
				return fmt.Errorf("queen %d is not the hive's queen pointer", b.ID)
			}
		}
		if b.IsOutside {
			continue
		}
		if !h.inComb(b.X, b.Y) {
			return fmt.Errorf("bee %d off the comb at %d,%d", b.ID, b.X, b.Y)
		}
		occ[h.idx(b.X, b.Y)]++
	}
	if queens > 1 {
		return fmt.Errorf("%d queens", queens)
	}
	if h.queen != nil && (h.queen.dead || h.queen.Role != Queen) {
		return fmt.Errorf("queen pointer is stale")
	}
	for i := range occ {
		if occ[i] != h.occ[i] {
			return fmt.Errorf("occupancy mismatch at %d,%d: %d vs %d", i%h.width, i/h.width, occ[i], h.occ[i])
		}
	}

	var honey, nectar, pollen float64
	brood, nectarCells, pollenCells, dirty, built := 0, 0, 0, 0, 0
	for y := range h.Grid {
		for x := range h.Grid[y] {
			c := &h.Grid[y][x]
			i := h.idx(x, y)
			if !h.inComb(x, y) && (c.State != Empty || c.Dirty) {
				return fmt.Errorf("frame cell %d,%d in use", x, y)
			}
			if !c.Built && (c.State != Empty || c.Dirty) {
				return fmt.Errorf("cell %d,%d used but no comb built", x, y)
			}
			if c.Built {
				built++
			}
			if h.inComb(x, y) && !c.Built && (h.frontier.pos[i] >= 0) != h.touchesComb(x, y) {
				return fmt.Errorf("frontier wrong at %d,%d", x, y)
			}
			if c.Amount < 0 || c.Amount > 1.0001 {
				return fmt.Errorf("cell %d,%d amount %f", x, y, c.Amount)
			}
			switch c.State {
			case Honey:
				honey += float64(c.Amount)
			case Nectar:
				nectar += float64(c.Amount)
				nectarCells++
				if h.nectarSet.pos[i] < 0 {
					return fmt.Errorf("nectar cell %d,%d missing from set", x, y)
				}
			case Pollen:
				pollen += float64(c.Amount)
				pollenCells++
				if h.pollenSet.pos[i] < 0 {
					return fmt.Errorf("pollen cell %d,%d missing from set", x, y)
				}
			case Egg, Larva, Pupa:
				brood++
				if p := h.broodPos[i]; p < 0 || h.brood[p] != int32(i) {
					return fmt.Errorf("brood cell %d,%d not indexed", x, y)
				}
			}
			if c.Dirty {
				dirty++
				if c.State != Empty || h.dirtySet.pos[i] < 0 {
					return fmt.Errorf("dirty cell %d,%d inconsistent", x, y)
				}
			}
		}
	}
	if built != h.combCells {
		return fmt.Errorf("comb count %d vs %d", built, h.combCells)
	}
	if brood != len(h.brood) || nectarCells != len(h.nectarSet.items) ||
		pollenCells != len(h.pollenSet.items) || dirty != len(h.dirtySet.items) {
		return fmt.Errorf("index sizes: brood %d/%d nectar %d/%d pollen %d/%d dirty %d/%d",
			brood, len(h.brood), nectarCells, len(h.nectarSet.items), pollenCells, len(h.pollenSet.items), dirty, len(h.dirtySet.items))
	}
	for name, pair := range map[string][2]float64{"honey": {honey, h.honey}, "nectar": {nectar, h.nectar}, "pollen": {pollen, h.pollen}} {
		if math.Abs(pair[0]-pair[1]) > 1e-3*(1+pair[0]) {
			return fmt.Errorf("%s total drifted: cells %f, counter %f", name, pair[0], pair[1])
		}
	}
	return nil
}

func TestInvariantsManyColonies(t *testing.T) {
	runs, days := 48, YearDays*2
	if testing.Short() {
		runs, days = 8, YearDays
	}
	var wg sync.WaitGroup
	errs := make(chan error, runs)
	sem := make(chan struct{}, 4)
	for seed := 0; seed < runs; seed++ {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			h := NewHiveWithSeed(seed, 120, 64, 32, DefaultParams())
			for tick := 0; tick < days*DayTicks; tick++ {
				h.Update()
				if tick%97 == 0 {
					if err := checkInvariants(h); err != nil {
						errs <- fmt.Errorf("seed %d tick %d: %w", seed, tick, err)
						return
					}
				}
			}
		}(int64(seed))
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestDeterministic(t *testing.T) {
	run := func() Snapshot {
		h := NewHiveWithSeed(42, 120, 64, 32, DefaultParams())
		for i := 0; i < 60*DayTicks; i++ {
			h.Update()
		}
		return h.Snapshot()
	}
	a, b := run(), run()
	if a != b {
		t.Fatalf("same seed gave different colonies:\n%+v\n%+v", a, b)
	}
}

// TestSeasonalCycle checks the colony behaves like a temperate colony on
// average: it grows through spring, peaks in early summer, shrinks into
// winter, stores honey above the brood, and most colonies survive.
func TestSeasonalCycle(t *testing.T) {
	runs := 32
	if testing.Short() {
		runs = 8
	}
	type sample struct{ may, oct, jan float64 }
	results := make([]sample, runs)
	alive := make([]bool, runs)
	arch := make([]bool, runs)
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i := 0; i < runs; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			h := NewHiveWithSeed(int64(1000+i), 120, 64, 32, DefaultParams())
			at := func(doy int) Snapshot {
				for int(h.DayOfYear()) != doy {
					h.Update()
				}
				return h.Snapshot()
			}
			may := at(90) // end of May
			arch[i] = may.HoneyMeanY < may.BroodMeanY
			oct := at(230)
			jan := at(320)
			results[i] = sample{float64(may.Counts.Workers), float64(oct.Counts.Workers), float64(jan.Counts.Workers)}
			alive[i] = !jan.Dead
		}(i)
	}
	wg.Wait()
	var may, oct, jan float64
	nAlive, nArch := 0, 0
	for i, r := range results {
		if alive[i] {
			nAlive++
			may += r.may
			oct += r.oct
			jan += r.jan
		}
		if arch[i] {
			nArch++
		}
	}
	if nAlive < runs*6/10 {
		t.Fatalf("only %d/%d colonies alive in January", nAlive, runs)
	}
	n := float64(nAlive)
	t.Logf("mean workers: May %.0f, Oct %.0f, Jan %.0f; alive %d/%d; honey above brood in May %d/%d",
		may/n, oct/n, jan/n, nAlive, runs, nArch, runs)
	if !(may > oct && oct > jan) {
		t.Errorf("population should peak in early summer and shrink into winter")
	}
	if nArch < runs*8/10 {
		t.Errorf("honey should be stored above the brood nest")
	}
}
