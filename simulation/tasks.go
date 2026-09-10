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
			// 1. CHECK YOUR CAP HERE
			if h.amountOutside < h.maxOutside {
				if rand.Float64() < 0.30 {
					h.vacate(b.X, b.Y)
					b.IsOutside = true
					h.amountOutside++ // INCREMENT YOUR COUNTER
					b.TaskTimer = 40 + rand.Intn(60)
					b.hasTarget = false
				}
			} else {
				// 2. CLEAR THE DOORWAY
				// If it's full, cancel foraging and send them back to wait at storage
				b.Task = Wander
				b.hasTarget = false
				b.TaskCooldown = 30 + rand.Intn(30)

				storage := h.nearestStorageWithSpace(b.X, b.Y)
				if storage != nil {
					x, y, ok := h.RandomCellNear(storage.centerX, storage.centerY, 4, Empty)
					if ok {
						b.targetX = x
						b.targetY = y
						b.hasTarget = true
					}
				}
			}
		}
		return
	}

	// Phase 2: Outside, performing foraging work until timer runs out
	if b.IsOutside && b.TaskTimer <= 0 {
		b.IsOutside = false
		h.amountOutside-- // DECREMENT YOUR COUNTER

		h.occupy(b.X, b.Y)
		b.CarryingHoney = true
		b.PrevTask = Forage
		b.hasTarget = false
		b.Task = SupplyToStorage
		b.TaskCooldown = rand.Intn(120)
	}
}

func (h *Hive) TaskSupplyToStorage(b *Bee) {
	b.Task = SupplyToStorage

	// If the depot we were heading to filled up while en route, pick another.
	if b.TargetStorage != nil && b.TargetStorage.storedAmount >= b.TargetStorage.capacity {
		b.TargetStorage = nil
		b.hasTarget = false
	}

	if b.TargetStorage == nil {
		storage := h.nearestStorageWithSpace(b.X, b.Y)
		if storage == nil {
			b.Task = Wander
			b.hasTarget = false
			return
		}
		b.TargetStorage = storage
	}
	storage := b.TargetStorage

	if !b.hasTarget {
		x, y, ok := h.RandomCellNear(storage.centerX, storage.centerY, 5, Empty)
		if !ok {
			b.TargetStorage = nil
			b.Task = Wander
			b.hasTarget = false
			return
		}
		b.targetX = x
		b.targetY = y
		b.hasTarget = true
	}

	if abs(b.X-b.targetX) <= 1 && abs(b.Y-b.targetY) <= 1 {
		b.CarryingHoney = false
		h.UpdateCellState(b.targetX, b.targetY, Honey)
		storage.storedAmount++
		b.PrevTask = SupplyToStorage
		b.Task = Wander
		b.hasTarget = false
		b.TargetStorage = nil
		b.TaskCooldown = 20 + rand.Intn(30)
	}
}

func (h *Hive) TaskCollectFromStorage(b *Bee) {
	b.Task = CollectFromStorage

	// If the depot we were heading to emptied out while en route, pick another.
	if b.TargetStorage != nil && b.TargetStorage.storedAmount <= 0 {
		b.TargetStorage = nil
		b.hasTarget = false
	}

	if b.TargetStorage == nil {
		storage := h.nearestStorageWithHoney(b.X, b.Y)
		if storage == nil {
			b.Task = Wander
			b.hasTarget = false
			return
		}
		b.TargetStorage = storage
	}
	storage := b.TargetStorage

	if !b.hasTarget {
		x, y, ok := h.RandomCellNear(storage.centerX, storage.centerY, 5, Honey)
		if !ok {
			b.TargetStorage = nil
			b.Task = Wander
			b.hasTarget = false
			return
		}
		b.targetX = x
		b.targetY = y
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
		b.TargetStorage = nil
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
	if abs(b.X-queen.X) <= 1 && abs(b.Y-queen.Y) <= 2 {
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

	regionRadius := 1
	x, y, ok := h.RandomCellNear(b.X, b.Y, regionRadius, Empty)
	if !ok {
		b.Task = Wander
		b.hasTarget = false
		return
	}

	if h.Grid[y][x].State == Empty && b.CarryingHoney {
		h.UpdateCellState(x, y, Egg)
		h.eggs++
		b.CarryingHoney = false
		b.Task = Wander
		b.hasTarget = false
		b.TaskCooldown = 15
	}
}
