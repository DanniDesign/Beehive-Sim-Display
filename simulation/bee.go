package simulation

import (
	"image/color"
	"math"
)

// Role is a bee's caste or, for workers, its current job. Workers move
// through the jobs as they age (temporal polyethism), but the colony
// speeds that up or reverses it when the balance of labour is off.
type Role uint8

const (
	Cleaner  Role = iota // 0-3 days: cleans cells, keeps the brood warm
	Nurse                // ~3-12 days: feeds larvae and the queen
	HouseBee             // ~12-20 days: caps honey, cleans, stores food
	Guard                // older house bees at the entrance
	Forager              // 20+ days: collects nectar and pollen outside
	Drone
	Queen
)

type Task uint8

const (
	Idle Task = iota
	Clean
	GetPollen
	FeedLarva
	FeedQueen
	CapHoney
	GuardEntrance
	ToExit
	Deposit
	Dance
	LayEgg
	BuildComb
)

// Trip is why a bee is heading out of the hive.
type Trip uint8

const (
	NectarTrip Trip = iota
	PollenTrip
	MatingFlight
	DroneFlight
	Leave // swarming or expelled: not coming back
)

type Bee struct {
	ID        int
	Role      Role
	Task      Task
	Age       float64 // days since emergence
	X, Y      int
	IsOutside bool
	Load      CellState // Nectar or Pollen while carrying a load
	Mated     bool      // queens only

	physAge   float64 // behavioural age, drives job changes
	vitality  float64 // remaining life, dies at 0
	tx, ty    int
	hasTarget bool
	trip      Trip
	timer     int
	loadAmt   float64
	jelly     int // feeds of brood food a nurse is carrying
	leaving   bool
	dead      bool
	stuck     int // consecutive ticks a move was blocked

	// queen state
	droneLayer bool
	energy     float64 // food from the retinue, one unit per egg
	layBudget  float64
	layReady   int
}

func (h *Hive) newBee(role Role) *Bee {
	h.nextID++
	b := &Bee{ID: h.nextID, Role: role, vitality: 1}
	h.bees = append(h.bees, b)
	if role == Queen {
		h.queen = b
	}
	return b
}

func (h *Hive) spawnAt(x, y int, role Role) *Bee {
	b := h.newBee(role)
	b.X, b.Y = x, y
	h.occ[h.idx(x, y)]++
	return b
}

func (h *Hive) spawnNear(x, y int, role Role) *Bee {
	x, y = clamp(x, 1, h.width-2), clamp(y, 1, h.height-2)
	if fx, fy, ok := h.nearest(x, y, h.radial, 100, func(x, y int, _ *Cell) bool { return h.occ[h.idx(x, y)] == 0 }); ok {
		x, y = fx, fy
	}
	return h.spawnAt(x, y, role)
}

func (h *Hive) spawnOutside(role Role, returnIn int) *Bee {
	b := h.newBee(role)
	b.IsOutside = true
	b.trip = DroneFlight // a plain flight: comes back without a load
	b.timer = returnIn
	return b
}

// kill removes a bee. counter (optional) records the cause of death.
func (h *Hive) kill(b *Bee, counter *int) {
	if b.dead {
		return
	}
	b.dead = true
	if !b.IsOutside {
		h.occ[h.idx(b.X, b.Y)]--
	}
	if counter != nil {
		*counter++
	}
	if h.queen == b {
		h.queen = nil
		h.queenlessFrom = h.Tick
	}
}

// CarryingHoney is kept for compatibility: true while carrying nectar.
func (b *Bee) CarryingHoney() bool { return b.Load == Nectar }

// Dancing reports whether the bee is doing a waggle dance.
func (b *Bee) Dancing() bool { return b.Task == Dance }

