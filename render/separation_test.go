package render

import (
	"image/color"
	"math"
	"sort"
	"testing"

	"beehive-sim2/simulation"
)

// Colours on the panel have to stay tellable apart once the diffuser has
// blurred each pixel into its surroundings. These tests model one pixel of
// each key colour sitting on empty comb, behind a diffuser of the default
// width, at full daylight and at night, and measure perceptual distances
// (CIELAB delta E) between the light that reaches the viewer.

const testSigma = 0.8 // diffuser width in LED pitches; stronger than the real one, for margin

type keyColour struct {
	name string
	c    color.RGBA
	bee  bool // bees and the queen dim less at night than the comb does
}

func keyColours() []keyColour {
	bee := func(r simulation.Role, t simulation.Task, load simulation.CellState) color.RGBA {
		b := &simulation.Bee{Role: r, Task: t, Load: load}
		return dim(b.GetColor(0), 1)
	}
	return []keyColour{
		{"queen", bee(simulation.Queen, 0, 0), true},
		{"nurse feeding queen", bee(simulation.Nurse, simulation.FeedQueen, 0), true},
		{"young bee", bee(simulation.Nurse, 0, 0), true},
		{"older bee", bee(simulation.Forager, 0, 0), true},
		{"nectar carrier", bee(simulation.Forager, 0, simulation.Nectar), true},
		{"pollen carrier", bee(simulation.Forager, 0, simulation.Pollen), true},
		{"drone", bee(simulation.Drone, 0, 0), true},
		{"egg", ColorEgg, false},
		{"larva", scale(ColorBrood, 0.7), false},
		{"capped brood", ColorCapped, false},
		{"queen cell", ColorRoyal, false},
		{"nectar", ColorNectar, false},
		{"honey", ColorHoney, false},
		{"pollen", ColorPollen, false},
		{"entrance", ColorExit, true},
	}
}

// seen is the linear light reaching the viewer from a pixel of colour c
// surrounded by background bg, at the given daylight (1 day, 0 night).
func seen(k keyColour, bg color.RGBA, light float64) [3]float64 {
	cellDim := CellDim(light)
	beeDim := BeeDim(light)
	c := k.c
	switch {
	case k.name == "queen" || k.name == "entrance":
		// drawn at full brightness day and night
	case k.bee:
		c = scale(c, beeDim)
	default:
		c = scale(c, cellDim)
	}
	bg = scale(bg, cellDim)
	w := DiffuserCenterWeight(testSigma)
	return [3]float64{
		w*PanelLight(c.R) + (1-w)*PanelLight(bg.R),
		w*PanelLight(c.G) + (1-w)*PanelLight(bg.G),
		w*PanelLight(c.B) + (1-w)*PanelLight(bg.B),
	}
}

// lab converts linear RGB (sRGB primaries, D65) to CIELAB.
func lab(l [3]float64) [3]float64 {
	x := 0.4124*l[0] + 0.3576*l[1] + 0.1805*l[2]
	y := 0.2126*l[0] + 0.7152*l[1] + 0.0722*l[2]
	z := 0.0193*l[0] + 0.1192*l[1] + 0.9505*l[2]
	f := func(t float64) float64 {
		if t > 216.0/24389 {
			return math.Cbrt(t)
		}
		return (24389.0/27*t + 16) / 116
	}
	fx, fy, fz := f(x/0.9505), f(y), f(z/1.089)
	return [3]float64{116*fy - 16, 500 * (fx - fy), 200 * (fy - fz)}
}

func deltaE(a, b [3]float64) float64 {
	la, lb := lab(a), lab(b)
	return math.Sqrt((la[0]-lb[0])*(la[0]-lb[0]) + (la[1]-lb[1])*(la[1]-lb[1]) + (la[2]-lb[2])*(la[2]-lb[2]))
}

// sameMaterial pairs are meant to look alike: a bee carrying pollen shows
// the colour of pollen.
var sameMaterial = map[[2]string]bool{
	{"pollen carrier", "pollen"}: true,
	{"nectar carrier", "nectar"}: true,
}

type pairDist struct {
	a, b string
	d    float64
}

func separations(light float64) (pairs []pairDist, vsComb []pairDist) {
	ks := keyColours()
	for i := range ks {
		si := seen(ks[i], ColorComb, light)
		for j := i + 1; j < len(ks); j++ {
			if sameMaterial[[2]string{ks[i].name, ks[j].name}] {
				continue
			}
			pairs = append(pairs, pairDist{ks[i].name, ks[j].name, deltaE(si, seen(ks[j], ColorComb, light))})
		}
		combOnly := seen(keyColour{name: "comb", c: ColorComb}, ColorComb, light)
		vsComb = append(vsComb, pairDist{ks[i].name, "empty comb", deltaE(si, combOnly)})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].d < pairs[j].d })
	sort.Slice(vsComb, func(i, j int) bool { return vsComb[i].d < vsComb[j].d })
	return pairs, vsComb
}

