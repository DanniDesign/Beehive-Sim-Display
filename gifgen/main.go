//go:build matrix

// Renders GIFs of the simulation exactly as the LED panel would show it.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"os"
	"sort"
	"strings"

	"beehive-sim2/render"
	"beehive-sim2/simulation"
)

const (
	W, H = 64, 32
	cell = 6
	capH = 26
	outW = W * cell
	outH = H*cell + capH
)

var font = map[rune][5]string{
	'A': {"010", "101", "111", "101", "101"}, 'B': {"110", "101", "110", "101", "110"},
	'C': {"011", "100", "100", "100", "011"}, 'D': {"110", "101", "101", "101", "110"},
	'E': {"111", "100", "110", "100", "111"}, 'F': {"111", "100", "110", "100", "100"},
	'G': {"011", "100", "101", "101", "011"}, 'H': {"101", "101", "111", "101", "101"},
	'I': {"111", "010", "010", "010", "111"}, 'J': {"001", "001", "001", "101", "010"},
	'K': {"101", "101", "110", "101", "101"}, 'L': {"100", "100", "100", "100", "111"},
	'M': {"101", "111", "111", "101", "101"}, 'N': {"110", "101", "101", "101", "101"},
	'O': {"010", "101", "101", "101", "010"}, 'P': {"110", "101", "110", "100", "100"},
	'Q': {"010", "101", "101", "110", "011"}, 'R': {"110", "101", "110", "101", "101"},
	'S': {"011", "100", "010", "001", "110"}, 'T': {"111", "010", "010", "010", "010"},
	'U': {"101", "101", "101", "101", "111"}, 'V': {"101", "101", "101", "101", "010"},
	'W': {"101", "101", "111", "111", "101"}, 'X': {"101", "101", "010", "101", "101"},
	'Y': {"101", "101", "010", "010", "010"}, 'Z': {"111", "001", "010", "100", "111"},
	'0': {"111", "101", "101", "101", "111"}, '1': {"010", "110", "010", "010", "111"},
	'2': {"110", "001", "010", "100", "111"}, '3': {"110", "001", "010", "001", "110"},
	'4': {"101", "101", "111", "001", "001"}, '5': {"111", "100", "110", "001", "110"},
	'6': {"011", "100", "111", "101", "111"}, '7': {"111", "001", "010", "010", "010"},
	'8': {"111", "101", "111", "101", "111"}, '9': {"111", "101", "111", "001", "110"},
	' ': {"000", "000", "000", "000", "000"}, ':': {"000", "010", "000", "010", "000"},
	'!': {"010", "010", "010", "000", "010"}, '-': {"000", "000", "111", "000", "000"},
	'.': {"000", "000", "000", "000", "010"}, '/': {"001", "001", "010", "100", "100"},
}

func text(img *image.RGBA, s string, x, y, sc int, c color.RGBA) {
	for _, r := range strings.ToUpper(s) {
		g, ok := font[r]
		if !ok {
			g = font[' ']
		}
		for gy, row := range g {
			for gx, ch := range row {
				if ch == '1' {
					for dy := 0; dy < sc; dy++ {
						for dx := 0; dx < sc; dx++ {
							img.SetRGBA(x+gx*sc+dx, y+gy*sc+dy, c)
						}
					}
				}
			}
		}
		x += 4 * sc
	}
}

var months = []string{"MAR", "APR", "MAY", "JUN", "JUL", "AUG", "SEP", "OCT", "NOV", "DEC", "JAN", "FEB"}
var monthLen = []int{31, 30, 31, 30, 31, 31, 30, 31, 30, 31, 31, 28}

func date(doy float64) string {
	d := int(doy)
	for i, l := range monthLen {
		if d < l {
			return fmt.Sprintf("%d %s", d+1, months[i])
		}
		d -= l
	}
	return "28 FEB"
}

