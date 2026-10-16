package simulation

import (
	"math"
	"math/rand"
	"time"

	"github.com/brianvoe/gofakeit/v7"
)

type Hive struct {
	Name         string
	ExitX, ExitY int // entrance, on the bottom or a side of the frame
	Grid         [][]Cell
	Tick         int
	Age          int

	p             Params
	rng           *rand.Rand
	faker         *gofakeit.Faker
	width, height int

	bees    []*Bee
	scratch []*Bee
	queen   *Bee
	occ     []uint8 // bees standing on each cell

	brood        []int32 // indices of every egg/larva/pupa cell
	broodScratch []int32
	broodPos     []int32 // cell index -> position in brood, -1 if not brood

	honey, nectar, pollen float64 // stores in honey-equivalent units

	nestX, nestY float64 // brood nest / winter cluster centre
	homeX, homeY float64

	inX, inY, inNX, inNY int     // comb cell just inside the entrance, and the inward direction
	honeyDX, honeyDY     float64 // direction (from the nest) in which honey is stored
	noise, storeNoise    []float32
	frontier             cellSet // unbuilt cells at the edge of the comb
	combCells            int
	freeNearNest         int
	nestR                float64 // current brood nest radius (combDist units)

	nectarSet, pollenSet, dirtySet cellSet

	radial    []offset // offsets sorted by elliptical distance
	honeyOffs []offset // same, but biased upwards (honey goes above the brood)

	rainy       bool
	env         env
	recruitment float64 // waggle-dance excitement, raises forager departures
	demand      float64 // honey eaten but not yet taken out of the cells
	starving    int     // ticks since the colony last ran out of food (-1 = never)
	honeyBound  bool    // foragers couldn't find a cell for their nectar today
	devRate     float64 // speed of behavioural maturation (colony feedback)

	counts        Counts
	queenlessFrom int // tick the colony lost its queen, -1 while queenright
	swarmPrep     bool
	dead          bool
	deadFrom      int
	nextID        int

	stats Counters
}

// NewHive creates a colony with a random seed.
func NewHive(beeAmount, width, height int) *Hive {
	return NewHiveWithSeed(time.Now().UnixNano(), beeAmount, width, height, DefaultParams())
}

// NewHiveWithSeed creates a reproducible colony: the same seed and params
// always produce exactly the same simulation.
func NewHiveWithSeed(seed int64, beeAmount, width, height int, p Params) *Hive {
	rng := rand.New(rand.NewSource(seed))
	h := &Hive{
		p:             p,
		rng:           rng,
		faker:         gofakeit.New(uint64(seed)),
		width:         width,
		height:        height,
		starving:      -1,
		devRate:       1,
		queenlessFrom: -1,
	}
	h.Name = h.faker.Bird()

	h.Grid = make([][]Cell, height)
	for y := range h.Grid {
		h.Grid[y] = make([]Cell, width)
	}
	h.occ = make([]uint8, width*height)
	h.broodPos = make([]int32, width*height)
	for i := range h.broodPos {
		h.broodPos[i] = -1
	}
	h.nectarSet = newCellSet(width * height)
	h.pollenSet = newCellSet(width * height)
	h.dirtySet = newCellSet(width * height)
	h.radial = buildOffsets(width, height, combDist)
	h.frontier = newCellSet(width * height)
	h.buildNoise()
	h.setupEntrance()
	h.honeyOffs = buildOffsets(width, height, func(dx, dy float64) float64 {
		return combDist(dx, dy) - 0.45*(dx*0.6*h.honeyDX+dy*h.honeyDY)
	})
	h.growInitialComb(380 + rng.Intn(520))
	h.placeHome()
	h.rollWeather()
	h.updateEnv()

	h.seedColony(beeAmount)
	return h
}

