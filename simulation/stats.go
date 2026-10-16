package simulation

// Counts is a per-tick census of the colony.
type Counts struct {
	Workers    int // adult workers, inside and out
	Inside     int // bees of any caste inside the hive
	Outside    int // bees out flying
	Drones     int
	Cleaners   int
	Nurses     int
	HouseBees  int
	Guards     int
	Foragers   int
	Attendants int // nurses currently feeding the queen
	Eggs       int
	Larvae     int // open worker/drone brood
	Pupae      int // capped brood
	RoyalBrood int // queen cells
	RoyalPupae int // capped queen cells
	DroneBrood int
}

// Counters are cumulative event totals since the hive was created.
type Counters struct {
	EggsLaid, WorkersBorn, DronesBorn, QueensEmerged int

	DiedOldAge, DiedForaging, DiedStarvation, LeftWithSwarm, DronesExpelled int
	LarvaeStarved, BroodCannibalised                                        int

	ForagingTrips                    int
	NectarCollected, PollenCollected float64
	HoneyEaten                       float64

	SwarmPreparations, Swarms, EmergencyQueenCells, Supersedures int
	QueensMated, QueensLostMating, QueensDied, DroneLayingQueens int
	LayingWorkerDays, StarvationEpisodes                         int

	Collapses, CollapseStarved, CollapseQueenless, CollapseDwindled int
	Recolonizations                                                 int
}

func (h *Hive) count() { h.counts = h.census() }

func (h *Hive) census() Counts {
	c := Counts{}
	for _, b := range h.bees {
		if b.dead {
			continue
		}
		if b.IsOutside {
			c.Outside++
		} else {
			c.Inside++
		}
		switch b.Role {
		case Drone:
			c.Drones++
			continue
		case Queen:
			continue
		case Cleaner:
			c.Cleaners++
		case Nurse:
			c.Nurses++
		case HouseBee:
			c.HouseBees++
		case Guard:
			c.Guards++
		case Forager:
			c.Foragers++
		}
		c.Workers++
		if b.Task == FeedQueen {
			c.Attendants++
		}
	}
	for _, ci := range h.brood {
		cell := &h.Grid[int(ci)/h.width][int(ci)%h.width]
		switch cell.State {
		case Egg:
			c.Eggs++
		case Larva:
			c.Larvae++
		case Pupa:
			c.Pupae++
		}
		switch cell.Kind {
		case QueenBrood:
			c.RoyalBrood++
			if cell.State == Pupa {
				c.RoyalPupae++
			}
		case DroneBrood:
			c.DroneBrood++
		}
	}
	return c
}

// Snapshot is a read-only summary of the colony for display and analysis.
type Snapshot struct {
	Tick        int
	Day         int
	DayOfYear   float64
	Season      int
	Dead        bool
	HasQueen    bool
	QueenMated  bool
	SwarmPrep   bool
	Starving    bool
	HoneyBound  bool
	Honey       float64
	Nectar      float64
	Pollen      float64
	NestX       float64
	NestY       float64
	Counts      Counts
	Stats       Counters
	HoneyCells  int
	PollenCells int
	DirtyCells  int
	// Mean row of brood/honey cells: honey should sit above the brood.
	BroodMeanY, HoneyMeanY float64
}

func (h *Hive) Snapshot() Snapshot {
	s := Snapshot{
		Tick:       h.Tick,
		Day:        h.Day(),
		DayOfYear:  h.DayOfYear(),
		Season:     h.Season(),
		Dead:       h.dead,
		HasQueen:   h.queen != nil,
		SwarmPrep:  h.swarmPrep,
		Starving:   h.starving >= 0 && h.starving < DayTicks,
		HoneyBound: h.honeyBound,
		Honey:      h.honey,
		Nectar:     h.nectar,
		Pollen:     h.pollen,
		NestX:      h.nestX,
		NestY:      h.nestY,
		Counts:     h.census(),
		Stats:      h.stats,
	}
	if h.queen != nil {
		s.QueenMated = h.queen.Mated && !h.queen.droneLayer
	}
	var by, hy float64
	var bn, hn int
	for y := 1; y < h.height-1; y++ {
		for x := 1; x < h.width-1; x++ {
			c := &h.Grid[y][x]
			switch {
			case c.IsBrood():
				by += float64(y)
				bn++
			case c.IsFood():
				hy += float64(y)
				hn++
			case c.State == Pollen:
				s.PollenCells++
			}
			if c.Dirty {
				s.DirtyCells++
			}
		}
	}
	s.HoneyCells = hn
	if bn > 0 {
		s.BroodMeanY = by / float64(bn)
	}
	if hn > 0 {
		s.HoneyMeanY = hy / float64(hn)
	}
	return s
}

// Bees returns every living bee (inside and outside the hive).
func (h *Hive) Bees() []*Bee { return h.bees }

// Cells returns the comb.
func (h *Hive) Cells() [][]Cell { return h.Grid }

// GetQueen returns the colony's queen, or nil while queenless.
func (h *Hive) GetQueen() *Bee { return h.queen }