func frame(h *simulation.Hive, banner string, clock bool) *image.RGBA {
	small := image.NewRGBA(image.Rect(0, 0, W, H))
	render.Draw(small, h)
	big := image.NewRGBA(image.Rect(0, 0, outW, outH))
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			c := small.RGBAAt(x, y)
			if c.R|c.G|c.B == 0 {
				c = color.RGBA{14, 14, 14, 255} // unlit LED
			}
			for dy := 0; dy < cell-1; dy++ {
				for dx := 0; dx < cell-1; dx++ {
					big.SetRGBA(x*cell+dx, y*cell+dy, c)
				}
			}
		}
	}
	s := h.Snapshot()
	grey := color.RGBA{170, 170, 170, 255}
	y0 := H*cell + 4
	left := date(s.DayOfYear)
	if clock {
		m := int(h.TimeOfDay() * 24 * 60)
		left += fmt.Sprintf(" %02d:%02d", m/60, m%60)
	}
	text(big, left, 4, y0, 2, grey)
	info := fmt.Sprintf("BEES %d  HONEY %d", s.Counts.Workers+s.Counts.Drones, int(s.Honey+s.Nectar))
	if s.Dead {
		info = "EMPTY HIVE"
	}
	text(big, info, outW-4-len(info)*8, y0, 2, grey)
	if banner != "" {
		text(big, banner, (outW-len(banner)*8)/2, y0+12, 2, color.RGBA{255, 220, 60, 255})
	} else {
		seasons := []string{"SPRING", "SUMMER", "AUTUMN", "WINTER"}
		st := seasons[s.Season]
		if h.Raining() {
			st += " - RAIN"
		}
		text(big, st, 4, y0+12, 2, color.RGBA{90, 90, 90, 255})
	}
	return big
}

// globalPalette keeps the 256 most common colours over all frames, so
// every frame shares one palette (decoders handle that most reliably).
func globalPalette(frames []*image.RGBA) color.Palette {
	counts := map[color.RGBA]int{}
	for _, img := range frames {
		for i := 0; i < len(img.Pix); i += 4 {
			counts[color.RGBA{img.Pix[i], img.Pix[i+1], img.Pix[i+2], 255}]++
		}
	}
	cols := make([]color.RGBA, 0, len(counts))
	for c := range counts {
		cols = append(cols, c)
	}
	sort.Slice(cols, func(i, j int) bool {
		if counts[cols[i]] != counts[cols[j]] {
			return counts[cols[i]] > counts[cols[j]]
		}
		a, b := cols[i], cols[j]
		return uint32(a.R)<<16|uint32(a.G)<<8|uint32(a.B) < uint32(b.R)<<16|uint32(b.G)<<8|uint32(b.B)
	})
	if len(cols) > 256 {
		cols = cols[:256]
	}
	pal := make(color.Palette, len(cols))
	for i, c := range cols {
		pal[i] = c
	}
	return pal
}

func toPaletted(img *image.RGBA, pal color.Palette, cache map[color.RGBA]uint8) *image.Paletted {
	out := image.NewPaletted(img.Bounds(), pal)
	for i, j := 0, 0; i < len(img.Pix); i, j = i+4, j+1 {
		c := color.RGBA{img.Pix[i], img.Pix[i+1], img.Pix[i+2], 255}
		k, ok := cache[c]
		if !ok {
			k = uint8(pal.Index(c))
			cache[c] = k
		}
		out.Pix[j] = k
	}
	return out
}

func save(name string, frames []*image.RGBA, delay int) {
	pal := globalPalette(frames)
	cache := map[color.RGBA]uint8{}
	g := &gif.GIF{}
	for _, f := range frames {
		g.Image = append(g.Image, toPaletted(f, pal, cache))
		g.Delay = append(g.Delay, delay)
		g.Disposal = append(g.Disposal, gif.DisposalNone)
	}
	g.Delay[len(g.Delay)-1] = 300
	f, err := os.Create(name)
	if err != nil {
		panic(err)
	}
	if err := gif.EncodeAll(f, g); err != nil {
		panic(err)
	}
	f.Close()
	st, _ := os.Stat(name)
	fmt.Printf("%s: %d frames, %.1f MB\n", name, len(frames), float64(st.Size())/1e6)
}

