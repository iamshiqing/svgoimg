package svg

import (
	"fmt"
	"math"
	"strings"

	"github.com/iamshiqing/svgoimg/internal/model"
)

func parsePathData(d string, tolerance float64) (model.Path, error) {
	s := numberScanner{s: d}
	var out model.Path

	var cmd byte
	var cur model.Point
	var subStart model.Point
	var hasCur bool

	var lastC2 model.Point
	var hasLastC2 bool
	var lastQ model.Point
	var hasLastQ bool

	startSubpath := func(p model.Point) {
		out.Subpaths = append(out.Subpaths, model.Subpath{
			Points: []model.Point{p},
		})
		subStart = p
		cur = p
		hasCur = true
	}
	appendPoint := func(p model.Point) {
		if len(out.Subpaths) == 0 {
			startSubpath(p)
			return
		}
		sp := &out.Subpaths[len(out.Subpaths)-1]
		if len(sp.Points) > 0 && sp.Points[len(sp.Points)-1].Equal(p) {
			cur = p
			hasCur = true
			return
		}
		sp.Points = append(sp.Points, p)
		cur = p
		hasCur = true
	}
	closeSubpath := func() {
		if len(out.Subpaths) == 0 {
			return
		}
		sp := &out.Subpaths[len(out.Subpaths)-1]
		sp.Closed = true
		cur = subStart
		hasCur = true
	}
	readCoord := func(relative bool) (model.Point, bool, error) {
		s.skipDelim()
		x, ok, err := s.readNumber()
		if err != nil || !ok {
			return model.Point{}, ok, err
		}
		s.skipDelim()
		y, ok, err := s.readNumber()
		if err != nil || !ok {
			if err == nil {
				err = fmt.Errorf("missing y coordinate")
			}
			return model.Point{}, false, err
		}
		p := model.Point{X: x, Y: y}
		if relative {
			p.X += cur.X
			p.Y += cur.Y
		}
		return p, true, nil
	}

	for {
		s.skipDelim()
		if s.eof() {
			break
		}

		ch := s.s[s.i]
		if isPathCmd(ch) {
			cmd = ch
			s.i++
		} else if cmd == 0 {
			return model.Path{}, fmt.Errorf("path must begin with a command")
		}

		rel := cmd >= 'a' && cmd <= 'z'
		switch cmd {
		case 'M', 'm':
			p, ok, err := readCoord(rel)
			if err != nil {
				return model.Path{}, err
			}
			if !ok {
				return model.Path{}, fmt.Errorf("moveto expects coordinates")
			}
			startSubpath(p)
			hasLastC2, hasLastQ = false, false

			// Extra pairs are implicit lineto.
			for {
				pos := s.i
				p, ok, err = readCoord(rel)
				if err != nil {
					return model.Path{}, err
				}
				if !ok {
					s.i = pos
					break
				}
				appendPoint(p)
			}
			if cmd == 'M' {
				cmd = 'L'
			} else {
				cmd = 'l'
			}

		case 'L', 'l':
			for {
				pos := s.i
				p, ok, err := readCoord(rel)
				if err != nil {
					return model.Path{}, err
				}
				if !ok {
					s.i = pos
					break
				}
				appendPoint(p)
			}
			hasLastC2, hasLastQ = false, false

		case 'H', 'h':
			for {
				s.skipDelim()
				pos := s.i
				v, ok, err := s.readNumber()
				if err != nil {
					return model.Path{}, err
				}
				if !ok {
					s.i = pos
					break
				}
				if !hasCur {
					return model.Path{}, fmt.Errorf("horizontal lineto without current point")
				}
				x := v
				if rel {
					x += cur.X
				}
				appendPoint(model.Point{X: x, Y: cur.Y})
			}
			hasLastC2, hasLastQ = false, false

		case 'V', 'v':
			for {
				s.skipDelim()
				pos := s.i
				v, ok, err := s.readNumber()
				if err != nil {
					return model.Path{}, err
				}
				if !ok {
					s.i = pos
					break
				}
				if !hasCur {
					return model.Path{}, fmt.Errorf("vertical lineto without current point")
				}
				y := v
				if rel {
					y += cur.Y
				}
				appendPoint(model.Point{X: cur.X, Y: y})
			}
			hasLastC2, hasLastQ = false, false

		case 'C', 'c':
			for {
				c1, ok, err := readCoord(rel)
				if err != nil {
					return model.Path{}, err
				}
				if !ok {
					break
				}
				c2, ok, err := readCoord(rel)
				if err != nil || !ok {
					if err == nil {
						err = fmt.Errorf("cubic bezier missing second control point")
					}
					return model.Path{}, err
				}
				p, ok, err := readCoord(rel)
				if err != nil || !ok {
					if err == nil {
						err = fmt.Errorf("cubic bezier missing end point")
					}
					return model.Path{}, err
				}
				if !hasCur {
					return model.Path{}, fmt.Errorf("cubic bezier without current point")
				}
				for _, pt := range flattenCubic(cur, c1, c2, p, tolerance) {
					appendPoint(pt)
				}
				lastC2 = c2
				hasLastC2 = true
				hasLastQ = false
			}

		case 'S', 's':
			for {
				c2, ok, err := readCoord(rel)
				if err != nil {
					return model.Path{}, err
				}
				if !ok {
					break
				}
				p, ok, err := readCoord(rel)
				if err != nil || !ok {
					if err == nil {
						err = fmt.Errorf("smooth cubic bezier missing end point")
					}
					return model.Path{}, err
				}
				if !hasCur {
					return model.Path{}, fmt.Errorf("smooth cubic bezier without current point")
				}
				c1 := cur
				if hasLastC2 {
					c1 = reflectPoint(lastC2, cur)
				}
				for _, pt := range flattenCubic(cur, c1, c2, p, tolerance) {
					appendPoint(pt)
				}
				lastC2 = c2
				hasLastC2 = true
				hasLastQ = false
			}

		case 'Q', 'q':
			for {
				c1, ok, err := readCoord(rel)
				if err != nil {
					return model.Path{}, err
				}
				if !ok {
					break
				}
				p, ok, err := readCoord(rel)
				if err != nil || !ok {
					if err == nil {
						err = fmt.Errorf("quadratic bezier missing end point")
					}
					return model.Path{}, err
				}
				if !hasCur {
					return model.Path{}, fmt.Errorf("quadratic bezier without current point")
				}
				for _, pt := range flattenQuadratic(cur, c1, p, tolerance) {
					appendPoint(pt)
				}
				lastQ = c1
				hasLastQ = true
				hasLastC2 = false
			}

		case 'T', 't':
			for {
				p, ok, err := readCoord(rel)
				if err != nil {
					return model.Path{}, err
				}
				if !ok {
					break
				}
				if !hasCur {
					return model.Path{}, fmt.Errorf("smooth quadratic bezier without current point")
				}
				c1 := cur
				if hasLastQ {
					c1 = reflectPoint(lastQ, cur)
				}
				for _, pt := range flattenQuadratic(cur, c1, p, tolerance) {
					appendPoint(pt)
				}
				lastQ = c1
				hasLastQ = true
				hasLastC2 = false
			}

		case 'A', 'a':
			for {
				s.skipDelim()
				rx, ok, err := s.readNumber()
				if err != nil || !ok {
					if err != nil {
						return model.Path{}, err
					}
					break
				}
				s.skipDelim()
				ry, ok, err := s.readNumber()
				if err != nil || !ok {
					if err == nil {
						err = fmt.Errorf("arc missing ry")
					}
					return model.Path{}, err
				}
				s.skipDelim()
				rot, ok, err := s.readNumber()
				if err != nil || !ok {
					if err == nil {
						err = fmt.Errorf("arc missing rotation")
					}
					return model.Path{}, err
				}
				s.skipDelim()
				large, ok, err := s.readNumber()
				if err != nil || !ok {
					if err == nil {
						err = fmt.Errorf("arc missing large-arc flag")
					}
					return model.Path{}, err
				}
				s.skipDelim()
				sweep, ok, err := s.readNumber()
				if err != nil || !ok {
					if err == nil {
						err = fmt.Errorf("arc missing sweep flag")
					}
					return model.Path{}, err
				}
				p, ok, err := readCoord(rel)
				if err != nil || !ok {
					if err == nil {
						err = fmt.Errorf("arc missing end point")
					}
					return model.Path{}, err
				}
				if !hasCur {
					return model.Path{}, fmt.Errorf("arc without current point")
				}
				for _, pt := range flattenArc(cur, rx, ry, rot, large != 0, sweep != 0, p, tolerance) {
					appendPoint(pt)
				}
				hasLastC2 = false
				hasLastQ = false
			}

		case 'Z', 'z':
			closeSubpath()
			hasLastC2, hasLastQ = false, false

		default:
			return model.Path{}, fmt.Errorf("unsupported path command %q", string(cmd))
		}
	}

	return out, nil
}

