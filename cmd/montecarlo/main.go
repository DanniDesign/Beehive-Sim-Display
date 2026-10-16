// Command montecarlo runs the colony model headless many times and reports
// how realistic and stable it is: survival, seasonal population cycle,
// swarming, food stores, comb layout and the causes of death.
//
//	go run ./cmd/montecarlo -runs 2000 -years 3
//	go run ./cmd/montecarlo -trace -seed 7 -years 1   # one colony, day by day
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"sort"
	"sync"
	"time"

	"beehive-sim2/simulation"
)

type result struct {
	years         [][4]seasonStat // per year, per season
	end           simulation.Snapshot
	firstCollapse int // day, -1 if never
	peakWorkers   int
	honeyArchOK   int // days honey sat above the brood
	archDays      int
	lifespanBins  []float64
}

type seasonStat struct {
	workers, drones, brood, honey, foragerFrac, outsideFrac float64
	n                                                       int
	aliveAtEnd                                              bool
}

func simulate(seed int64, years int, p simulation.Params, w, hgt int) result {
	h := simulation.NewHiveWithSeed(seed, 120, w, hgt, p)
	r := result{firstCollapse: -1, years: make([][4]seasonStat, years)}
	totalTicks := years * simulation.YearDays * simulation.DayTicks
	for t := 0; t < totalTicks; t++ {
		h.Update()
		if h.Tick%(simulation.DayTicks/4) != 0 {
			continue
		}
		s := h.Snapshot()
		year := t / (simulation.YearDays * simulation.DayTicks)
		ss := &r.years[year][s.Season]
		c := s.Counts
		ss.workers += float64(c.Workers)
		ss.drones += float64(c.Drones)
		ss.brood += float64(c.Eggs + c.Larvae + c.Pupae)
		ss.honey += s.Honey + s.Nectar
		if c.Workers > 0 {
			ss.foragerFrac += float64(c.Foragers) / float64(c.Workers)
			ss.outsideFrac += float64(c.Outside) / float64(c.Workers)
		}
		ss.n++
		ss.aliveAtEnd = !s.Dead
		if c.Workers > r.peakWorkers {
			r.peakWorkers = c.Workers
		}
		if s.Dead && r.firstCollapse < 0 {
			r.firstCollapse = s.Day
		}
		if c.Eggs+c.Larvae+c.Pupae > 30 && s.HoneyCells > 30 {
			r.archDays++
			if s.HoneyMeanY < s.BroodMeanY {
				r.honeyArchOK++
			}
		}
	}
	r.end = h.Snapshot()
	return r
}

