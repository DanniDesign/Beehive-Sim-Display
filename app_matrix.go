//go:build matrix

package main

import (
	"image"
	"log"
	"time"

	"beehive-sim2/render"
	"beehive-sim2/simulation"

	"github.com/mcuadros/go-rpi-rgb-led-matrix"
)

func RunApp(hive *simulation.Hive, cfg Config) error {
	config := &rgbmatrix.HardwareConfig{
		Rows:            cfg.Height,
		Cols:            cfg.Width,
		ChainLength:     1,
		Parallel:        1,
		PWMBits:         11,
		Brightness:      60,
		HardwareMapping: "adafruit-hat",
	}

	m, err := rgbmatrix.NewRGBLedMatrix(config)
	if err != nil {
		return err
	}
	defer m.Close()

	// Create canvas buffer for rendering frames
	canvas := rgbmatrix.NewCanvas(m)
	frameImg := image.NewRGBA(image.Rect(0, 0, cfg.Width, cfg.Height))

	ticker := time.NewTicker(time.Second / time.Duration(cfg.FPS))
	defer ticker.Stop()

	log.Println("Starting Matrix Simulation Loop on Raspberry Pi...")

	for range ticker.C {
		hive.Update()

		// Render simulation state to frame buffer image
		render.DrawHiveToImage(frameImg, hive)

		// Copy image pixels to canvas
		bounds := frameImg.Bounds()
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				c := frameImg.At(x, y)
				canvas.Set(x, y, c)
			}
		}

		// Flush canvas to physical display
		if err := canvas.Render(); err != nil {
			log.Printf("Render error: %v", err)
		}
	}

	return nil
}