package raster

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/iamshiqing/svgoimg/internal/model"
)

func Render(scene model.Scene, opts Options) (*image.NRGBA, error) {
	opts = opts.withDefaults()

	viewBox := scene.ViewBox
	if !viewBox.IsValid() {
		viewBox = model.Rect{X: 0, Y: 0, W: scene.Width, H: scene.Height}
	}
	if !viewBox.IsValid() {
		viewBox = model.Rect{X: 0, Y: 0, W: 300, H: 150}
	}

	canvasW, canvasH, err := resolveCanvasSize(viewBox.W, viewBox.H, opts.Width, opts.Height)
	if err != nil {
		return nil, err
	}

	img := image.NewNRGBA(image.Rect(0, 0, canvasW, canvasH))
	if opts.Background != nil {
		draw.Draw(img, img.Bounds(), image.NewUniform(opts.Background), image.Point{}, draw.Src)
	}

	target := fitTarget(float64(canvasW), float64(canvasH), viewBox.W, viewBox.H, opts.Fit)
	global := model.Matrix{
		A: target.W / viewBox.W,
		D: target.H / viewBox.H,
		E: target.X - viewBox.X*(target.W/viewBox.W),
		F: target.Y - viewBox.Y*(target.H/viewBox.H),
	}
	globalScale := global.ApproxScale()

	for _, cmd := range scene.Commands {
		path := transformPath(cmd.Path, global)
		if len(path.Subpaths) == 0 {
			continue
		}

		style := cmd.Style
		style.StrokeWidth = style.StrokeWidth * globalScale

		if !style.Fill.None {
			fillColor := applyAlpha(style.Fill.Color, style.Opacity*style.FillOpacity)
			fillPath(img, path, fillColor, style.FillRule)
		}
		if !style.Stroke.None && style.StrokeWidth > 0 {
			strokeColor := applyAlpha(style.Stroke.Color, style.Opacity*style.StrokeOpacity)
			strokePath(img, path, strokeColor, style.StrokeWidth)
		}
	}

	return img, nil
}

type rect struct {
	X float64
	Y float64
	W float64
	H float64
}

func resolveCanvasSize(srcW, srcH float64, reqW, reqH int) (int, int, error) {
	if reqW < 0 || reqH < 0 {
		return 0, 0, fmt.Errorf("width and height must be >= 0")
	}
	if srcW <= 0 || srcH <= 0 {
		srcW, srcH = 300, 150
	}

	switch {
	case reqW > 0 && reqH > 0:
		return reqW, reqH, nil
	case reqW > 0:
		h := int(math.Round(float64(reqW) * srcH / srcW))
		if h < 1 {
			h = 1
		}
		return reqW, h, nil
	case reqH > 0:
		w := int(math.Round(float64(reqH) * srcW / srcH))
		if w < 1 {
			w = 1
		}
		return w, reqH, nil
	default:
		w := int(math.Round(srcW))
		h := int(math.Round(srcH))
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}
		return w, h, nil
	}
}

func fitTarget(canvasW, canvasH, srcW, srcH float64, fit FitMode) rect {
	if fit == FitStretch || srcW <= 0 || srcH <= 0 {
		return rect{X: 0, Y: 0, W: canvasW, H: canvasH}
	}
	sx := canvasW / srcW
	sy := canvasH / srcH
	scale := sx
	if fit == FitContain {
		if sy < sx {
			scale = sy
		}
	} else {
		if sy > sx {
			scale = sy
		}
	}
	w := srcW * scale
	h := srcH * scale
	return rect{
		X: (canvasW - w) * 0.5,
		Y: (canvasH - h) * 0.5,
		W: w,
		H: h,
	}
}

