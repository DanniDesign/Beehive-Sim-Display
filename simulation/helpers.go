package simulation

import (
	"math/rand"
)

func clamp(v, min, max int) int {
	if v < min {
		return min
	} else if v > max {
		return max
	}
	return v
}

func (h *Hive) Cells() [][]Cell {
	return h.Grid
}

func (h *Hive) Bees() []*Bee {
	return h.bees
}

// IsOccupied reports whether a bee is currently standing on (x, y).
// Backed by an incrementally-maintained count map (kept in sync by
// occupy/vacate) instead of scanning every bee, so it's cheap to call
// from the hot movement path.
func (h *Hive) IsOccupied(x, y int) bool {
	return h.occupied[y*h.width+x] > 0
}

func (h *Hive) occupy(x, y int) {
	if h.occupied == nil {
		h.occupied = make(map[int]int)
	}
	h.occupied[y*h.width+x]++
}

func (h *Hive) vacate(x, y int) {
	key := y*h.width + x
	if h.occupied[key] <= 1 {
		delete(h.occupied, key)
	} else {
		h.occupied[key]--
	}
}

func sign(v int) int {
	if v < 0 {
		return -1
	} else if v > 0 {
		return 1
	}
	return 0
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func (h *Hive) getRegion(centerX, centerY, radius int) [][]Cell {
	region := [][]Cell{}

	for y := centerY - radius; y <= centerY+radius; y++ {
		if y < 0 || y >= h.height {
			continue
		}
		row := []Cell{}
		for x := centerX - radius; x <= centerX+radius; x++ {
			if x < 0 || x >= h.width {
				continue
			}
			row = append(row, h.Grid[y][x])
		}
		region = append(region, row)
	}
	return region
}

// Updated to return a boolean success flag to prevent clustering bugs
func (h *Hive) RandomCellInRegion(region [][]Cell, state CellState, offsetX, offsetY int) (int, int, bool) {
	foundCells := []struct{ x, y int }{}

	for y, row := range region {
		for x, cell := range row {
			if cell.State == state {
				rx := clamp(x+offsetX, 0, h.width-1)
				ry := clamp(y+offsetY, 0, h.height-1)
				foundCells = append(foundCells, struct{ x, y int }{rx, ry})
			}
		}
	}

	if len(foundCells) == 0 {
		return 0, 0, false
	}

	choice := foundCells[rand.Intn(len(foundCells))]
	return choice.x, choice.y, true
}

// RandomCellNear returns a uniformly-random cell of the given state within
// radius of (centerX, centerY), scanning the grid once with reservoir
// sampling instead of building an intermediate region slice plus a found-
// cells slice (what getRegion + RandomCellInRegion used to do together).
// This is the hot path for foraging/storage/egg-laying target selection,
// so avoiding the double allocation matters as populations grow.
func (h *Hive) RandomCellNear(centerX, centerY, radius int, state CellState) (int, int, bool) {
	minX, maxX := centerX-radius, centerX+radius
	minY, maxY := centerY-radius, centerY+radius
	if minX < 0 {
		minX = 0
	}
	if minY < 0 {
		minY = 0
	}
	if maxX >= h.width {
		maxX = h.width - 1
	}
	if maxY >= h.height {
		maxY = h.height - 1
	}

	foundX, foundY, count := 0, 0, 0
	for y := minY; y <= maxY; y++ {
		row := h.Grid[y]
		for x := minX; x <= maxX; x++ {
			if row[x].State == state {
				count++
				if rand.Intn(count) == 0 {
					foundX, foundY = x, y
				}
			}
		}
	}

	if count == 0 {
		return 0, 0, false
	}
	return foundX, foundY, true
}

func (h *Hive) GetQueen() *Bee {
	return h.queen
}

func (h *Hive) RemoveBee(bee *Bee) {
	for i, b := range h.bees {
		if b == bee {
			h.bees = append(h.bees[:i], h.bees[i+1:]...)
			if !bee.IsOutside {
				h.vacate(bee.X, bee.Y)
			}
			break
		}
	}
	if h.queen == bee {
		h.queen = nil
	}
}
