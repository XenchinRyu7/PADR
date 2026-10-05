package gui

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"

	"fyne.io/fyne/v2"
)

// GetAppIcon generates a stylized purple-cyan hexagon logo for PADR tray & window
func GetAppIcon() fyne.Resource {
	const size = 64
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	// Dark sleek background with rounded icon look
	bg := color.RGBA{R: 20, G: 24, B: 36, A: 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)

	// Draw stylized 'P' mark in cyan/purple
	cyan := color.RGBA{R: 0, G: 216, B: 255, A: 255}
	purple := color.RGBA{R: 147, G: 51, B: 234, A: 255}

	// Vertical bar
	for x := 16; x < 26; x++ {
		for y := 12; y < 52; y++ {
			img.Set(x, y, cyan)
		}
	}

	// Top horizontal bar
	for x := 26; x < 44; x++ {
		for y := 12; y < 20; y++ {
			img.Set(x, y, purple)
		}
	}

	// Loop curved bar
	for x := 38; x < 46; x++ {
		for y := 18; y < 34; y++ {
			img.Set(x, y, purple)
		}
	}

	// Mid horizontal bar
	for x := 26; x < 44; x++ {
		for y := 28; y < 36; y++ {
			img.Set(x, y, cyan)
		}
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)

	return fyne.NewStaticResource("padr-icon.png", buf.Bytes())
}
