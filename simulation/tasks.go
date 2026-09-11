package simulation

import (
	"fmt"
	"math/rand"

	"github.com/gookit/slog"
)

func (h *Hive) TaskForage(b *Bee) {
	state := b.State
	switch state {
	case StateMovingToExit:
		if b.exitTargetX == 0 && b.exitTargetY == 0 {
			b.exitTargetX, b.exitTargetY = h.ChooseExit()

		}
		if !b.hasTarget {
			b.targetX = clamp(b.exitTargetX+rand.Intn(3)-1, 1, h.Width-2)
			b.targetY = clamp(b.exitTargetY+rand.Intn(3)-1, 1, h.Height-2)
			b.hasTarget = true
		}

		// Near exit door check
		if abs(b.X-b.exitTargetX) <= 1 && abs(b.Y-b.exitTargetY) <= 1 {
			// 1. CHECK YOUR CAP HERE
			if h.AmountOutside < h.MaxOutside {
				if rand.Float64() < 0.30 {
					h.vacate(b.X, b.Y)
					err := b.FSM.Fire(EventWentOutside)
					if err != nil {
						slog.Fatal(err)
					}
					h.AmountOutside++
					b.exitTargetX = 0
					b.exitTargetY = 0
					b.TaskTimer = 40 + rand.Intn(60)
				}
			} else {
				// 2. CLEAR THE DOORWAY
				// If it's full, cancel foraging and send them back to wait at storage

				fmt.Println("STATE WANDER")
				b.FSM.Fire(EventWander)
				b.TaskCooldown = 30 + rand.Intn(30)

				storage := h.nearestStorageWithSpace(b.X, b.Y)
				if storage != nil {
					x, y, ok := h.RandomCellNear(storage.CenterX, storage.CenterY, 4, Empty)
					if ok {
						b.targetX = x
						b.targetY = y
					}
				}
			}
		}
	case StateOutside:
		if b.TaskTimer <= 0 {
			b.FSM.Fire(EventHarvestedHoney)
			h.AmountOutside--

			// Move the bee to the entrance coordinates before occupying the grid
			eX, eY := h.ChooseEntrance()
			b.X = eX
			b.Y = eY
			h.Occupy(b.X, b.Y)

			b.Carrying = true
			b.hasTarget = false
		}
	case StateBringingHoneyToStorage:
		// If the depot we were heading to filled up while en route, pick another.
		if b.TargetStorage != nil && b.TargetStorage.storedAmount >= b.TargetStorage.capacity {
			b.TargetStorage = nil
			b.hasTarget = false
		}

		if b.TargetStorage == nil {
			storage := h.nearestStorageWithSpace(b.X, b.Y)
			if storage == nil {
				b.FSM.Fire(EventWander)
				b.hasTarget = false
				return
			}
			b.TargetStorage = storage
		}
		storage := b.TargetStorage

		if !b.hasTarget {
			x, y, ok := h.RandomCellNear(storage.CenterX, storage.CenterY, 5, Empty)
			if !ok {
				fmt.Println("wandering because no target")

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
			b.Carrying = false
			h.UpdateCellState(b.targetX, b.targetY, Honey)
			storage.storedAmount++
			b.FSM.Fire(EventStoredHoney)
			b.hasTarget = false
			b.TargetStorage = nil
			b.TaskCooldown = 50 + rand.Intn(100)
		}
	}
}
func (h *Hive) TaskCollectFromStorage(b *Bee) {
	state := b.State
	switch state {
	case StateCollectingHoneyFromStorage:
		// If the depot we were heading to emptied out while en route, pick another.
		if b.TargetStorage != nil && b.TargetStorage.storedAmount <= 0 {
			b.TargetStorage = nil
			b.hasTarget = false
		}

		if b.TargetStorage == nil {
			storage := h.nearestStorageWithHoney(b.X, b.Y)
			if storage == nil {
				b.FSM.Fire(EventWander)
				b.hasTarget = false
				return
			}
			b.TargetStorage = storage
		}
		storage := b.TargetStorage
		if !b.hasTarget {
			x, y, ok := h.RandomCellNear(storage.CenterX, storage.CenterY, 5, Honey)
			if !ok {
				b.TargetStorage = nil
				b.FSM.Fire(EventWander)
				b.hasTarget = false
				return
			}
			b.targetX = x
			b.targetY = y
			b.hasTarget = true
		}

		if abs(b.X-b.targetX) <= 1 && abs(b.Y-b.targetY) <= 1 {
			h.UpdateCellState(b.targetX, b.targetY, Empty)
			if storage.storedAmount > 0 {
				storage.storedAmount--
			}
			b.FSM.Fire(EventCollectedHoney)
			b.hasTarget = false
			b.TargetStorage = nil
		}
	case StateSupplyingHoneyToQueen:
		Queen := h.GetQueen()

		if Queen == nil {
			b.FSM.Fire(EventWander)
			b.hasTarget = false
			return
		}

		// Continually track the Queen's active position each frame
		b.targetX = Queen.X
		b.targetY = Queen.Y
		b.hasTarget = true

		// Increased reach distance slightly (within 1 cells) to make handoffs fluid
		if abs(b.X-Queen.X) <= 1 && abs(b.Y-Queen.Y) <= 2 {
			b.Carrying = false
			Queen.Carrying = true
			b.FSM.Fire(EventDeliveredHoney)
			b.hasTarget = false
			b.TaskCooldown = 10
		}
	}
}

func (h *Hive) TaskLayEggs(b *Bee) {
	state := b.State
	switch state {
	case StateCarryingHoney:
		regionRadius := 1
		x, y, ok := h.RandomCellNear(b.X, b.Y, regionRadius, Empty)
		if !ok {
			b.hasTarget = false
			return
		}

		if h.Grid[y][x].State == Empty && b.Carrying {
			h.UpdateCellState(x, y, Egg)
			h.Eggs++
			b.Carrying = false

			err := b.FSM.Fire(EventEggPlaced)
			if err != nil {
				slog.Errorf("Queen FSM Error (EventEggPlaced): %v", err)
			}

			b.hasTarget = false
			b.TaskCooldown = 15
		}
	}
}