// seedColony sets up an overwintered colony in early spring: a small
// brood nest, a band of honey above it, a little pollen, and workers of
// mixed ages around a mated queen.
func (h *Hive) seedColony(workers int) {
	h.updateNestRadius(float64(workers) / 4)

	for i := 0; i < 30+workers/3; i++ {
		x, y, ok := h.findStoreCell(Nectar)
		if !ok {
			break
		}
		h.deposit(x, y, Nectar, 1)
		h.ripen(x, y)
	}
	for i := 0; i < 5; i++ {
		x, y, ok := h.findStoreCell(Pollen)
		if !ok {
			break
		}
		h.deposit(x, y, Pollen, 0.8)
	}
	cx, cy := h.nestCenter()
	for i := 0; i < workers/4; i++ {
		x, y, ok := h.nearestPick(cx, cy, h.radial, 4, func(_, _ int, c *Cell) bool { return c.Usable() })
		if !ok {
			break
		}
		h.layEgg(x, y, WorkerBrood)
		c := &h.Grid[y][x]
		ageTicks := h.rng.Intn(int((eggDays + workerLarvaDays + workerPupaDays) * DayTicks))
		switch {
		case ageTicks < int(eggDays*DayTicks):
			c.since = int32(-ageTicks)
		case ageTicks < int((eggDays+workerLarvaDays)*DayTicks):
			h.setState(x, y, Larva)
			c.since = int32(-(ageTicks - int(eggDays*DayTicks)))
		default:
			h.setState(x, y, Pupa)
			c.since = int32(-(ageTicks - int((eggDays+workerLarvaDays)*DayTicks)))
		}
	}
	h.stats.EggsLaid = 0

	q := h.spawnNear(cx, cy, Queen)
	q.Mated = true
	q.Age = 100
	q.energy = 6
	for i := 0; i < workers; i++ {
		b := h.spawnNear(cx+h.rng.Intn(21)-10, cy+h.rng.Intn(9)-4, Forager)
		b.Age = h.rng.Float64() * 40
		b.physAge = b.Age
		b.vitality = 1 - b.Age/200
		h.assignRole(b)
	}
}

// Update advances the simulation by one tick.
func (h *Hive) Update() {
	h.Tick++
	h.Age++

	if h.Tick%DayTicks == 0 {
		h.rollWeather()
		h.updateEnv()
		h.daily()
	}
	h.count()
	h.updateEnv()
	h.developBrood()

	h.scratch = append(h.scratch[:0], h.bees...)
	h.rng.Shuffle(len(h.scratch), func(i, j int) { h.scratch[i], h.scratch[j] = h.scratch[j], h.scratch[i] })
	for _, b := range h.scratch {
		if !b.dead {
			h.updateBee(b)
		}
	}
	h.accrueDemand()
	if h.Tick%12 == 0 {
		h.eat()
	}
	if h.starving >= 0 {
		h.starving++
	}
	h.recruitment *= 0.995

	h.checkSwarmDeparture()
	h.checkCollapse()
	h.removeDead()
}

func (h *Hive) removeDead() {
	alive := h.bees[:0]
	for _, b := range h.bees {
		if !b.dead {
			alive = append(alive, b)
		}
	}
	for i := len(alive); i < len(h.bees); i++ {
		h.bees[i] = nil
	}
	h.bees = alive
}

// daily runs once per simulated day.
func (h *Hive) daily() {
	h.honeyBound = false
	h.count()
	c := &h.counts

	h.updateNestRadius(float64(c.Eggs + c.Larvae + c.Pupae))
	h.moveNest()
	h.countFreeNearNest()

	// Behavioural development speeds up when foragers are scarce and
	// slows down when there are plenty (or in winter): the colony's
	// division of labour is self-regulating.
	// Brood pheromone drives the switch from nursing to foraging: with
	// little open brood (late summer, autumn) young bees stay at home as
	// long-lived "winter bees" instead of turning into foragers.
	h.devRate = 1
	if c.Workers > 0 {
		frac := float64(c.Foragers) / float64(c.Workers)
		if frac < 0.2 {
			h.devRate = 1.6
		} else if frac > 0.45 {
			h.devRate = 0.6
		}
		h.devRate *= clampF(h.openBroodRatio()/0.2, 0.15, 1)
	}
	if h.Cold() {
		h.devRate = 0.1
	}

	if h.dead {
		h.robDeadHive()
		if h.Tick-h.deadFrom >= h.p.RecolonizeDays*DayTicks && h.inSwarmSeason() && !h.rainy {
			h.wildSwarmArrives()
		}
		return
	}

	h.maybeStartSwarming()
	h.requeen()
	h.expelDrones()
}

