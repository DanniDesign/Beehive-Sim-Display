//go:build !matrix

package render

import (
	"beehive-sim2/simulation"
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

type DesktopRenderer struct {
	width        int
	height       int
	scale        int
	currentImage image.Image
}

func NewRenderer(width, height, scale int) (Renderer, error) {
	return &DesktopRenderer{
		width:  width,
		height: height,
		scale:  scale,
	}, nil
}

func (r *DesktopRenderer) Init() error {
	ebiten.SetWindowSize(r.width*r.scale, r.height*r.scale)
	ebiten.SetWindowTitle("Beehive Simulation Preview")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	return nil
}

func (r *DesktopRenderer) Present(img image.Image) error {
	r.currentImage = img
	return nil
}

func (r *DesktopRenderer) Close() error {
	return nil
}

func (r *DesktopRenderer) Update() error {
	return nil
}

func (r *DesktopRenderer) Draw(screen *ebiten.Image) {
	if r.currentImage != nil {
		ebImg := ebiten.NewImageFromImage(r.currentImage)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(float64(r.scale), float64(r.scale))
		screen.DrawImage(ebImg, op)
	}
}

func (r *DesktopRenderer) Layout(outsideWidth, outsideHeight int) (int, int) {
	return r.width * r.scale, r.height * r.scale
}

var CellSize = 1

func DrawHive(screen *ebiten.Image, h *simulation.Hive) {
	screen.Fill(ColorVoid)

	for y := range h.Grid {
		for x := range h.Grid[y] {
			cell := h.Grid[y][x]
			switch cell.State {
			case simulation.Honey:
				screen.Set(x, y, ColorHoney)
			case simulation.Egg:
				screen.Set(x, y, ColorBrood)
			}
		}
	}

	screen.Set(h.ExitX, h.ExitY, ColorExit)

	for _, bee := range h.Bees() {
		if bee.IsOutside {
			continue
		}
		screen.Set(bee.X, bee.Y, bee.GetColor(h.Tick))
	}
}
