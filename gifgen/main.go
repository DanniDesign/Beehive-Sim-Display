//go:build matrix

// Renders GIFs of the simulation exactly as the LED panel would show it.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"sort"
	"strconv"
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
	'.': {"000", "000", "000", "000", "010"}, '+': {"000", "010", "111", "010", "000"}, '/': {"001", "001", "010", "100", "100"},
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

func frame(h *simulation.Hive) *image.RGBA {
	small := image.NewRGBA(image.Rect(0, 0, W, H))
	render.Draw(small, h)
	big := image.NewRGBA(image.Rect(0, 0, outW, outH))
	for i := 3; i < len(big.Pix); i += 4 {
		big.Pix[i] = 255 // opaque black background between the LEDs
	}
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
	m := int(h.TimeOfDay() * 24 * 60)
	text(big, fmt.Sprintf("%s  %02d:%02d", date(h.DayOfYear()), m/60, m%60), 4, H*cell+4, 2, color.RGBA{170, 170, 170, 255})
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

// timelapse: one frame per simulated day, at noon.
func timelapse(name string, seed int64, days int) {
	h := newHive(seed)
	var frames []*image.RGBA
	for d := 0; d < days; d++ {
		for i := 0; i < simulation.DayTicks; i++ {
			h.Update()
			if i == simulation.DayTicks/2 {
				frames = append(frames, frame(h))
			}
		}
	}
	save(name, frames, 8)
}

// realtime: every tick, as the panel shows it at 5 FPS.
func realtime(name string, seed int64, startTick, ticks int) {
	h := newHive(seed)
	for h.Tick < startTick {
		h.Update()
	}
	var frames []*image.RGBA
	for i := 0; i < ticks; i++ {
		h.Update()
		frames = append(frames, frame(h))
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

// gallery: many colonies side by side on the same date, to compare shapes.
func gallery(name string, seeds []int64, doy int) {
	cols := 2
	rows := (len(seeds) + cols - 1) / cols
	img := image.NewRGBA(image.Rect(0, 0, cols*(outW+8), rows*(outH+8)))
	for i, seed := range seeds {
		h := newHive(seed)
		for int(h.DayOfYear()) != doy || h.TimeOfDay() < 0.5 {
			h.Update()
		}
		f := frame(h)
		ox, oy := (i%cols)*(outW+8), (i/cols)*(outH+8)
		for y := 0; y < outH; y++ {
			for x := 0; x < outW; x++ {
				img.Set(ox+x, oy+y, f.At(x, y))
			}
		}
	}
	fh, _ := os.Create(name)
	png.Encode(fh, img)
	fh.Close()
	fmt.Println(name)
}

// legend renders a colour key in the same LED style as the display.
func legend(name string) {
	type entry struct {
		c     color.Color
		label string
	}
	bee := func(r simulation.Role, t simulation.Task, load simulation.CellState) color.Color {
		b := &simulation.Bee{Role: r, Task: t, Load: load}
		return b.GetColor(0)
	}
	groups := []struct {
		title   string
		entries []entry
	}{
		{"BEES", []entry{
			{bee(simulation.Queen, 0, 0), "QUEEN"},
			{bee(simulation.Nurse, simulation.FeedQueen, 0), "NURSE FEEDING THE QUEEN"},
			{bee(simulation.Nurse, 0, 0), "YOUNG BEE - CLEANER OR NURSE"},
			{bee(simulation.Forager, 0, 0), "OLDER BEE - HOUSE BEE OR FORAGER"},
			{bee(simulation.Forager, 0, simulation.Nectar), "CARRYING NECTAR"},
			{bee(simulation.Forager, 0, simulation.Pollen), "CARRYING POLLEN"},
			{color.RGBA{255, 255, 120, 255}, "FLASHING - WAGGLE DANCE"},
			{bee(simulation.Drone, 0, 0), "DRONE - MALE BEE"},
		}},
		{"COMB", []entry{
			{render.ColorEgg, "EGG"},
			{render.ColorBrood, "LARVA - GROWS BRIGHTER"},
			{render.ColorCapped, "CAPPED BROOD - PUPA"},
			{render.ColorRoyal, "QUEEN CELL"},
			{render.ColorNectar, "NECTAR - NOT YET HONEY"},
			{render.ColorHoney, "HONEY"},
			{render.ColorPollen, "POLLEN"},
			{render.ColorComb, "EMPTY WAX COMB"},
			{render.ColorExit, "ENTRANCE"},
		}},
	}
	rowH, sw := 22, 14
	rows := 0
	for _, g := range groups {
		rows += len(g.entries) + 2
	}
	img := image.NewRGBA(image.Rect(0, 0, 420, rows*rowH+8))
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 255
	}
	y := 10
	for _, g := range groups {
		text(img, g.title, 12, y, 2, color.RGBA{255, 220, 60, 255})
		y += rowH + 4
		for _, e := range g.entries {
			r, gg, b, _ := e.c.RGBA()
			c := color.RGBA{uint8(r >> 8), uint8(gg >> 8), uint8(b >> 8), 255}
			for dy := 0; dy < sw; dy++ {
				for dx := 0; dx < sw; dx++ {
					img.SetRGBA(14+dx, y-2+dy, c)
				}
			}
			text(img, e.label, 40, y, 2, color.RGBA{200, 200, 200, 255})
			y += rowH
		}
		y += rowH - 4
	}
	f, _ := os.Create(name)
	png.Encode(f, img)
	f.Close()
	fmt.Println(name)
}

// rawDots draws the frame's colour values straight onto LED dots (this is
// what the earlier GIFs showed, before modelling the panel).
func rawDots(small *image.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, W*cell, H*cell))
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 255
	}
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			c := small.RGBAAt(x, y)
			if c.R|c.G|c.B == 0 {
				c = color.RGBA{14, 14, 14, 255}
			}
			for dy := 0; dy < cell-1; dy++ {
				for dx := 0; dx < cell-1; dx++ {
					img.SetRGBA(x*cell+dx, y*cell+dy, c)
				}
			}
		}
	}
	return img
}