// openBroodRatio is open brood (eggs and larvae) per adult worker.
func (h *Hive) openBroodRatio() float64 { return h.env.broodRatio }

func (h *Hive) updateNestRadius(brood float64) {
	h.nestR = math.Sqrt(0.6*(brood+10)/math.Pi) + 0.5
}

// moveNest: in winter the cluster slowly creeps towards its honey; the rest
// of the year the brood nest drifts back to the warmest, central spot.
func (h *Hive) moveNest() {
	cx, cy := h.nestCenter()
	if h.Cold() {
		near := 0.0
		r := float32(h.clusterRadius() + 2)
		h.nearest(cx, cy, h.radial, r, func(_, _ int, c *Cell) bool {
			if c.IsFood() {
				near += float64(c.Amount)
			}
			return false
		})
		if near < 3 {
			if x, y, ok := h.nearest(cx, cy, h.radial, 200, func(_, _ int, c *Cell) bool { return c.IsFood() }); ok {
				h.nestX += clampF(float64(x)-h.nestX, -1, 1)
				h.nestY += clampF(float64(y)-h.nestY, -1, 1)
			}
		}
		return
	}
	// The nest wanders with the brood (the queen lays where there is
	// room), with a gentle pull back towards the warm core near the entrance.
	if len(h.brood) > 20 {
		var sx, sy float64
		for _, ci := range h.brood {
			sx += float64(int(ci) % h.width)
			sy += float64(int(ci) / h.width)
		}
		n := float64(len(h.brood))
		h.nestX += clampF(sx/n-h.nestX, -0.6, 0.6) * 0.3
		h.nestY += clampF(sy/n-h.nestY, -0.6, 0.6) * 0.3
	}
	h.nestX += clampF(h.homeX-h.nestX, -0.3, 0.3)
	h.nestY += clampF(h.homeY-h.nestY, -0.3, 0.3)
}

func (h *Hive) clusterRadius() float64 {
	return math.Sqrt(0.6*float64(h.counts.Inside+1)/math.Pi)*1.15 + 1
}

// developBrood ages every egg, larva and pupa by one tick.
func (h *Hive) developBrood() {
	starve := int(h.p.LarvaStarveDays * DayTicks)
	// Iterate over a copy: an emerging queen destroys other brood cells.
	h.broodScratch = append(h.broodScratch[:0], h.brood...)
	for _, ci32 := range h.broodScratch {
		ci := int(ci32)
		x, y := ci%h.width, ci/h.width
		c := &h.Grid[y][x]
		if !c.IsBrood() {
			continue
		}
		age := h.Tick - int(c.since)
		switch c.State {
		case Egg:
			if age >= stageTicks(c) {
				h.setState(x, y, Larva)
				c.fed = int32(h.Tick)
			}
		case Larva:
			if c.Kind == QueenBrood && h.counts.Workers > 0 {
				c.fed = int32(h.Tick) // royal larvae float in royal jelly
			}
			if h.Tick-int(c.fed) > starve {
				h.clearCell(x, y, true)
				h.stats.LarvaeStarved++
				continue
			}
			if age >= stageTicks(c) {
				h.setState(x, y, Pupa)
			}
		case Pupa:
			if age < stageTicks(c) {
				continue
			}
			if c.Kind == QueenBrood && h.queen != nil && !h.queen.leaving {
				if !h.queen.Mated {
					continue // workers keep spare queens caged until the virgin is mated
				}
				if !h.queen.droneLayer {
					h.clearCell(x, y, true) // a mated queen is in charge: rival torn down
					continue
				}
			}
			h.emerge(x, y, c.Kind)
		}
	}
}

