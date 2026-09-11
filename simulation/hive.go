package simulation

import (
	"context"
	"image/color"
	"math/rand"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/qmuntal/stateless"
)

type BehaviorFSM interface {
	State() int
	Fire(ctx context.Context, event int, args ...any) error
}

type FSMFactory func(b *Bee) BehaviorFSM

// BeeFSMFactory builds the concrete stateless.StateMachine used by a bee.
// This lives on Hive (rather than being called directly from SpawnBee)
// because the role-specific FSM packages (forager/queenattendant/queen)
// import this simulation package — simulation importing them back would
// be a cycle. The caller (main.go) wires these in once via
// SetFSMFactories after constructing the Hive.
type BeeFSMFactory func(b *Bee) *stateless.StateMachine

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
	EggAge float64
}

type HiveState int

const (
	Healty HiveState = iota
	Infected
	Threatened
)

type Hive struct {
	Name             string
	CenterX, CenterY int
	Height, Width    int
	State            HiveState
	Age              int
	Grid             [][]Cell // Exported to align with renderers
	Bees             []*Bee
	Queen            *Bee // cached Queen pointer, avoids scanning Bees every lookup
	Storages         []*Storage
	Eggs             int
	EggCells         map[int]struct{} // y*Width+x -> present; keeps AgeEgg from scanning the whole grid
	Occupied         map[int]int      // y*Width+x -> bee count; keeps Bees from stacking on each other
	Tick             int
	AmountOutside    int
	MaxOutside       int

	foragerFSM        BeeFSMFactory
	queenAttendantFSM BeeFSMFactory
	queenFSM          BeeFSMFactory
}

// SetFSMFactories wires up the constructors used to build each bee's FSM
// (typically forager.New, queenattendant.New, queen.New). Call this once
// after NewHive, before running the simulation. Without it, bees spawned
// later (hatched eggs, role-promoted attendants) will be left with a nil
// or stale FSM and firing an event on them will panic.
func (h *Hive) SetFSMFactories(forager, queenAttendant, queen BeeFSMFactory) {
	h.foragerFSM = forager
	h.queenAttendantFSM = queenAttendant
	h.queenFSM = queen
}

// newFSMFor builds the correct FSM for a bee based on its current Role,
// using whichever factory was registered via SetFSMFactories.
func (h *Hive) newFSMFor(b *Bee) *stateless.StateMachine {
	switch b.Role {
	case Forager:
		if h.foragerFSM != nil {
			return h.foragerFSM(b)
		}
	case QueenAttendant:
		if h.queenAttendantFSM != nil {
			return h.queenAttendantFSM(b)
		}
	case Queen:
		if h.queenFSM != nil {
			return h.queenFSM(b)
		}
	}
	return nil
}

func NewHive(beeAmount, Width, Height int) *Hive {
	gofakeit.Seed(0)
	cells := make([][]Cell, Height)
	for y := range cells {
		cells[y] = make([]Cell, Width)
		for x := range cells[y] {
			cells[y][x] = Cell{
				State: Empty,
				Color: color.RGBA{50, 50, 50, 255},
			}
		}
	}

	margin := 10
	hiveStartX := rand.Intn(Width-2*margin) + margin
	hiveStartY := rand.Intn(Height-2*margin) + margin

	h := &Hive{
		Name:          gofakeit.Bird(),
		CenterX:       hiveStartX,
		CenterY:       hiveStartY,
		Grid:          cells,
		State:         Healty,
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

// AssignFSMs sets every current bee's FSM according to its Role, using
// whichever factories were registered via SetFSMFactories. Call this once,
// right after SetFSMFactories, before the simulation starts running.
//
// This replaces hand-rolled per-role assignment in main.go: a switch over
// Role there is easy to leave incomplete (e.g. forgetting the single Queen
// bee, since there's only ever one), which leaves that bee's FSM nil.
// Calling Fire on a nil *stateless.StateMachine panics with a nil-pointer
// dereference the moment anything tries to fire an event on that bee —
// for the Queen specifically, that's TaskLayEggs and the Queen case in
// UpdateTask, which is why commenting those out "fixes" it: nothing ever
// touches the nil FSM anymore.
func (h *Hive) AssignFSMs() {
	for _, b := range h.Bees {
		if fsm := h.newFSMFor(b); fsm != nil {
			b.FSM = fsm
		}
	}
}
