package raster

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/iamshiqing/svgoimg/internal/model"
)

var msaa4Samples = [4][2]float64{
	{0.25, 0.25},
	{0.75, 0.25},
	{0.25, 0.75},
	{0.75, 0.75},
}

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
		var clip *clipRaster
		if cmd.Clip != nil {
			clipPath := transformPath(cmd.Clip.Path, global)
			clip = newClipRaster(clipPath, cmd.Clip.Rule)
		}
		var mask *maskRaster
		if cmd.Mask != nil {
			maskPath := transformPath(cmd.Mask.Path, global)
			mask = newMaskRaster(maskPath, cmd.Mask.Rule, cmd.Mask.Luminance)
		}

		targetImg := img
		var tmp *image.NRGBA
		if clip != nil || mask != nil {
			tmp = image.NewNRGBA(img.Bounds())
			targetImg = tmp
		}

		if cmd.Image != nil {
			renderImage(targetImg, *cmd.Image, global)
		} else {
			path := transformPath(cmd.Path, global)
			if len(path.Subpaths) == 0 {
				continue
			}

			style := cmd.Style
			scaleStrokeStyle(&style, globalScale)

			if !style.Fill.None {
				fillAlpha := style.Opacity * style.FillOpacity
				switch style.Fill.Kind {
				case model.PaintKindGradient:
					if style.Fill.GradientID != "" {
						if g, ok := scene.Gradients[style.Fill.GradientID]; ok {
							fillPathGradient(targetImg, path, g, fillAlpha, style.FillRule)
						} else if style.Fill.HasFallback {
							fillColor := applyAlpha(style.Fill.Color, fillAlpha)
							fillPath(targetImg, path, fillColor, style.FillRule)
						}
					}
				case model.PaintKindPattern:
					if style.Fill.PatternID != "" {
						if pat, ok := scene.Patterns[style.Fill.PatternID]; ok {
							fillPathPattern(targetImg, path, pat, fillAlpha, style.FillRule)
						} else if style.Fill.HasFallback {
							fillColor := applyAlpha(style.Fill.Color, fillAlpha)
							fillPath(targetImg, path, fillColor, style.FillRule)
						}
					}
				default:
					fillColor := applyAlpha(style.Fill.Color, fillAlpha)
					fillPath(targetImg, path, fillColor, style.FillRule)
				}
			}

			if !style.Stroke.None && style.StrokeWidth > 0 {
				strokeAlpha := style.Opacity * style.StrokeOpacity
				switch style.Stroke.Kind {
				case model.PaintKindGradient:
					if style.Stroke.GradientID != "" {
						if g, ok := scene.Gradients[style.Stroke.GradientID]; ok {
							strokePathGradient(targetImg, path, g, strokeAlpha, style)
						} else if style.Stroke.HasFallback {
							strokeColor := applyAlpha(style.Stroke.Color, strokeAlpha)
							strokePath(targetImg, path, strokeColor, style)
						}
					}
				case model.PaintKindPattern:
					if style.Stroke.PatternID != "" {
						if pat, ok := scene.Patterns[style.Stroke.PatternID]; ok {
							strokePathPattern(targetImg, path, pat, strokeAlpha, style)
						} else if style.Stroke.HasFallback {
							strokeColor := applyAlpha(style.Stroke.Color, strokeAlpha)
							strokePath(targetImg, path, strokeColor, style)
						}
					}
				default:
					strokeColor := applyAlpha(style.Stroke.Color, strokeAlpha)
					strokePath(targetImg, path, strokeColor, style)
				}
			}
		}

		if tmp != nil {
			compositeWithFilters(img, tmp, clip, mask)
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

func renderImage(dst *image.NRGBA, drawCmd model.ImageDraw, global model.Matrix) {
	if dst == nil || drawCmd.Img == nil {
		return
	}
	alpha := clamp01(drawCmd.Opacity)
	if alpha <= 0 {
		return
	}

	p0 := global.Apply(drawCmd.P0)
	p1 := global.Apply(drawCmd.P1)
	p3 := global.Apply(drawCmd.P3)
	c0 := global.Apply(drawCmd.C0)
	c1 := global.Apply(drawCmd.C1)
	c3 := global.Apply(drawCmd.C3)

	su, sv, ok := inverseBasis(p0, p1, p3)
	if !ok {
		return
	}
	cu, cv, ok := inverseBasis(c0, c1, c3)
	if !ok {
		return
	}
	clipBBox, ok := quadBounds(c0, c1, c3)
	if !ok {
		return
	}

	minX := clampInt(int(math.Floor(clipBBox.minX)), 0, dst.Bounds().Dx()-1)
	maxX := clampInt(int(math.Ceil(clipBBox.maxX)), 0, dst.Bounds().Dx()-1)
	minY := clampInt(int(math.Floor(clipBBox.minY)), 0, dst.Bounds().Dy()-1)
	maxY := clampInt(int(math.Ceil(clipBBox.maxY)), 0, dst.Bounds().Dy()-1)
	if minX > maxX || minY > maxY {
		return
	}

	sb := drawCmd.Img.Bounds()
	sw := sb.Dx()
	sh := sb.Dy()
	if sw <= 0 || sh <= 0 {
		return
	}

	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			px := float64(x) + 0.5
			py := float64(y) + 0.5

			cx, cy := mapPointToUV(model.Point{X: px, Y: py}, c0, cu, cv)
			if cx < 0 || cy < 0 || cx > 1 || cy > 1 {
				continue
			}

			u, v := mapPointToUV(model.Point{X: px, Y: py}, p0, su, sv)
			if u < 0 || v < 0 || u > 1 || v > 1 {
				continue
			}

			src := sampleImageBilinear(drawCmd.Img, sb, u, v)
			src = applyAlpha(src, alpha)
			blendAt(dst, x, y, src)
		}
	}
}

