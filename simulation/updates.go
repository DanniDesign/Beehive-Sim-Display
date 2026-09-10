package simulation

import (
	"image/color"
	"math/rand"
)

func (h *Hive) Update() {
	h.Age += 1
	h.Tick += 1

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

	for y := range h.Grid {
		for x := range h.Grid[y] {
			if h.Grid[y][x].State == Egg {
				h.AgeEgg(x, y)
			}
		}
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
		// Failsafe: Check if the hive lacks an active Queen Attendant
		hasAttendant := false
		for _, bee := range h.bees {
			if bee.Role == QueenAttendant {
				hasAttendant = true
				break
			}
		}

		// Promote this forager if no attendant exists
		if !hasAttendant {
			b.Role = QueenAttendant
			b.Task = Wander
			b.hasTarget = false
			return
		}

		if b.CarryingHoney {
			h.TaskSupplyToStorage(b)
			return
		}

		if b.Task == Forage {
			h.TaskForage(b)
			return
		}

		if h.storage.storedAmount < h.storage.capacity {
			if rand.Float64() < 0.15 {
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
		if queen != nil && !queen.CarryingHoney && h.storage.storedAmount > 0 && h.eggs < 25 {
			// Roll chance (e.g., 5-10%) so attendants stagger their runs
			if rand.Float64() < 0.10 {
				b.Task = CollectFromStorage
				b.hasTarget = false
				h.TaskCollectFromStorage(b)
				return
			}
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
	h.Grid[y][x] = Cell{
		State: state,
		Color: clr,
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

	if rand.Float64() < 0.50 {
		cell.eggAge += 0.005
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
		QueenAttendant: 15,
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
		beeAgeIncrement = 0.0015
	}
	bee.Age += beeAgeIncrement

	if bee.Age > 1.0 {
		if bee.CarryingHoney && h.storage.storedAmount < h.storage.capacity + 15{
			h.UpdateCellState(bee.X, bee.Y, Honey)
		}
		h.RemoveBee(bee)
	}
}
