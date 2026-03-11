package raster

import (
	"image"
	"image/color"
	"math"

	"github.com/iamshiqing/svgoimg/internal/model"
)

func fillPathGradient(img *image.NRGBA, path model.Path, g model.Gradient, alpha float64, rule model.FillRule) {
	edges := closedEdges(path)
	if len(edges) == 0 {
		return
	}
	paintBounds, ok := pathBounds(path)
	if !ok {
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
	sampler := newGradientSampler(g, paintBounds)

	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			inside := 0
			for _, s := range msaa4Samples {
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
			coverage := float64(inside) / float64(len(msaa4Samples))
			c := sampler.sample(model.Point{X: float64(x) + 0.5, Y: float64(y) + 0.5})
			src := applyAlpha(c, alpha*coverage)
			blendAt(img, x, y, src)
		}
	}
}

func strokePathGradient(img *image.NRGBA, path model.Path, g model.Gradient, alpha float64, width float64) {
	if width <= 0 {
		return
	}
	segments := strokeSegments(path)
	if len(segments) == 0 {
		return
	}
	paintBounds, ok := pathBounds(path)
	if !ok {
		return
	}

	half := width * 0.5
	halfSq := half * half

	b, ok := strokeSegmentsBounds(segments, half)
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
	sampler := newGradientSampler(g, paintBounds)

	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			hit := 0
			for _, s := range msaa4Samples {
				px := float64(x) + s[0]
				py := float64(y) + s[1]
				p := model.Point{X: px, Y: py}
				if pointOnStroke(p, segments, half, halfSq) {
					hit++
				}
			}
			if hit == 0 {
				continue
			}
			coverage := float64(hit) / float64(len(msaa4Samples))
			c := sampler.sample(model.Point{X: float64(x) + 0.5, Y: float64(y) + 0.5})
			src := applyAlpha(c, alpha*coverage)
			blendAt(img, x, y, src)
		}
	}
}

type gradientSampler struct {
	kind   model.GradientKind
	spread model.GradientSpread
	stops  []model.GradientStop
	hasInv bool
	inv    model.Matrix
	x1     float64
	y1     float64
	dx     float64
	dy     float64
	den    float64
	cx     float64
	cy     float64
	r      float64
	fx     float64
	fy     float64
}

func newGradientSampler(g model.Gradient, b bounds) gradientSampler {
	s := gradientSampler{
		kind:   g.Kind,
		spread: g.Spread,
		stops:  g.Stops,
	}
	if g.Transform != model.IdentityMatrix {
		if inv, ok := g.Transform.Inverse(); ok {
			s.hasInv = true
			s.inv = inv
		}
	}
	if g.Kind == model.GradientKindLinear {
		x1, y1, x2, y2 := linearGradientPoints(g, b)
		s.x1 = x1
		s.y1 = y1
		s.dx = x2 - x1
		s.dy = y2 - y1
		s.den = s.dx*s.dx + s.dy*s.dy
	} else {
		s.cx, s.cy, s.r, s.fx, s.fy = radialGradientParams(g, b)
	}
	return s
}

func (s gradientSampler) sample(p model.Point) color.NRGBA {
	gp := p
	if s.hasInv {
		gp = s.inv.Apply(p)
	}

	t := 0.0
	if s.kind == model.GradientKindLinear {
		if s.den > 1e-12 {
			t = ((gp.X-s.x1)*s.dx + (gp.Y-s.y1)*s.dy) / s.den
		}
	} else {
		t = radialGradientT(gp.X, gp.Y, s.cx, s.cy, s.r, s.fx, s.fy)
	}
	t = spreadT(t, s.spread)
	return colorAtStops(s.stops, t)
}

func linearGradientPoints(g model.Gradient, b bounds) (x1, y1, x2, y2 float64) {
	if g.Units == model.GradientUnitsUserSpaceOnUse {
		return g.X1, g.Y1, g.X2, g.Y2
	}
	w := b.maxX - b.minX
	h := b.maxY - b.minY
	return b.minX + g.X1*w,
		b.minY + g.Y1*h,
		b.minX + g.X2*w,
		b.minY + g.Y2*h
}

