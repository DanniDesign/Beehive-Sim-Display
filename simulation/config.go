package simulation

// Time scale. One simulated day is DayTicks ticks (at 5 FPS that is 36 s),
// and the year has its real length, so bee development times and the
// seasons keep their true proportions. Day 0 of the year is 1 March.
const (
	DayTicks       = 180
	YearDays       = 365
	StartDayOfYear = 15 // simulations start in mid-March
)

// Brood development times in days (real values for Apis mellifera).
const (
	eggDays         = 3.0
	workerLarvaDays = 6.0
	workerPupaDays  = 12.0
	droneLarvaDays  = 6.5
	dronePupaDays   = 14.5
	queenLarvaDays  = 5.5
	queenPupaDays   = 7.5
)

// Params holds the tunable knobs of the colony model. The defaults were
// tuned with cmd/montecarlo over thousands of runs so the colony shows a
// realistic seasonal cycle on a 64x32 comb.
type Params struct {
	// Queen
	QueenMaxLayPerDay   float64 // peak eggs per day in spring/summer
	QueenLifespanYears  float64 // mean queen lifespan (exponential hazard)
	QueenMatingLossProb float64 // chance a virgin queen is lost on her mating flight
	DroneEggFraction    float64 // share of eggs laid as drones in drone season

	// Worker life
	InHiveLifeDays   float64 // lifespan budget of a bee that never forages
	NurseLifeDays    float64 // brood rearing wears bees out faster
	ForagerLifeDays  float64 // lifespan budget once a bee has become a forager
	FlightBudgetDays float64 // total time a worker can spend flying before it wears out
	WinterLifeDays   float64 // lifespan budget while clustering in winter
	DroneLifeDays    float64
	TripMortality    float64 // chance a forager does not come back from a trip
	NurseAgeDays     float64 // workers start nursing at this (physiological) age...
	HouseAgeDays     float64 // ...become house bees/guards at this age...
	ForagerAgeDays   float64 // ...and start foraging at this age.

	// Food
	NectarLoad         float64 // honey-equivalent units brought per nectar trip
	PollenLoad         float64 // pollen units brought per pollen trip
	PollenPerRefill    float64 // pollen a nurse eats to make one batch of jelly
	FeedsPerRefill     int     // larvae a nurse can feed per refill
	WorkerHoneyPerDay  float64
	ClusterHoneyPerDay float64 // consumption per bee in the winter cluster
	ForagerExtraPerDay float64 // extra fuel burned by bees that are flying
	DroneHoneyPerDay   float64
	LarvaHoneyPerDay   float64
	NectarRipenDays    float64 // nectar needs this long before it can be capped

	// Larva care
	LarvaFeedAfterTicks int // nurses consider a larva hungry after this long
	LarvaStarveDays     float64

	// Swarming
	SwarmPopulation int     // adult population that makes a colony want to swarm
	SwarmFraction   float64 // share of workers that leave with the old queen

	// Empty hive gets reoccupied by a wild swarm after this many days.
	RecolonizeDays int
	NewSwarmSize   int
}

func DefaultParams() Params {
	return Params{
		QueenMaxLayPerDay:   16,
		QueenLifespanYears:  4,
		QueenMatingLossProb: 0.12,
		DroneEggFraction:    0.07,

		InHiveLifeDays:   70,
		NurseLifeDays:    50,
		ForagerLifeDays:  30,
		FlightBudgetDays: 4,
		WinterLifeDays:   400,
		DroneLifeDays:    35,
		TripMortality:    0.006,
		NurseAgeDays:     3,
		HouseAgeDays:     12,
		ForagerAgeDays:   20,

		NectarLoad:         0.18,
		PollenLoad:         0.12,
		PollenPerRefill:    0.03,
		FeedsPerRefill:     4,
		WorkerHoneyPerDay:  0.010,
		ClusterHoneyPerDay: 0.007,
		ForagerExtraPerDay: 0.010,
		DroneHoneyPerDay:   0.030,
		LarvaHoneyPerDay:   0.015,
		NectarRipenDays:    1.5,

		LarvaFeedAfterTicks: DayTicks / 5,
		LarvaStarveDays:     1.2,

		SwarmPopulation: 260,
		SwarmFraction:   0.6,

		RecolonizeDays: 6,
		NewSwarmSize:   110,
	}
}
