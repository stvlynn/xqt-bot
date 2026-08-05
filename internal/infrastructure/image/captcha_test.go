package image

import (
	"bytes"
	goimage "image"
	"image/png"
	"testing"
)

var pngMagic = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}

func TestRenderCaptchaPNG(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	data, err := r.RenderCaptcha("3 + 4 = ?")
	if err != nil {
		t.Fatalf("RenderCaptcha: %v", err)
	}
	if !bytes.HasPrefix(data, pngMagic) {
		t.Fatal("output is not a PNG (bad magic bytes)")
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode PNG: %v", err)
	}
	if got := img.Bounds().Size(); got != (goimage.Point{X: captchaWidth, Y: captchaHeight}) {
		t.Errorf("size = %v, want %dx%d", got, captchaWidth, captchaHeight)
	}
}

func TestRenderCaptchaNotBlank(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	data, err := r.RenderCaptcha("1 + 2 = ?")
	if err != nil {
		t.Fatalf("RenderCaptcha: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode PNG: %v", err)
	}
	// The rendered question and noise lines must leave non-white pixels.
	nonWhite := 0
	for y := 0; y < captchaHeight; y++ {
		for x := 0; x < captchaWidth; x++ {
			rr, gg, bb, _ := img.At(x, y).RGBA()
			if rr != 0xffff || gg != 0xffff || bb != 0xffff {
				nonWhite++
			}
		}
	}
	if nonWhite == 0 {
		t.Error("image is blank")
	}
}
