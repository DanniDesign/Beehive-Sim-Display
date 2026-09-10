//go:build !matrix

package main

import (
	"beehive-sim2/render"
	"beehive-sim2/simulation"

	"github.com/hajimehoshi/ebiten/v2"
)

type DesktopApp struct {
	hive      *simulation.Hive
	tickCount int
	cfg       Config
}

func (a *DesktopApp) Update() error {
	a.tickCount++
	if a.tickCount%10 == 0 {
		a.hive.Update()
	}
	return nil
}

func (a *DesktopApp) Draw(screen *ebiten.Image) {
	render.DrawHive(screen, a.hive)
}

func (a *DesktopApp) Layout(outsideWidth, outsideHeight int) (int, int) {
	return a.cfg.Width, a.cfg.Height
}

func RunApp(hive *simulation.Hive, cfg Config) error {
	ebiten.SetWindowSize(cfg.Width*15, cfg.Height*15)
	ebiten.SetWindowTitle("Beehive Simulation")

	app := &DesktopApp{
		hive: hive,
		cfg:  cfg,
	}
	return ebiten.RunGame(app)
}
