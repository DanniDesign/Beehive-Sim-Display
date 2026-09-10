package simulation

import (
	"image/color"
	"math/rand"

	"github.com/brianvoe/gofakeit/v7"
)

type CellState int

const (
	Empty CellState = iota
	Occupied
	Egg
	Honey
)

type Cell struct {
	State  CellState
	Color  color.Color
	eggAge float64
}

type HiveState int

const (
	Healty HiveState = iota
	Infected
	Threatened
)

type Hive struct {
	Name             string
	centerX, centerY int
	ExitX, ExitY     int
	height, width    int
	State            HiveState
	Age              int
	Grid             [][]Cell // Exported to align with renderers
	bees             []*Bee
	queen            *Bee // cached queen pointer, avoids scanning bees every lookup
	storages         []*Storage
	eggs             int
	eggCells         map[int]struct{} // y*width+x -> present; keeps AgeEgg from scanning the whole grid
	occupied         map[int]int      // y*width+x -> bee count; keeps bees from stacking on each other
	Tick             int
	amountOutside    int
	maxOutside       int
}

func NewHive(beeAmount, width, height int) *Hive {
	gofakeit.Seed(0)
	cells := make([][]Cell, height)
	for y := range cells {
		cells[y] = make([]Cell, width)
		for x := range cells[y] {
			cells[y][x] = Cell{
				State: Empty,
				Color: color.RGBA{50, 50, 50, 255},
			}
		}
	}

	margin := 10
	hiveStartX := rand.Intn(width-2*margin) + margin
	hiveStartY := rand.Intn(height-2*margin) + margin

	var exitX, exitY int
	edge := rand.Intn(4)

	switch edge {
	case 0:
		exitX = clamp(rand.Intn(width), 0, width-1)
		exitY = 1
	case 1:
		exitX = width - 1
		exitY = clamp(rand.Intn(height), 0, height-1)
	case 2:
		exitX = clamp(rand.Intn(width), 0, width-1)
		exitY = height - 1
	case 3:
		exitX = 1
		exitY = clamp(rand.Intn(height), 0, height-1)
	}

	h := &Hive{
		Name:          gofakeit.Bird(),
		centerX:       hiveStartX,
		centerY:       hiveStartY,
		ExitX:         exitX,
		ExitY:         exitY,
		Grid:          cells,
		State:         Healty,
		Age:           0,
		height:        height,
		width:         width,
		eggs:          0,
		eggCells:      make(map[int]struct{}),
		occupied:      make(map[int]int),
		amountOutside: 0,
		maxOutside:    10,
	}

	h.addStorage() // initial depot; more open up automatically as the colony grows

	h.bees = h.GenerateBees(beeAmount, 3)
	for _, b := range h.bees {
		h.occupy(b.X, b.Y)
	}

	return h
}
