package simulation

import (
	"math"
	"math/rand"

	"github.com/brianvoe/gofakeit/v7"
)

// Scaling tuning knobs: as the colony's population grows, the hive
// automatically opens additional storage depots, raises each depot's
// capacity, and allows the Queen to keep more Eggs going at once
// (more depots ~= more foraging throughput ~= more mouths can be fed).
const (
	beesPerStorageUnit         = 20 // ~1 extra depot per this many Bees
	storageBaseCapacity        = 15
	storageCapacityStep        = 5  // capacity added per growth tier
	storageGrowthPerBees       = 40 // Bees needed to unlock one capacity tier
	baseMaxEggs                = 25
	eggsPerExtraStorage        = 8  // extra concurrent Eggs supported per extra depot
	expansionCheckEvery        = 30 // ticks between scaling checks
	maxStoragePlacementAttempt = 50
	storageAvoidRadius         = 5
)

type Storage struct {
	Name             string
	CenterX, CenterY int
	capacity         int
	storedAmount     int
	reservedAmount   int
}

// TotalStorageCapacity returns the combined capacity across every depot.
func (h *Hive) TotalStorageCapacity() int {
	total := 0
	for _, s := range h.Storages {
		total += s.capacity
	}
	return total
}

// TotalStorageStored returns the combined honey currently held across every depot.
func (h *Hive) TotalStorageStored() int {
	total := 0
	for _, s := range h.Storages {
		total += s.storedAmount
	}
	return total
}

// MaxEggs reports how many Eggs the hive can currently support. It grows
// as more storage depots come online.
func (h *Hive) MaxEggs() int {
	return baseMaxEggs + eggsPerExtraStorage*(len(h.Storages)-1)
}

func sqDist(x1, y1, x2, y2 int) int {
	dx, dy := x1-x2, y1-y2
	return dx*dx + dy*dy
}

// nearestStorageWithSpace returns the closest depot that isn't full, or nil.
func (h *Hive) nearestStorageWithSpace(x, y int) *Storage {
	var best *Storage
	bestDist := math.MaxInt // Safer than 1 << 30 for large coordinate grids

	// Note: If Storages are modified concurrently, lock the hive here.
	// h.mu.RLock()
	// defer h.mu.RUnlock()

	for _, s := range h.Storages {
		// Skip full Storages
		if s.storedAmount >= s.capacity {
			continue
		}

		// Update best if this storage is closer
		if d := sqDist(x, y, s.CenterX, s.CenterY); d < bestDist {
			bestDist = d
			best = s
		}
	}

	return best
}

// nearestStorageWithHoney returns the closest depot holding honey, or nil.
func (h *Hive) nearestStorageWithHoney(x, y int) *Storage {
	var best *Storage
	bestDist := 1 << 30
	for _, s := range h.Storages {
		if s.storedAmount <= 0 {
			continue
		}
		if d := sqDist(x, y, s.CenterX, s.CenterY); d < bestDist {
			bestDist = d
			best = s
		}
	}
	return best
}

// AddStorage tries to place a new depot away from the hive center and any
// existing depots. Returns false if no valid spot could be found.
func (h *Hive) AddStorage() bool {
	margin := 4
	spanX := h.Width - 2*margin
	spanY := h.Height - 2*margin
	if spanX <= 0 || spanY <= 0 {
		return false
	}

	for attempt := 0; attempt < maxStoragePlacementAttempt; attempt++ {
		x := margin + rand.Intn(spanX)
		y := margin + rand.Intn(spanY)

		if abs(x-h.CenterX) <= storageAvoidRadius && abs(y-h.CenterY) <= storageAvoidRadius {
			continue
		}

		tooClose := false
		for _, s := range h.Storages {
			if abs(x-s.CenterX) <= storageAvoidRadius && abs(y-s.CenterY) <= storageAvoidRadius {
				tooClose = true
				break
			}
		}
		if tooClose {
			continue
		}

		h.Storages = append(h.Storages, &Storage{
			Name:         gofakeit.Name(),
			CenterX:      x,
			CenterY:      y,
			storedAmount: 0,
			capacity:     storageBaseCapacity,
		})
		return true
	}
	return false
}

// MaybeExpandStorage grows the hive's logistics to match its population:
// it opens new depots and raises per-depot capacity so a bigger colony
// isn't bottlenecked by the original single small storage room.
func (h *Hive) MaybeExpandStorage() {
	population := len(h.Bees)

	desired := population/beesPerStorageUnit + 1
	for len(h.Storages) < desired {
		if !h.AddStorage() {
			break
		}
	}

	tier := population / storageGrowthPerBees
	targetCap := storageBaseCapacity + tier*storageCapacityStep
	for _, s := range h.Storages {
		if s.capacity < targetCap {
			s.capacity = targetCap
		}
	}
}
