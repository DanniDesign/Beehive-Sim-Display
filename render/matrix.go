//go:build matrix

package render

import (
	"image"
	"image/draw"

	"github.com/DanniDesign/Beehive-Sim-Display/simulation"
)

func DrawHiveToImage(img *image.RGBA, h *simulation.Hive) {
	draw.Draw(img, img.Bounds(), &image.Uniform{ColorVoid}, image.Point{}, draw.Src)

	for y := range h.Grid {
		for x := range h.Grid[y] {
			cell := h.Grid[y][x]
			switch cell.State {
			case simulation.Honey:
				img.Set(x, y, ColorHoney)
			case simulation.Egg:
				img.Set(x, y, ColorBrood)
			}
		}
	}

	for _, bee := range h.GetBees() {
		state := bee.State
		if state == simulation.StateOutside {
			continue
		}
		img.Set(bee.X, bee.Y, bee.GetColor(h.Tick))
	}
}
