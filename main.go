package main

import (
	"beehive-sim2/simulation"
	"log"
)

type Config struct {
	Width  int
	Height int
	FPS    int
}

func main() {
	cfg := Config{
		Width:  64,
		Height: 32,
		FPS:    5,
	}

	hive := simulation.NewHive(4, cfg.Width, cfg.Height)

	if err := RunApp(hive, cfg); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}