// assignRole picks a worker's job from its behavioural age.
func (h *Hive) assignRole(b *Bee) {
	if b.Role == Drone || b.Role == Queen {
		return
	}
	c := &h.counts
	// Reversion: with too few nurses for the open brood, young foragers
	// regrow their brood food glands and go back to nursing.
	if b.Role == Forager && b.physAge < h.p.ForagerAgeDays+10 && c.Nurses*4 < c.Larvae &&
		c.Foragers*5 > c.Workers && h.rng.Float64() < 0.15 {
		c.Foragers--
		c.Nurses++
		b.physAge = h.p.NurseAgeDays + 3
	}
	switch a := b.physAge; {
	case a < h.p.NurseAgeDays:
		b.Role = Cleaner
	case a < h.p.HouseAgeDays:
		b.Role = Nurse
	case a < h.p.ForagerAgeDays:
		b.Role = HouseBee
		if a > h.p.ForagerAgeDays-3 && c.Guards < 4 {
			b.Role = Guard
			c.Guards++
		}
	default:
		b.Role = Forager
	}
	if b.Task == GuardEntrance && b.Role != Guard {
		b.Task = Idle
		b.hasTarget = false
	}
}

func (h *Hive) updateBee(b *Bee) {
	b.Age += 1.0 / DayTicks
	if b.IsOutside {
		h.updateOutside(b)
		return
	}

	switch {
	case b.Role == Queen:
		if h.rng.Float64() < 1/(h.p.QueenLifespanYears*YearDays*DayTicks) {
			h.kill(b, &h.stats.QueensDied)
			return
		}
	case b.Role == Drone:
		b.vitality -= 1 / (h.p.DroneLifeDays * DayTicks)
	case b.Role == Forager || b.Role == Guard:
		// Once a bee has turned forager its body is "used up" (low
		// vitellogenin): it dies within weeks, even if it can't fly.
		b.vitality -= 1 / (h.p.ForagerLifeDays * DayTicks)
	case h.Cold() || h.openBroodRatio() < 0.05:
		// Bees that rear little brood keep their fat body and live for
		// months: this is how a colony overwinters.
		b.vitality -= 1 / (h.p.WinterLifeDays * DayTicks)
	case b.Role == Nurse && h.counts.Larvae > 0:
		b.vitality -= 1 / (h.p.NurseLifeDays * DayTicks)
	default:
		b.vitality -= 1 / (h.p.InHiveLifeDays * DayTicks)
	}
	if b.vitality <= 0 {
		h.kill(b, &h.stats.DiedOldAge)
		return
	}

	if b.Role != Queen && b.Role != Drone && (h.Tick+b.ID)%DayTicks == 0 {
		b.physAge += h.devRate
		h.assignRole(b)
	}

	if b.timer > 0 {
		b.timer--
		if b.Task == Dance {
			h.recruitment += 0.002
			h.jiggle(b)
		}
		return
	}

	if b.leaving {
		b.Task = ToExit
	}
	switch b.Role {
	case Queen:
		h.queenBehave(b)
	case Drone:
		h.droneBehave(b)
	default:
		h.workerBehave(b)
	}
}

func (h *Hive) workerBehave(b *Bee) {
	switch b.Task {
	case Idle:
		if b.Load != Empty {
			b.Task = Deposit
			b.hasTarget = false
		} else if h.rng.Float64() < 0.15 {
			h.decide(b)
		}
		if b.Task == Idle {
			h.loiter(b)
			return
		}
	case Dance:
		b.Task = Idle
		return
	}

	switch b.Task {
	case ToExit:
		h.goExit(b)
	case Clean:
		h.doClean(b)
	case GetPollen:
		h.doGetPollen(b)
	case FeedLarva:
		h.doFeedLarva(b)
	case FeedQueen:
		h.doFeedQueen(b)
	case CapHoney:
		h.doCapHoney(b)
	case GuardEntrance:
		h.doGuard(b)
	case Deposit:
		h.doDeposit(b)
	case BuildComb:
		h.doBuild(b)
	}
}

