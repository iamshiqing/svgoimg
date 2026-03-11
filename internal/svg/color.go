package svg

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"github.com/iamshiqing/svgoimg/internal/model"
)

var namedColors = map[string]color.NRGBA{
	"black":   {R: 0, G: 0, B: 0, A: 255},
	"white":   {R: 255, G: 255, B: 255, A: 255},
	"red":     {R: 255, G: 0, B: 0, A: 255},
	"green":   {R: 0, G: 128, B: 0, A: 255},
	"blue":    {R: 0, G: 0, B: 255, A: 255},
	"yellow":  {R: 255, G: 255, B: 0, A: 255},
	"cyan":    {R: 0, G: 255, B: 255, A: 255},
	"magenta": {R: 255, G: 0, B: 255, A: 255},
	"gray":    {R: 128, G: 128, B: 128, A: 255},
	"grey":    {R: 128, G: 128, B: 128, A: 255},
	"orange":  {R: 255, G: 165, B: 0, A: 255},
	"purple":  {R: 128, G: 0, B: 128, A: 255},
	"brown":   {R: 165, G: 42, B: 42, A: 255},
	"pink":    {R: 255, G: 192, B: 203, A: 255},
}

func parsePaint(raw string, currentColor color.NRGBA) (model.Paint, error) {
	v := strings.TrimSpace(strings.ToLower(raw))
	if v == "" {
		return model.Paint{None: true}, nil
	}
	if v == "none" {
		return model.Paint{None: true}, nil
	}
	if strings.HasPrefix(v, "url(") {
		p, err := parseURLPaint(v, currentColor)
		if err != nil {
			return model.Paint{}, err
		}
		return p, nil
	}
	c, err := parseColorToken(v, currentColor)
	if err != nil {
		return model.Paint{}, err
	}
	return model.Paint{
		Kind:  model.PaintKindSolid,
		Color: c,
	}, nil
}

func parseURLPaint(raw string, currentColor color.NRGBA) (model.Paint, error) {
	closeIdx := strings.IndexByte(raw, ')')
	if closeIdx < 0 {
		return model.Paint{}, fmt.Errorf("invalid paint url %q", raw)
	}
	ref := strings.TrimSpace(raw[len("url("):closeIdx])
	id := ""
	if strings.HasPrefix(ref, "#") {
		id = strings.TrimPrefix(ref, "#")
	} else if i := strings.IndexByte(ref, '#'); i >= 0 && i < len(ref)-1 {
		id = ref[i+1:]
	}
	if id == "" {
		return model.Paint{}, fmt.Errorf("paint url must reference an id, got %q", raw)
	}

	p := model.Paint{
		Kind:       model.PaintKindGradient,
		GradientID: id,
	}

	// Keep compatibility with common "url(#id) fallback" forms and provide
	// a safe default fallback to currentColor if the gradient can't be resolved.
	p.Color = currentColor
	p.HasFallback = true

	tail := strings.TrimSpace(raw[closeIdx+1:])
	if tail != "" {
		c, err := parseColorToken(tail, currentColor)
		if err != nil {
			return model.Paint{}, err
		}
		p.Color = c
		p.HasFallback = true
	}
	return p, nil
}

