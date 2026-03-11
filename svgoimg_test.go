package svgoimg

import (
	"image/color"
	"testing"
)

func TestDecode_UsesViewBoxSizeByDefault(t *testing.T) {
	svg := `<svg viewBox="0 0 10 20" xmlns="http://www.w3.org/2000/svg"><rect x="0" y="0" width="10" height="20" fill="#ff0000"/></svg>`
	img, err := DecodeString(svg, nil)
	if err != nil {
		t.Fatalf("DecodeString failed: %v", err)
	}
	if got := img.Bounds().Dx(); got != 10 {
		t.Fatalf("width = %d, want 10", got)
	}
	if got := img.Bounds().Dy(); got != 20 {
		t.Fatalf("height = %d, want 20", got)
	}
}

func TestDecode_WidthOnlyKeepsAspectRatio(t *testing.T) {
	svg := `<svg viewBox="0 0 10 20" xmlns="http://www.w3.org/2000/svg"><rect x="0" y="0" width="10" height="20" fill="#00ff00"/></svg>`
	img, err := DecodeString(svg, &Options{Width: 40})
	if err != nil {
		t.Fatalf("DecodeString failed: %v", err)
	}
	if got := img.Bounds().Dx(); got != 40 {
		t.Fatalf("width = %d, want 40", got)
	}
	if got := img.Bounds().Dy(); got != 80 {
		t.Fatalf("height = %d, want 80", got)
	}
}

func TestDecode_ContainAddsLetterBoxWithBackground(t *testing.T) {
	svg := `<svg viewBox="0 0 10 10" xmlns="http://www.w3.org/2000/svg"><rect x="0" y="0" width="10" height="10" fill="#ff0000"/></svg>`
	img, err := DecodeString(svg, &Options{
		Width:      20,
		Height:     10,
		Fit:        FitContain,
		Background: color.NRGBA{R: 255, G: 255, B: 255, A: 255},
	})
	if err != nil {
		t.Fatalf("DecodeString failed: %v", err)
	}

	left := color.NRGBAModel.Convert(img.At(1, 5)).(color.NRGBA)
	center := color.NRGBAModel.Convert(img.At(10, 5)).(color.NRGBA)
	if !isMostlyWhite(left) {
		t.Fatalf("left pixel = %#v, want white-like", left)
	}
	if !isMostlyRed(center) {
		t.Fatalf("center pixel = %#v, want red-like", center)
	}
}

func TestDecode_InvalidSize(t *testing.T) {
	svg := `<svg viewBox="0 0 10 10" xmlns="http://www.w3.org/2000/svg"><rect x="0" y="0" width="10" height="10" fill="#ff0000"/></svg>`
	_, err := DecodeString(svg, &Options{
		Width: -1,
	})
	if err == nil {
		t.Fatalf("expected error for negative width")
	}
}

func isMostlyWhite(c color.NRGBA) bool {
	return c.R >= 240 && c.G >= 240 && c.B >= 240 && c.A >= 240
}

func isMostlyRed(c color.NRGBA) bool {
	return c.R >= 200 && c.G <= 50 && c.B <= 50 && c.A >= 200
}