// decide picks a new task for an idle worker.
func (h *Hive) decide(b *Bee) {
	switch b.Role {
	case Forager:
		if h.CanFly() && h.rng.Float64() < h.departChance() {
			h.startForaging(b)
			return
		}
		h.tryClean(b, 6)
	case Guard:
		b.Task = GuardEntrance
		b.hasTarget = false
	case HouseBee:
		if x, y, ok := h.nearestIn(&h.nectarSet, b.X, b.Y, 25, h.nectarRipe); ok {
			h.setTask(b, CapHoney, x, y)
			return
		}
		if h.counts.Builders < 8 && h.needComb() && h.rng.Float64() < 0.3 {
			cx, cy := h.nestCenter()
			x, y := h.pickFrontier(cx, cy)
			h.setTask(b, BuildComb, x, y)
			h.counts.Builders++
			return
		}
		h.tryClean(b, 12)
	case Nurse:
		if b.jelly == 0 {
			b.Task = GetPollen
			b.hasTarget = false
			return
		}
		if q := h.queen; q != nil && !q.IsOutside && q.energy < 4 && h.counts.Attendants < 3 &&
			chebyshev(b.X, b.Y, q.X, q.Y) < 12 {
			b.Task = FeedQueen
			h.counts.Attendants++
			return
		}
		hungry := int32(h.Tick - h.p.LarvaFeedAfterTicks)
		if x, y, ok := h.nearestHungryLarva(b.X, b.Y, 14, hungry); ok {
			h.setTask(b, FeedLarva, x, y)
			return
		}
		h.tryClean(b, 5)
	case Cleaner:
		h.tryClean(b, 14)
	}
}

func (h *Hive) setTask(b *Bee, t Task, x, y int) {
	b.Task = t
	b.tx, b.ty = x, y
	b.hasTarget = true
}

func (h *Hive) tryClean(b *Bee, radius int) {
	if x, y, ok := h.nearestIn(&h.dirtySet, b.X, b.Y, radius, nil); ok {
		h.setTask(b, Clean, x, y)
	}
}

// nearestHungryLarva finds the closest larva that is due a feed, preferring
// the hungriest when several are equally close.
func (h *Hive) nearestHungryLarva(x, y, maxD int, hungry int32) (int, int, bool) {
	best, bestD := -1, maxD+1
	var bestFed int32
	for _, ci := range h.brood {
		cx, cy := int(ci)%h.width, int(ci)/h.width
		c := &h.Grid[cy][cx]
		if c.State != Larva || c.Kind == QueenBrood || c.fed >= hungry {
			continue
		}
		d := chebyshev(x, y, cx, cy)
		if d < bestD || (d == bestD && c.fed < bestFed) {
			best, bestD, bestFed = int(ci), d, c.fed
		}
	}
	if best < 0 {
		return 0, 0, false
	}
	return best % h.width, best / h.width, true
}

// departChance: foragers leave more readily when there is a lot to
// collect and when returning bees are dancing about it.
func (h *Hive) departChance() float64 {
	f := h.ForageAvailability()
	if f <= 0 {
		return 0
	}
	p := 0.25 * (0.2 + f) * (1 + math.Min(h.recruitment, 2))
	if h.honeyBound {
		p *= 0.4
	}
	return p
}

func (h *Hive) startForaging(b *Bee) {
	// Open brood stimulates pollen foraging; full pollen stores damp it.
	need := 2 + 0.04*float64(h.counts.Eggs+h.counts.Larvae)
	pollenNeed := clampF(1-h.pollen/need, 0, 1)
	b.trip = NectarTrip
	if h.rng.Float64() < 0.08+0.45*pollenNeed {
		b.trip = PollenTrip
	}
	b.Task = ToExit
	b.hasTarget = false
}

