package simulation

import "math"

// Comb is not a given: bees build it. A colony hangs its wax comb from the
// ceiling of its cavity and extends it downward and outward in irregular
// lobes. Brood, pollen and honey can only go into built cells, so every
// colony ends up with its own organic shape. A per-hive noise field adds
// further irregularity to where the brood nest and stores end.

// buildNoise fills h.noise (brood/comb shape) and h.storeNoise (where
// stores pile up) with smooth value noise in roughly [-1, 1].
func (h *Hive) buildNoise() {
	h.noise = h.valueNoise()
	h.storeNoise = h.valueNoise()
}

func (h *Hive) valueNoise() []float32 {
	field := make([]float32, h.width*h.height)
	octave := func(step int, amp float64) {
		gw, gh := h.width/step+2, h.height/step+2
		grid := make([]float64, gw*gh)
		for i := range grid {
			grid[i] = h.rng.Float64()*2 - 1
		}
		smooth := func(t float64) float64 { return t * t * (3 - 2*t) }
		for y := 0; y < h.height; y++ {
			for x := 0; x < h.width; x++ {
				gx, gy := x/step, y/step
				fx, fy := smooth(float64(x%step)/float64(step)), smooth(float64(y%step)/float64(step))
				a := grid[gy*gw+gx]*(1-fx) + grid[gy*gw+gx+1]*fx
				b := grid[(gy+1)*gw+gx]*(1-fx) + grid[(gy+1)*gw+gx+1]*fx
				field[y*h.width+x] += float32(amp * (a*(1-fy) + b*fy))
			}
		}
	}
	octave(8, 0.7)
	octave(4, 0.3)
	return field
}

func (h *Hive) noiseAt(x, y int) float64 { return float64(h.noise[y*h.width+x]) }

// setupEntrance puts the entrance on the bottom or low on a side wall.
// Bees keep their honey away from the entrance (and up), the brood near it.
func (h *Hive) setupEntrance() {
	w, ht := h.width, h.height
	switch r := h.rng.Float64(); {
	case r < 0.6: // bottom
		h.ExitX, h.ExitY = 4+h.rng.Intn(w-8), ht-1
		h.inX, h.inY, h.inNX, h.inNY = h.ExitX, ht-2, 0, -1
	case r < 0.8: // left wall
		h.ExitX, h.ExitY = 0, ht/2+h.rng.Intn(ht/2-3)
		h.inX, h.inY, h.inNX, h.inNY = 1, h.ExitY, 1, 0
	default: // right wall
		h.ExitX, h.ExitY = w-1, ht/2+h.rng.Intn(ht/2-3)
		h.inX, h.inY, h.inNX, h.inNY = w-2, h.ExitY, -1, 0
	}
	// Honey direction: up, and away from the entrance.
	ax := float64(w)/2 - float64(h.inX)
	ay := float64(ht)/2 - float64(h.inY)
	n := math.Hypot(ax, ay)
	hx, hy := 0.6*ax/n, 0.6*ay/n-0.8
	n = math.Hypot(hx, hy)
	h.honeyDX, h.honeyDY = hx/n, hy/n
}

// atEntrance reports whether a comb cell is close enough to the entrance
// slot to fly out from (or land on).
func (h *Hive) atEntrance(x, y int) bool {
	dx, dy := x-h.inX, y-h.inY
	depth := dx*h.inNX + dy*h.inNY
	along := abs(dx*h.inNY - dy*h.inNX)
	return depth >= 0 && depth <= 1 && along <= 2
}

// growInitialComb builds last year's comb: one to three lobes hanging from
// the ceiling, grown cell by cell with noise-driven preferences.
func (h *Hive) growInitialComb(cells int) {
	lobes := 1 + h.rng.Intn(3)
	for i := 0; i < lobes; i++ {
		x := 6 + h.rng.Intn(h.width-12)
		for dx := -2; dx <= 2; dx++ {
			h.buildCell(clamp(x+dx, 1, h.width-2), 1)
		}
	}
	for h.combCells < cells && len(h.frontier.items) > 0 {
		x, y := h.pickFrontier(-1, -1)
		h.buildCell(x, y)
	}
}

// pickFrontier chooses where comb grows next: a few random edge cells are
// considered and the most attractive wins (noise, being well supported by
// neighbouring comb, and - when given - closeness to a point).
func (h *Hive) pickFrontier(nearX, nearY int) (int, int) {
	best, bestScore := -1, math.Inf(-1)
	for i := 0; i < 8; i++ {
		ci := int(h.frontier.items[h.rng.Intn(len(h.frontier.items))])
		x, y := ci%h.width, ci/h.width
		s := 1.2*h.noiseAt(x, y) + 0.35*float64(h.builtNeighbours(x, y)) + 0.6*h.rng.Float64()
		if nearX >= 0 {
			s -= 0.08 * float64(chebyshev(x, y, nearX, nearY))
		}
		if s > bestScore {
			best, bestScore = ci, s
		}
	}
	return best % h.width, best / h.width
}

func (h *Hive) builtNeighbours(x, y int) int {
	n := 0
	for _, o := range h.radial[1:9] {
		nx, ny := x+int(o.dx), y+int(o.dy)
		if h.inComb(nx, ny) && h.Grid[ny][nx].Built {
			n++
		}
	}
	return n
}