// panelFrames renders one hive at the given day of year, at noon and at
// night, as raw 64x32 frames.
func panelFrames(seed int64, doy int) (day, night *image.RGBA) {
	h := newHive(seed)
	for int(h.DayOfYear()) != doy || h.TimeOfDay() < 0.5 {
		h.Update()
	}
	day = image.NewRGBA(image.Rect(0, 0, W, H))
	render.Draw(day, h)
	for h.TimeOfDay() > 0.04 {
		h.Update()
	}
	night = image.NewRGBA(image.Rect(0, 0, W, H))
	render.Draw(night, h)
	return day, night
}

// contactSheet: rows are rendering stages, columns day and night.
func contactSheet(name string, seed int64, doy int, sigma float64, rows []string) {
	day, night := panelFrames(seed, doy)
	pw, ph, lab, gap := W*cell, H*cell, 18, 10
	img := image.NewRGBA(image.Rect(0, 0, 2*pw+3*gap, len(rows)*(ph+lab+gap)+gap))
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 255
	}
	for r, stage := range rows {
		y0 := gap + r*(ph+lab+gap)
		text(img, stage, gap, y0, 2, color.RGBA{255, 220, 60, 255})
		for c, f := range []*image.RGBA{day, night} {
			var p *image.RGBA
			switch stage {
			case "AS DRAWN":
				p = rawDots(f)
			case "PANEL":
				p = render.PanelPreview(f, cell, 0)
			default:
				p = render.PanelPreview(f, cell, sigma)
			}
			x0 := gap + c*(pw+gap)
			for y := 0; y < ph; y++ {
				for x := 0; x < pw; x++ {
					img.SetRGBA(x0+x, y0+lab+y, p.RGBAAt(x, y))
				}
			}
		}
	}
	text(img, "DAY", gap+pw-30, gap, 2, color.RGBA{150, 150, 150, 255})
	text(img, "NIGHT", 2*gap+2*pw-46, gap, 2, color.RGBA{150, 150, 150, 255})
	f, _ := os.Create(name)
	png.Encode(f, img)
	f.Close()
	fmt.Println(name)
}

func main() {
	out := os.Args[1]
	if len(os.Args) > 2 && os.Args[2] == "preview" {
		sigma := 0.8
		if len(os.Args) > 3 {
			sigma, _ = strconv.ParseFloat(os.Args[3], 64)
		}
		rows := []string{"AS DRAWN", "PANEL", "PANEL + DIFFUSER"}
		if len(os.Args) > 4 {
			rows = os.Args[4:]
		}
		contactSheet(out+"/panel-preview.png", 4, 140, sigma, rows)
		return
	}
	if len(os.Args) > 2 && os.Args[2] == "legend" {
		legend(out + "/colour-key.png")
		return
	}
	if len(os.Args) > 2 && os.Args[2] == "gallery" {
		gallery(out+"/gallery-july.png", []int64{1, 2, 3, 4, 5, 6, 7, 8}, 140)
		gallery(out+"/gallery-january.png", []int64{1, 2, 3, 4, 5, 6, 7, 8}, 320)
		return
	}
	day := simulation.DayTicks
	if len(os.Args) > 2 && os.Args[2] == "organic" {
		timelapse(out+"/hive-4-year.gif", 4, 365)
		timelapse(out+"/hive-5-year.gif", 5, 365)
		timelapse(out+"/hive-7-year.gif", 7, 365)
		return
	}
	// Seconds-long, real speed clips (one frame per tick).
	realtime(out+"/1-summer-morning-realtime.gif", 3, (95-simulation.StartDayOfYear)*day+day*38/100, 200)
	sw := findSwarmTick(3)
	realtime(out+"/2-swarm-departing-realtime.gif", 3, sw-15, 220)
	// Whole-year timelapses, one frame per day.
	timelapse(out+"/3-seed3-year.gif", 3, 420)
	timelapse(out+"/4-seed2-year.gif", 2, 548)
	timelapse(out+"/5-seed11-year.gif", 11, 548)
}
