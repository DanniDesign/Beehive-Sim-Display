//go:build !matrix

package render

import (
	"image"

	"github.com/DanniDesign/Beehive-Sim-Display/simulation"

	"github.com/hajimehoshi/ebiten/v2"
)

type DesktopRenderer struct {
	Width        int
	Height       int
	scale        int
	currentImage image.Image
}

func NewRenderer(Width, Height, scale int) (Renderer, error) {
	return &DesktopRenderer{
		Width:  Width,
		Height: Height,
		scale:  scale,
	}, nil
}

func (r *DesktopRenderer) Init() error {
	ebiten.SetWindowSize(r.Width*r.scale, r.Height*r.scale)
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
	return r.Width * r.scale, r.Height * r.scale
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

	for _, bee := range h.GetBees() {
		state := bee.State

		if state == simulation.StateOutside {
			continue
		}
		screen.Set(bee.X, bee.Y, bee.GetColor(h.Tick))
	}
}
