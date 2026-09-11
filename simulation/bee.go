package simulation

import (
	"image/color"
	"math/rand"

	"github.com/google/uuid"
	"github.com/qmuntal/stateless"
)

type Role int

const (
	Forager Role = iota
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
	ID                       uuid.UUID
	Role                     Role
	Age                      float64
	Alive                    bool
	Carrying                 bool
	X, Y                     int
	targetX, targetY         int
	exitTargetX, exitTargetY int
	hasTarget                bool
	Task                     Task
	PrevTask                 Task
	TaskTimer                int
	TaskCooldown             int
	TargetStorage            *Storage // which depot this bee is currently headed to/from
	stuckTicks               int
	ageRate                  float64
	FSM                      *stateless.StateMachine
	State                    State
}

func (h *Hive) SpawnBee(x, y int, role Role) *Bee {
	b := &Bee{
		ID:          uuid.New(),
		Role:        role,
		Age:         0.0,
		Alive:       true,
		exitTargetX: 0,
		exitTargetY: 0,
		X:           x,
		Y:           y,
		targetX:     x,
		targetY:     y,
		Carrying:    false,
		Task:        Wander,
		PrevTask:    None,
		ageRate:     0.65 + rand.Float64()*0.7,
	}
	if role == Queen {
		h.Queen = b
	}
	return b
}

func (h *Hive) GenerateBees(amount, spawnRadius int) []*Bee {
	bs := make([]*Bee, 0, amount+2)
	hiveX, hiveY := h.CenterX, h.CenterY

	for range amount {
		spawnX := clamp(hiveX+rand.Intn(spawnRadius*2+1)-spawnRadius, 1, h.Width-2)
		spawnY := clamp(hiveY+rand.Intn(spawnRadius*2+1)-spawnRadius, 1, h.Height-2)

		bee := h.SpawnBee(spawnX, spawnY, Forager)

		bee.TaskCooldown = rand.Intn(120)
		bs = append(bs, bee)
	}

	Queen := h.SpawnBee(hiveX, hiveY, Queen)
	bs = append(bs, Queen)
	for range 3 {
		queenAtt := h.SpawnBee(clamp(hiveX+1, 1, h.Width-2), clamp(hiveY+1, 1, h.Height-2), QueenAttendant)
		bs = append(bs, queenAtt)

	}

	return bs
}

func (h *Hive) MoveBee(b *Bee) {
	state := b.State

	if state == StateOutside {
		return
	}

	if b.Role == Queen {
		if rand.Float64() > 0.15 {
			return
		}
		// 1. Check FSM state instead of b.Task
	} else if rand.Float64() > 0.70 && state == StateWandering {
		return
	}

	var dx, dy int

	// 2. Map the legacy Wander check directly to StateWandering
	if state == StateWandering || !b.hasTarget {
		dx = rand.Intn(3) - 1
		dy = rand.Intn(3) - 1
	} else {
		// Directed movement handles all other target-based states
		// (StateMovingToExit, StateBringingHoneyToStorage, etc.)
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

	newX := clamp(b.X+dx, 1, h.Width-2)
	newY := clamp(b.Y+dy, 1, h.Height-2)

	if newX == b.X && newY == b.Y {
		return
	}

	if h.IsOccupied(newX, newY) {
		if b.hasTarget {
			b.stuckTicks++
			if b.stuckTicks > 8 {
				b.hasTarget = false
				b.stuckTicks = 0
			}
		}
		return
	}

	b.stuckTicks = 0
	h.vacate(b.X, b.Y)
	b.X, b.Y = newX, newY
	h.Occupy(b.X, b.Y)
}

func (b *Bee) GetColor(tick int) color.Color {
	if b.Role == Queen {
		// Magenta / Hot Pink Queen (Instantly readable across the room)
		return color.RGBA{255, 0, 255, 255}
	}

	if b.Role == QueenAttendant {
		return color.RGBA{255, 5, 5, 255}
	}

	if b.Carrying {
		// Pure Yellow LED (Honey carrier)
		return color.RGBA{255, 255, 0, 255}
	}

	// Pure Bright White Worker
	return color.RGBA{255, 255, 255, 255}
}
