package render

import (
	"image"
	"image/color"
	"math"
)

// PanelBrightness is the brightness (percent) the HUB75 panel is driven at.
// app_matrix.go passes it to the matrix library; the preview uses it too.
const PanelBrightness = 55

// The rpi-rgb-led-matrix C library always applies CIE1931 luminance
// correction (do_luminance_correct_ defaults to true and the Go binding
// cannot turn it off), mapping each 8-bit channel to an 11-bit PWM level.
// We must not gamma-correct on top of that; instead the preview models the
// library so it shows the light the LEDs actually emit.
const panelBitPlanes = 11

var panelLUT [256]float64

func init() {
	level := func(c int) float64 {
		v := float64(c) * PanelBrightness / 255
		var y float64
		if v <= 8 {
			y = v / 902.3
		} else {
			y = math.Pow((v+16)/116, 3)
		}
		return math.Round(float64(int(1)<<panelBitPlanes-1) * y)
	}
	full := level(255)
	for c := range panelLUT {
		panelLUT[c] = level(c) / full
	}
}

// PanelLight returns the light an LED channel emits for an 8-bit value,
// relative to that channel at full value (0..1, linear light).
func PanelLight(c uint8) float64 { return panelLUT[c] }

// encodeSRGB turns linear light (0..1) into an sRGB byte for viewing the
// preview on an ordinary screen.
func encodeSRGB(l float64) uint8 {
	if l <= 0 {
		return 0
	}
	if l >= 1 {
		return 255
	}
	var s float64
	if l <= 0.0031308 {
		s = 12.92 * l
	} else {
		s = 1.055*math.Pow(l, 1/2.4) - 0.055
	}
	return uint8(math.Round(s * 255))
}

// PanelColor is how an 8-bit colour looks once the panel has shown it,
// re-encoded as sRGB.
func PanelColor(c color.RGBA) color.RGBA {
	return color.RGBA{encodeSRGB(PanelLight(c.R)), encodeSRGB(PanelLight(c.G)), encodeSRGB(PanelLight(c.B)), 255}
}

// DiffuserCenterWeight is the share of one LED's light that stays inside
// its own pixel cell after a Gaussian diffuser of width sigma (in LED
// pitches). The rest spills into the neighbouring cells.
func DiffuserCenterWeight(sigma float64) float64 {
	p := math.Erf(0.5 / (sigma * math.Sqrt2))
	return p * p
}

// PanelPreview renders a frame as seen on the panel, scaled up by cell
// pixels per LED. Without a diffuser each LED is a separate dot; with one
// (sigma > 0, in LED pitches) light is spread with a Gaussian, computed in
// linear light so bright pixels bleed into dark ones the way they do on a
// real diffuser.
func PanelPreview(src *image.RGBA, cell int, sigma float64) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx()*cell, b.Dy()*cell
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	if sigma <= 0 {
		for i := 3; i < len(out.Pix); i += 4 {
			out.Pix[i] = 255
		}
		for y := 0; y < b.Dy(); y++ {
			for x := 0; x < b.Dx(); x++ {
				c := PanelColor(src.RGBAAt(b.Min.X+x, b.Min.Y+y))
				if c.R|c.G|c.B == 0 {
					c = color.RGBA{14, 14, 14, 255} // an unlit LED is still faintly visible
				}
				for dy := 0; dy < cell-1; dy++ {
					for dx := 0; dx < cell-1; dx++ {
						out.SetRGBA(x*cell+dx, y*cell+dy, c)
					}
				}
			}
		}
		return out
	}

	// Each LED lights its whole cell of the diffuser, then the light spreads.
	buf := make([]float64, w*h*3)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c := src.RGBAAt(b.Min.X+x, b.Min.Y+y)
			l := [3]float64{PanelLight(c.R), PanelLight(c.G), PanelLight(c.B)}
			for dy := 0; dy < cell; dy++ {
				for dx := 0; dx < cell; dx++ {
					i := ((y*cell+dy)*w + x*cell + dx) * 3
					buf[i], buf[i+1], buf[i+2] = l[0], l[1], l[2]
				}
			}
		}
	}
	s := sigma * float64(cell)
	r := int(math.Ceil(3 * s))
	kernel := make([]float64, 2*r+1)
	var sum float64
	for i := range kernel {
		d := float64(i - r)
		kernel[i] = math.Exp(-d * d / (2 * s * s))
		sum += kernel[i]
	}
	for i := range kernel {
		kernel[i] /= sum
	}
	tmp := make([]float64, len(buf))
	blur := func(dst, src []float64, horizontal bool) {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				var acc [3]float64
				for k, kv := range kernel {
					sx, sy := x, y
					if horizontal {
						sx += k - r
					} else {
						sy += k - r
					}
					if sx < 0 || sy < 0 || sx >= w || sy >= h {
						continue // nothing lights the diffuser beyond the panel
					}
					j := (sy*w + sx) * 3
					acc[0] += kv * src[j]
					acc[1] += kv * src[j+1]
					acc[2] += kv * src[j+2]
				}
				i := (y*w + x) * 3
				dst[i], dst[i+1], dst[i+2] = acc[0], acc[1], acc[2]
			}
		}
	}
	blur(tmp, buf, true)
	blur(buf, tmp, false)
	for i, j := 0, 0; i < len(buf); i, j = i+3, j+4 {
		out.Pix[j] = encodeSRGB(buf[i])
		out.Pix[j+1] = encodeSRGB(buf[i+1])
		out.Pix[j+2] = encodeSRGB(buf[i+2])
		out.Pix[j+3] = 255
	}
	return out
}