// goExit walks to the entrance and leaves when standing next to it.
func (h *Hive) goExit(b *Bee) {
	if (b.trip == NectarTrip || b.trip == PollenTrip) && !b.leaving && !h.CanFly() {
		b.Task = Idle // weather turned or night fell: stay home
		b.hasTarget = false
		return
	}
	if h.atEntrance(b.X, b.Y) {
		h.exit(b)
		return
	}
	b.tx, b.ty, b.hasTarget = h.inX, h.inY, true
	h.step(b, 0.9)
}

func (h *Hive) exit(b *Bee) {
	h.occ[h.idx(b.X, b.Y)]--
	b.IsOutside = true
	b.hasTarget = false
	b.Task = Idle
	if b.leaving {
		b.dead = true // gone with the swarm, or dragged out
		if b.Role == Drone {
			h.stats.DronesExpelled++
		} else if b.Role != Queen {
			h.stats.LeftWithSwarm++
		}
		return
	}
	switch b.trip {
	case NectarTrip, PollenTrip:
		f := h.ForageAvailability()
		b.timer = int(float64(DayTicks) * (0.1 + 0.12*h.rng.Float64()) * (1.6 - 0.6*f))
		h.stats.ForagingTrips++
	case MatingFlight:
		b.timer = 40 + h.rng.Intn(30)
	default:
		b.timer = 40 + h.rng.Intn(40)
	}
}

func (h *Hive) updateOutside(b *Bee) {
	if b.Role != Queen && b.Role != Drone {
		b.vitality -= 1 / (h.p.FlightBudgetDays * DayTicks)
		if b.vitality <= 0 {
			h.kill(b, &h.stats.DiedForaging)
			return
		}
	}
	if b.timer > 0 {
		b.timer--
		return
	}
	switch b.trip {
	case NectarTrip, PollenTrip:
		if b.Load == Empty {
			if h.rng.Float64() < h.p.TripMortality {
				h.kill(b, &h.stats.DiedForaging) // birds, spiders, weather
				return
			}
			if h.rng.Float64() < h.ForageAvailability() {
				if b.trip == NectarTrip {
					b.Load, b.loadAmt = Nectar, h.p.NectarLoad
				} else {
					b.Load, b.loadAmt = Pollen, h.p.PollenLoad
				}
			}
			b.trip = DroneFlight // don't roll again while waiting to land
		}
	case MatingFlight:
		if !b.Mated && b.trip == MatingFlight {
			if h.rng.Float64() < h.p.QueenMatingLossProb {
				h.kill(b, &h.stats.QueensLostMating)
				return
			}
			if h.rng.Float64() < 0.85 {
				b.Mated = true
				b.layReady = h.Tick + 3*DayTicks
				h.stats.QueensMated++
			}
			b.trip = DroneFlight
		}
	}
	// Land on a free cell just inside the entrance.
	for _, o := range h.radial[:15] {
		x, y := h.inX+int(o.dx), h.inY+int(o.dy)
		if h.inComb(x, y) && h.atEntrance(x, y) && h.occ[h.idx(x, y)] < 2 {
			b.IsOutside = false
			b.X, b.Y = x, y
			h.occ[h.idx(x, y)]++
			b.Task = Idle
			b.hasTarget = false
			if b.Load == Nectar {
				h.handOff(b)
			}
			return
		}
	}
	b.timer = 1 // entrance jammed, try again next tick
}

// handOff: a returning nectar forager passes her load mouth-to-mouth to a
// receiver bee waiting near the entrance, who takes it to the stores, so
// the forager can turn straight round (and perhaps dance first).
func (h *Hive) handOff(b *Bee) {
	for _, r := range h.bees {
		if r.Role != HouseBee || r.IsOutside || r.dead || r.Task != Idle || r.Load != Empty || r.timer > 0 ||
			chebyshev(r.X, r.Y, b.X, b.Y) > 4 {
			continue
		}
		r.Load, r.loadAmt = b.Load, b.loadAmt
		r.Task = Deposit
		r.hasTarget = false
		r.timer = 4
		b.Load, b.loadAmt = Empty, 0
		b.timer = 4
		h.stats.NectarCollected += r.loadAmt
		h.stats.Handoffs++
		h.maybeDance(b)
		return
	}
}

