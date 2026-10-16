package simulation

import (
	"math"
	"sort"
)

type CellState uint8

const (
	Empty  CellState = iota
	Egg              // freshly laid egg
	Larva            // open brood, has to be fed by nurses
	Pupa             // capped brood
	Nectar           // unripe nectar, still being dried
	Honey            // ripe, capped honey
	Pollen           // "bee bread", protein for the nurses
)

// BroodKind says what an egg/larva/pupa will become.
type BroodKind uint8

const (
	WorkerBrood BroodKind = iota
	DroneBrood
	QueenBrood // a royal cell
)

type Cell struct {
	State  CellState
	Kind   BroodKind
	Built  bool    // wax comb exists here (bees build it over time)
	Dirty  bool    // used cell that has to be cleaned before reuse
	Amount float32 // food in the cell (nectar/honey/pollen), 0..1
	since  int32   // tick the cell entered its current state
	fed    int32   // tick a larva was last fed
}

// IsBrood reports whether the cell holds an egg, larva or pupa.
func (c *Cell) IsBrood() bool { return c.State == Egg || c.State == Larva || c.State == Pupa }

// IsFood reports whether bees can eat from the cell.
func (c *Cell) IsFood() bool { return c.State == Honey || c.State == Nectar }

// Usable reports whether the cell is clean and empty.
func (c *Cell) Usable() bool { return c.Built && c.State == Empty && !c.Dirty }

// BroodProgress returns 0..1 for how far a brood cell is through its
// current stage (used by the renderer).
func (h *Hive) BroodProgress(x, y int) float64 {
	c := &h.Grid[y][x]
	if !c.IsBrood() {
		return 0
	}
	return clampF(float64(h.Tick-int(c.since))/float64(stageTicks(c)), 0, 1)
}

func stageTicks(c *Cell) int {
	var days float64
	switch c.State {
	case Egg:
		days = eggDays
	case Larva:
		days = [...]float64{workerLarvaDays, droneLarvaDays, queenLarvaDays}[c.Kind]
	case Pupa:
		days = [...]float64{workerPupaDays, dronePupaDays, queenPupaDays}[c.Kind]
	}
	return int(days * DayTicks)
}

// inComb reports whether (x, y) is a comb cell (the outer ring is the frame).
func (h *Hive) inComb(x, y int) bool {
	return x >= 1 && y >= 1 && x <= h.width-2 && y <= h.height-2
}

func (h *Hive) idx(x, y int) int { return y*h.width + x }

// cellSet is an unordered set of cell indices with O(1) add/remove, so
// bees can search a short list ("where is the nearest pollen?") instead
// of scanning the comb.
type cellSet struct {
	items []int32
	pos   []int32 // cell index -> position in items, -1 if absent
}

func newCellSet(n int) cellSet {
	s := cellSet{pos: make([]int32, n)}
	for i := range s.pos {
		s.pos[i] = -1
	}
	return s
}

func (s *cellSet) add(i int) {
	if s.pos[i] >= 0 {
		return
	}
	s.pos[i] = int32(len(s.items))
	s.items = append(s.items, int32(i))
}

func (s *cellSet) remove(i int) {
	p := s.pos[i]
	if p < 0 {
		return
	}
	last := s.items[len(s.items)-1]
	s.items[p] = last
	s.pos[last] = p
	s.items = s.items[:len(s.items)-1]
	s.pos[i] = -1
}

// nearestIn returns the cell of set closest to (x, y) (Chebyshev distance,
// ties to the first found) that pred accepts, within maxD.
func (h *Hive) nearestIn(set *cellSet, x, y, maxD int, pred func(c *Cell) bool) (int, int, bool) {
	best, bestD := -1, maxD+1
	for _, ci := range set.items {
		cx, cy := int(ci)%h.width, int(ci)/h.width
		d := chebyshev(x, y, cx, cy)
		if d < bestD && (pred == nil || pred(&h.Grid[cy][cx])) {
			best, bestD = int(ci), d
		}
	}
	if best < 0 {
		return 0, 0, false
	}
	return best % h.width, best / h.width, true
}

func (h *Hive) setDirty(x, y int, dirty bool) {
	h.Grid[y][x].Dirty = dirty
	if dirty {
		h.dirtySet.add(h.idx(x, y))
	} else {
		h.dirtySet.remove(h.idx(x, y))
	}
}

