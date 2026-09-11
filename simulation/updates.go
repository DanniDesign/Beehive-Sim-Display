package simulation

import (
	"image/color"
	"math/rand"

	"github.com/gookit/slog"
)

func (h *Hive) Update() {

	h.Age += 1
	h.Tick += 1
	// Single O(n) pass: check whether an attendant exists, and remember the
	// first forager as a promotion candidate in case one is needed. This
	// still promotes at most ONE bee per tick (matching the original
	// behavior) instead of computing a stale hasAttendant flag and handing
	// it to every forager, which would let all of them self-promote in the
	// same tick the instant the hive's last attendant dies of old age.
	// Pass 1: resync every bee's cached State from its FSM. This must
	// cover the full slice unconditionally — breaking out early here
	// (as a prior version did, once the first QueenAttendant was seen)
	// leaves every later bee's State stale for this whole tick. A bee
	// that already transitioned in a previous tick can then be read as
	// still being in its old state by this tick's task logic, which
	// fires events that don't match its *real*, unsynced FSM state —
	// that's what caused the StateOutside/EventWander panic.
	for _, b := range h.Bees {
		b.State = b.FSM.MustState().(State)
	}

	// Pass 2: is there a QueenAttendant, and if not, who's the first
	// Forager we could promote? Safe to stop early here — this pass
	// only reads Role, it doesn't touch State.
	hasAttendant := false
	var promotable *Bee
	for _, b := range h.Bees {
		if b.Role == QueenAttendant {
			hasAttendant = true
			break
		}
		if b.Role == Forager && promotable == nil {
			promotable = b
		}
	}
	if !hasAttendant && promotable != nil {
		promotable.Role = QueenAttendant
		promotable.Task = Wander
		promotable.hasTarget = false
		// The bee keeps its old ForagerFSM otherwise, which has no
		// transitions configured for QueenAttendant states/events —
		// firing one of those on it is what was crashing UpdateState.
		if fsm := h.newFSMFor(promotable); fsm != nil {
			promotable.FSM = fsm
		}
	}

	beesCopy := make([]*Bee, len(h.Bees))
	copy(beesCopy, h.Bees)

	for _, bee := range beesCopy {
		h.AgeBee(bee)
		if bee.Age > 1.0 {
			continue
		}
		h.UpdateTask(bee)
		moveChance := 1.0 - bee.Age
		if rand.Float64() < moveChance {
			h.MoveBee(bee)
		}
	}

	// Only visit cells that are actually Eggs, instead of scanning the
	// whole grid every tick.
	for key := range h.EggCells {
		x, y := key%h.Width, key/h.Width
		h.AgeEgg(x, y)
	}

	// Cheap periodic check rather than every tick.
	if h.Tick%expansionCheckEvery == 0 {
		h.MaybeExpandStorage()
	}
}

func (h *Hive) UpdateTask(b *Bee) {
	state := b.State
	if b.TaskTimer > 0 && state == StateOutside {
		b.TaskTimer--
	}
	if b.TaskCooldown > 0 {
		b.TaskCooldown--
		return
	}

	switch b.Role {
	case Forager:
		switch state {
		case StateWandering:
			// If storage is not full and there is not too many bees outside, forage.
			if h.TotalStorageStored() < h.TotalStorageCapacity() && rand.Float64() < 0.15 && h.AmountOutside < h.MaxOutside {
				h.UpdateState(b, EventHoneySupplyLow)
				return
			}
			// Otherwise keep wandering
			return
		default:
			h.TaskForage(b)
		}

	case QueenAttendant:
		switch state {
		case StateWandering:
			Queen := h.GetQueen()
			queenNeedsHoney := Queen != nil && !Queen.Carrying && h.TotalStorageStored() > 0 && h.Eggs < h.MaxEggs()

			if queenNeedsHoney {
				if b.Carrying {
					b.FSM.Fire(EventHoneyNeeded)
				} else {
					b.FSM.Fire(EventNoHoneyOnHand)
				}
			}
		default:
			h.TaskCollectFromStorage(b)
		}

	case Queen:
		if b.Carrying && state != StateCarryingHoney {
			err := b.FSM.Fire(EventReceivedHoney)
			if err != nil {
				slog.Errorf("Queen FSM Error (EventReceivedHoney): %v", err)
			}
			return
		}

		switch state {
		case StateCarryingHoney:
			h.TaskLayEggs(b)
		default:
			b.hasTarget = false
		}
	}
}

