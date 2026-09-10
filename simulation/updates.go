package simulation

import (
	"image/color"
	"math/rand"
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
	hasAttendant := false
	var promotable *Bee
	for _, b := range h.bees {
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
	}

	beesCopy := make([]*Bee, len(h.bees))
	copy(beesCopy, h.bees)

	for _, bee := range beesCopy {
		h.AgeBee(bee)
		h.UpdateTask(bee)
		moveChance := 1.0 - bee.Age
		if rand.Float64() < moveChance {
			h.MoveBee(bee)
		}
	}

	// Only visit cells that are actually eggs, instead of scanning the
	// whole grid every tick.
	for key := range h.eggCells {
		x, y := key%h.width, key/h.width
		h.AgeEgg(x, y)
	}

	// Cheap periodic check rather than every tick.
	if h.Tick%expansionCheckEvery == 0 {
		h.MaybeExpandStorage()
	}
}

func (h *Hive) UpdateTask(b *Bee) {
	if b.TaskTimer > 0 && b.IsOutside {
		b.TaskTimer--
	}
	if b.TaskCooldown > 0 {
		b.TaskCooldown--
		return
	}

	switch b.Role {
	case Forager:
		if b.CarryingHoney {
			h.TaskSupplyToStorage(b)
			return
		}

		if b.Task == Forage {
			h.TaskForage(b)
			return
		}

		if h.TotalStorageStored() < h.TotalStorageCapacity() {
			if rand.Float64() < 0.15 && h.amountOutside < h.maxOutside {
				b.Task = Forage
				b.hasTarget = false
				h.TaskForage(b)
				return
			}
		}

		b.Task = Wander
		b.hasTarget = false

	case QueenAttendant:
		// 1. If carrying honey, head to the Queen to supply her
		if b.CarryingHoney {
			h.TaskSupplyToQueen(b)
			return
		}

		// 2. If already on a mission to fetch honey from storage, keep doing it
		if b.Task == CollectFromStorage {
			h.TaskCollectFromStorage(b)
			return
		}

		// 3. Only START a new collection mission if conditions are met AND using a low tick roll
		queen := h.GetQueen()
		if queen != nil && !queen.CarryingHoney && h.TotalStorageStored() > 0 && h.eggs < h.MaxEggs() {
			// Roll chance (e.g., 5-10%) so attendants stagger their runs
			b.Task = CollectFromStorage
			b.hasTarget = false
			h.TaskCollectFromStorage(b)
			return
		}

		// Otherwise, wander
		b.Task = Wander
		b.hasTarget = false

	case Queen:
		if b.CarryingHoney {
			h.TaskLayEggs(b)
		} else {
			b.Task = Wander
			b.hasTarget = false
		}
	}
}

// UpdateCellState is the single place cell state changes, so it's also the
// single place the egg-position index (h.eggCells) is kept in sync.
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

	key := y*h.width + x
	if prev == Egg && state != Egg {
		delete(h.eggCells, key)
	} else if state == Egg && prev != Egg {
		h.eggCells[key] = struct{}{}
	}
}

func (h *Hive) AgeEgg(x, y int) error {
	cell := &h.Grid[y][x]

	if cell.State != Egg {
		return nil
	}

	if cell.eggAge >= 1.0 {
		h.HatchEgg(x, y)
		return nil
	}

	if rand.Float64() < 0.30 {
		cell.eggAge += 0.02
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
	h.bees = append(h.bees, newBee)
	h.occupy(newBee.X, newBee.Y)
	h.eggs--
}

func (h *Hive) AgeBee(bee *Bee) {
	var beeAgeIncrement float64
	switch bee.Role {
	case Queen:
		beeAgeIncrement = 0.00001
	case QueenAttendant:
		beeAgeIncrement = 0.0010
	default:
		beeAgeIncrement = 0.0012
	}
	bee.Age += beeAgeIncrement

	if bee.Age > 1.0 {
		if bee.CarryingHoney && h.TotalStorageStored() < h.TotalStorageCapacity()+15 {
			if rand.Float64() < 0.2 {
				h.UpdateCellState(bee.X, bee.Y, Honey)
			}
		}
		if bee.IsOutside {
			h.amountOutside--
		}
		h.RemoveBee(bee)
	}
}
