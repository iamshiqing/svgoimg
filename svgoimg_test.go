package svgoimg

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"
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

func TestDecode_DefsUseBasic(t *testing.T) {
	svg := `<svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <rect id="sq" x="0" y="0" width="8" height="8" fill="#ff0000"/>
  </defs>
  <use href="#sq" x="6" y="6"/>
</svg>`
	img, err := DecodeString(svg, nil)
	if err != nil {
		t.Fatalf("DecodeString failed: %v", err)
	}
	bg := color.NRGBAModel.Convert(img.At(2, 2)).(color.NRGBA)
	usePx := color.NRGBAModel.Convert(img.At(8, 8)).(color.NRGBA)
	if bg.A != 0 {
		t.Fatalf("defs content should not render directly, pixel=%#v", bg)
	}
	if !isMostlyRed(usePx) {
		t.Fatalf("use pixel = %#v, want red-like", usePx)
	}
}

func TestDecode_DefsUseSymbolViewBox(t *testing.T) {
	svg := `<svg viewBox="0 0 20 10" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <symbol id="s" viewBox="0 0 10 10">
      <circle cx="5" cy="5" r="5" fill="#0000ff"/>
    </symbol>
  </defs>
  <use href="#s" x="10" y="0" width="10" height="10"/>
</svg>`
	img, err := DecodeString(svg, nil)
	if err != nil {
		t.Fatalf("DecodeString failed: %v", err)
	}
	left := color.NRGBAModel.Convert(img.At(5, 5)).(color.NRGBA)
	right := color.NRGBAModel.Convert(img.At(15, 5)).(color.NRGBA)
	if left.A != 0 {
		t.Fatalf("left side should be transparent, got %#v", left)
	}
	if !isMostlyBlue(right) {
		t.Fatalf("right side should be blue-like, got %#v", right)
	}
}

func TestDecode_LinearGradient(t *testing.T) {
	svg := `<svg viewBox="0 0 100 10" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <linearGradient id="g" x1="0%" y1="0%" x2="100%" y2="0%">
      <stop offset="0%" stop-color="#ff0000"/>
      <stop offset="100%" stop-color="#0000ff"/>
    </linearGradient>
  </defs>
  <rect x="0" y="0" width="100" height="10" fill="url(#g)"/>
</svg>`
	img, err := DecodeString(svg, nil)
	if err != nil {
		t.Fatalf("DecodeString failed: %v", err)
	}
	left := color.NRGBAModel.Convert(img.At(5, 5)).(color.NRGBA)
	right := color.NRGBAModel.Convert(img.At(95, 5)).(color.NRGBA)
	if !isMostlyRed(left) {
		t.Fatalf("left gradient pixel = %#v, want red-like", left)
	}
	if !isMostlyBlue(right) {
		t.Fatalf("right gradient pixel = %#v, want blue-like", right)
	}
}

func TestDecode_LinearGradientHrefInherit(t *testing.T) {
	svg := `<svg viewBox="0 0 10 100" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <linearGradient id="base">
      <stop offset="0%" stop-color="#ff0000"/>
      <stop offset="100%" stop-color="#0000ff"/>
    </linearGradient>
    <linearGradient id="derived" href="#base" x1="0%" y1="0%" x2="0%" y2="100%"/>
  </defs>
  <rect x="0" y="0" width="10" height="100" fill="url(#derived)"/>
</svg>`
	img, err := DecodeString(svg, nil)
	if err != nil {
		t.Fatalf("DecodeString failed: %v", err)
	}
	top := color.NRGBAModel.Convert(img.At(5, 5)).(color.NRGBA)
	bottom := color.NRGBAModel.Convert(img.At(5, 95)).(color.NRGBA)
	if !isMostlyRed(top) {
		t.Fatalf("top gradient pixel = %#v, want red-like", top)
	}
	if !isMostlyBlue(bottom) {
		t.Fatalf("bottom gradient pixel = %#v, want blue-like", bottom)
	}
}