// UpdateCellState is the single place cell state changes, so it's also the
// single place the egg-position index (h.EggCells) is kept in sync.
func (h *Hive) UpdateCellState(x, y int, state CellState) {
	var clr color.RGBA
	switch state {
	case Honey:
		clr = color.RGBA{255, 215, 0, 255}
	case Empty:
		clr = color.RGBA{50, 50, 50, 255}
	case Egg:
		clr = color.RGBA{10, 224, 10, 255}
	}

	prev := h.Grid[y][x].State
	h.Grid[y][x] = Cell{
		State: state,
		Color: clr,
	}

	key := y*h.Width + x
	if prev == Egg && state != Egg {
		delete(h.EggCells, key)
	} else if state == Egg && prev != Egg {
		h.EggCells[key] = struct{}{}
	}
}

func (h *Hive) AgeEgg(x, y int) error {
	cell := &h.Grid[y][x]

	if cell.State != Egg {
		return nil
	}

	if cell.EggAge >= 1.0 {
		h.HatchEgg(x, y)
		return nil
	}

	if rand.Float64() < 0.30 {
		cell.EggAge += 0.02
	}

	r, g, b, _ := cell.Color.RGBA()
	r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)
	brown := color.RGBA{139, 69, 19, 255}

	blend := func(current, target uint8) uint8 {
		return uint8(float64(current)*0.95 + float64(target)*0.05)
	}

	cell.Color = color.RGBA{
		R: blend(r8, brown.R),
		G: blend(g8, brown.G),
		B: blend(b8, brown.B),
		A: 255,
	}
	return nil
}

func (h *Hive) HatchEgg(x, y int) {
	h.UpdateCellState(x, y, Empty)

	weights := map[Role]int{
		Forager:        70,
		QueenAttendant: 20,
	}

	sum := 0
	for _, weight := range weights {
		sum += weight
	}

	r := rand.Intn(sum)
	currentSum := 0
	chosen := Forager

	for role, weight := range weights {
		currentSum += weight
		if r < currentSum {
			chosen = role
			break
		}
	}

	newBee := h.SpawnBee(x, y, chosen)
	newBee.FSM = h.newFSMFor(newBee)
	if newBee.FSM == nil {
		// Fail loudly right here instead of leaving a bee with a nil FSM
		// that panics on some later tick as soon as anything calls
		// b.FSM.MustState()/.Fire() on it. If you hit this, it means
		// SetFSMFactories was never called (or was called with a nil
		// entry for this role) before the simulation started running —
		// add hive.SetFSMFactories(...) + hive.AssignFSMs() right after
		// simulation.NewHive(...) in main.go.
		slog.Fatalf("HatchEgg: no FSM factory registered for role %v — call Hive.SetFSMFactories before running the simulation", chosen)
	}
	h.Bees = append(h.Bees, newBee)
	h.Occupy(newBee.X, newBee.Y)
	h.Eggs--
}

func (h *Hive) AgeBee(bee *Bee) {
	state := bee.State
	var beeAgeIncrement float64
	switch bee.Role {
	case Queen:
		beeAgeIncrement = 0.00001
	case QueenAttendant:
		beeAgeIncrement = 0.0010
	default:
		beeAgeIncrement = 0.0012
	}
	bee.Age += beeAgeIncrement * bee.ageRate

	if bee.Age > 1.0 {
		if bee.Carrying && h.TotalStorageStored() < h.TotalStorageCapacity()+15 {
			if rand.Float64() < 0.2 {
				h.UpdateCellState(bee.X, bee.Y, Honey)
			}
		}
		if state == StateOutside {
			h.AmountOutside--
		}
		h.RemoveBee(bee)
	}
}