// maybeDance: good forage is worth advertising with a waggle dance.
func (h *Hive) maybeDance(b *Bee) {
	f := h.ForageAvailability()
	if f > 0.5 && h.CanFly() && h.rng.Float64() < 0.3*f {
		b.Task = Dance
		b.timer += 15 + h.rng.Intn(15)
	}
}

func (h *Hive) doClean(b *Bee) {
	c := &h.Grid[b.ty][b.tx]
	if !c.Dirty {
		b.Task = Idle
		return
	}
	if h.arrived(b, 0.8) {
		h.setDirty(b.tx, b.ty, false)
		b.timer = 5
		b.Task = Idle
	}
}

// doBuild: wax glands turn honey into new cells at the edge of the comb.
func (h *Hive) doBuild(b *Bee) {
	if h.Grid[b.ty][b.tx].Built {
		b.Task = Idle
		b.hasTarget = false
		return
	}
	if h.arrived(b, 0.8) {
		h.buildCell(b.tx, b.ty)
		h.demand += 0.05
		h.stats.CombBuilt++
		b.timer = 20
		b.Task = Idle
		b.hasTarget = false
	}
}

func (h *Hive) doGetPollen(b *Bee) {
	if !b.hasTarget || h.Grid[b.ty][b.tx].State != Pollen {
		x, y, ok := h.nearestIn(&h.pollenSet, b.X, b.Y, 1<<20, nil)
		if !ok {
			// No bee bread: nurses can make a little jelly from their own
			// body protein, at a cost to their lifespan.
			b.jelly = 2
			b.vitality -= 0.01
			b.Task = Idle
			return
		}
		b.tx, b.ty, b.hasTarget = x, y, true
	}
	if h.arrived(b, 0.8) {
		h.take(b.tx, b.ty, h.p.PollenPerRefill)
		b.jelly = h.p.FeedsPerRefill
		b.timer = 4
		b.Task = Idle
		b.hasTarget = false
	}
}

func (h *Hive) doFeedLarva(b *Bee) {
	c := &h.Grid[b.ty][b.tx]
	if c.State != Larva || h.Tick-int(c.fed) < h.p.LarvaFeedAfterTicks/2 {
		b.Task = Idle
		b.hasTarget = false
		return
	}
	if h.arrived(b, 0.8) {
		c.fed = int32(h.Tick)
		b.jelly--
		b.timer = 3
		b.Task = Idle
		b.hasTarget = false
	}
}

func (h *Hive) doFeedQueen(b *Bee) {
	q := h.queen
	if q == nil || q.IsOutside || q.leaving || q.energy >= 8 || b.jelly == 0 {
		b.Task = Idle
		b.hasTarget = false
		return
	}
	b.tx, b.ty, b.hasTarget = q.X, q.Y, true
	if h.arrived(b, 0.85) {
		q.energy += 3
		b.jelly--
		b.timer = 6
		b.Task = Idle
		b.hasTarget = false
	}
}

func (h *Hive) doCapHoney(b *Bee) {
	if !h.nectarRipe(&h.Grid[b.ty][b.tx]) {
		b.Task = Idle
		b.hasTarget = false
		return
	}
	if h.arrived(b, 0.7) {
		h.ripen(b.tx, b.ty)
		b.timer = 6
		b.Task = Idle
		b.hasTarget = false
	}
}

func (h *Hive) doGuard(b *Bee) {
	if !b.hasTarget {
		b.tx = clamp(h.inX+h.rng.Intn(5)-2+h.inNX, 1, h.width-2)
		b.ty = clamp(h.inY+h.rng.Intn(5)-2+h.inNY, 1, h.height-2)
		b.hasTarget = true
	}
	if chebyshev(b.X, b.Y, b.tx, b.ty) > 0 {
		h.step(b, 0.6)
	} else if h.rng.Float64() < 0.01 {
		b.hasTarget = false
	}
}

