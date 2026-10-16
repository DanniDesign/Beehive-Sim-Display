//go:build matrix

package render

import (
	"beehive-sim2/simulation"
	"image"
)

func DrawHiveToImage(img *image.RGBA, h *simulation.Hive) {
	Draw(img, h)
}