func isPathCmd(ch byte) bool {
	return strings.IndexByte("MmLlHhVvCcSsQqTtAaZz", ch) >= 0
}

func reflectPoint(p, center model.Point) model.Point {
	return model.Point{
		X: center.X*2 - p.X,
		Y: center.Y*2 - p.Y,
	}
}

func flattenCubic(p0, p1, p2, p3 model.Point, tolerance float64) []model.Point {
	if tolerance <= 0 {
		tolerance = 0.6
	}
	approx := dist(p0, p1) + dist(p1, p2) + dist(p2, p3)
	steps := int(math.Ceil(approx / tolerance))
	if steps < 4 {
		steps = 4
	}
	if steps > 256 {
		steps = 256
	}
	out := make([]model.Point, 0, steps)
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		out = append(out, cubicPoint(p0, p1, p2, p3, t))
	}
	return out
}

func cubicPoint(p0, p1, p2, p3 model.Point, t float64) model.Point {
	u := 1 - t
	tt, uu := t*t, u*u
	uuu := uu * u
	ttt := tt * t
	return model.Point{
		X: uuu*p0.X + 3*uu*t*p1.X + 3*u*tt*p2.X + ttt*p3.X,
		Y: uuu*p0.Y + 3*uu*t*p1.Y + 3*u*tt*p2.Y + ttt*p3.Y,
	}
}

