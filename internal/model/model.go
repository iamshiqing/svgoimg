package model

import (
	"image"
	"image/color"
	"math"
)

type Scene struct {
	Width     float64
	Height    float64
	ViewBox   Rect
	Commands  []Command
	Gradients map[string]Gradient
	Patterns  map[string]Pattern
}

type Command struct {
	Path  Path
	Style Style
	Image *ImageDraw
	Clip  *ClipPath
	Mask  *MaskRef
}

type ImageDraw struct {
	Img     image.Image
	Opacity float64
	// Content parallelogram (top-left, top-right, bottom-left) in scene coordinates.
	P0 Point
	P1 Point
	P3 Point
	// Viewport clip parallelogram (top-left, top-right, bottom-left) in scene coordinates.
	C0 Point
	C1 Point
	C3 Point
}

type ClipPath struct {
	Path Path
	Rule FillRule
}

type MaskRef struct {
	Commands  []Command
	Luminance bool
}

type Path struct {
	Subpaths []Subpath
}

type Subpath struct {
	Points []Point
	Closed bool
}

type Rect struct {
	X float64
	Y float64
	W float64
	H float64
}

func (r Rect) IsValid() bool {
	return r.W > 0 && r.H > 0 && !math.IsNaN(r.W) && !math.IsNaN(r.H)
}

type Point struct {
	X float64
	Y float64
}

func (p Point) Equal(o Point) bool {
	return p.X == o.X && p.Y == o.Y
}

type Matrix struct {
	A float64
	B float64
	C float64
	D float64
	E float64
	F float64
}

var IdentityMatrix = Matrix{
	A: 1,
	D: 1,
}

func (m Matrix) Apply(p Point) Point {
	return Point{
		X: m.A*p.X + m.C*p.Y + m.E,
		Y: m.B*p.X + m.D*p.Y + m.F,
	}
}

// Then composes two transforms in draw order:
// first apply m, then apply n.
func (m Matrix) Then(n Matrix) Matrix {
	return Matrix{
		A: n.A*m.A + n.C*m.B,
		B: n.B*m.A + n.D*m.B,
		C: n.A*m.C + n.C*m.D,
		D: n.B*m.C + n.D*m.D,
		E: n.A*m.E + n.C*m.F + n.E,
		F: n.B*m.E + n.D*m.F + n.F,
	}
}

func Translate(tx, ty float64) Matrix {
	return Matrix{
		A: 1,
		D: 1,
		E: tx,
		F: ty,
	}
}

func Scale(sx, sy float64) Matrix {
	return Matrix{
		A: sx,
		D: sy,
	}
}

func Rotate(rad float64) Matrix {
	s, c := math.Sin(rad), math.Cos(rad)
	return Matrix{
		A: c,
		B: s,
		C: -s,
		D: c,
	}
}

func SkewX(rad float64) Matrix {
	return Matrix{
		A: 1,
		C: math.Tan(rad),
		D: 1,
	}
}

func SkewY(rad float64) Matrix {
	return Matrix{
		A: 1,
		B: math.Tan(rad),
		D: 1,
	}
}

func (m Matrix) ApproxScale() float64 {
	sx := math.Hypot(m.A, m.B)
	sy := math.Hypot(m.C, m.D)
	if sx == 0 && sy == 0 {
		return 1
	}
	if sx == 0 {
		return sy
	}
	if sy == 0 {
		return sx
	}
	return (sx + sy) * 0.5
}

func (m Matrix) Inverse() (Matrix, bool) {
	det := m.A*m.D - m.B*m.C
	if math.Abs(det) < 1e-12 || math.IsNaN(det) || math.IsInf(det, 0) {
		return Matrix{}, false
	}
	invDet := 1.0 / det
	return Matrix{
		A: m.D * invDet,
		B: -m.B * invDet,
		C: -m.C * invDet,
		D: m.A * invDet,
		E: (m.C*m.F - m.D*m.E) * invDet,
		F: (m.B*m.E - m.A*m.F) * invDet,
	}, true
}

type FillRule uint8

const (
	FillRuleNonZero FillRule = iota
	FillRuleEvenOdd
)

type PaintKind uint8

const (
	PaintKindSolid PaintKind = iota
	PaintKindGradient
	PaintKindPattern
)

type Paint struct {
	None        bool
	Kind        PaintKind
	Color       color.NRGBA
	GradientID  string
	PatternID   string
	HasFallback bool
}

type Style struct {
	Fill             Paint
	Stroke           Paint
	StrokeWidth      float64
	StrokeLineCap    StrokeLineCap
	StrokeLineJoin   StrokeLineJoin
	StrokeMiterLimit float64
	StrokeDashArray  []float64
	StrokeDashOffset float64
	Opacity          float64
	FillOpacity      float64
	StrokeOpacity    float64
	FillRule         FillRule
	Visible          bool
	CurrentColor     color.NRGBA
}

type StrokeLineCap uint8

const (
	StrokeLineCapButt StrokeLineCap = iota
	StrokeLineCapRound
	StrokeLineCapSquare
)

type StrokeLineJoin uint8

const (
	StrokeLineJoinMiter StrokeLineJoin = iota
	StrokeLineJoinRound
	StrokeLineJoinBevel
)

type GradientKind uint8

const (
	GradientKindLinear GradientKind = iota
	GradientKindRadial
)

type GradientUnits uint8

const (
	GradientUnitsObjectBoundingBox GradientUnits = iota
	GradientUnitsUserSpaceOnUse
)

type GradientSpread uint8

const (
	GradientSpreadPad GradientSpread = iota
	GradientSpreadRepeat
	GradientSpreadReflect
)

type GradientStop struct {
	Offset float64
	Color  color.NRGBA
}

type Gradient struct {
	ID        string
	Kind      GradientKind
	Units     GradientUnits
	Spread    GradientSpread
	Transform Matrix
	Stops     []GradientStop

	// Linear
	X1 float64
	Y1 float64
	X2 float64
	Y2 float64

	// Radial
	CX float64
	CY float64
	R  float64
	FX float64
	FY float64
}

type PatternUnits uint8

const (
	PatternUnitsObjectBoundingBox PatternUnits = iota
	PatternUnitsUserSpaceOnUse
)

type Pattern struct {
	ID        string
	Units     PatternUnits
	Transform Matrix
	X         float64
	Y         float64
	W         float64
	H         float64
	Commands  []Command
}

func DefaultStyle() Style {
	return Style{
		Fill: Paint{
			Kind:  PaintKindSolid,
			Color: color.NRGBA{R: 0, G: 0, B: 0, A: 255},
		},
		Stroke: Paint{
			None: true,
			Kind: PaintKindSolid,
		},
		StrokeWidth:      1,
		StrokeLineCap:    StrokeLineCapButt,
		StrokeLineJoin:   StrokeLineJoinMiter,
		StrokeMiterLimit: 4,
		Opacity:          1,
		FillOpacity:      1,
		StrokeOpacity:    1,
		FillRule:         FillRuleNonZero,
		Visible:          true,
		CurrentColor:     color.NRGBA{R: 0, G: 0, B: 0, A: 255},
	}
}