func (h *Hive) doDeposit(b *Bee) {
	if !b.hasTarget || !h.canStore(b.tx, b.ty, b.Load) {
		x, y, ok := h.findStoreCell(b.Load)
		if !ok {
			// Nowhere to put it: house bees hold the nectar in their crops
			// and it gets eaten. A honey-bound colony stops expanding.
			h.honeyBound = true
			if b.Load == Nectar {
				h.demand -= b.loadAmt
			}
			b.Load, b.loadAmt = Empty, 0
			b.Task = Idle
			return
		}
		b.tx, b.ty, b.hasTarget = x, y, true
	}
	if h.arrived(b, 0.85) {
		h.deposit(b.tx, b.ty, b.Load, b.loadAmt)
		if b.Role != Forager {
			// a receiver bee storing a forager's load (already counted)
		} else if b.Load == Nectar {
			h.stats.NectarCollected += b.loadAmt
		} else {
			h.stats.PollenCollected += b.loadAmt
		}
		b.Load, b.loadAmt = Empty, 0
		b.hasTarget = false
		b.Task = Idle
		if b.Role == Forager {
			h.maybeDance(b)
		}
	}
}

func (h *Hive) canStore(x, y int, s CellState) bool {
	c := &h.Grid[y][x]
	return c.Usable() || (c.State == s && c.Amount < 0.95)
}

func (h *Hive) queenBehave(b *Bee) {
	if b.leaving {
		h.goExit(b)
		return
	}
	if b.Task == ToExit {
		h.goExit(b)
		return
	}
	if !b.Mated {
		// Virgin queen: a few days to mature, then mating flights.
		if b.Age > 25 {
			b.Mated, b.droneLayer = true, true // never mated: lays only drones
			h.stats.DroneLayingQueens++
		} else if b.Age >= 5 && h.matingWeather() && h.rng.Float64() < 0.02 {
			b.trip = MatingFlight
			b.Task = ToExit
			h.goExit(b)
			return
		}
		h.loiterNest(b, 0.15)
		return
	}
	if h.Tick < b.layReady {
		h.loiterNest(b, 0.15)
		return
	}

	b.layBudget = math.Min(b.layBudget+h.layRate()/DayTicks, 2)
	if b.layBudget < 1 || b.energy < 1 {
		b.Task = Idle
		h.loiterNest(b, 0.15)
		return
	}
	kind := WorkerBrood
	if b.droneLayer || (h.inDroneSeason() && h.counts.Workers > 120 && h.rng.Float64() < h.p.DroneEggFraction) {
		kind = DroneBrood
	}
	// Lay in a clean cell right next to her if it is part of the nest...
	for _, o := range h.radial[1:9] {
		x, y := b.X+int(o.dx), b.Y+int(o.dy)
		if h.inComb(x, y) && h.Grid[y][x].Usable() && h.distToNest(x, y) <= h.nestR+1.5 {
			h.queenLays(b, x, y, kind)
			return
		}
	}
	// ...otherwise walk to the free cell closest to the nest centre.
	if b.Task != LayEgg || !h.Grid[b.ty][b.tx].Usable() {
		cx, cy := h.nestCenter()
		x, y, ok := h.nearestPick(cx, cy, h.radial, 4, func(x, y int, c *Cell) bool {
			return c.Usable() && h.distToNest(x, y) <= h.nestR+4
		})
		if !ok {
			x, y, ok = h.nearest(cx, cy, h.radial, 1e9, func(_, _ int, c *Cell) bool { return c.Usable() })
		}
		if !ok {
			b.layBudget = 0 // comb is full
			return
		}
		h.setTask(b, LayEgg, x, y)
	}
	if h.arrived(b, 0.4) {
		h.queenLays(b, b.tx, b.ty, kind)
	}
}