func (h *Hive) emerge(x, y int, kind BroodKind) {
	h.clearCell(x, y, true)
	switch kind {
	case WorkerBrood:
		b := h.spawnAt(x, y, Cleaner)
		b.vitality = 1
		h.stats.WorkersBorn++
	case DroneBrood:
		h.spawnAt(x, y, Drone)
		h.stats.DronesBorn++
	case QueenBrood:
		if h.queen != nil && h.queen.Mated && !h.queen.leaving {
			if h.queen.droneLayer {
				h.kill(h.queen, &h.stats.Supersedures)
			} else {
				// The old queen is still home: the colony swarms now.
				h.departSwarm()
			}
		}
		if h.queen != nil && !h.queen.leaving {
			return // a rival virgin is already in charge; this one is killed
		}
		q := h.spawnAt(x, y, Queen)
		q.energy = 4
		h.queenlessFrom = -1
		h.stats.QueensEmerged++
		// The first virgin out stings rivals that are still developing;
		// capped queen cells are guarded by the workers as a reserve in
		// case she is lost on her mating flight.
		for i := len(h.brood) - 1; i >= 0; i-- {
			ci := int(h.brood[i])
			bx, by := ci%h.width, ci/h.width
			if c := &h.Grid[by][bx]; c.Kind == QueenBrood && (c.State != Pupa || h.rng.Float64() < 0.5) {
				h.clearCell(bx, by, true)
			}
		}
	}
}

// accrueDemand adds this tick's food consumption to the colony's bill.
func (h *Hive) accrueDemand() {
	c := &h.counts
	perBee := h.p.WorkerHoneyPerDay
	if h.Cold() {
		perBee = h.p.ClusterHoneyPerDay
	}
	d := float64(c.Workers)*perBee +
		float64(c.Outside)*h.p.ForagerExtraPerDay +
		float64(c.Drones)*h.p.DroneHoneyPerDay +
		float64(c.Larvae)*h.p.LarvaHoneyPerDay
	if h.queen != nil {
		d += perBee
	}
	h.demand += d / DayTicks
}

// eat takes the accrued demand out of the food cells nearest the nest,
// which is how real bees empty the inner edge of the honey arch first.
func (h *Hive) eat() {
	if h.demand <= 0 {
		return
	}
	cx, cy := h.nestCenter()
	for h.demand > 1e-6 {
		x, y, ok := h.nearest(cx, cy, h.radial, 200, func(_, _ int, c *Cell) bool { return c.IsFood() })
		if !ok {
			h.starve()
			return
		}
		got := h.take(x, y, h.demand)
		h.demand -= got
		h.stats.HoneyEaten += got
	}
	h.demand = 0
}

// starve: with no food left, the colony eats its brood and bees die.
func (h *Hive) starve() {
	h.demand = 0
	if h.starving < 0 || h.starving > DayTicks {
		h.stats.StarvationEpisodes++
	}
	h.starving = 0
	for i := len(h.brood) - 1; i >= 0; i-- {
		ci := int(h.brood[i])
		x, y := ci%h.width, ci/h.width
		if h.Grid[y][x].State != Pupa && h.rng.Float64() < 0.3 {
			h.clearCell(x, y, true)
			h.stats.BroodCannibalised++
		}
	}
	for _, b := range h.bees {
		if b.dead || b.Role == Queen {
			continue
		}
		if b.Role == Drone {
			b.leaving = true
		} else if h.rng.Float64() < 0.04 {
			h.kill(b, &h.stats.DiedStarvation)
		}
	}
}

