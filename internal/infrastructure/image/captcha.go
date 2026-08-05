// Package image implements ports.ImageRenderer: it draws the captcha
// challenge prompt as a PNG with colored interference lines. Pure Go, no
// cgo, so it runs unchanged on host and js/wasm.
package image

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math/rand"

	"github.com/stvlynn/xqt-bot/internal/domain/ports"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Canvas and text geometry for the captcha image.
const (
	captchaWidth  = 320
	captchaHeight = 120
	fontSize      = 56
	fontDPI       = 72
	noiseLines    = 6
)

var (
	backgroundColor = color.RGBA{0xff, 0xff, 0xff, 0xff}
	textColor       = color.RGBA{0x1a, 0x1a, 0x2e, 0xff}
	noisePalette    = []color.RGBA{
		{0xe5, 0x39, 0x35, 0xff}, // red
		{0x1e, 0x88, 0xe5, 0xff}, // blue
		{0x43, 0xa0, 0x47, 0xff}, // green
		{0xfb, 0x8c, 0x00, 0xff}, // orange
		{0x8e, 0x24, 0xaa, 0xff}, // purple
	}
)

// Renderer implements ports.ImageRenderer.
type Renderer struct {
	face font.Face
	rng  *rand.Rand
}

// NewRenderer parses the embedded Go Regular font and prepares a face at
// the captcha text size.
func NewRenderer() (*Renderer, error) {
	f, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return nil, err
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    fontSize,
		DPI:     fontDPI,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, err
	}
	return &Renderer{face: face, rng: rand.New(rand.NewSource(rand.Int63()))}, nil
}

var _ ports.ImageRenderer = (*Renderer)(nil)

// RenderCaptcha implements ports.ImageRenderer: white background, a few
// colored diagonal noise lines, and the question centered in dark text.
func (r *Renderer) RenderCaptcha(question string) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, captchaWidth, captchaHeight))
	draw.Draw(img, img.Bounds(), &image.Uniform{backgroundColor}, image.Point{}, draw.Src)

	// Interference lines behind the text, drawn after the background but
	// before the glyphs.
	for i := 0; i < noiseLines; i++ {
		c := noisePalette[r.rng.Intn(len(noisePalette))]
		drawLine(img,
			r.rng.Intn(captchaWidth), r.rng.Intn(captchaHeight),
			r.rng.Intn(captchaWidth), r.rng.Intn(captchaHeight),
			c)
	}

	// Center the question horizontally and vertically (approximate
	// midline-based vertical centering is fine for all-caps arithmetic).
	textWidth := font.MeasureString(r.face, question).Ceil()
	x := (captchaWidth - textWidth) / 2
	if x < 4 {
		x = 4
	}
	metrics := r.face.Metrics()
	y := (captchaHeight+metrics.Ascent.Ceil()-metrics.Descent.Ceil())/2 - metrics.Descent.Ceil()/2

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(textColor),
		Face: r.face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(question)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// drawLine rasterizes a 1px line between two points (Bresenham).
func drawLine(img *image.RGBA, x0, y0, x1, y1 int, c color.Color) {
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx := 1
	if x0 > x1 {
		sx = -1
	}
	sy := 1
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	for {
		if image.Pt(x0, y0).In(img.Bounds()) {
			img.Set(x0, y0, c)
		}
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
