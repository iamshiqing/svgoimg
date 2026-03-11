package svg

import (
	"fmt"
	"math"
	"strings"

	"github.com/iamshiqing/svgoimg/internal/model"
)

func parseTransform(raw string) (model.Matrix, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return model.IdentityMatrix, nil
	}
	m := model.IdentityMatrix
	for raw != "" {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			break
		}
		open := strings.IndexByte(raw, '(')
		closeIdx := strings.IndexByte(raw, ')')
		if open <= 0 || closeIdx <= open {
			return model.IdentityMatrix, fmt.Errorf("invalid transform %q", raw)
		}
		name := strings.ToLower(strings.TrimSpace(raw[:open]))
		argStr := raw[open+1 : closeIdx]
		args, err := parseNumberList(argStr)
		if err != nil {
			return model.IdentityMatrix, fmt.Errorf("invalid transform args in %q: %w", raw[:closeIdx+1], err)
		}
		op, err := transformFrom(name, args)
		if err != nil {
			return model.IdentityMatrix, err
		}
		m = m.Then(op)
		raw = strings.TrimSpace(raw[closeIdx+1:])
	}
	return m, nil
}

func transformFrom(name string, args []float64) (model.Matrix, error) {
	switch name {
	case "matrix":
		if len(args) != 6 {
			return model.IdentityMatrix, fmt.Errorf("matrix() expects 6 args")
		}
		return model.Matrix{
			A: args[0],
			B: args[1],
			C: args[2],
			D: args[3],
			E: args[4],
			F: args[5],
		}, nil
	case "translate":
		if len(args) == 1 {
			return model.Translate(args[0], 0), nil
		}
		if len(args) == 2 {
			return model.Translate(args[0], args[1]), nil
		}
		return model.IdentityMatrix, fmt.Errorf("translate() expects 1 or 2 args")
	case "scale":
		if len(args) == 1 {
			return model.Scale(args[0], args[0]), nil
		}
		if len(args) == 2 {
			return model.Scale(args[0], args[1]), nil
		}
		return model.IdentityMatrix, fmt.Errorf("scale() expects 1 or 2 args")
	case "rotate":
		if len(args) == 1 {
			return model.Rotate(args[0] * math.Pi / 180.0), nil
		}
		if len(args) == 3 {
			rot := model.Rotate(args[0] * math.Pi / 180.0)
			return model.Translate(args[1], args[2]).Then(rot).Then(model.Translate(-args[1], -args[2])), nil
		}
		return model.IdentityMatrix, fmt.Errorf("rotate() expects 1 or 3 args")
	case "skewx":
		if len(args) != 1 {
			return model.IdentityMatrix, fmt.Errorf("skewX() expects 1 arg")
		}
		return model.SkewX(args[0] * math.Pi / 180.0), nil
	case "skewy":
		if len(args) != 1 {
			return model.IdentityMatrix, fmt.Errorf("skewY() expects 1 arg")
		}
		return model.SkewY(args[0] * math.Pi / 180.0), nil
	default:
		return model.IdentityMatrix, fmt.Errorf("unsupported transform %q", name)
	}
}

func parseNumberList(raw string) ([]float64, error) {
	s := numberScanner{s: raw}
	var out []float64
	for {
		s.skipDelim()
		if s.eof() {
			break
		}
		v, ok, err := s.readNumber()
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("expected number near %q", s.rest())
		}
		out = append(out, v)
	}
	return out, nil
}

type numberScanner struct {
	s string
	i int
}

func (n *numberScanner) eof() bool {
	return n.i >= len(n.s)
}

func (n *numberScanner) rest() string {
	if n.eof() {
		return ""
	}
	return n.s[n.i:]
}

func (n *numberScanner) skipDelim() {
	for !n.eof() {
		ch := n.s[n.i]
		if ch == ',' || ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			n.i++
			continue
		}
		break
	}
}

func (n *numberScanner) readNumber() (float64, bool, error) {
	start := n.i
	if n.eof() {
		return 0, false, nil
	}
	if n.s[n.i] == '+' || n.s[n.i] == '-' {
		n.i++
	}
	digits := false
	for !n.eof() {
		ch := n.s[n.i]
		if ch >= '0' && ch <= '9' {
			n.i++
			digits = true
			continue
		}
		break
	}
	if !n.eof() && n.s[n.i] == '.' {
		n.i++
		for !n.eof() {
			ch := n.s[n.i]
			if ch >= '0' && ch <= '9' {
				n.i++
				digits = true
				continue
			}
			break
		}
	}
	if !digits {
		n.i = start
		return 0, false, nil
	}
	if !n.eof() && (n.s[n.i] == 'e' || n.s[n.i] == 'E') {
		j := n.i + 1
		if j < len(n.s) && (n.s[j] == '+' || n.s[j] == '-') {
			j++
		}
		hasExpDigits := false
		for j < len(n.s) && n.s[j] >= '0' && n.s[j] <= '9' {
			j++
			hasExpDigits = true
		}
		if hasExpDigits {
			n.i = j
		}
	}
	v, err := parseFloat(n.s[start:n.i])
	if err != nil {
		return 0, false, err
	}
	return v, true, nil
}