func applyAlpha(c color.NRGBA, alpha float64) color.NRGBA {
	alpha = clamp01(alpha)
	if alpha <= 0 {
		c.A = 0
		return c
	}
	c.A = uint8(float64(c.A)*alpha + 0.5)
	return c
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func transformPath(path model.Path, m model.Matrix) model.Path {
	if m == model.IdentityMatrix {
		return path
	}
	out := model.Path{
		Subpaths: make([]model.Subpath, 0, len(path.Subpaths)),
	}
	for _, sp := range path.Subpaths {
		tsp := model.Subpath{
			Closed: sp.Closed,
			Points: make([]model.Point, 0, len(sp.Points)),
		}
		for _, p := range sp.Points {
			tsp.Points = append(tsp.Points, m.Apply(p))
		}
		out.Subpaths = append(out.Subpaths, tsp)
	}
	return out
}

type edge struct {
	A model.Point
	B model.Point
}

func fillPath(img *image.NRGBA, path model.Path, clr color.NRGBA, rule model.FillRule) {
	if clr.A == 0 {
		return
	}
	edges := closedEdges(path)
	if len(edges) == 0 {
		return
	}
	b, ok := edgesBounds(edges, 0)
	if !ok {
		return
	}
	minX := clampInt(int(math.Floor(b.minX)), 0, img.Bounds().Dx()-1)
	maxX := clampInt(int(math.Ceil(b.maxX)), 0, img.Bounds().Dx()-1)
	minY := clampInt(int(math.Floor(b.minY)), 0, img.Bounds().Dy()-1)
	maxY := clampInt(int(math.Ceil(b.maxY)), 0, img.Bounds().Dy()-1)
	if minX > maxX || minY > maxY {
		return
	}

	samples := [4][2]float64{
		{0.25, 0.25},
		{0.75, 0.25},
		{0.25, 0.75},
		{0.75, 0.75},
	}

	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			inside := 0
			for _, s := range samples {
				px := float64(x) + s[0]
				py := float64(y) + s[1]
				hit := false
				if rule == model.FillRuleEvenOdd {
					hit = pointInEvenOdd(px, py, edges)
				} else {
					hit = pointInNonZero(px, py, edges)
				}
				if hit {
					inside++
				}
			}
			if inside == 0 {
				continue
			}
			src := clr
			src.A = uint8(float64(clr.A)*(float64(inside)/float64(len(samples))) + 0.5)
			blendAt(img, x, y, src)
		}
	}
}

func strokePath(img *image.NRGBA, path model.Path, clr color.NRGBA, width float64) {
	if clr.A == 0 || width <= 0 {
		return
	}
	segments := strokeEdges(path)
	if len(segments) == 0 {
		return
	}
	half := width * 0.5
	halfSq := half * half

	b, ok := edgesBounds(segments, half)
	if !ok {
		return
	}
	minX := clampInt(int(math.Floor(b.minX)), 0, img.Bounds().Dx()-1)
	maxX := clampInt(int(math.Ceil(b.maxX)), 0, img.Bounds().Dx()-1)
	minY := clampInt(int(math.Floor(b.minY)), 0, img.Bounds().Dy()-1)
	maxY := clampInt(int(math.Ceil(b.maxY)), 0, img.Bounds().Dy()-1)
	if minX > maxX || minY > maxY {
		return
	}

	samples := [4][2]float64{
		{0.25, 0.25},
		{0.75, 0.25},
		{0.25, 0.75},
		{0.75, 0.75},
	}

	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			hit := 0
			for _, s := range samples {
				px := float64(x) + s[0]
				py := float64(y) + s[1]
				p := model.Point{X: px, Y: py}
				if pointOnStroke(p, segments, halfSq) {
					hit++
				}
			}
			if hit == 0 {
				continue
			}
			src := clr
			src.A = uint8(float64(clr.A)*(float64(hit)/float64(len(samples))) + 0.5)
			blendAt(img, x, y, src)
		}
	}
}

type bounds struct {
	minX float64
	minY float64
	maxX float64
	maxY float64
}

func edgesBounds(edges []edge, pad float64) (bounds, bool) {
	if len(edges) == 0 {
		return bounds{}, false
	}
	b := bounds{
		minX: math.Inf(1),
		minY: math.Inf(1),
		maxX: math.Inf(-1),
		maxY: math.Inf(-1),
	}
	for _, e := range edges {
		b.minX = math.Min(b.minX, math.Min(e.A.X, e.B.X))
		b.minY = math.Min(b.minY, math.Min(e.A.Y, e.B.Y))
		b.maxX = math.Max(b.maxX, math.Max(e.A.X, e.B.X))
		b.maxY = math.Max(b.maxY, math.Max(e.A.Y, e.B.Y))
	}
	b.minX -= pad
	b.minY -= pad
	b.maxX += pad
	b.maxY += pad
	return b, finiteBounds(b)
}