func flattenQuadratic(p0, p1, p2 model.Point, tolerance float64) []model.Point {
	if tolerance <= 0 {
		tolerance = 0.6
	}
	approx := dist(p0, p1) + dist(p1, p2)
	steps := int(math.Ceil(approx / tolerance))
	if steps < 3 {
		steps = 3
	}
	if steps > 192 {
		steps = 192
	}
	out := make([]model.Point, 0, steps)
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		out = append(out, quadraticPoint(p0, p1, p2, t))
	}
	return out
}

func quadraticPoint(p0, p1, p2 model.Point, t float64) model.Point {
	u := 1 - t
	return model.Point{
		X: u*u*p0.X + 2*u*t*p1.X + t*t*p2.X,
		Y: u*u*p0.Y + 2*u*t*p1.Y + t*t*p2.Y,
	}
}

func flattenArc(from model.Point, rx, ry, xAxisDeg float64, largeArc, sweep bool, to model.Point, tolerance float64) []model.Point {
	rx = math.Abs(rx)
	ry = math.Abs(ry)
	if rx == 0 || ry == 0 {
		return []model.Point{to}
	}
	if from.Equal(to) {
		return nil
	}

	phi := xAxisDeg * math.Pi / 180.0
	cphi := math.Cos(phi)
	sphi := math.Sin(phi)

	dx2 := (from.X - to.X) / 2
	dy2 := (from.Y - to.Y) / 2
	x1p := cphi*dx2 + sphi*dy2
	y1p := -sphi*dx2 + cphi*dy2

	lambda := x1p*x1p/(rx*rx) + y1p*y1p/(ry*ry)
	if lambda > 1 {
		scale := math.Sqrt(lambda)
		rx *= scale
		ry *= scale
	}

	num := rx*rx*ry*ry - rx*rx*y1p*y1p - ry*ry*x1p*x1p
	den := rx*rx*y1p*y1p + ry*ry*x1p*x1p
	if den == 0 {
		return []model.Point{to}
	}
	coef := 0.0
	if num > 0 {
		coef = math.Sqrt(num / den)
	}
	if largeArc == sweep {
		coef = -coef
	}

	cxp := coef * (rx * y1p / ry)
	cyp := coef * (-ry * x1p / rx)

	cx := cphi*cxp - sphi*cyp + (from.X+to.X)/2
	cy := sphi*cxp + cphi*cyp + (from.Y+to.Y)/2

	v1x := (x1p - cxp) / rx
	v1y := (y1p - cyp) / ry
	v2x := (-x1p - cxp) / rx
	v2y := (-y1p - cyp) / ry

	start := math.Atan2(v1y, v1x)
	delta := angleBetween(v1x, v1y, v2x, v2y)
	if !sweep && delta > 0 {
		delta -= 2 * math.Pi
	}
	if sweep && delta < 0 {
		delta += 2 * math.Pi
	}

	maxR := math.Max(rx, ry)
	segByTol := int(math.Ceil(math.Abs(delta) * maxR / math.Max(tolerance, 0.5)))
	segByAng := int(math.Ceil(math.Abs(delta) / (math.Pi / 8)))
	segments := segByTol
	if segments < segByAng {
		segments = segByAng
	}
	if segments < 4 {
		segments = 4
	}
	if segments > 256 {
		segments = 256
	}

	out := make([]model.Point, 0, segments)
	for i := 1; i <= segments; i++ {
		t := start + delta*(float64(i)/float64(segments))
		ct := math.Cos(t)
		st := math.Sin(t)
		x := cx + rx*cphi*ct - ry*sphi*st
		y := cy + rx*sphi*ct + ry*cphi*st
		out = append(out, model.Point{X: x, Y: y})
	}
	return out
}

