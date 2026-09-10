package render

import (
	"image"
	"image/color"
)

// Renderer defines the contract for both desktop preview and matrix output.
type Renderer interface {
	// Init prepares window or display hardware.
	Init() error

	// Present displays the current frame buffer.
	Present(img image.Image) error

	// Close releases any held resources.
	Close() error
}

// ClearImage creates a solid background image of the specified size.
func ClearImage(bounds image.Rectangle, c color.Color) *image.RGBA {
	img := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}
