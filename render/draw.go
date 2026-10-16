package render

import (
	"image/color"

	"beehive-sim2/simulation"
)

// Setter is anything pixels can be written to (ebiten.Image, image.RGBA).
type Setter interface {
	Set(x, y int, c color.Color)
}

// Draw renders the comb, the entrance and the bees. Cells are dimmed at
// night so the display follows the simulated day.
func Draw(dst Setter, h *simulation.Hive) {
	light := 0.35 + 0.65*h.Daylight()
	for y := range h.Grid {
		for x := range h.Grid[y] {
			dst.Set(x, y, dim(cellColor(h, x, y), light))
		}
	}
	dst.Set(h.ExitX, h.ExitY, ColorExit)

	var queen *simulation.Bee
	for _, b := range h.Bees() {
		if b.IsOutside {
			continue
		}
		if b.Role == simulation.Queen {
			queen = b
			continue
		}
		dst.Set(b.X, b.Y, dim(b.GetColor(h.Tick), 0.5+0.5*light))
	}
	if queen != nil {
		dst.Set(queen.X, queen.Y, queen.GetColor(h.Tick)) // always on top
	}
}

func cellColor(h *simulation.Hive, x, y int) color.RGBA {
	c := &h.Grid[y][x]
	switch c.State {
	case simulation.Honey:
		return scale(ColorHoney, 0.45+0.55*float64(c.Amount))
	case simulation.Nectar:
		return scale(ColorNectar, 0.4+0.6*float64(c.Amount))
	case simulation.Pollen:
		return scale(ColorPollen, 0.5+0.5*float64(c.Amount))
	case simulation.Egg:
		if c.Kind == simulation.QueenBrood {
			return ColorRoyal
		}
		return ColorEgg
	case simulation.Larva:
		if c.Kind == simulation.QueenBrood {
			return ColorRoyal
		}
		// Larvae grow from a speck into a fat grub that fills the cell.
		return scale(ColorBrood, 0.35+0.65*h.BroodProgress(x, y))
	case simulation.Pupa:
		if c.Kind == simulation.QueenBrood {
			return ColorRoyal
		}
		if c.Kind == simulation.DroneBrood {
			return scale(ColorCapped, 1.4) // domed drone cappings
		}
		return ColorCapped
	}
	if c.Dirty {
		return ColorDirty
	}
	if c.Built {
		return ColorComb
	}
	return ColorVoid
}

func scale(c color.RGBA, f float64) color.RGBA {
	m := func(v uint8) uint8 {
		r := float64(v) * f
		if r > 255 {
			return 255
		}
		return uint8(r)
	}
	return color.RGBA{m(c.R), m(c.G), m(c.B), 255}
}

func dim(c color.Color, f float64) color.RGBA {
	r, g, b, _ := c.RGBA()
	return scale(color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255}, f)
}