func inverseBasis(p0, p1, p3 model.Point) (model.Point, model.Point, bool) {
	ax := p1.X - p0.X
	ay := p1.Y - p0.Y
	bx := p3.X - p0.X
	by := p3.Y - p0.Y
	det := ax*by - ay*bx
	if math.Abs(det) <= 1e-12 {
		return model.Point{}, model.Point{}, false
	}
	invDet := 1.0 / det
	u := model.Point{X: by * invDet, Y: -bx * invDet}
	v := model.Point{X: -ay * invDet, Y: ax * invDet}
	return u, v, true
}

func mapPointToUV(p, origin, uBasis, vBasis model.Point) (float64, float64) {
	dx := p.X - origin.X
	dy := p.Y - origin.Y
	return dx*uBasis.X + dy*uBasis.Y, dx*vBasis.X + dy*vBasis.Y
}

func quadBounds(p0, p1, p3 model.Point) (bounds, bool) {
	p2 := model.Point{X: p1.X + p3.X - p0.X, Y: p1.Y + p3.Y - p0.Y}
	b := bounds{
		minX: math.Min(math.Min(p0.X, p1.X), math.Min(p2.X, p3.X)),
		minY: math.Min(math.Min(p0.Y, p1.Y), math.Min(p2.Y, p3.Y)),
		maxX: math.Max(math.Max(p0.X, p1.X), math.Max(p2.X, p3.X)),
		maxY: math.Max(math.Max(p0.Y, p1.Y), math.Max(p2.Y, p3.Y)),
	}
	return b, finiteBounds(b)
}

