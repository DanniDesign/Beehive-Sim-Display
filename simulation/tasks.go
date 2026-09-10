package simulation

import (
	"math/rand"
)

func (h *Hive) TaskForage(b *Bee) {
    b.Task = Forage

    // Phase 1: Inside the hive, heading for the exit door
    if !b.IsOutside {
        if !b.hasTarget {
            b.targetX = clamp(h.ExitX+rand.Intn(3)-1, 1, h.width-2)
            b.targetY = clamp(h.ExitY+rand.Intn(3)-1, 1, h.height-2)
            b.hasTarget = true
        }

        // Near exit door check
        if abs(b.X-h.ExitX) <= 1 && abs(b.Y-h.ExitY) <= 1 {
            if rand.Float64() < 0.30 {
                b.IsOutside = true
                b.TaskTimer = 40 + rand.Intn(60)
                b.hasTarget = false // Reset target for outside phase
            }
        }
        return
    }

    // Phase 2: Outside, performing foraging work until timer runs out
    if b.IsOutside && b.TaskTimer <= 0 {
        b.IsOutside = false
        b.CarryingHoney = true
        b.PrevTask = Forage
        b.hasTarget = false
        b.Task = SupplyToStorage
        b.TaskCooldown = rand.Intn(120)
    }
}

func (h *Hive) TaskSupplyToStorage(b *Bee) {
	b.Task = SupplyToStorage
	storage := h.storage

	if !b.hasTarget {
		regionRadius := 5
		region := h.getRegion(storage.centerX, storage.centerY, regionRadius)
		emptyX, emptyY, ok := h.RandomCellInRegion(region, Empty, storage.centerX-regionRadius, storage.centerY-regionRadius)

		if !ok {
			b.Task = Wander
			b.hasTarget = false
			return
		}

		b.targetX = emptyX
		b.targetY = emptyY
		b.hasTarget = true
	}

	if abs(b.X-b.targetX) <= 1 && abs(b.Y-b.targetY) <= 1 {
		b.CarryingHoney = false
		h.UpdateCellState(b.targetX, b.targetY, Honey)
		storage.storedAmount++
		b.PrevTask = SupplyToStorage
		b.Task = Wander
		b.hasTarget = false
		b.TaskCooldown = 20 + rand.Intn(30)
	}
}

func (h *Hive) TaskCollectFromStorage(b *Bee) {
	b.Task = CollectFromStorage
	storage := h.storage

	if !b.hasTarget {
		regionRadius := 5
		region := h.getRegion(storage.centerX, storage.centerY, regionRadius)
		honeyX, honeyY, ok := h.RandomCellInRegion(region, Honey, storage.centerX-regionRadius, storage.centerY-regionRadius)

		if !ok {
			b.Task = Wander
			b.hasTarget = false
			return
		}

		b.targetX = honeyX
		b.targetY = honeyY
		b.hasTarget = true
	}

	if abs(b.X-b.targetX) <= 1 && abs(b.Y-b.targetY) <= 1 && !b.CarryingHoney {
		b.CarryingHoney = true
		h.UpdateCellState(b.targetX, b.targetY, Empty)
		if storage.storedAmount > 0 {
			storage.storedAmount--
		}
		b.PrevTask = CollectFromStorage
		b.Task = SupplyToQueen
		b.hasTarget = false
	}
}

func (h *Hive) TaskSupplyToQueen(b *Bee) {
	b.Task = SupplyToQueen
	queen := h.GetQueen()

	if queen == nil {
		b.Task = Wander
		b.hasTarget = false
		return
	}

	// Continually track the Queen's active position each frame
	b.targetX = queen.X
	b.targetY = queen.Y
	b.hasTarget = true

	// Increased reach distance slightly (within 1 cells) to make handoffs fluid
	if abs(b.X-queen.X) <= 1 && abs(b.Y-queen.Y) <= 1 {
		b.CarryingHoney = false
		queen.CarryingHoney = true
		b.PrevTask = SupplyToQueen
		b.Task = Wander
		b.hasTarget = false
		b.TaskCooldown = 10
	}
}

func (h *Hive) TaskLayEggs(b *Bee) {
	b.Task = LayEggs

	regionRadius := 2
	region := h.getRegion(b.X, b.Y, regionRadius)
	emptyX, emptyY, ok := h.RandomCellInRegion(region, Empty, b.X-regionRadius, b.Y-regionRadius)

	if !ok {
		b.Task = Wander
		b.hasTarget = false
		return
	}

	if rand.Float64() > 0.5 {
		return
	}

	if h.Grid[emptyY][emptyX].State == Empty && b.CarryingHoney {
		h.UpdateCellState(emptyX, emptyY, Egg)
		h.eggs++
		b.CarryingHoney = false
		b.Task = Wander
		b.hasTarget = false
		b.TaskCooldown = 15
	}
}
