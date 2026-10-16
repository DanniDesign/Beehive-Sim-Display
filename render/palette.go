package render

import "image/color"

// Colours were tuned for a HUB75 panel behind a diffuser: see the
// separation tests in separation_test.go and `gifgen out/ preview`.
var (
	ColorBg     = color.RGBA{0, 0, 0, 255}       // Pure Black (LEDs OFF)
	ColorVoid   = color.RGBA{0, 0, 0, 255}       // Pure Black (LEDs OFF)
	ColorExit   = color.RGBA{0, 255, 255, 255}   // Cyan (High Visibility Entrance)
	ColorHoney  = color.RGBA{235, 175, 0, 255}   // Amber (capped honey); a touch below full so large honey areas don't bloom
	ColorNectar = color.RGBA{130, 190, 30, 255}  // Greenish yellow (unripe nectar), set apart from honey by hue
	ColorPollen = color.RGBA{240, 80, 0, 255}    // Orange bee bread, between honey amber and the red retinue
	ColorEgg    = color.RGBA{170, 170, 150, 255} // Off-white eggs, bright enough to show among capped brood
	ColorBrood  = color.RGBA{0, 255, 128, 255}   // Bright Neon Spring Green (open brood / larvae)
	ColorCapped = color.RGBA{140, 60, 0, 255}    // Brown wax cappings over pupae
	ColorRoyal  = color.RGBA{150, 0, 120, 255}   // Queen cell
	ColorComb   = color.RGBA{45, 27, 4, 255}     // Empty wax comb (dim, but must survive the diffuser)
	ColorDirty  = color.RGBA{65, 35, 4, 255}     // Used cell waiting to be cleaned
)