func sampleImageBilinear(src image.Image, sb image.Rectangle, u, v float64) color.NRGBA {
	if u <= 0 {
		u = 0
	} else if u >= 1 {
		u = 1
	}
	if v <= 0 {
		v = 0
	} else if v >= 1 {
		v = 1
	}
	fx := u * float64(sb.Dx()-1)
	fy := v * float64(sb.Dy()-1)

	x0 := int(math.Floor(fx))
	y0 := int(math.Floor(fy))
	x1 := x0 + 1
	y1 := y0 + 1
	if x1 >= sb.Dx() {
		x1 = sb.Dx() - 1
	}
	if y1 >= sb.Dy() {
		y1 = sb.Dy() - 1
	}
	tx := fx - float64(x0)
	ty := fy - float64(y0)

	c00 := color.NRGBAModel.Convert(src.At(sb.Min.X+x0, sb.Min.Y+y0)).(color.NRGBA)
	c10 := color.NRGBAModel.Convert(src.At(sb.Min.X+x1, sb.Min.Y+y0)).(color.NRGBA)
	c01 := color.NRGBAModel.Convert(src.At(sb.Min.X+x0, sb.Min.Y+y1)).(color.NRGBA)
	c11 := color.NRGBAModel.Convert(src.At(sb.Min.X+x1, sb.Min.Y+y1)).(color.NRGBA)

	top := lerpColorNRGBA(c00, c10, tx)
	btm := lerpColorNRGBA(c01, c11, tx)
	return lerpColorNRGBA(top, btm, ty)
}

func lerpColorNRGBA(a, b color.NRGBA, t float64) color.NRGBA {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	return color.NRGBA{
		R: toByte(float64(a.R)*(1-t) + float64(b.R)*t),
		G: toByte(float64(a.G)*(1-t) + float64(b.G)*t),
		B: toByte(float64(a.B)*(1-t) + float64(b.B)*t),
		A: toByte(float64(a.A)*(1-t) + float64(b.A)*t),
	}
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

type strokeSegment struct {
	A          model.Point
	B          model.Point
	vx         float64
	vy         float64
	lenSq      float64
	minX       float64
	minY       float64
	maxX       float64
	maxY       float64
	allowStart bool
	allowEnd   bool
}

type strokeCapKind uint8

const (
	strokeCapKindRound strokeCapKind = iota
	strokeCapKindSquare
)

type strokeCap struct {
	Kind   strokeCapKind
	Center model.Point
	Radius float64
	Poly   []model.Point
}

type strokeJoinKind uint8

const (
	strokeJoinKindRound strokeJoinKind = iota
	strokeJoinKindBevel
	strokeJoinKindMiter
)

type strokeJoin struct {
	Kind   strokeJoinKind
	Center model.Point
	Radius float64
	Poly   []model.Point
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

	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			inside := 0
			for _, s := range msaa4Samples {
				px := float64(x) + s[0]
				py := float64(y) + s[1]
				var hit bool
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
			src.A = uint8(float64(clr.A)*(float64(inside)/float64(len(msaa4Samples))) + 0.5)
			blendAt(img, x, y, src)
		}
	}
}

func strokePath(img *image.NRGBA, path model.Path, clr color.NRGBA, style model.Style) {
	if clr.A == 0 || style.StrokeWidth <= 0 {
		return
	}
	geo, ok := prepareStrokeGeometry(path, style)
	if !ok {
		return
	}
	minX := clampInt(int(math.Floor(geo.bounds.minX)), 0, img.Bounds().Dx()-1)
	maxX := clampInt(int(math.Ceil(geo.bounds.maxX)), 0, img.Bounds().Dx()-1)
	minY := clampInt(int(math.Floor(geo.bounds.minY)), 0, img.Bounds().Dy()-1)
	maxY := clampInt(int(math.Ceil(geo.bounds.maxY)), 0, img.Bounds().Dy()-1)
	if minX > maxX || minY > maxY {
		return
	}

	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			hit := 0
			for _, s := range msaa4Samples {
				p := model.Point{X: float64(x) + s[0], Y: float64(y) + s[1]}
				if pointOnStrokeGeometry(p, geo) {
					hit++
				}
			}
			if hit == 0 {
				continue
			}
			src := clr
			src.A = uint8(float64(clr.A)*(float64(hit)/float64(len(msaa4Samples))) + 0.5)
			blendAt(img, x, y, src)
		}
	}
}

type strokeGeometry struct {
	segments []strokeSegment
	joins    []strokeJoin
	caps     []strokeCap
	half     float64
	halfSq   float64
	bounds   bounds
}

type clipRaster struct {
	edges []edge
	rule  model.FillRule
}

func newClipRaster(path model.Path, rule model.FillRule) *clipRaster {
	edges := closedEdges(path)
	if len(edges) == 0 {
		return nil
	}
	return &clipRaster{
		edges: edges,
		rule:  rule,
	}
}