func (h *Hive) queenLays(b *Bee, x, y int, kind BroodKind) {
	h.layEgg(x, y, kind)
	b.layBudget--
	b.energy--
	b.Task = Idle
	b.hasTarget = false
	b.timer = 2
}

// layRate is the queen's current eggs/day: driven by season, and throttled
// by the colony when food is short or there are too few nurses.
func (h *Hive) layRate() float64 {
	r := h.p.QueenMaxLayPerDay * interp(layCurve, h.DayOfYear())
	c := &h.counts
	stores := h.honey + h.nectar
	r *= clampF(stores/(2+0.03*float64(c.Workers)), 0.1, 1)
	if h.pollen < 0.2 && h.ForageAvailability() < 0.2 {
		r *= 0.4
	}
	if c.Larvae > 3*c.Nurses+4 {
		r *= 0.3
	}
	if h.starving >= 0 && h.starving < 2*DayTicks {
		r = 0
	}
	return r
}

func (h *Hive) droneBehave(b *Bee) {
	if b.leaving {
		h.goExit(b)
		return
	}
	if b.Task == ToExit {
		h.goExit(b)
		return
	}
	if h.matingWeather() && h.TimeOfDay() > 0.45 && h.TimeOfDay() < 0.62 && h.rng.Float64() < 0.01 {
		b.trip = DroneFlight
		b.Task = ToExit
		return
	}
	h.loiter(b)
}

// loiter makes idle bees mill about in the part of the comb their job
// belongs to: young bees on the brood, house bees on the honey, foragers
// near the entrance. In winter everyone packs into the cluster.
func (h *Hive) loiter(b *Bee) {
	day := h.Daylight() > 0.5
	move := 0.3
	if !day {
		move = 0.12
	}
	if b.hasTarget && chebyshev(b.X, b.Y, b.tx, b.ty) > 0 {
		h.step(b, move)
		return
	}
	if h.rng.Float64() > 0.05 {
		return
	}
	cx, cy, r := h.nestX, h.nestY, math.Max(h.nestR+1, h.clusterRadius()*0.8)
	switch {
	case h.Cold():
		r = h.clusterRadius()
		move = 0.08
	case b.Role == HouseBee && h.rng.Float64() < 0.4:
		// receivers wait for incoming foragers near the entrance
		cx += (float64(h.inX) - cx) * 0.75
		cy += (float64(h.inY) - cy) * 0.75
		r = 4
	case b.Role == HouseBee:
		cx += h.honeyDX * h.nestR * 0.8 / 0.6
		cy += h.honeyDY * h.nestR * 0.8
		r = h.nestR + 3
	case b.Role == Forager:
		cx += (float64(h.inX) - cx) * 0.5
		cy += (float64(h.inY) - cy) * 0.5
		r = math.Max(5, h.nestR)
	case b.Role == Drone:
		r = h.nestR + 4
	}
	h.randomTarget(b, cx, cy, r)
}

func (h *Hive) loiterNest(b *Bee, move float64) {
	if b.hasTarget && chebyshev(b.X, b.Y, b.tx, b.ty) > 0 {
		h.step(b, move)
		return
	}
	if h.rng.Float64() < 0.05 {
		h.randomTarget(b, h.nestX, h.nestY, math.Max(h.nestR*0.7, 2))
	}
}

func (h *Hive) randomTarget(b *Bee, cx, cy, r float64) {
	for i := 0; i < 4; i++ {
		dx := (h.rng.Float64()*2 - 1) * r / 0.6
		dy := (h.rng.Float64()*2 - 1) * r
		if combDist(dx, dy) > r {
			continue
		}
		x, y := int(math.Round(cx+dx)), int(math.Round(cy+dy))
		if h.inComb(x, y) {
			b.tx, b.ty, b.hasTarget = x, y, true
			return
		}
	}
}

