//go:build matrix

package render

import (
	"beehive-sim2/simulation"
	"image"
	"image/draw"
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

	img.Set(h.ExitX, h.ExitY, ColorExit)

	for _, bee := range h.Bees() {
		if bee.IsOutside {
			continue
		}
		img.Set(bee.X, bee.Y, bee.GetColor(h.Tick))
	}
}