func finiteBounds(b bounds) bool {
	return finite(b.minX) && finite(b.minY) && finite(b.maxX) && finite(b.maxY) && b.maxX >= b.minX && b.maxY >= b.minY
}

func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func closedEdges(path model.Path) []edge {
	out := make([]edge, 0, 16)
	for _, sp := range path.Subpaths {
		if !sp.Closed || len(sp.Points) < 3 {
			continue
		}
		for i := 1; i < len(sp.Points); i++ {
			out = append(out, edge{A: sp.Points[i-1], B: sp.Points[i]})
		}
		out = append(out, edge{A: sp.Points[len(sp.Points)-1], B: sp.Points[0]})
	}
	return out
}

func strokeEdges(path model.Path) []edge {
	out := make([]edge, 0, 16)
	for _, sp := range path.Subpaths {
		if len(sp.Points) < 2 {
			continue
		}
		for i := 1; i < len(sp.Points); i++ {
			out = append(out, edge{A: sp.Points[i-1], B: sp.Points[i]})
		}
		if sp.Closed {
			out = append(out, edge{A: sp.Points[len(sp.Points)-1], B: sp.Points[0]})
		}
	}
	return out
}

func pointInEvenOdd(px, py float64, edges []edge) bool {
	inside := false
	for _, e := range edges {
		y1, y2 := e.A.Y, e.B.Y
		if (y1 > py) == (y2 > py) {
			continue
		}
		xi := e.A.X + (py-y1)*(e.B.X-e.A.X)/(y2-y1)
		if px < xi {
			inside = !inside
		}
	}
	return inside
}

func pointInNonZero(px, py float64, edges []edge) bool {
	wn := 0
	for _, e := range edges {
		y1, y2 := e.A.Y, e.B.Y
		if y1 <= py {
			if y2 > py && isLeft(e.A, e.B, px, py) > 0 {
				wn++
			}
		} else {
			if y2 <= py && isLeft(e.A, e.B, px, py) < 0 {
				wn--
			}
		}
	}
	return wn != 0
}

func isLeft(a, b model.Point, px, py float64) float64 {
	return (b.X-a.X)*(py-a.Y) - (px-a.X)*(b.Y-a.Y)
}

func pointOnStroke(p model.Point, segments []edge, halfSq float64) bool {
	for _, s := range segments {
		if distToSegmentSq(p, s.A, s.B) <= halfSq {
			return true
		}
	}
	return false
}

func distToSegmentSq(p, a, b model.Point) float64 {
	vx := b.X - a.X
	vy := b.Y - a.Y
	if vx == 0 && vy == 0 {
		dx := p.X - a.X
		dy := p.Y - a.Y
		return dx*dx + dy*dy
	}
	t := ((p.X-a.X)*vx + (p.Y-a.Y)*vy) / (vx*vx + vy*vy)
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	cx := a.X + t*vx
	cy := a.Y + t*vy
	dx := p.X - cx
	dy := p.Y - cy
	return dx*dx + dy*dy
}

func blendAt(img *image.NRGBA, x, y int, src color.NRGBA) {
	if src.A == 0 {
		return
	}
	i := img.PixOffset(x, y)
	dr := float64(img.Pix[i+0])
	dg := float64(img.Pix[i+1])
	db := float64(img.Pix[i+2])
	da := float64(img.Pix[i+3]) / 255.0

	sa := float64(src.A) / 255.0
	outA := sa + da*(1-sa)
	if outA <= 0 {
		img.Pix[i+0] = 0
		img.Pix[i+1] = 0
		img.Pix[i+2] = 0
		img.Pix[i+3] = 0
		return
	}

	outR := (float64(src.R)*sa + dr*da*(1-sa)) / outA
	outG := (float64(src.G)*sa + dg*da*(1-sa)) / outA
	outB := (float64(src.B)*sa + db*da*(1-sa)) / outA

	img.Pix[i+0] = toByte(outR)
	img.Pix[i+1] = toByte(outG)
	img.Pix[i+2] = toByte(outB)
	img.Pix[i+3] = toByte(outA * 255.0)
}

func toByte(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(v + 0.5)
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