func newHive(seed int64) *simulation.Hive {
	return simulation.NewHiveWithSeed(seed, 120, W, H, simulation.DefaultParams())
}

// events turns changes in the cumulative counters into a banner.
func events(prev, cur simulation.Counters) string {
	switch {
	case cur.Recolonizations > prev.Recolonizations:
		return "A WILD SWARM MOVES IN!"
	case cur.Collapses > prev.Collapses:
		return "THE COLONY HAS DIED"
	case cur.Swarms > prev.Swarms:
		return "SWARM! OLD QUEEN LEAVES"
	case cur.QueensLostMating > prev.QueensLostMating:
		return "QUEEN LOST ON MATING FLIGHT"
	case cur.QueensDied > prev.QueensDied:
		return "THE QUEEN HAS DIED"
	case cur.QueensMated > prev.QueensMated:
		return "NEW QUEEN MATED"
	case cur.QueensEmerged > prev.QueensEmerged:
		return "VIRGIN QUEEN EMERGES"
	case cur.EmergencyQueenCells > prev.EmergencyQueenCells:
		return "EMERGENCY QUEEN CELLS"
	case cur.SwarmPreparations > prev.SwarmPreparations:
		return "QUEEN CELLS BUILT"
	case cur.DroneLayingQueens > prev.DroneLayingQueens:
		return "QUEEN NEVER MATED"
	}
	return ""
}

// timelapse: one frame per simulated day (at noon), with event banners.
func timelapse(name string, seed int64, days int) {
	h := newHive(seed)
	var frames []*image.RGBA
	prev := h.Snapshot().Stats
	banner, hold := "", 0
	for d := 0; d < days; d++ {
		for i := 0; i < simulation.DayTicks; i++ {
			h.Update()
			if i == simulation.DayTicks/2 {
				break
			}
		}
		cur := h.Snapshot().Stats
		if e := events(prev, cur); e != "" {
			banner, hold = e, 12
		}
		prev = cur
		b := ""
		if hold > 0 {
			b = banner
			hold--
		}
		frames = append(frames, frame(h, b, false))
		for i := simulation.DayTicks/2 + 1; i < simulation.DayTicks; i++ {
			h.Update()
			if e := events(prev, h.Snapshot().Stats); e != "" && hold == 0 {
				banner, hold = e, 12
				prev = h.Snapshot().Stats
			}
		}
		prev = h.Snapshot().Stats
	}
	save(name, frames, 8)
}

// realtime: every tick, as the panel shows it at 5 FPS.
func realtime(name string, seed int64, startTick, ticks int, banner string) {
	h := newHive(seed)
	for h.Tick < startTick {
		h.Update()
	}
	var frames []*image.RGBA
	for i := 0; i < ticks; i++ {
		h.Update()
		frames = append(frames, frame(h, banner, true))
	}
	save(name, frames, 20)
}

func findSwarmTick(seed int64) int {
	h := newHive(seed)
	for h.Snapshot().Stats.Swarms == 0 {
		h.Update()
	}
	return h.Tick
}

func main() {
	out := os.Args[1]
	day := simulation.DayTicks
	// Seconds-long, real speed clips (one frame per tick).
	realtime(out+"/1-summer-morning-realtime.gif", 3, (95-simulation.StartDayOfYear)*day+day*38/100, 200, "")
	sw := findSwarmTick(3)
	realtime(out+"/2-swarm-departing-realtime.gif", 3, sw-15, 220, "")
	// Whole-year timelapses, one frame per day.
	timelapse(out+"/3-seed3-year-swarms-and-survives.gif", 3, 420)
	timelapse(out+"/4-seed2-starves-then-recolonised.gif", 2, 548)
	timelapse(out+"/5-seed11-queen-lost-colony-dies.gif", 11, 548)
}
