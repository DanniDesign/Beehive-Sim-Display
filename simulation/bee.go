package simulation

import (
	"image/color"
	"math/rand"

	"github.com/google/uuid"
)

type Role int

const (
	Forager Role = iota
	Drone
	Nurse
	QueenAttendant
	Queen
)

type Task int

const (
	None Task = iota
	Forage
	Wander
	SupplyToStorage
	SupplyToQueen
	CollectFromStorage
	LayEggs
)

type Bee struct {
	ID               uuid.UUID
	Role             Role
	Age              float64
	Alive            bool
	CarryingHoney    bool
	X, Y             int
	targetX, targetY int
	hasTarget        bool
	Task             Task
	PrevTask         Task
	TaskTimer        int
	IsOutside        bool
	TaskCooldown     int
}

func (h *Hive) SpawnBee(x, y int, role Role) *Bee {
	return &Bee{
		ID:            uuid.New(),
		Role:          role,
		Age:           0.0,
		Alive:         true,
		X:             x,
		Y:             y,
		targetX:       x,
		targetY:       y,
		CarryingHoney: false,
		Task:          Wander,
		PrevTask:      None,
	}
}

func (h *Hive) GenerateBees(amount, spawnRadius int) []*Bee {
	bs := make([]*Bee, 0, amount+2)
	hiveX, hiveY := h.centerX, h.centerY

	for i := 0; i < amount; i++ {
		spawnX := clamp(hiveX+rand.Intn(spawnRadius*2+1)-spawnRadius, 1, h.width-2)
		spawnY := clamp(hiveY+rand.Intn(spawnRadius*2+1)-spawnRadius, 1, h.height-2)

		bee := h.SpawnBee(spawnX, spawnY, Forager)
		bee.TaskCooldown = rand.Intn(120)
		bs = append(bs, bee)
	}

	queen := h.SpawnBee(hiveX, hiveY, Queen)
	bs = append(bs, queen)

	queenAtt := h.SpawnBee(clamp(hiveX+1, 1, h.width-2), clamp(hiveY+1, 1, h.height-2), QueenAttendant)
	bs = append(bs, queenAtt)

	return bs
}

func (h *Hive) MoveBee(b *Bee) {
	if b.IsOutside {
		return
	}
	if b.Role == Queen {
		if rand.Float64() > 0.15 {
			return
		}
	} else if rand.Float64() > 0.70 && b.Task == Wander {
		return
	}

	var dx, dy int

	if b.Task == Wander || !b.hasTarget {
		dx = rand.Intn(3) - 1
		dy = rand.Intn(3) - 1
	} else {
		distX := b.targetX - b.X
		distY := b.targetY - b.Y

		dx = sign(distX)
		dy = sign(distY)

		if rand.Float64() < 0.10 {
			if rand.Float64() < 0.5 {
				dx = rand.Intn(3) - 1
			} else {
				dy = rand.Intn(3) - 1
			}
		}
	}

	newX := clamp(b.X+dx, 1, h.width-2)
	newY := clamp(b.Y+dy, 1, h.height-2)

	b.X, b.Y = newX, newY
}

func (b *Bee) GetColor(tick int) color.Color {
	if b.Role == Queen {
		// Magenta / Hot Pink Queen (Instantly readable across the room)
		return color.RGBA{255, 0, 255, 255}
	}

	if b.Role == QueenAttendant {
		return color.RGBA{255, 5, 5, 255}
	}

	if b.CarryingHoney {
		// Pure Yellow LED (Honey carrier)
		return color.RGBA{255, 255, 0, 255}
	}

	// Pure Bright White Worker
	return color.RGBA{255, 255, 255, 255}
}