func radialGradientParams(g model.Gradient, b bounds) (cx, cy, r, fx, fy float64) {
	if g.Units == model.GradientUnitsUserSpaceOnUse {
		return g.CX, g.CY, g.R, g.FX, g.FY
	}
	w := b.maxX - b.minX
	h := b.maxY - b.minY
	minWH := w
	if h < minWH {
		minWH = h
	}
	if minWH <= 0 {
		minWH = 1
	}
	return b.minX + g.CX*w,
		b.minY + g.CY*h,
		g.R * minWH,
		b.minX + g.FX*w,
		b.minY + g.FY*h
}

func radialGradientT(px, py, cx, cy, r, fx, fy float64) float64 {
	if r <= 1e-12 {
		return 1
	}

	vx := px - fx
	vy := py - fy
	if vx == 0 && vy == 0 {
		return 0
	}

	dx := fx - cx
	dy := fy - cy

	a := vx*vx + vy*vy
	b := 2 * (vx*dx + vy*dy)
	c := dx*dx + dy*dy - r*r

	disc := b*b - 4*a*c
	if disc < 0 {
		return math.Hypot(px-cx, py-cy) / r
	}

	sd := math.Sqrt(disc)
	t1 := (-b + sd) / (2 * a)
	t2 := (-b - sd) / (2 * a)
	tHit := math.Max(t1, t2)
	if tHit <= 1e-12 {
		tHit = math.Min(t1, t2)
	}
	if tHit <= 1e-12 {
		return math.Hypot(px-cx, py-cy) / r
	}
	return 1.0 / tHit
}

func spreadT(t float64, spread model.GradientSpread) float64 {
	switch spread {
	case model.GradientSpreadRepeat:
		t = t - math.Floor(t)
		if t < 0 {
			t += 1
		}
		return t
	case model.GradientSpreadReflect:
		t = math.Mod(t, 2)
		if t < 0 {
			t += 2
		}
		if t > 1 {
			t = 2 - t
		}
		return t
	default:
		if t < 0 {
			return 0
		}
		if t > 1 {
			return 1
		}
		return t
	}
}

func colorAtStops(stops []model.GradientStop, t float64) color.NRGBA {
	if len(stops) == 0 {
		return color.NRGBA{}
	}
	if t <= stops[0].Offset {
		return stops[0].Color
	}
	last := stops[len(stops)-1]
	if t >= last.Offset {
		return last.Color
	}

	for i := 1; i < len(stops); i++ {
		a := stops[i-1]
		b := stops[i]
		if t > b.Offset {
			continue
		}
		span := b.Offset - a.Offset
		if span <= 1e-12 {
			return b.Color
		}
		u := (t - a.Offset) / span
		return lerpColor(a.Color, b.Color, u)
	}
	return last.Color
}

func lerpColor(a, b color.NRGBA, t float64) color.NRGBA {
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

func pathBounds(path model.Path) (bounds, bool) {
	b := bounds{
		minX: math.Inf(1),
		minY: math.Inf(1),
		maxX: math.Inf(-1),
		maxY: math.Inf(-1),
	}
	seen := false
	for _, sp := range path.Subpaths {
		for _, pt := range sp.Points {
			if !finite(pt.X) || !finite(pt.Y) {
				continue
			}
			if !seen {
				b.minX, b.maxX = pt.X, pt.X
				b.minY, b.maxY = pt.Y, pt.Y
				seen = true
				continue
			}
			if pt.X < b.minX {
				b.minX = pt.X
			}
			if pt.X > b.maxX {
				b.maxX = pt.X
			}
			if pt.Y < b.minY {
				b.minY = pt.Y
			}
			if pt.Y > b.maxY {
				b.maxY = pt.Y
			}
		}
	}
	if !seen {
		return bounds{}, false
	}
	if b.maxX-b.minX <= 1e-12 {
		b.maxX = b.minX + 1
	}
	if b.maxY-b.minY <= 1e-12 {
		b.maxY = b.minY + 1
	}
	return b, true
}