// Thresholds (CIELAB delta E after the diffuser). Set by measuring the
// original palette: everything the preview showed as hard to read fell
// below them (nectar in honey 6.7 by day / 2.5 at night, empty comb against
// the dark cavity 5.3 / 1.8), everything that read clearly was above.
const (
	minPairDay      = 8.0
	minPairNight    = 5.0
	minCombDay      = 8.0
	minCombNight    = 5.0
	minContextDay   = 8.0
	minContextNight = 4.0
)

func TestColourSeparation(t *testing.T) {
	for _, tc := range []struct {
		when           string
		light          float64
		minPair, minBg float64
	}{
		{"day", 1, minPairDay, minCombDay},
		{"night", 0, minPairNight, minCombNight},
	} {
		pairs, vsComb := separations(tc.light)
		for i, p := range pairs {
			if i < 6 {
				t.Logf("%s: closest pairs %-20s vs %-20s dE %.1f", tc.when, p.a, p.b, p.d)
			}
			if p.d < tc.minPair {
				t.Errorf("%s: %s and %s are too alike behind the diffuser (dE %.1f < %.1f)", tc.when, p.a, p.b, p.d, tc.minPair)
			}
		}
		for i, p := range vsComb {
			if i < 3 {
				t.Logf("%s: least visible on comb %-20s dE %.1f", tc.when, p.a, p.d)
			}
			if p.d < tc.minBg {
				t.Errorf("%s: %s hardly shows on empty comb (dE %.1f < %.1f)", tc.when, p.a, p.d, tc.minBg)
			}
		}
	}
}

// contexts: where each key colour actually appears on the display, and
// what it has to stand out from there.
var contexts = []struct{ target, surround string }{
	{"queen", "young bee"},               // the queen walks inside a crowd of nurses
	{"nurse feeding queen", "young bee"}, // her retinue, in the same crowd
	{"drone", "young bee"},
	{"drone", "honey"},
	{"nectar carrier", "older bee"},
	{"pollen carrier", "older bee"},
	{"older bee", "young bee"},
	{"entrance", "void"}, // a single pixel on the black frame edge
	{"egg", "capped brood"},
	{"larva", "capped brood"},
	{"queen cell", "capped brood"},
	{"nectar", "honey"},
	{"pollen", "honey"},
	{"empty comb", "void"}, // the comb's outline against the dark cavity
}

func colourNamed(name string) keyColour {
	switch name {
	case "void":
		return keyColour{name: name, c: ColorVoid}
	case "empty comb":
		return keyColour{name: name, c: ColorComb}
	}
	for _, k := range keyColours() {
		if k.name == name {
			return k
		}
	}
	panic("unknown colour " + name)
}

// contrastIn is how much a pixel of target changes the look of a patch of
// surround, once diffused.
func contrastIn(target, surround string, light float64) float64 {
	t, s := colourNamed(target), colourNamed(surround)
	return deltaE(seenOn(t, s, light), seenOn(s, s, light))
}

func dimmedAs(k keyColour, light float64) color.RGBA {
	switch {
	case k.name == "queen" || k.name == "entrance" || k.name == "void":
		return k.c
	case k.bee:
		return scale(k.c, BeeDim(light))
	}
	return scale(k.c, CellDim(light))
}

// seenOn: pixel k surrounded by pixels of s, both drawn with their own
// day/night dimming, through the diffuser.
func seenOn(k, s keyColour, light float64) [3]float64 {
	c, bg := dimmedAs(k, light), dimmedAs(s, light)
	w := DiffuserCenterWeight(testSigma)
	return [3]float64{
		w*PanelLight(c.R) + (1-w)*PanelLight(bg.R),
		w*PanelLight(c.G) + (1-w)*PanelLight(bg.G),
		w*PanelLight(c.B) + (1-w)*PanelLight(bg.B),
	}
}

func TestContrastInContext(t *testing.T) {
	for _, tc := range []struct {
		when  string
		light float64
		min   float64
	}{{"day", 1, minContextDay}, {"night", 0, minContextNight}} {
		for _, cx := range contexts {
			d := contrastIn(cx.target, cx.surround, tc.light)
			t.Logf("%-5s %-20s in %-14s dE %.1f", tc.when, cx.target, cx.surround, d)
			if d < tc.min {
				t.Errorf("%s: %s doesn't stand out in %s behind the diffuser (dE %.1f < %.1f)", tc.when, cx.target, cx.surround, d, tc.min)
			}
		}
	}
}

func TestPanelLight(t *testing.T) {
	if PanelLight(0) != 0 || PanelLight(255) != 1 {
		t.Fatalf("panel light should run from 0 to 1, got %v..%v", PanelLight(0), PanelLight(255))
	}
	for c := 1; c < 256; c++ {
		if PanelLight(uint8(c)) < PanelLight(uint8(c-1)) {
			t.Fatalf("panel light not monotonic at %d", c)
		}
	}
	if w := DiffuserCenterWeight(testSigma); w <= 0 || w >= 1 {
		t.Fatalf("diffuser centre weight %v out of range", w)
	}
}