// maybeStartSwarming: a crowded colony in spring raises queen cells and
// prepares to split. Congestion of the brood nest (no room for the queen
// to lay, often because it is filled with honey) is the main trigger.
func (h *Hive) maybeStartSwarming() {
	q := h.queen
	if q == nil || !q.Mated || q.droneLayer || q.leaving || h.swarmPrep || !h.inSwarmSeason() {
		return
	}
	adults := h.counts.Workers
	if adults < h.p.SwarmPopulation {
		return
	}
	cx, cy := h.nestCenter()
	free := 0
	h.nearest(cx, cy, h.radial, float32(h.nestR+2), func(_, _ int, c *Cell) bool {
		if c.Usable() {
			free++
		}
		return false
	})
	congested := free < 12 || h.honeyBound
	chance := 0.08 // a strong colony may swarm anyway...
	if congested {
		chance = 0.4 // ...but a crowded brood nest is the main trigger
	}
	if h.rng.Float64() > chance {
		return
	}
	// Queen cups hang from the lower edge of the brood nest.
	ex := clamp(int(h.nestX-h.honeyDX*h.nestR/0.6), 1, h.width-2)
	ey := clamp(int(h.nestY-h.honeyDY*h.nestR), 1, h.height-2)
	built := 0
	for built < 3 {
		x, y, ok := h.nearest(ex, ey, h.radial, 12, func(_, _ int, c *Cell) bool { return c.Usable() })
		if !ok {
			break
		}
		h.layEgg(x, y, QueenBrood)
		built++
	}
	if built > 0 {
		h.swarmPrep = true
		h.stats.SwarmPreparations++
	}
}

// checkSwarmDeparture: once the first queen cell is capped the old queen
// leaves with most of the workers on the next fine day around noon.
func (h *Hive) checkSwarmDeparture() {
	if !h.swarmPrep || h.counts.RoyalPupae == 0 || !h.CanFly() {
		return
	}
	if t := h.TimeOfDay(); t < 0.42 || t > 0.6 {
		return
	}
	h.departSwarm()
}

func (h *Hive) departSwarm() {
	h.swarmPrep = false
	q := h.queen
	if q == nil {
		return
	}
	q.leaving = true
	q.Task = ToExit
	q.hasTarget = false
	h.queen = nil
	h.queenlessFrom = h.Tick
	for _, b := range h.bees {
		if b.dead || b.IsOutside || b.Role == Drone || b.Role == Queen {
			continue
		}
		chance := h.p.SwarmFraction
		if b.Role == Cleaner {
			chance *= 0.5 // the youngest bees mostly stay behind
		}
		if h.rng.Float64() < chance {
			b.leaving = true
			b.Task = ToExit
			b.hasTarget = false
			b.Load = Empty
		}
	}
	h.stats.Swarms++
}

// requeen: a queenless colony turns young worker larvae into emergency
// queens. If none are young enough, workers eventually start laying
// unfertilised (drone) eggs and the colony is doomed.
func (h *Hive) requeen() {
	if h.queen != nil || h.counts.RoyalBrood > 0 || h.counts.Workers == 0 {
		return
	}
	if h.queenlessFrom < 0 {
		h.queenlessFrom = h.Tick
	}
	made := 0
	for _, ci := range h.brood {
		x, y := int(ci)%h.width, int(ci)/h.width
		c := &h.Grid[y][x]
		young := c.State == Egg || (c.State == Larva && h.Tick-int(c.since) < 2*DayTicks)
		if c.Kind == WorkerBrood && young {
			c.Kind = QueenBrood
			made++
			if made == 2 {
				break
			}
		}
	}
	if made > 0 {
		h.stats.EmergencyQueenCells++
		return
	}
	if h.Tick-h.queenlessFrom > 10*DayTicks && !h.Cold() {
		h.stats.LayingWorkerDays++
		cx, cy := h.nestCenter()
		for i := 0; i < 1+h.rng.Intn(3); i++ {
			x, y, ok := h.nearest(cx+h.rng.Intn(7)-3, cy+h.rng.Intn(5)-2, h.radial, 15, func(_, _ int, c *Cell) bool { return c.Usable() })
			if ok {
				h.layEgg(x, y, DroneBrood)
			}
		}
	}
}

// expelDrones: in autumn, or whenever food runs short, workers drag the
// drones out of the hive.
func (h *Hive) expelDrones() {
	stores := h.honey + h.nectar
	if !h.lateSeason() && stores > 4+0.03*float64(h.counts.Workers) && h.starving < 0 {
		return
	}
	for _, b := range h.bees {
		if b.Role == Drone && !b.dead && !b.leaving && h.rng.Float64() < 0.35 {
			b.leaving = true
			b.Task = ToExit
			b.hasTarget = false
		}
	}
}