func parseColorToken(raw string, currentColor color.NRGBA) (color.NRGBA, error) {
	v := strings.TrimSpace(strings.ToLower(raw))
	if v == "" {
		return color.NRGBA{}, fmt.Errorf("empty color")
	}
	if v == "currentcolor" {
		return currentColor, nil
	}
	if strings.HasPrefix(v, "#") {
		c, _, err := parseHexColor(v)
		return c, err
	}
	if strings.HasPrefix(v, "rgb(") && strings.HasSuffix(v, ")") {
		body := strings.TrimSpace(v[4 : len(v)-1])
		parts := splitCSV(body)
		if len(parts) != 3 {
			return color.NRGBA{}, fmt.Errorf("invalid rgb() value %q", raw)
		}
		r, err := parseRGBPart(parts[0])
		if err != nil {
			return color.NRGBA{}, err
		}
		g, err := parseRGBPart(parts[1])
		if err != nil {
			return color.NRGBA{}, err
		}
		b, err := parseRGBPart(parts[2])
		if err != nil {
			return color.NRGBA{}, err
		}
		return color.NRGBA{R: r, G: g, B: b, A: 255}, nil
	}
	if strings.HasPrefix(v, "rgba(") && strings.HasSuffix(v, ")") {
		body := strings.TrimSpace(v[5 : len(v)-1])
		parts := splitCSV(body)
		if len(parts) != 4 {
			return color.NRGBA{}, fmt.Errorf("invalid rgba() value %q", raw)
		}
		r, err := parseRGBPart(parts[0])
		if err != nil {
			return color.NRGBA{}, err
		}
		g, err := parseRGBPart(parts[1])
		if err != nil {
			return color.NRGBA{}, err
		}
		b, err := parseRGBPart(parts[2])
		if err != nil {
			return color.NRGBA{}, err
		}
		a, err := parseAlphaPart(parts[3])
		if err != nil {
			return color.NRGBA{}, err
		}
		return color.NRGBA{R: r, G: g, B: b, A: a}, nil
	}
	if c, ok := namedColors[v]; ok {
		return c, nil
	}
	return color.NRGBA{}, fmt.Errorf("unsupported color %q", raw)
}

func parseHexColor(v string) (color.NRGBA, bool, error) {
	h := strings.TrimPrefix(strings.TrimSpace(v), "#")
	switch len(h) {
	case 3:
		r, _ := strconv.ParseUint(strings.Repeat(string(h[0]), 2), 16, 8)
		g, _ := strconv.ParseUint(strings.Repeat(string(h[1]), 2), 16, 8)
		b, _ := strconv.ParseUint(strings.Repeat(string(h[2]), 2), 16, 8)
		return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}, false, nil
	case 4:
		r, _ := strconv.ParseUint(strings.Repeat(string(h[0]), 2), 16, 8)
		g, _ := strconv.ParseUint(strings.Repeat(string(h[1]), 2), 16, 8)
		b, _ := strconv.ParseUint(strings.Repeat(string(h[2]), 2), 16, 8)
		a, _ := strconv.ParseUint(strings.Repeat(string(h[3]), 2), 16, 8)
		return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: uint8(a)}, false, nil
	case 6:
		u, err := strconv.ParseUint(h, 16, 32)
		if err != nil {
			return color.NRGBA{}, true, fmt.Errorf("invalid hex color %q", v)
		}
		return color.NRGBA{
			R: uint8((u >> 16) & 0xFF),
			G: uint8((u >> 8) & 0xFF),
			B: uint8(u & 0xFF),
			A: 255,
		}, false, nil
	case 8:
		u, err := strconv.ParseUint(h, 16, 32)
		if err != nil {
			return color.NRGBA{}, true, fmt.Errorf("invalid hex color %q", v)
		}
		return color.NRGBA{
			R: uint8((u >> 24) & 0xFF),
			G: uint8((u >> 16) & 0xFF),
			B: uint8((u >> 8) & 0xFF),
			A: uint8(u & 0xFF),
		}, false, nil
	default:
		return color.NRGBA{}, true, fmt.Errorf("invalid hex color %q", v)
	}
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseRGBPart(raw string) (uint8, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasSuffix(raw, "%") {
		v, err := parseFloat(strings.TrimSuffix(raw, "%"))
		if err != nil {
			return 0, err
		}
		if v < 0 {
			v = 0
		}
		if v > 100 {
			v = 100
		}
		return uint8(v*255/100 + 0.5), nil
	}
	v, err := parseFloat(raw)
	if err != nil {
		return 0, err
	}
	if v < 0 {
		v = 0
	}
	if v > 255 {
		v = 255
	}
	return uint8(v + 0.5), nil
}

func parseAlphaPart(raw string) (uint8, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasSuffix(raw, "%") {
		v, err := parseFloat(strings.TrimSuffix(raw, "%"))
		if err != nil {
			return 0, err
		}
		v = clamp01(v / 100)
		return uint8(v*255 + 0.5), nil
	}
	v, err := parseFloat(raw)
	if err != nil {
		return 0, err
	}
	v = clamp01(v)
	return uint8(v*255 + 0.5), nil
}
