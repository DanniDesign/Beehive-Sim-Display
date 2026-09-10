package render

import "image/color"

var (
	ColorBg    = color.RGBA{0, 0, 0, 255}     // Pure Black (LEDs OFF)
	ColorVoid  = color.RGBA{0, 0, 0, 255}     // Pure Black (LEDs OFF)
	ColorExit  = color.RGBA{0, 255, 255, 255} // Cyan (High Visibility Entrance)
	ColorHoney = color.RGBA{255, 200, 0, 255} // Pure Bright Amber/Gold
	ColorBrood = color.RGBA{0, 255, 128, 255} // Bright Neon Spring Green (Egg/Brood)
)
