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

func (h *Hive) IsOccupied(x, y int) bool {
	for _, b := range h.bees {
		if b.X == x && b.Y == y {
			return true
		}
	}
	return false
	
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

func (h *Hive) GetQueen() *Bee {
	for _, b := range h.bees {
		if b.Role == Queen {
			return b
		}
	}
	return nil
}

func (h *Hive) RemoveBee(bee *Bee) {
	for i, b := range h.bees {
		if b == bee {
			h.bees = append(h.bees[:i], h.bees[i+1:]...)
			break
		}
	}
}