func (h *Hive) buildCell(x, y int) {
	c := &h.Grid[y][x]
	if c.Built {
		return
	}
	c.Built = true
	h.combCells++
	i := h.idx(x, y)
	h.frontier.remove(i)
	for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		nx, ny := x+d[0], y+d[1]
		if h.inComb(nx, ny) && !h.Grid[ny][nx].Built {
			h.frontier.add(h.idx(nx, ny))
		}
	}
}

// unbuildCell: wax moth larvae destroy unguarded empty comb.
func (h *Hive) unbuildCell(x, y int) {
	c := &h.Grid[y][x]
	if !c.Built || c.State != Empty {
		return
	}
	h.setDirty(x, y, false)
	c.Built = false
	h.combCells--
	for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		nx, ny := x+d[0], y+d[1]
		if h.inComb(nx, ny) && h.Grid[ny][nx].Built {
			h.frontier.add(h.idx(x, y))
		}
	}
	for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		nx, ny := x+d[0], y+d[1]
		if h.inComb(nx, ny) && !h.Grid[ny][nx].Built && !h.touchesComb(nx, ny) {
			h.frontier.remove(h.idx(nx, ny))
		}
	}
}

func (h *Hive) touchesComb(x, y int) bool {
	for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		nx, ny := x+d[0], y+d[1]
		if h.inComb(nx, ny) && h.Grid[ny][nx].Built {
			return true
		}
	}
	return false
}

// placeHome puts the brood nest in the part of the comb nearest the
// entrance, which is where colonies rear their brood.
func (h *Hive) placeHome() {
	var sx, sy float64
	n := 0
	for y := 1; y < h.height-1; y++ {
		for x := 1; x < h.width-1; x++ {
			if h.Grid[y][x].Built {
				sx += float64(x)
				sy += float64(y)
				n++
			}
		}
	}
	cx, cy := sx/float64(n), sy/float64(n)
	cx += (float64(h.inX) - cx) * 0.45
	cy += (float64(h.inY) - cy) * 0.45
	x, y, _ := h.nearest(int(cx), int(cy), h.radial, 1e9, func(_, _ int, c *Cell) bool { return c.Built })
	h.homeX, h.homeY = float64(x), float64(y)
	h.nestX, h.nestY = h.homeX, h.homeY
}

// needComb: wax is made from honey, so bees only build when nectar is
// flowing and they are short of space.
func (h *Hive) needComb() bool {
	if h.Cold() || h.ForageAvailability() < 0.3 || h.honey+h.nectar < 5 || len(h.frontier.items) == 0 {
		return false
	}
	return h.honeyBound || h.freeNearNest < 25
}

// countFreeNearNest counts clean empty cells in and around the brood nest.
func (h *Hive) countFreeNearNest() {
	cx, cy := h.nestCenter()
	free := 0
	h.nearest(cx, cy, h.radial, float32(h.nestR+3), func(_, _ int, c *Cell) bool {
		if c.Usable() {
			free++
		}
		return false
	})
	h.freeNearNest = free
}

// pickStoreCell scans outward from the nest and, among the first few dozen
// acceptable cells, takes the one the store-noise field favours: honey and
// pollen pile up in irregular patches and lobes instead of a neat ring.
func (h *Hive) pickStoreCell(cx, cy int, offs []offset, pred func(x, y int, c *Cell) bool) (int, int, bool) {
	best, bestScore := -1, math.Inf(1)
	var first float32 = -1
	n := 0
	for _, o := range offs {
		if first >= 0 && (o.d > first+5 || n == 40) {
			break
		}
		x, y := cx+int(o.dx), cy+int(o.dy)
		if !h.inComb(x, y) || !pred(x, y, &h.Grid[y][x]) {
			continue
		}
		if first < 0 {
			first = o.d
		}
		n++
		s := float64(o.d) + 3.5*float64(h.storeNoise[y*h.width+x]) + 0.6*h.rng.Float64()
		if c := &h.Grid[y][x]; c.State != Empty {
			s -= 1 // top up a part-filled cell first
		}
		if s < bestScore {
			best, bestScore = y*h.width+x, s
		}
	}
	if best < 0 {
		return 0, 0, false
	}
	return best % h.width, best / h.width, true
}

// nearestPick is like nearest, but picks randomly among the first few
// matches that are about as close as the best one, so brood and stores
// spread with ragged, natural edges.
func (h *Hive) nearestPick(cx, cy int, offs []offset, k int, pred func(x, y int, c *Cell) bool) (int, int, bool) {
	var xs, ys [8]int
	n := 0
	var first float32 = -1
	for _, o := range offs {
		if first >= 0 && (o.d > first+1.5 || n == k) {
			break
		}
		x, y := cx+int(o.dx), cy+int(o.dy)
		if !h.inComb(x, y) || !pred(x, y, &h.Grid[y][x]) {
			continue
		}
		if first < 0 {
			first = o.d
		}
		xs[n], ys[n] = x, y
		n++
	}
	if n == 0 {
		return 0, 0, false
	}
	i := h.rng.Intn(n)
	return xs[i], ys[i], true
}