func (h *Hive) setState(x, y int, s CellState) {
	c := &h.Grid[y][x]
	i := h.idx(x, y)
	if set := h.stateSet(c.State); set != nil {
		set.remove(i)
	}
	if set := h.stateSet(s); set != nil {
		set.add(i)
	}
	wasBrood := c.IsBrood()
	c.State = s
	c.since = int32(h.Tick)
	isBrood := c.IsBrood()
	if isBrood && !wasBrood {
		h.broodPos[i] = int32(len(h.brood))
		h.brood = append(h.brood, int32(i))
	} else if wasBrood && !isBrood {
		p := h.broodPos[i]
		last := h.brood[len(h.brood)-1]
		h.brood[p] = last
		h.broodPos[last] = p
		h.brood = h.brood[:len(h.brood)-1]
		h.broodPos[i] = -1
	}
}

func (h *Hive) stateSet(s CellState) *cellSet {
	switch s {
	case Nectar:
		return &h.nectarSet
	case Pollen:
		return &h.pollenSet
	}
	return nil
}

func (h *Hive) foodTotal(s CellState) *float64 {
	switch s {
	case Honey:
		return &h.honey
	case Nectar:
		return &h.nectar
	case Pollen:
		return &h.pollen
	}
	return nil
}

// clearCell empties a cell, removing whatever food it held.
func (h *Hive) clearCell(x, y int, dirty bool) {
	c := &h.Grid[y][x]
	if t := h.foodTotal(c.State); t != nil {
		*t -= float64(c.Amount)
	}
	h.setState(x, y, Empty)
	c.Amount = 0
	c.Kind = WorkerBrood
	h.setDirty(x, y, dirty)
}

func (h *Hive) layEgg(x, y int, kind BroodKind) {
	c := &h.Grid[y][x]
	h.setState(x, y, Egg)
	c.Kind = kind
	h.setDirty(x, y, false)
	h.stats.EggsLaid++
}

// deposit adds food to an empty or same-type cell; returns the amount stored.
func (h *Hive) deposit(x, y int, s CellState, amount float64) float64 {
	c := &h.Grid[y][x]
	if c.State == Empty {
		if c.Dirty || !c.Built {
			return 0
		}
		h.setState(x, y, s)
		c.Amount = 0
	} else if c.State != s {
		return 0
	}
	room := 1 - float64(c.Amount)
	if amount > room {
		amount = room
	}
	c.Amount += float32(amount)
	*h.foodTotal(s) += amount
	return amount
}

// take removes up to amount of food from a cell; returns what was taken.
func (h *Hive) take(x, y int, amount float64) float64 {
	c := &h.Grid[y][x]
	t := h.foodTotal(c.State)
	if t == nil {
		return 0
	}
	if amount >= float64(c.Amount) {
		amount = float64(c.Amount)
		h.clearCell(x, y, false)
		return amount
	}
	c.Amount -= float32(amount)
	*t -= amount
	return amount
}

// offset is a relative cell position, pre-sorted by distance so "nearest
// cell matching X" searches can walk outward from any point.
type offset struct {
	dx, dy int16
	d      float32
}

// combDist is the distance metric used for the brood nest: comb cells are
// laid out wider than tall, so the nest is an ellipse.
func combDist(dx, dy float64) float64 {
	return math.Sqrt(0.36*dx*dx + dy*dy)
}

func buildOffsets(w, h int, metric func(dx, dy float64) float64) []offset {
	offs := make([]offset, 0, (2*w+1)*(2*h+1))
	for dy := -h; dy <= h; dy++ {
		for dx := -w; dx <= w; dx++ {
			offs = append(offs, offset{int16(dx), int16(dy), float32(metric(float64(dx), float64(dy)))})
		}
	}
	sort.SliceStable(offs, func(i, j int) bool { return offs[i].d < offs[j].d })
	return offs
}

// nearest walks offs outward from (cx, cy) and returns the first comb cell
// accepted by pred, giving up beyond maxD.
func (h *Hive) nearest(cx, cy int, offs []offset, maxD float32, pred func(x, y int, c *Cell) bool) (int, int, bool) {
	for _, o := range offs {
		if o.d > maxD {
			break
		}
		x, y := cx+int(o.dx), cy+int(o.dy)
		if !h.inComb(x, y) {
			continue
		}
		if pred(x, y, &h.Grid[y][x]) {
			return x, y, true
		}
	}
	return 0, 0, false
}

func (h *Hive) nestCenter() (int, int) {
	return int(math.Round(h.nestX)), int(math.Round(h.nestY))
}

// distToNest is the elliptical distance of a cell from the brood nest
// centre, warped by the hive's noise field so the nest has ragged edges.
func (h *Hive) distToNest(x, y int) float64 {
	return combDist(float64(x)-h.nestX, float64(y)-h.nestY) + 2.4*h.noiseAt(x, y)
}