// arrived moves the bee towards its target and reports whether it is now
// on or next to the target cell.
func (h *Hive) arrived(b *Bee, speed float64) bool {
	if chebyshev(b.X, b.Y, b.tx, b.ty) <= 1 {
		return true
	}
	h.step(b, speed)
	return chebyshev(b.X, b.Y, b.tx, b.ty) <= 1
}

// step moves the bee one cell towards its target, side-stepping bees in
// the way. Only one bee stands on a cell.
func (h *Hive) step(b *Bee, chance float64) {
	if h.rng.Float64() > chance {
		return
	}
	dx, dy := sign(b.tx-b.X), sign(b.ty-b.Y)
	if dx == 0 && dy == 0 {
		return
	}
	var tries [3][2]int
	tries[0] = [2]int{dx, dy}
	switch {
	case dx != 0 && dy != 0:
		tries[1], tries[2] = [2]int{dx, 0}, [2]int{0, dy}
	case dx != 0:
		tries[1], tries[2] = [2]int{dx, 1}, [2]int{dx, -1}
	default:
		tries[1], tries[2] = [2]int{1, dy}, [2]int{-1, dy}
	}
	if h.rng.Intn(2) == 0 {
		tries[1], tries[2] = tries[2], tries[1]
	}
	for _, t := range tries {
		if h.moveTo(b, b.X+t[0], b.Y+t[1]) {
			b.stuck = 0
			return
		}
	}
	b.stuck++
	// Boxed in: occasionally shuffle sideways to break up jams.
	if h.rng.Float64() < 0.2 {
		o := h.radial[1+h.rng.Intn(8)]
		h.moveTo(b, b.X+int(o.dx), b.Y+int(o.dy))
	}
}

func (h *Hive) jiggle(b *Bee) {
	if h.rng.Float64() < 0.5 {
		o := h.radial[1+h.rng.Intn(8)]
		h.moveTo(b, b.X+int(o.dx), b.Y+int(o.dy))
	}
}

func (h *Hive) moveTo(b *Bee, x, y int) bool {
	if !h.inComb(x, y) {
		return false
	}
	// Resting bees keep a cell to themselves, but a bee on an errand can
	// clamber over one other bee, and everyone makes way for the queen.
	limit := uint8(1)
	if b.Role == Queen || b.stuck > 4 {
		limit = 255 // pushes through the crowd
	} else if b.Task != Idle {
		limit = 2
	}
	if h.occ[h.idx(x, y)] >= limit {
		return false
	}
	h.occ[h.idx(b.X, b.Y)]--
	b.X, b.Y = x, y
	h.occ[h.idx(x, y)]++
	return true
}

func chebyshev(x1, y1, x2, y2 int) int {
	return max(abs(x1-x2), abs(y1-y2))
}

// GetColor returns the LED colour of a bee.
func (b *Bee) GetColor(tick int) color.Color {
	switch {
	case b.Role == Queen:
		return color.RGBA{255, 0, 255, 255} // magenta
	case b.Role == Drone:
		return color.RGBA{0, 90, 255, 255} // blue, big-eyed drones
	case b.Task == FeedQueen:
		return color.RGBA{255, 5, 5, 255} // red: queen's retinue
	case b.Task == Dance:
		if (tick/2)%2 == 0 {
			return color.RGBA{255, 255, 0, 255}
		}
		return color.RGBA{255, 255, 255, 255}
	case b.Load == Nectar:
		return color.RGBA{255, 255, 0, 255} // yellow: nectar in the crop
	case b.Load == Pollen:
		return color.RGBA{255, 110, 0, 255} // orange pollen baskets
	case b.Role == Cleaner || b.Role == Nurse:
		return color.RGBA{110, 110, 140, 255} // young, fuzzy house bees (dimmer than older bees)
	}
	return color.RGBA{255, 255, 255, 255}
}
