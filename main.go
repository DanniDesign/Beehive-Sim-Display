package main

import (
	"image/color"
	"log"
	"math/rand"

	"github.com/DanniDesign/Beehive-Sim-Display/simulation"
	"github.com/DanniDesign/Beehive-Sim-Display/simulation/bees/forager"
	"github.com/DanniDesign/Beehive-Sim-Display/simulation/bees/queen"
	"github.com/DanniDesign/Beehive-Sim-Display/simulation/bees/queenattendant"
	"github.com/brianvoe/gofakeit/v7"
)

type Config struct {
	Width  int
	Height int
	FPS    int
}

func main() {
	cfg := Config{
		Width:  64,
		Height: 32,
		FPS:    5,
	}

	hive := simulation.NewHive(4, cfg.Width, cfg.Height)

	hive.Bees = hive.GenerateBees(4, 3)

	for _, b := range hive.Bees {
		switch b.Role {
		case simulation.Forager:
			fsm := forager.New(b)
			b.FSM = fsm
		case simulation.Queen:
			fsm := queen.New(b)
			b.FSM = fsm
		case simulation.QueenAttendant:
			fsm := queenattendant.New(b)
			b.FSM = fsm
		}
	}
	hive.SetFSMFactories(forager.New, queenattendant.New, queen.New)
	hive.AssignFSMs()
	if err := RunApp(hive, cfg); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}

func NewHive(beeAmount, Width, Height int) *simulation.Hive {
	gofakeit.Seed(0)
	cells := make([][]simulation.Cell, Height)
	for y := range cells {
		cells[y] = make([]simulation.Cell, Width)
		for x := range cells[y] {
			cells[y][x] = simulation.Cell{
				State: simulation.CellState(simulation.Empty),
				Color: color.RGBA{50, 50, 50, 255},
			}
		}
	}

	margin := 10
	hiveStartX := rand.Intn(Width-2*margin) + margin
	hiveStartY := rand.Intn(Height-2*margin) + margin

	h := &simulation.Hive{
		Name:          gofakeit.Bird(),
		CenterX:       hiveStartX,
		CenterY:       hiveStartY,
		Grid:          cells,
		State:         simulation.HiveState(simulation.Healty),
		Age:           0,
		Height:        Height,
		Width:         Width,
		Eggs:          0,
		EggCells:      make(map[int]struct{}),
		Occupied:      make(map[int]int),
		AmountOutside: 0,
		MaxOutside:    10,
	}

	h.AddStorage() // initial depot; more open up automatically as the colony grows

	h.Bees = h.GenerateBees(beeAmount, 3)
	for _, b := range h.Bees {
		h.Occupy(b.X, b.Y)
	}
	return h
}
