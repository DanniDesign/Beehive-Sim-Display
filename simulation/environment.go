package simulation

import "math"

// keyframe is a (dayOfYear, value) point; curves are linearly interpolated
// and wrap around the year.
type keyframe struct{ day, v float64 }

func interp(frames []keyframe, day float64) float64 {
	for i := 1; i < len(frames); i++ {
		if day <= frames[i].day {
			a, b := frames[i-1], frames[i]
			t := (day - a.day) / (b.day - a.day)
			return a.v + (b.v-a.v)*t
		}
	}
	return frames[len(frames)-1].v
}

// Curves for a temperate (north-west European) climate. Day 0 is
// 1 March, the start of meteorological spring.
var (
	// Nectar/pollen in the landscape: willow and crocus in March, fruit
	// blossom in April, rapeseed and hawthorn in May, the "June gap",
	// lime and clover in July, heather in August, ivy into October.
	forageCurve = []keyframe{
		{0, 0.15}, {20, 0.35}, {45, 0.75}, {65, 1.0}, {92, 0.95}, {100, 0.6},
		{110, 0.65}, {125, 0.9}, {145, 0.8}, {165, 0.45}, {180, 0.5}, {200, 0.3},
		{225, 0.25}, {245, 0.08}, {260, 0}, {335, 0}, {350, 0.05}, {365, 0.15},
	}
	// Mean daily temperature (deg C).
	temperatureCurve = []keyframe{
		{0, 6}, {30, 9.5}, {61, 13.5}, {92, 16.5}, {122, 18.5}, {153, 18.5},
		{184, 15}, {214, 11}, {245, 7}, {275, 4}, {306, 3}, {336, 4}, {365, 6},
	}
	// How strongly the queen is inclined to lay (day length, temperature):
	// she starts again in late winter, peaks in May-June and nearly stops
	// by October.
	layCurve = []keyframe{
		{0, 0.35}, {30, 0.7}, {55, 1.0}, {115, 1.0}, {140, 0.7}, {165, 0.4},
		{195, 0.2}, {220, 0.05}, {235, 0}, {320, 0}, {340, 0.1}, {365, 0.35},
	}
	// Chance of a wet day (too rainy to fly).
	rainCurve = []keyframe{
		{0, 0.35}, {60, 0.28}, {120, 0.22}, {180, 0.28}, {240, 0.4}, {365, 0.35},
	}
)

// DayOfYear returns the current position in the year, 0 <= d < YearDays.
func (h *Hive) DayOfYear() float64 {
	return math.Mod(float64(StartDayOfYear)+float64(h.Tick)/DayTicks, YearDays)
}

// Day returns the number of whole days simulated so far.
func (h *Hive) Day() int { return h.Tick / DayTicks }

// TimeOfDay returns 0..1, 0 = midnight, 0.5 = noon.
func (h *Hive) TimeOfDay() float64 { return float64(h.Tick%DayTicks) / DayTicks }

// Season returns 0 spring, 1 summer, 2 autumn, 3 winter.
func (h *Hive) Season() int {
	d := h.DayOfYear()
	switch {
	case d < 92:
		return 0
	case d < 184:
		return 1
	case d < 275:
		return 2
	}
	return 3
}

// dayLength is the fraction of the day that is light: 8 hours at the
// winter solstice, 16.5 at midsummer (equinox on 21 March, day 20).
func (h *Hive) dayLength() float64 {
	return 0.51 + 0.18*math.Sin(2*math.Pi*(h.DayOfYear()-20)/YearDays)
}

// env caches this tick's weather so the many per-bee queries are cheap.
type env struct {
	light, temp, forage, broodRatio float64
	canFly, cold                    bool
}

func (h *Hive) updateEnv() {
	e := &h.env
	e.light = h.computeDaylight()
	e.temp = interp(temperatureCurve, h.DayOfYear()) + 5*math.Sin(2*math.Pi*(h.TimeOfDay()-0.3))
	e.forage = interp(forageCurve, h.DayOfYear())
	e.canFly = !h.rainy && e.light > 0.5 && e.temp >= 10
	e.cold = interp(temperatureCurve, h.DayOfYear()) < 7.5
	e.broodRatio = float64(h.counts.Eggs+h.counts.Larvae) / math.Max(1, float64(h.counts.Workers))
}

// Daylight returns 0 at night, 1 in full daylight, with a short dawn/dusk.
func (h *Hive) Daylight() float64 { return h.env.light }

func (h *Hive) computeDaylight() float64 {
	half := h.dayLength() / 2
	edge := half - math.Abs(h.TimeOfDay()-0.5)
	return clampF(edge/0.04+0.5, 0, 1)
}

// Temperature returns the outside temperature, including a day/night swing.
func (h *Hive) Temperature() float64 { return h.env.temp }

// Raining reports today's weather.
func (h *Hive) Raining() bool { return h.rainy }

// ForageAvailability is how much nectar/pollen is out there right now (0..1).
func (h *Hive) ForageAvailability() float64 { return h.env.forage }

// CanFly reports whether conditions allow bees to leave the hive.
func (h *Hive) CanFly() bool { return h.env.canFly }

// Cold reports whether the colony should form a winter cluster.
func (h *Hive) Cold() bool { return h.env.cold }

// inDroneSeason: drones are reared from April to mid-August.
func (h *Hive) inDroneSeason() bool {
	d := h.DayOfYear()
	return d >= 30 && d <= 165
}

// inSwarmSeason: mid-April to the end of June.
func (h *Hive) inSwarmSeason() bool {
	d := h.DayOfYear()
	return d >= 45 && d <= 122
}

// lateSeason: from mid-August the colony winds down for winter.
func (h *Hive) lateSeason() bool {
	return h.DayOfYear() > 165
}

// matingWeather: queens and drones only fly for mating when it is warm.
func (h *Hive) matingWeather() bool {
	d := h.DayOfYear()
	return h.CanFly() && d >= 30 && d <= 215
}

func (h *Hive) rollWeather() {
	h.rainy = h.rng.Float64() < interp(rainCurve, h.DayOfYear())
}