func (c *clipRaster) contains(px, py float64) bool {
	if c == nil || len(c.edges) == 0 {
		return true
	}
	if c.rule == model.FillRuleEvenOdd {
		return pointInEvenOdd(px, py, c.edges)
	}
	return pointInNonZero(px, py, c.edges)
}

type maskRaster struct {
	edges     []edge
	rule      model.FillRule
	luminance bool
}

func newMaskRaster(path model.Path, rule model.FillRule, luminance bool) *maskRaster {
	edges := closedEdges(path)
	if len(edges) == 0 {
		return nil
	}
	return &maskRaster{
		edges:     edges,
		rule:      rule,
		luminance: luminance,
	}
}

func (m *maskRaster) alphaAt(px, py float64) float64 {
	if m == nil || len(m.edges) == 0 {
		return 1
	}
	var inside bool
	if m.rule == model.FillRuleEvenOdd {
		inside = pointInEvenOdd(px, py, m.edges)
	} else {
		inside = pointInNonZero(px, py, m.edges)
	}
	if !inside {
		return 0
	}
	// Current pure-Go mask implementation uses geometry mask coverage.
	// Luminance/alpha mask channels are intentionally simplified for phase-3 baseline.
	return 1
}

func prepareStrokeGeometry(path model.Path, style model.Style) (strokeGeometry, bool) {
	if style.StrokeWidth <= 0 {
		return strokeGeometry{}, false
	}
	dashed := applyDash(path, style.StrokeDashArray, style.StrokeDashOffset)
	segments := strokeSegments(dashed, style.StrokeLineCap)
	if len(segments) == 0 {
		return strokeGeometry{}, false
	}
	half := style.StrokeWidth * 0.5
	halfSq := half * half
	joins := strokeJoins(dashed, style.StrokeLineJoin, style.StrokeMiterLimit, half)
	caps := strokeCaps(dashed, style.StrokeLineCap, half)

	b, ok := strokeSegmentsBounds(segments, half)
	if !ok {
		return strokeGeometry{}, false
	}
	if jb, ok := strokeJoinBounds(joins); ok {
		b.minX = math.Min(b.minX, jb.minX)
		b.minY = math.Min(b.minY, jb.minY)
		b.maxX = math.Max(b.maxX, jb.maxX)
		b.maxY = math.Max(b.maxY, jb.maxY)
	}
	if cb, ok := strokeCapBounds(caps); ok {
		b.minX = math.Min(b.minX, cb.minX)
		b.minY = math.Min(b.minY, cb.minY)
		b.maxX = math.Max(b.maxX, cb.maxX)
		b.maxY = math.Max(b.maxY, cb.maxY)
	}
	if !finiteBounds(b) {
		return strokeGeometry{}, false
	}

	return strokeGeometry{
		segments: segments,
		joins:    joins,
		caps:     caps,
		half:     half,
		halfSq:   halfSq,
		bounds:   b,
	}, true
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

func strokeSegments(path model.Path, cap model.StrokeLineCap) []strokeSegment {
	out := make([]strokeSegment, 0, 16)
	appendSegment := func(a, b model.Point, allowStart, allowEnd bool) {
		vx := b.X - a.X
		vy := b.Y - a.Y
		out = append(out, strokeSegment{
			A:          a,
			B:          b,
			vx:         vx,
			vy:         vy,
			lenSq:      vx*vx + vy*vy,
			minX:       math.Min(a.X, b.X),
			minY:       math.Min(a.Y, b.Y),
			maxX:       math.Max(a.X, b.X),
			maxY:       math.Max(a.Y, b.Y),
			allowStart: allowStart,
			allowEnd:   allowEnd,
		})
	}
	for _, sp := range path.Subpaths {
		n := len(sp.Points)
		if n < 2 {
			continue
		}
		for i := 1; i < n; i++ {
			allowStart := true
			allowEnd := true
			if cap == model.StrokeLineCapButt {
				allowStart = sp.Closed || i > 1
				allowEnd = sp.Closed || i < n-1
			}
			appendSegment(sp.Points[i-1], sp.Points[i], allowStart, allowEnd)
		}
		if sp.Closed {
			appendSegment(sp.Points[n-1], sp.Points[0], false, false)
		}
	}
	return out
}

func strokeSegmentsBounds(segments []strokeSegment, pad float64) (bounds, bool) {
	if len(segments) == 0 {
		return bounds{}, false
	}
	b := bounds{
		minX: math.Inf(1),
		minY: math.Inf(1),
		maxX: math.Inf(-1),
		maxY: math.Inf(-1),
	}
	for _, s := range segments {
		b.minX = math.Min(b.minX, s.minX)
		b.minY = math.Min(b.minY, s.minY)
		b.maxX = math.Max(b.maxX, s.maxX)
		b.maxY = math.Max(b.maxY, s.maxY)
	}
	b.minX -= pad
	b.minY -= pad
	b.maxX += pad
	b.maxY += pad
	return b, finiteBounds(b)
}

func strokeCaps(path model.Path, cap model.StrokeLineCap, half float64) []strokeCap {
	if half <= 0 || cap == model.StrokeLineCapButt {
		return nil
	}
	out := make([]strokeCap, 0, 8)
	for _, sp := range path.Subpaths {
		if sp.Closed || len(sp.Points) < 2 {
			continue
		}
		start := sp.Points[0]
		next := sp.Points[1]
		end := sp.Points[len(sp.Points)-1]
		prev := sp.Points[len(sp.Points)-2]
		if cap == model.StrokeLineCapRound {
			out = append(out, strokeCap{Kind: strokeCapKindRound, Center: start, Radius: half})
			out = append(out, strokeCap{Kind: strokeCapKindRound, Center: end, Radius: half})
			continue
		}
		if poly, ok := squareCapPolygon(start, next, half, true); ok {
			out = append(out, strokeCap{Kind: strokeCapKindSquare, Poly: poly})
		}
		if poly, ok := squareCapPolygon(end, prev, half, false); ok {
			out = append(out, strokeCap{Kind: strokeCapKindSquare, Poly: poly})
		}
	}
	return out
}

func squareCapPolygon(endpoint, neighbor model.Point, half float64, start bool) ([]model.Point, bool) {
	dx := neighbor.X - endpoint.X
	dy := neighbor.Y - endpoint.Y
	if !start {
		dx = endpoint.X - neighbor.X
		dy = endpoint.Y - neighbor.Y
	}
	l := math.Hypot(dx, dy)
	if l <= 1e-9 {
		return nil, false
	}
	ux, uy := dx/l, dy/l
	nx, ny := -uy, ux
	cx, cy := endpoint.X, endpoint.Y
	if start {
		cx -= ux * half
		cy -= uy * half
	} else {
		cx += ux * half
		cy += uy * half
	}
	p0 := model.Point{X: cx + nx*half, Y: cy + ny*half}
	p1 := model.Point{X: cx - nx*half, Y: cy - ny*half}
	p2 := model.Point{X: p1.X + ux*half, Y: p1.Y + uy*half}
	p3 := model.Point{X: p0.X + ux*half, Y: p0.Y + uy*half}
	return []model.Point{p0, p1, p2, p3}, true
}

func strokeCapBounds(caps []strokeCap) (bounds, bool) {
	if len(caps) == 0 {
		return bounds{}, false
	}
	b := bounds{minX: math.Inf(1), minY: math.Inf(1), maxX: math.Inf(-1), maxY: math.Inf(-1)}
	seen := false
	for _, c := range caps {
		if c.Kind == strokeCapKindRound {
			b.minX = math.Min(b.minX, c.Center.X-c.Radius)
			b.minY = math.Min(b.minY, c.Center.Y-c.Radius)
			b.maxX = math.Max(b.maxX, c.Center.X+c.Radius)
			b.maxY = math.Max(b.maxY, c.Center.Y+c.Radius)
			seen = true
			continue
		}
		for _, p := range c.Poly {
			b.minX = math.Min(b.minX, p.X)
			b.minY = math.Min(b.minY, p.Y)
			b.maxX = math.Max(b.maxX, p.X)
			b.maxY = math.Max(b.maxY, p.Y)
			seen = true
		}
	}
	if !seen {
		return bounds{}, false
	}
	return b, finiteBounds(b)
}

func strokeJoins(path model.Path, join model.StrokeLineJoin, miterLimit, half float64) []strokeJoin {
	if half <= 0 {
		return nil
	}
	out := make([]strokeJoin, 0, 16)
	for _, sp := range path.Subpaths {
		n := len(sp.Points)
		if n < 3 {
			continue
		}
		start := 1
		end := n - 1
		if sp.Closed {
			start = 0
			end = n
		}
		for i := start; i < end; i++ {
			prevIdx := i - 1
			nextIdx := i + 1
			if sp.Closed {
				prevIdx = (i - 1 + n) % n
				nextIdx = (i + 1) % n
			}
			p0 := sp.Points[prevIdx]
			p1 := sp.Points[i%n]
			p2 := sp.Points[nextIdx]
			j, ok := buildStrokeJoin(p0, p1, p2, join, miterLimit, half)
			if ok {
				out = append(out, j)
			}
		}
	}
	return out
}

func buildStrokeJoin(p0, p1, p2 model.Point, join model.StrokeLineJoin, miterLimit, half float64) (strokeJoin, bool) {
	v1x := p1.X - p0.X
	v1y := p1.Y - p0.Y
	v2x := p2.X - p1.X
	v2y := p2.Y - p1.Y
	l1 := math.Hypot(v1x, v1y)
	l2 := math.Hypot(v2x, v2y)
	if l1 <= 1e-9 || l2 <= 1e-9 {
		return strokeJoin{}, false
	}
	u1x, u1y := v1x/l1, v1y/l1
	u2x, u2y := v2x/l2, v2y/l2
	cross := u1x*u2y - u1y*u2x
	if math.Abs(cross) <= 1e-9 {
		return strokeJoin{}, false
	}
	n1x, n1y := -u1y, u1x
	n2x, n2y := -u2y, u2x
	side := 1.0
	if cross < 0 {
		side = -1
	}
	n1x *= side
	n1y *= side
	n2x *= side
	n2y *= side

	a := model.Point{X: p1.X + n1x*half, Y: p1.Y + n1y*half}
	b := model.Point{X: p1.X + n2x*half, Y: p1.Y + n2y*half}

	switch join {
	case model.StrokeLineJoinRound:
		return strokeJoin{Kind: strokeJoinKindRound, Center: p1, Radius: half}, true
	case model.StrokeLineJoinBevel:
		return strokeJoin{Kind: strokeJoinKindBevel, Poly: []model.Point{p1, a, b}}, true
	default:
		m1 := model.Point{X: a.X + u1x, Y: a.Y + u1y}
		m2 := model.Point{X: b.X + u2x, Y: b.Y + u2y}
		miter, ok := lineIntersection(a, m1, b, m2)
		if !ok {
			return strokeJoin{Kind: strokeJoinKindBevel, Poly: []model.Point{p1, a, b}}, true
		}
		if miterLimit < 1 {
			miterLimit = 1
		}
		if math.Hypot(miter.X-p1.X, miter.Y-p1.Y) > miterLimit*half {
			return strokeJoin{Kind: strokeJoinKindBevel, Poly: []model.Point{p1, a, b}}, true
		}
		return strokeJoin{Kind: strokeJoinKindMiter, Poly: []model.Point{a, miter, b}}, true
	}
}

func strokeJoinBounds(joins []strokeJoin) (bounds, bool) {
	if len(joins) == 0 {
		return bounds{}, false
	}
	b := bounds{minX: math.Inf(1), minY: math.Inf(1), maxX: math.Inf(-1), maxY: math.Inf(-1)}
	seen := false
	for _, j := range joins {
		if j.Kind == strokeJoinKindRound {
			b.minX = math.Min(b.minX, j.Center.X-j.Radius)
			b.minY = math.Min(b.minY, j.Center.Y-j.Radius)
			b.maxX = math.Max(b.maxX, j.Center.X+j.Radius)
			b.maxY = math.Max(b.maxY, j.Center.Y+j.Radius)
			seen = true
			continue
		}
		for _, p := range j.Poly {
			b.minX = math.Min(b.minX, p.X)
			b.minY = math.Min(b.minY, p.Y)
			b.maxX = math.Max(b.maxX, p.X)
			b.maxY = math.Max(b.maxY, p.Y)
			seen = true
		}
	}
	if !seen {
		return bounds{}, false
	}
	return b, finiteBounds(b)
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

func pointOnStrokeGeometry(p model.Point, geo strokeGeometry) bool {
	for _, s := range geo.segments {
		if p.X < s.minX-geo.half || p.X > s.maxX+geo.half || p.Y < s.minY-geo.half || p.Y > s.maxY+geo.half {
			continue
		}
		if distToStrokeSegmentSqWithCaps(p, s, geo.half) <= geo.halfSq {
			return true
		}
	}
	for _, c := range geo.caps {
		switch c.Kind {
		case strokeCapKindRound:
			dx := p.X - c.Center.X
			dy := p.Y - c.Center.Y
			if dx*dx+dy*dy <= c.Radius*c.Radius {
				return true
			}
		case strokeCapKindSquare:
			if pointInConvexPolygon(p, c.Poly) {
				return true
			}
		}
	}
	for _, j := range geo.joins {
		if j.Kind == strokeJoinKindRound {
			dx := p.X - j.Center.X
			dy := p.Y - j.Center.Y
			if dx*dx+dy*dy <= j.Radius*j.Radius {
				return true
			}
			continue
		}
		if pointInConvexPolygon(p, j.Poly) {
			return true
		}
	}
	return false
}

func distToStrokeSegmentSqWithCaps(p model.Point, s strokeSegment, half float64) float64 {
	if s.lenSq <= 1e-12 {
		dx := p.X - s.A.X
		dy := p.Y - s.A.Y
		return dx*dx + dy*dy
	}
	t := ((p.X-s.A.X)*s.vx + (p.Y-s.A.Y)*s.vy) / s.lenSq
	if t < 0 {
		if !s.allowStart {
			return math.Inf(1)
		}
		t = 0
	}
	if t > 1 {
		if !s.allowEnd {
			return math.Inf(1)
		}
		t = 1
	}
	cx := s.A.X + t*s.vx
	cy := s.A.Y + t*s.vy
	dx := p.X - cx
	dy := p.Y - cy
	return dx*dx + dy*dy
}

func lineIntersection(a1, a2, b1, b2 model.Point) (model.Point, bool) {
	dax := a2.X - a1.X
	day := a2.Y - a1.Y
	dbx := b2.X - b1.X
	dby := b2.Y - b1.Y
	den := dax*dby - day*dbx
	if math.Abs(den) <= 1e-12 {
		return model.Point{}, false
	}
	dx := b1.X - a1.X
	dy := b1.Y - a1.Y
	t := (dx*dby - dy*dbx) / den
	return model.Point{X: a1.X + t*dax, Y: a1.Y + t*day}, true
}

func pointInConvexPolygon(p model.Point, poly []model.Point) bool {
	if len(poly) < 3 {
		return false
	}
	sign := 0
	for i := 0; i < len(poly); i++ {
		a := poly[i]
		b := poly[(i+1)%len(poly)]
		cross := (b.X-a.X)*(p.Y-a.Y) - (b.Y-a.Y)*(p.X-a.X)
		if math.Abs(cross) <= 1e-12 {
			continue
		}
		if cross > 0 {
			if sign < 0 {
				return false
			}
			sign = 1
		} else {
			if sign > 0 {
				return false
			}
			sign = -1
		}
	}
	return true
}

func applyDash(path model.Path, dash []float64, offset float64) model.Path {
	pattern := normalizeDashPattern(dash)
	if len(pattern) == 0 {
		return path
	}
	period := 0.0
	for _, v := range pattern {
		period += v
	}
	if period <= 1e-9 {
		return path
	}
	offset = math.Mod(offset, period)
	if offset < 0 {
		offset += period
	}
	startIdx := 0
	remain := pattern[0]
	for offset > 0 {
		if offset < remain {
			remain -= offset
			offset = 0
			break
		}
		offset -= remain
		startIdx = (startIdx + 1) % len(pattern)
		remain = pattern[startIdx]
	}

	out := model.Path{Subpaths: make([]model.Subpath, 0, len(path.Subpaths))}
	for _, sp := range path.Subpaths {
		if len(sp.Points) < 2 {
			continue
		}
		patIdx := startIdx
		patRemain := remain
		patOn := patIdx%2 == 0

		var cur *model.Subpath
		flush := func() {
			if cur != nil && len(cur.Points) >= 2 {
				out.Subpaths = append(out.Subpaths, *cur)
			}
			cur = nil
		}
		appendPoint := func(p model.Point) {
			if cur == nil {
				cur = &model.Subpath{Closed: false, Points: []model.Point{p}}
				return
			}
			last := cur.Points[len(cur.Points)-1]
			if last.Equal(p) {
				return
			}
			cur.Points = append(cur.Points, p)
		}

		emitEdge := func(a, b model.Point) {
			dx := b.X - a.X
			dy := b.Y - a.Y
			L := math.Hypot(dx, dy)
			if L <= 1e-12 {
				return
			}
			pos := 0.0
			for pos < L {
				step := patRemain
				if step > L-pos {
					step = L - pos
				}
				t1 := pos / L
				t2 := (pos + step) / L
				p1 := model.Point{X: a.X + dx*t1, Y: a.Y + dy*t1}
				p2 := model.Point{X: a.X + dx*t2, Y: a.Y + dy*t2}
				if patOn {
					appendPoint(p1)
					appendPoint(p2)
				} else {
					flush()
				}
				pos += step
				patRemain -= step
				if patRemain <= 1e-9 {
					patIdx = (patIdx + 1) % len(pattern)
					patRemain = pattern[patIdx]
					patOn = patIdx%2 == 0
				}
			}
		}

		for i := 1; i < len(sp.Points); i++ {
			emitEdge(sp.Points[i-1], sp.Points[i])
		}
		if sp.Closed {
			emitEdge(sp.Points[len(sp.Points)-1], sp.Points[0])
		}
		flush()
	}

	return out
}

func normalizeDashPattern(dash []float64) []float64 {
	if len(dash) == 0 {
		return nil
	}
	out := make([]float64, 0, len(dash))
	for _, v := range dash {
		if v > 1e-9 {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return nil
	}
	if len(out)%2 == 1 {
		dup := make([]float64, 0, len(out)*2)
		dup = append(dup, out...)
		dup = append(dup, out...)
		out = dup
	}
	return out
}

func scaleStrokeStyle(style *model.Style, scale float64) {
	if style == nil || scale == 1 {
		return
	}
	style.StrokeWidth *= scale
	if style.StrokeDashOffset != 0 {
		style.StrokeDashOffset *= scale
	}
	if len(style.StrokeDashArray) > 0 {
		scaled := make([]float64, len(style.StrokeDashArray))
		for i, v := range style.StrokeDashArray {
			scaled[i] = v * scale
		}
		style.StrokeDashArray = scaled
	}
}

func compositeWithFilters(dst, src *image.NRGBA, clip *clipRaster, mask *maskRaster) {
	if dst == nil || src == nil {
		return
	}
	b := dst.Bounds().Intersect(src.Bounds())
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if clip != nil && !clip.contains(float64(x)+0.5, float64(y)+0.5) {
				continue
			}
			i := src.PixOffset(x, y)
			a := src.Pix[i+3]
			if a == 0 {
				continue
			}
			if mask != nil {
				ma := mask.alphaAt(float64(x)+0.5, float64(y)+0.5)
				if ma <= 0 {
					continue
				}
				a = uint8(float64(a)*ma + 0.5)
				if a == 0 {
					continue
				}
			}
			c := color.NRGBA{
				R: src.Pix[i+0],
				G: src.Pix[i+1],
				B: src.Pix[i+2],
				A: a,
			}
			blendAt(dst, x, y, c)
		}
	}
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