func trace(seed int64, years int, p simulation.Params, w, hgt int) {
	h := simulation.NewHiveWithSeed(seed, 120, w, hgt, p)
	fmt.Printf("%4s %5s %2s | %4s %3s %3s %3s %3s %3s %3s | %3s %3s %3s %2s | %6s %6s %5s | %s\n",
		"day", "doy", "Q", "wkr", "cln", "nrs", "hse", "for", "out", "drn", "egg", "lrv", "pup", "RC", "honey", "nectar", "pol", "events")
	prev := h.Snapshot().Stats
	for d := 0; d < years*simulation.YearDays; d++ {
		for t := 0; t < simulation.DayTicks; t++ {
			h.Update()
		}
		s := h.Snapshot()
		c := s.Counts
		q := "-"
		if s.HasQueen {
			q = "v"
			if s.QueenMated {
				q = "Q"
			}
		}
		ev := ""
		st := s.Stats
		add := func(name string, a, b int) {
			if a != b {
				ev += fmt.Sprintf("%s+%d ", name, a-b)
			}
		}
		add("swarm", st.Swarms, prev.Swarms)
		add("prep", st.SwarmPreparations, prev.SwarmPreparations)
		add("emerg", st.EmergencyQueenCells, prev.EmergencyQueenCells)
		add("qborn", st.QueensEmerged, prev.QueensEmerged)
		add("mated", st.QueensMated, prev.QueensMated)
		add("qlost", st.QueensLostMating, prev.QueensLostMating)
		add("qdied", st.QueensDied, prev.QueensDied)
		add("starve", st.StarvationEpisodes, prev.StarvationEpisodes)
		add("lstarve", st.LarvaeStarved, prev.LarvaeStarved)
		add("collapse", st.Collapses, prev.Collapses)
		add("recol", st.Recolonizations, prev.Recolonizations)
		add("expel", st.DronesExpelled, prev.DronesExpelled)
		ev += fmt.Sprintf("in%.1f eat%.1f trips%d ", st.NectarCollected-prev.NectarCollected, st.HoneyEaten-prev.HoneyEaten, st.ForagingTrips-prev.ForagingTrips)
		if s.HoneyBound {
			ev += "honeybound "
		}
		prev = st
		fmt.Printf("%4d %5.1f %2s | %4d %3d %3d %3d %3d %3d %3d | %3d %3d %3d %2d | %6.1f %6.1f %5.2f | %s\n",
			s.Day, s.DayOfYear, q, c.Workers, c.Cleaners, c.Nurses, c.HouseBees+c.Guards, c.Foragers, c.Outside, c.Drones,
			c.Eggs, c.Larvae, c.Pupae, c.RoyalBrood, s.Honey, s.Nectar, s.Pollen, ev)
	}
	st := h.Snapshot().Stats
	fmt.Printf("\ntotals: %+v\n", st)
}

func pct(a, b int) float64 { return 100 * float64(a) / math.Max(1, float64(b)) }

func quantiles(v []float64) string {
	if len(v) == 0 {
		return "-"
	}
	sort.Float64s(v)
	q := func(f float64) float64 { return v[int(f*float64(len(v)-1))] }
	return fmt.Sprintf("p10 %6.1f  p50 %6.1f  p90 %6.1f", q(0.1), q(0.5), q(0.9))
}