func (h *Hive) checkCollapse() {
	if h.dead || h.counts.Workers > 0 {
		return
	}
	// Nobody left to keep the brood warm or feed the queen.
	h.dead = true
	h.deadFrom = h.Tick
	h.stats.Collapses++
	switch {
	case h.starving >= 0 && h.starving < 5*DayTicks:
		h.stats.CollapseStarved++
	case h.queen == nil || h.queen.droneLayer || !h.queen.Mated:
		h.stats.CollapseQueenless++
	default:
		h.stats.CollapseDwindled++
	}
	for _, b := range h.bees {
		if !b.dead {
			h.kill(b, nil)
		}
	}
	for i := len(h.brood) - 1; i >= 0; i-- {
		ci := int(h.brood[i])
		h.clearCell(ci%h.width, ci/h.width, true)
	}
	h.swarmPrep = false
}

// robDeadHive: bees from other colonies steal the honey of a dead hive.
func (h *Hive) robDeadHive() {
	if h.ForageAvailability() < 0.05 && h.Cold() {
		return
	}
	for y := 1; y < h.height-1; y++ {
		for x := 1; x < h.width-1; x++ {
			c := &h.Grid[y][x]
			if (c.IsFood() || c.State == Pollen) && h.rng.Float64() < 0.2 {
				h.clearCell(x, y, false)
			}
			if c.Dirty && h.rng.Float64() < 0.1 {
				h.setDirty(x, y, false)
			}
			// Wax moth larvae chew through unguarded comb, edges first.
			if c.Built && c.State == Empty && h.builtNeighbours(x, y) < 8 && h.rng.Float64() < 0.03 {
				h.unbuildCell(x, y)
			}
		}
	}
}

// wildSwarmArrives: an empty hive with drawn comb is a prime home for a
// passing swarm. They pour in through the entrance over a minute or so.
func (h *Hive) wildSwarmArrives() {
	h.dead = false
	h.starving = -1
	h.queenlessFrom = -1
	h.placeHome()
	h.stats.Recolonizations++
	h.Name = h.faker.Bird()

	q := h.spawnOutside(Queen, 40+h.rng.Intn(40))
	q.Mated = true
	q.Age = 30 + h.rng.Float64()*100
	q.energy = 6
	q.layReady = h.Tick + 2*DayTicks
	for i := 0; i < h.p.NewSwarmSize; i++ {
		b := h.spawnOutside(Forager, 1+h.rng.Intn(150))
		b.Age = 4 + h.rng.Float64()*20
		b.physAge = b.Age
		h.assignRole(b)
		// Swarm bees gorge on honey before leaving their old home and
		// unload it into the new comb.
		b.Load, b.loadAmt = Nectar, 0.08
	}
}

// findStoreCell picks where a load of nectar or pollen should go. Pollen
// is packed right around the brood; nectar goes just beyond that, mostly
// above the nest, which builds the classic honey arch.
func (h *Hive) findStoreCell(s CellState) (int, int, bool) {
	cx, cy := h.nestCenter()
	minD := h.nestR
	offs := h.radial
	if s == Nectar {
		minD = h.nestR + 2.5
		offs = h.honeyOffs
	}
	return h.pickStoreCell(cx, cy, offs, func(x, y int, c *Cell) bool {
		if h.distToNest(x, y) < minD {
			return false
		}
		return c.Usable() || (c.State == s && c.Amount < 0.95)
	})
}

// ripen turns a nectar cell into capped honey.
func (h *Hive) ripen(x, y int) {
	c := &h.Grid[y][x]
	if c.State != Nectar {
		return
	}
	h.nectar -= float64(c.Amount)
	h.honey += float64(c.Amount)
	h.setState(x, y, Honey)
}

func (h *Hive) nectarRipe(c *Cell) bool {
	return c.State == Nectar && h.Tick-int(c.since) > int(h.p.NectarRipenDays*DayTicks)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	} else if v > hi {
		return hi
	}
	return v
}

func clampF(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}

func sign(v int) int {
	if v < 0 {
		return -1
	} else if v > 0 {
		return 1
	}
	return 0
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
