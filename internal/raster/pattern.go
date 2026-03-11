package raster

import (
	"image"
	"image/color"
	"math"

	"github.com/iamshiqing/svgoimg/internal/model"
)

func fillPathPattern(img *image.NRGBA, path model.Path, pat model.Pattern, alpha float64, rule model.FillRule) {
	edges := closedEdges(path)
	if len(edges) == 0 {
		return
	}
	paintBounds, ok := pathBounds(path)
	if !ok {
		return
	}
	sampler, ok := newPatternSampler(pat, paintBounds)
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
			coverage := float64(inside) / float64(len(msaa4Samples))
			c := sampler.sample(model.Point{X: float64(x) + 0.5, Y: float64(y) + 0.5})
			src := applyAlpha(c, alpha*coverage)
			blendAt(img, x, y, src)
		}
	}
}

func strokePathPattern(img *image.NRGBA, path model.Path, pat model.Pattern, alpha float64, style model.Style) {
	geo, ok := prepareStrokeGeometry(path, style)
	if !ok {
		return
	}
	paintBounds, ok := pathBounds(path)
	if !ok {
		return
	}
	sampler, ok := newPatternSampler(pat, paintBounds)
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
			coverage := float64(hit) / float64(len(msaa4Samples))
			c := sampler.sample(model.Point{X: float64(x) + 0.5, Y: float64(y) + 0.5})
			src := applyAlpha(c, alpha*coverage)
			blendAt(img, x, y, src)
		}
	}
}

type patternOpKind uint8

const (
	patternOpFill patternOpKind = iota
	patternOpStroke
)

type patternOp struct {
	kind  patternOpKind
	color color.NRGBA
	alpha float64

	edges []edge
	rule  model.FillRule
	geo   strokeGeometry
}

type patternSampler struct {
	x      float64
	y      float64
	w      float64
	h      float64
	hasInv bool
	inv    model.Matrix
	ops    []patternOp
}

func newPatternSampler(pat model.Pattern, obj bounds) (patternSampler, bool) {
	x, y, w, h := patternTileRect(pat, obj)
	if w <= 1e-12 || h <= 1e-12 {
		return patternSampler{}, false
	}
	s := patternSampler{
		x:   x,
		y:   y,
		w:   w,
		h:   h,
		ops: make([]patternOp, 0, len(pat.Commands)*2),
	}
	if pat.Transform != model.IdentityMatrix {
		if inv, ok := pat.Transform.Inverse(); ok {
			s.hasInv = true
			s.inv = inv
		}
	}
	for _, cmd := range pat.Commands {
		if cmd.Image != nil {
			continue
		}
		st := cmd.Style
		if !st.Fill.None && st.Fill.Kind == model.PaintKindSolid {
			edges := closedEdges(cmd.Path)
			if len(edges) > 0 {
				s.ops = append(s.ops, patternOp{
					kind:  patternOpFill,
					color: st.Fill.Color,
					alpha: st.Opacity * st.FillOpacity,
					edges: edges,
					rule:  st.FillRule,
				})
			}
		}
		if !st.Stroke.None && st.Stroke.Kind == model.PaintKindSolid && st.StrokeWidth > 0 {
			geo, ok := prepareStrokeGeometry(cmd.Path, st)
			if ok {
				s.ops = append(s.ops, patternOp{
					kind:  patternOpStroke,
					color: st.Stroke.Color,
					alpha: st.Opacity * st.StrokeOpacity,
					geo:   geo,
				})
			}
		}
	}
	return s, len(s.ops) > 0
}

func patternTileRect(pat model.Pattern, obj bounds) (x, y, w, h float64) {
	if pat.Units == model.PatternUnitsUserSpaceOnUse {
		return pat.X, pat.Y, pat.W, pat.H
	}
	bw := obj.maxX - obj.minX
	bh := obj.maxY - obj.minY
	return obj.minX + pat.X*bw,
		obj.minY + pat.Y*bh,
		pat.W * bw,
		pat.H * bh
}

func (s patternSampler) sample(p model.Point) color.NRGBA {
	gp := p
	if s.hasInv {
		gp = s.inv.Apply(p)
	}
	lx := wrapCoord(gp.X, s.x, s.w)
	ly := wrapCoord(gp.Y, s.y, s.h)
	lp := model.Point{X: lx, Y: ly}

	out := color.NRGBA{}
	for _, op := range s.ops {
		switch op.kind {
		case patternOpFill:
			var hit bool
			if op.rule == model.FillRuleEvenOdd {
				hit = pointInEvenOdd(lp.X, lp.Y, op.edges)
			} else {
				hit = pointInNonZero(lp.X, lp.Y, op.edges)
			}
			if hit {
				src := applyAlpha(op.color, op.alpha)
				out = blendColor(out, src)
			}
		case patternOpStroke:
			if pointOnStrokeGeometry(lp, op.geo) {
				src := applyAlpha(op.color, op.alpha)
				out = blendColor(out, src)
			}
		}
	}
	return out
}

func wrapCoord(v, origin, period float64) float64 {
	if period <= 1e-12 {
		return origin
	}
	u := math.Mod(v-origin, period)
	if u < 0 {
		u += period
	}
	return origin + u
}

func blendColor(dst, src color.NRGBA) color.NRGBA {
	sa := float64(src.A) / 255.0
	if sa <= 0 {
		return dst
	}
	da := float64(dst.A) / 255.0
	outA := sa + da*(1-sa)
	if outA <= 0 {
		return color.NRGBA{}
	}
	outR := (float64(src.R)*sa + float64(dst.R)*da*(1-sa)) / outA
	outG := (float64(src.G)*sa + float64(dst.G)*da*(1-sa)) / outA
	outB := (float64(src.B)*sa + float64(dst.B)*da*(1-sa)) / outA
	return color.NRGBA{
		R: toByte(outR),
		G: toByte(outG),
		B: toByte(outB),
		A: toByte(outA * 255),
	}
}
