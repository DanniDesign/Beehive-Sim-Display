package render

import "image/color"

var (
	ColorBg     = color.RGBA{0, 0, 0, 255}     // Pure Black (LEDs OFF)
	ColorVoid   = color.RGBA{0, 0, 0, 255}     // Pure Black (LEDs OFF)
	ColorExit   = color.RGBA{0, 255, 255, 255} // Cyan (High Visibility Entrance)
	ColorHoney  = color.RGBA{255, 200, 0, 255} // Pure Bright Amber/Gold (capped honey)
	ColorNectar = color.RGBA{150, 120, 0, 255} // Pale, dimmer gold (unripe nectar)
	ColorPollen = color.RGBA{230, 60, 0, 255}  // Orange-red bee bread
	ColorEgg    = color.RGBA{90, 90, 70, 255}  // Tiny off-white eggs
	ColorBrood  = color.RGBA{0, 255, 128, 255} // Bright Neon Spring Green (open brood / larvae)
	ColorCapped = color.RGBA{110, 45, 0, 255}  // Brown wax cappings over pupae
	ColorRoyal  = color.RGBA{150, 0, 120, 255} // Queen cell
	ColorComb   = color.RGBA{24, 15, 2, 255}   // Empty wax comb (faint, shows the comb's shape)
	ColorDirty  = color.RGBA{38, 20, 2, 255}   // Used cell waiting to be cleaned
)
