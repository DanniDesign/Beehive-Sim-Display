package simulation

import (
	"math/rand"

	"github.com/gookit/slog"
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

func (h *Hive) GetBees() []*Bee {
	return h.Bees
}

// IsOccupied reports whether a bee is currently standing on (x, y).
// Backed by an incrementally-maintained count map (kept in sync by
// Occupy/vacate) instead of scanning every bee, so it's cheap to call
// from the hot movement path.
func (h *Hive) IsOccupied(x, y int) bool {
	return h.Occupied[y*h.Width+x] > 0
}

func (h *Hive) Occupy(x, y int) {
	if h.Occupied == nil {
		h.Occupied = make(map[int]int)
	}
	h.Occupied[y*h.Width+x]++
}

func (h *Hive) vacate(x, y int) {
	key := y*h.Width + x
	if h.Occupied[key] <= 1 {
		delete(h.Occupied, key)
	} else {
		h.Occupied[key]--
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

// Updated to return a boolean success flag to prevent clustering bugs
func (h *Hive) RandomCellInRegion(region [][]Cell, state CellState, offsetX, offsetY int) (int, int, bool) {
	foundCells := []struct{ x, y int }{}

	for y, row := range region {
		for x, cell := range row {
			if cell.State == state {
				rx := clamp(x+offsetX, 0, h.Width-1)
				ry := clamp(y+offsetY, 0, h.Height-1)
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
// radius of (CenterX, CenterY), scanning the grid once with reservoir
// sampling instead of building an intermediate region slice plus a found-
// cells slice (what getRegion + RandomCellInRegion used to do together).
// This is the hot path for foraging/storage/egg-laying target selection,
// so avoiding the double allocation matters as populations grow.
func (h *Hive) RandomCellNear(CenterX, CenterY, radius int, state CellState) (int, int, bool) {
	minX, maxX := CenterX-radius, CenterX+radius
	minY, maxY := CenterY-radius, CenterY+radius
	if minX < 0 {
		minX = 0
	}
	if minY < 0 {
		minY = 0
	}
	if maxX >= h.Width {
		maxX = h.Width - 1
	}
	if maxY >= h.Height {
		maxY = h.Height - 1
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
	return h.Queen
}

func (h *Hive) RemoveBee(bee *Bee) {
	for i, b := range h.Bees {
		if b == bee {
			h.Bees = append(h.Bees[:i], h.Bees[i+1:]...)
			state := b.State
			if state != StateOutside {
				h.vacate(bee.X, bee.Y)
			}
			break
		}
	}
	if h.Queen == bee {
		h.Queen = nil
	}
}

// ChooseExit randomly selects one of the four corners to leave the hive.
func (h *Hive) ChooseExit() (x, y int) {
	choice := rand.Intn(4)
	switch choice {
	case 0:
		return 1, 1 // Top-Left corner
	case 1:
		return h.Width - 2, 1 // Top-Right corner
	case 2:
		return 1, h.Height - 2 // Bottom-Left corner
	default:
		return h.Width - 2, h.Height - 2 // Bottom-Right corner
	}
}

// ChooseEntrance randomly selects one of the four wall centers to return to the hive.
func (h *Hive) ChooseEntrance() (x, y int) {
	choice := rand.Intn(4)
	switch choice {
	case 0:
		return h.Width / 2, 1 // Top-Middle
	case 1:
		return h.Width - 2, h.Height / 2 // Right-Middle
	case 2:
		return h.Width / 2, h.Height - 2 // Bottom-Middle
	default:
		return 1, h.Height / 2 // Left-Middle
	}
}

func (h *Hive) UpdateState(b *Bee, task Event) {
	err := b.FSM.Fire(task)

	if err != nil {
		slog.Fatalf("failed to set task: %v", err)
	}
	state := b.FSM.MustState().(State)
	b.State = state
}