func main() {
	runs := flag.Int("runs", 1000, "number of independent colonies")
	years := flag.Int("years", 3, "simulated years per colony")
	seed := flag.Int64("seed", 1, "first seed")
	doTrace := flag.Bool("trace", false, "print one colony day by day")
	w := flag.Int("w", 64, "comb width")
	hgt := flag.Int("h", 32, "comb height")
	flag.Parse()
	p := simulation.DefaultParams()

	if *doTrace {
		trace(*seed, *years, p, *w, *hgt)
		return
	}

	start := time.Now()
	results := make([]result, *runs)
	var wg sync.WaitGroup
	jobs := make(chan int)
	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				results[i] = simulate(*seed+int64(i), *years, p, *w, *hgt)
			}
		}()
	}
	for i := 0; i < *runs; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	elapsed := time.Since(start)
	ticks := float64(*runs * *years * simulation.YearDays * simulation.DayTicks)

	fmt.Printf("%d colonies x %d years (%d ticks each) in %s, %.1f us/tick/core\n\n",
		*runs, *years, *years*simulation.YearDays*simulation.DayTicks, elapsed.Round(time.Second),
		elapsed.Seconds()*float64(runtime.NumCPU())/ticks*1e6)

	seasons := []string{"spring", "summer", "autumn", "winter"}
	fmt.Println("Seasonal means (colonies alive at that time):")
	fmt.Printf("  %-12s %8s %7s %7s %8s %8s %8s %8s\n", "", "alive%", "workers", "drones", "brood", "stores", "forager%", "outside%")
	for y := 0; y < *years; y++ {
		for s := 0; s < 4; s++ {
			var agg seasonStat
			alive := 0
			for _, r := range results {
				ss := r.years[y][s]
				if ss.n == 0 {
					continue
				}
				if ss.aliveAtEnd {
					alive++
				}
				agg.workers += ss.workers / float64(ss.n)
				agg.drones += ss.drones / float64(ss.n)
				agg.brood += ss.brood / float64(ss.n)
				agg.honey += ss.honey / float64(ss.n)
				agg.foragerFrac += ss.foragerFrac / float64(ss.n)
				agg.outsideFrac += ss.outsideFrac / float64(ss.n)
			}
			n := float64(len(results))
			fmt.Printf("  y%d %-9s %7.1f%% %7.0f %7.1f %8.0f %8.0f %7.0f%% %7.0f%%\n", y+1, seasons[s],
				pct(alive, len(results)), agg.workers/n, agg.drones/n, agg.brood/n, agg.honey/n,
				100*agg.foragerFrac/n, 100*agg.outsideFrac/n)
		}
	}

	var collapses, starved, queenless, dwindled, recol, swarms, everCollapsed, deadAtEnd int
	var emergency, qlost, qdied, droneLayers, starveEp, larvaStarved int
	var arch, archDays int
	var peak, swarmsPerYear, firstCollapse []float64
	var deaths [4]int
	var born int
	for _, r := range results {
		st := r.end.Stats
		collapses += st.Collapses
		starved += st.CollapseStarved
		queenless += st.CollapseQueenless
		dwindled += st.CollapseDwindled
		recol += st.Recolonizations
		swarms += st.Swarms
		emergency += st.EmergencyQueenCells
		qlost += st.QueensLostMating
		qdied += st.QueensDied
		droneLayers += st.DroneLayingQueens
		starveEp += st.StarvationEpisodes
		larvaStarved += st.LarvaeStarved
		deaths[0] += st.DiedOldAge
		deaths[1] += st.DiedForaging
		deaths[2] += st.DiedStarvation
		deaths[3] += st.LeftWithSwarm
		born += st.WorkersBorn
		arch += r.honeyArchOK
		archDays += r.archDays
		if st.Collapses > 0 {
			everCollapsed++
			firstCollapse = append(firstCollapse, float64(r.firstCollapse))
		}
		if r.end.Dead {
			deadAtEnd++
		}
		peak = append(peak, float64(r.peakWorkers))
		swarmsPerYear = append(swarmsPerYear, float64(st.Swarms)/float64(*years))
	}
	n := len(results)
	fmt.Printf("\nColony fate over %d years:\n", *years)
	fmt.Printf("  never collapsed        %6.1f%%\n", pct(n-everCollapsed, n))
	fmt.Printf("  dead at end            %6.1f%%  (empty hives get re-occupied by wild swarms: %d times)\n", pct(deadAtEnd, n), recol)
	fmt.Printf("  collapses              %d  (starved %d, queenless %d, dwindled %d)\n", collapses, starved, queenless, dwindled)
	fmt.Printf("  first collapse day     %s\n", quantiles(firstCollapse))
	fmt.Printf("  peak workers           %s\n", quantiles(peak))
	fmt.Printf("  swarms / year          %s  (total %d, %.2f per colony-year)\n", quantiles(swarmsPerYear), swarms, float64(swarms)/float64(n**years))
	fmt.Printf("  emergency queen cells  %d, queens lost mating %d, queens died %d, drone-laying queens %d\n", emergency, qlost, qdied, droneLayers)
	fmt.Printf("  starvation episodes    %d, larvae starved %d\n", starveEp, larvaStarved)
	td := deaths[0] + deaths[1] + deaths[2] + deaths[3]
	fmt.Printf("  worker exits           old age %.0f%%, lost foraging %.0f%%, starved %.0f%%, left with swarm %.0f%%  (born %d)\n",
		pct(deaths[0], td), pct(deaths[1], td), pct(deaths[2], td), pct(deaths[3], td), born)
	fmt.Printf("  honey above brood      %.1f%% of colony-days with a brood nest\n", pct(arch, archDays))
	if deadAtEnd*2 > n {
		os.Exit(1)
	}
}