func angleBetween(ux, uy, vx, vy float64) float64 {
	dot := ux*vx + uy*vy
	cross := ux*vy - uy*vx
	return math.Atan2(cross, dot)
}

func dist(a, b model.Point) float64 {
	return math.Hypot(a.X-b.X, a.Y-b.Y)
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

func parsePoints(raw string) ([]model.Point, error) {
	s := numberScanner{s: raw}
	points := make([]model.Point, 0, 8)
	for {
		s.skipDelim()
		if s.eof() {
			break
		}
		x, ok, err := s.readNumber()
		if err != nil || !ok {
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("invalid points list")
		}
		s.skipDelim()
		y, ok, err := s.readNumber()
		if err != nil || !ok {
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("invalid points list")
		}
		points = append(points, model.Point{X: x, Y: y})
	}
	return points, nil
}

func rectPath(x, y, w, h, rx, ry, tolerance float64) model.Path {
	if w <= 0 || h <= 0 {
		return model.Path{}
	}
	if rx < 0 {
		rx = 0
	}
	if ry < 0 {
		ry = 0
	}
	if rx == 0 && ry == 0 {
		return model.Path{
			Subpaths: []model.Subpath{
				{
					Closed: true,
					Points: []model.Point{
						{X: x, Y: y},
						{X: x + w, Y: y},
						{X: x + w, Y: y + h},
						{X: x, Y: y + h},
					},
				},
			},
		}
	}
	if rx == 0 {
		rx = ry
	}
	if ry == 0 {
		ry = rx
	}
	if rx > w/2 {
		rx = w / 2
	}
	if ry > h/2 {
		ry = h / 2
	}

	sp := model.Subpath{
		Closed: true,
		Points: []model.Point{
			{X: x + rx, Y: y},
			{X: x + w - rx, Y: y},
		},
	}
	appendEllipseArc(&sp.Points, x+w-rx, y+ry, rx, ry, -math.Pi/2, 0, tolerance)
	sp.Points = append(sp.Points, model.Point{X: x + w, Y: y + h - ry})
	appendEllipseArc(&sp.Points, x+w-rx, y+h-ry, rx, ry, 0, math.Pi/2, tolerance)
	sp.Points = append(sp.Points, model.Point{X: x + rx, Y: y + h})
	appendEllipseArc(&sp.Points, x+rx, y+h-ry, rx, ry, math.Pi/2, math.Pi, tolerance)
	sp.Points = append(sp.Points, model.Point{X: x, Y: y + ry})
	appendEllipseArc(&sp.Points, x+rx, y+ry, rx, ry, math.Pi, 3*math.Pi/2, tolerance)

	return model.Path{Subpaths: []model.Subpath{sp}}
}

func circlePath(cx, cy, r, tolerance float64) model.Path {
	return ellipsePath(cx, cy, r, r, tolerance)
}

func ellipsePath(cx, cy, rx, ry, tolerance float64) model.Path {
	if rx <= 0 || ry <= 0 {
		return model.Path{}
	}
	steps := int(math.Ceil(2 * math.Pi * math.Max(rx, ry) / math.Max(tolerance, 0.6)))
	if steps < 24 {
		steps = 24
	}
	if steps > 720 {
		steps = 720
	}
	sp := model.Subpath{
		Closed: true,
		Points: make([]model.Point, 0, steps),
	}
	for i := 0; i < steps; i++ {
		a := 2 * math.Pi * float64(i) / float64(steps)
		sp.Points = append(sp.Points, model.Point{
			X: cx + rx*math.Cos(a),
			Y: cy + ry*math.Sin(a),
		})
	}
	return model.Path{Subpaths: []model.Subpath{sp}}
}

func polylinePath(points []model.Point, closed bool) model.Path {
	if len(points) < 2 {
		return model.Path{}
	}
	return model.Path{
		Subpaths: []model.Subpath{
			{
				Closed: closed,
				Points: points,
			},
		},
	}
}

func linePath(x1, y1, x2, y2 float64) model.Path {
	return polylinePath([]model.Point{
		{X: x1, Y: y1},
		{X: x2, Y: y2},
	}, false)
}

func appendEllipseArc(points *[]model.Point, cx, cy, rx, ry, start, end, tolerance float64) {
	angle := math.Abs(end - start)
	if angle == 0 {
		return
	}
	steps := int(math.Ceil(angle * math.Max(rx, ry) / math.Max(tolerance, 0.6)))
	if steps < 2 {
		steps = 2
	}
	if steps > 128 {
		steps = 128
	}
	for i := 1; i <= steps; i++ {
		t := start + (end-start)*(float64(i)/float64(steps))
		*points = append(*points, model.Point{
			X: cx + rx*math.Cos(t),
			Y: cy + ry*math.Sin(t),
		})
	}
}