func TestDecode_RadialGradient(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <radialGradient id="rg" cx="50%" cy="50%" r="50%">
      <stop offset="0%" stop-color="#ffffff"/>
      <stop offset="100%" stop-color="#000000"/>
    </radialGradient>
  </defs>
  <rect x="0" y="0" width="100" height="100" fill="url(#rg)"/>
</svg>`
	img, err := DecodeString(svg, nil)
	if err != nil {
		t.Fatalf("DecodeString failed: %v", err)
	}
	center := color.NRGBAModel.Convert(img.At(50, 50)).(color.NRGBA)
	edge := color.NRGBAModel.Convert(img.At(5, 50)).(color.NRGBA)
	if luminance(center) <= luminance(edge) {
		t.Fatalf("radial gradient center should be brighter: center=%#v edge=%#v", center, edge)
	}
}

func TestDecode_GradientIDCaseSensitiveReference(t *testing.T) {
	svg := `<svg viewBox="0 0 100 10" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <linearGradient id="MyGrad" x1="0%" y1="0%" x2="100%" y2="0%">
      <stop offset="0%" stop-color="#ff0000"/>
      <stop offset="100%" stop-color="#0000ff"/>
    </linearGradient>
  </defs>
  <rect x="0" y="0" width="100" height="10" fill="url(#MyGrad)"/>
</svg>`
	img, err := DecodeString(svg, nil)
	if err != nil {
		t.Fatalf("DecodeString failed: %v", err)
	}
	left := color.NRGBAModel.Convert(img.At(5, 5)).(color.NRGBA)
	right := color.NRGBAModel.Convert(img.At(95, 5)).(color.NRGBA)
	if !isMostlyRed(left) {
		t.Fatalf("left gradient pixel = %#v, want red-like", left)
	}
	if !isMostlyBlue(right) {
		t.Fatalf("right gradient pixel = %#v, want blue-like", right)
	}
}

func TestDecode_UseSymbolTranslateNotScaled(t *testing.T) {
	svg := `<svg viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <symbol id="box" viewBox="0 0 10 10">
      <rect x="0" y="0" width="10" height="10" fill="#ff0000"/>
    </symbol>
  </defs>
  <use href="#box" x="40" y="10" width="20" height="20"/>
</svg>`
	img, err := DecodeString(svg, nil)
	if err != nil {
		t.Fatalf("DecodeString failed: %v", err)
	}
	expected := color.NRGBAModel.Convert(img.At(50, 20)).(color.NRGBA)
	unexpected := color.NRGBAModel.Convert(img.At(90, 20)).(color.NRGBA)
	if !isMostlyRed(expected) {
		t.Fatalf("expected symbol area to be red-like, got %#v", expected)
	}
	if unexpected.A != 0 {
		t.Fatalf("unexpected shifted symbol area should be transparent, got %#v", unexpected)
	}
}

func TestDecode_MissingGradientWithoutFallbackIsNone(t *testing.T) {
	svg := `<svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg">
  <rect x="0" y="0" width="20" height="20" fill="url(#missingGrad)"/>
</svg>`
	img, err := DecodeString(svg, nil)
	if err != nil {
		t.Fatalf("DecodeString failed: %v", err)
	}
	center := color.NRGBAModel.Convert(img.At(10, 10)).(color.NRGBA)
	if center.A != 0 {
		t.Fatalf("missing gradient without fallback should render none, got %#v", center)
	}
}

func TestDecode_MissingGradientWithColorFallback(t *testing.T) {
	svg := `<svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg">
  <rect x="0" y="0" width="20" height="20" fill="url(#missingGrad) #ff0000"/>
</svg>`
	img, err := DecodeString(svg, nil)
	if err != nil {
		t.Fatalf("DecodeString failed: %v", err)
	}
	center := color.NRGBAModel.Convert(img.At(10, 10)).(color.NRGBA)
	if !isMostlyRed(center) {
		t.Fatalf("missing gradient with fallback should use fallback color, got %#v", center)
	}
}

func TestDecode_URLQuotedGradientID(t *testing.T) {
	svg := `<svg viewBox="0 0 100 10" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <linearGradient id="g" x1="0%" y1="0%" x2="100%" y2="0%">
      <stop offset="0%" stop-color="#ff0000"/>
      <stop offset="100%" stop-color="#0000ff"/>
    </linearGradient>
  </defs>
  <rect x="0" y="0" width="100" height="10" fill="url('#g')"/>
</svg>`
	img, err := DecodeString(svg, nil)
	if err != nil {
		t.Fatalf("DecodeString failed: %v", err)
	}
	left := color.NRGBAModel.Convert(img.At(5, 5)).(color.NRGBA)
	right := color.NRGBAModel.Convert(img.At(95, 5)).(color.NRGBA)
	if !isMostlyRed(left) {
		t.Fatalf("left gradient pixel = %#v, want red-like", left)
	}
	if !isMostlyBlue(right) {
		t.Fatalf("right gradient pixel = %#v, want blue-like", right)
	}
}

func TestDecode_TransparentNamedColor(t *testing.T) {
	svg := `<svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg">
  <rect x="0" y="0" width="20" height="20" fill="transparent"/>
</svg>`
	img, err := DecodeString(svg, nil)
	if err != nil {
		t.Fatalf("DecodeString failed: %v", err)
	}
	center := color.NRGBAModel.Convert(img.At(10, 10)).(color.NRGBA)
	if center.A != 0 {
		t.Fatalf("transparent color should have alpha 0, got %#v", center)
	}
}

func TestDecode_InvalidShortHexInStrictModeReturnsError(t *testing.T) {
	svg := `<svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg">
  <rect x="0" y="0" width="20" height="20" fill="#zz0"/>
</svg>`
	_, err := DecodeString(svg, &Options{ParseMode: ParseStrict})
	if err == nil {
		t.Fatalf("expected strict parse error for invalid short hex")
	}
}

func TestDecode_InvalidPathTokenDoesNotHang(t *testing.T) {
	svg := `<svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg">
  <path d="M0 0 L10 10X" stroke="#ff0000" fill="none"/>
</svg>`
	done := make(chan error, 1)
	go func() {
		_, err := DecodeString(svg, nil)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("DecodeString in ignore mode should not return error, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("DecodeString timed out, possible parser loop")
	}

	_, err := DecodeString(svg, &Options{ParseMode: ParseStrict})
	if err == nil {
		t.Fatalf("expected strict parse error for invalid path token")
	}
}

func TestDecode_GradientStopsKeepDocumentOrder(t *testing.T) {
	svg := `<svg viewBox="0 0 100 10" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <linearGradient id="g" x1="0%" y1="0%" x2="100%" y2="0%">
      <stop offset="60%" stop-color="#ff0000"/>
      <stop offset="40%" stop-color="#0000ff"/>
    </linearGradient>
  </defs>
  <rect x="0" y="0" width="100" height="10" fill="url(#g)"/>
</svg>`
	img, err := DecodeString(svg, nil)
	if err != nil {
		t.Fatalf("DecodeString failed: %v", err)
	}

	mid := color.NRGBAModel.Convert(img.At(50, 5)).(color.NRGBA)
	right := color.NRGBAModel.Convert(img.At(90, 5)).(color.NRGBA)
	if !isMostlyRed(mid) {
		t.Fatalf("mid gradient pixel = %#v, want red-like (stop order should follow document order)", mid)
	}
	if !isMostlyBlue(right) {
		t.Fatalf("right gradient pixel = %#v, want blue-like", right)
	}
}

func TestDecode_OpacityPercent(t *testing.T) {
	svg := `<svg viewBox="0 0 10 10" xmlns="http://www.w3.org/2000/svg">
  <rect x="0" y="0" width="10" height="10" fill="#ff0000" opacity="50%"/>
</svg>`
	img, err := DecodeString(svg, &Options{ParseMode: ParseStrict})
	if err != nil {
		t.Fatalf("DecodeString failed: %v", err)
	}
	c := color.NRGBAModel.Convert(img.At(5, 5)).(color.NRGBA)
	if c.A < 120 || c.A > 136 {
		t.Fatalf("alpha = %d, want around 128 for 50%% opacity", c.A)
	}
}

func TestDecode_NamedColorAliceBlue(t *testing.T) {
	svg := `<svg viewBox="0 0 10 10" xmlns="http://www.w3.org/2000/svg">
  <rect x="0" y="0" width="10" height="10" fill="aliceblue"/>
</svg>`
	img, err := DecodeString(svg, nil)
	if err != nil {
		t.Fatalf("DecodeString failed: %v", err)
	}
	c := color.NRGBAModel.Convert(img.At(5, 5)).(color.NRGBA)
	if !isNearColor(c, color.NRGBA{R: 240, G: 248, B: 255, A: 255}, 4) {
		t.Fatalf("named color aliceblue parsed to %#v, want near #F0F8FF", c)
	}
}

func TestDecode_ParseWarnReportsWarnings(t *testing.T) {
	svg := `<svg viewBox="0 0 10 10" xmlns="http://www.w3.org/2000/svg">
  <rect x="0" y="0" width="10" height="10" fill="not-a-color"/>
</svg>`
	warnCount := 0
	img, err := DecodeString(svg, &Options{
		ParseMode: ParseWarn,
		OnWarning: func(err error) {
			if err != nil {
				warnCount++
			}
		},
	})
	if err != nil {
		t.Fatalf("ParseWarn should not fail decode, got: %v", err)
	}
	if warnCount == 0 {
		t.Fatalf("ParseWarn should report warnings via callback")
	}
	center := color.NRGBAModel.Convert(img.At(5, 5)).(color.NRGBA)
	if center.A == 0 {
		t.Fatalf("rect should still render with inherited/default fill, got %#v", center)
	}
}

func TestDecode_StrokeLinecapRoundVsButt(t *testing.T) {
	svg := `<svg viewBox="0 0 60 20" xmlns="http://www.w3.org/2000/svg">
  <line x1="10" y1="6" x2="30" y2="6" stroke="#ff0000" stroke-width="8" stroke-linecap="butt"/>
  <line x1="10" y1="14" x2="30" y2="14" stroke="#0000ff" stroke-width="8" stroke-linecap="round"/>
</svg>`
	img, err := DecodeString(svg, nil)
	if err != nil {
		t.Fatalf("DecodeString failed: %v", err)
	}
	buttStart := color.NRGBAModel.Convert(img.At(7, 6)).(color.NRGBA)
	roundStart := color.NRGBAModel.Convert(img.At(7, 14)).(color.NRGBA)
	if buttStart.A != 0 {
		t.Fatalf("butt cap should not extend, got %#v", buttStart)
	}
	if !isMostlyBlue(roundStart) {
		t.Fatalf("round cap should extend, got %#v", roundStart)
	}
}

func TestDecode_StrokeDashArray(t *testing.T) {
	svg := `<svg viewBox="0 0 60 20" xmlns="http://www.w3.org/2000/svg">
  <line x1="5" y1="10" x2="55" y2="10" stroke="#ff0000" stroke-width="4" stroke-dasharray="8 8"/>
</svg>`
	img, err := DecodeString(svg, nil)
	if err != nil {
		t.Fatalf("DecodeString failed: %v", err)
	}
	on := color.NRGBAModel.Convert(img.At(8, 10)).(color.NRGBA)
	off := color.NRGBAModel.Convert(img.At(20, 10)).(color.NRGBA)
	if !isMostlyRed(on) {
		t.Fatalf("expected on-dash pixel red-like, got %#v", on)
	}
	if off.A != 0 {
		t.Fatalf("expected off-dash pixel transparent, got %#v", off)
	}
}

func TestDecode_ImageDataURIAndPreserveAspectRatioMeet(t *testing.T) {
	dataURI := tinyPNGDataURI()
	svg := `<svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg">
  <image href="` + dataURI + `" x="0" y="0" width="20" height="20"/>
</svg>`
	img, err := DecodeString(svg, nil)
	if err != nil {
		t.Fatalf("DecodeString failed: %v", err)
	}
	top := color.NRGBAModel.Convert(img.At(10, 2)).(color.NRGBA)
	center := color.NRGBAModel.Convert(img.At(10, 10)).(color.NRGBA)
	if top.A != 0 {
		t.Fatalf("meet should letterbox top, got %#v", top)
	}
	if center.A == 0 {
		t.Fatalf("center should contain image content")
	}
}

func TestDecode_ClipPathBasic(t *testing.T) {
	svg := `<svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <clipPath id="c">
      <circle cx="10" cy="10" r="6"/>
    </clipPath>
  </defs>
  <rect x="0" y="0" width="20" height="20" fill="#ff0000" clip-path="url(#c)"/>
</svg>`
	img, err := DecodeString(svg, nil)
	if err != nil {
		t.Fatalf("DecodeString failed: %v", err)
	}
	corner := color.NRGBAModel.Convert(img.At(1, 1)).(color.NRGBA)
	center := color.NRGBAModel.Convert(img.At(10, 10)).(color.NRGBA)
	if corner.A != 0 {
		t.Fatalf("corner should be clipped out, got %#v", corner)
	}
	if !isMostlyRed(center) {
		t.Fatalf("center should remain red-like, got %#v", center)
	}
}

func TestDecode_MaskBasic(t *testing.T) {
	svg := `<svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <mask id="m" mask-type="alpha">
      <circle cx="10" cy="10" r="6" fill="#ffffff"/>
    </mask>
  </defs>
  <rect x="0" y="0" width="20" height="20" fill="#0000ff" mask="url(#m)"/>
</svg>`
	img, err := DecodeString(svg, nil)
	if err != nil {
		t.Fatalf("DecodeString failed: %v", err)
	}
	corner := color.NRGBAModel.Convert(img.At(1, 1)).(color.NRGBA)
	center := color.NRGBAModel.Convert(img.At(10, 10)).(color.NRGBA)
	if corner.A != 0 {
		t.Fatalf("corner should be masked out, got %#v", corner)
	}
	if !isMostlyBlue(center) {
		t.Fatalf("center should remain blue-like, got %#v", center)
	}
}

func TestDecode_CSSBasicSelectors(t *testing.T) {
	svg := `<svg viewBox="0 0 20 20" xmlns="http://www.w3.org/2000/svg">
  <style>
    rect { fill: #ff0000; }
    .k { fill: #00ff00; }
    #a { fill: #0000ff; }
  </style>
  <rect id="a" class="k" x="0" y="0" width="20" height="20" fill="#00ff00"/>
</svg>`
	img, err := DecodeString(svg, nil)
	if err != nil {
		t.Fatalf("DecodeString failed: %v", err)
	}
	center := color.NRGBAModel.Convert(img.At(10, 10)).(color.NRGBA)
	// Inline attribute should override CSS selector results.
	if center.G < 200 || center.R > 60 || center.B > 60 {
		t.Fatalf("inline fill should override CSS, got %#v", center)
	}
}

func TestDecode_MarkerEndBasic(t *testing.T) {
	svg := `<svg viewBox="0 0 40 20" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <marker id="arrow" markerWidth="4" markerHeight="4" refX="0" refY="2" orient="auto">
      <path d="M0,0 L4,2 L0,4 Z" fill="#ff0000"/>
    </marker>
  </defs>
  <line x1="5" y1="10" x2="25" y2="10" stroke="#0000ff" stroke-width="2" marker-end="url(#arrow)"/>
</svg>`
	img, err := DecodeString(svg, nil)
	if err != nil {
		t.Fatalf("DecodeString failed: %v", err)
	}
	markerPx := color.NRGBAModel.Convert(img.At(27, 10)).(color.NRGBA)
	if !isMostlyRed(markerPx) {
		t.Fatalf("marker-end pixel should be red-like, got %#v", markerPx)
	}
}

func TestDecode_PatternBasic(t *testing.T) {
	svg := `<svg viewBox="0 0 20 10" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <pattern id="p" x="0" y="0" width="4" height="4" patternUnits="userSpaceOnUse">
      <rect x="0" y="0" width="2" height="4" fill="#ff0000"/>
      <rect x="2" y="0" width="2" height="4" fill="#0000ff"/>
    </pattern>
  </defs>
  <rect x="0" y="0" width="20" height="10" fill="url(#p)"/>
</svg>`
	img, err := DecodeString(svg, nil)
	if err != nil {
		t.Fatalf("DecodeString failed: %v", err)
	}
	c1 := color.NRGBAModel.Convert(img.At(1, 5)).(color.NRGBA)
	c2 := color.NRGBAModel.Convert(img.At(3, 5)).(color.NRGBA)
	c3 := color.NRGBAModel.Convert(img.At(5, 5)).(color.NRGBA)
	if !isMostlyRed(c1) {
		t.Fatalf("x=1 should be red-like, got %#v", c1)
	}
	if !isMostlyBlue(c2) {
		t.Fatalf("x=3 should be blue-like, got %#v", c2)
	}
	if !isMostlyRed(c3) {
		t.Fatalf("x=5 should repeat red-like, got %#v", c3)
	}
}

func isMostlyWhite(c color.NRGBA) bool {
	return c.R >= 240 && c.G >= 240 && c.B >= 240 && c.A >= 240
}

func isMostlyRed(c color.NRGBA) bool {
	return c.R >= 200 && c.G <= 50 && c.B <= 50 && c.A >= 200
}

func isMostlyBlue(c color.NRGBA) bool {
	return c.B >= 200 && c.R <= 50 && c.G <= 50 && c.A >= 200
}

func luminance(c color.NRGBA) float64 {
	return 0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)
}

func isNearColor(got, want color.NRGBA, tolerance uint8) bool {
	dr := absDiff(got.R, want.R)
	dg := absDiff(got.G, want.G)
	db := absDiff(got.B, want.B)
	da := absDiff(got.A, want.A)
	return dr <= tolerance && dg <= tolerance && db <= tolerance && da <= tolerance
}

func absDiff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

func tinyPNGDataURI() string {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, color.NRGBA{R: 255, G: 0, B: 0, A: 255})
	img.Set(1, 0, color.NRGBA{R: 0, G: 0, B: 255, A: 255})
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}
