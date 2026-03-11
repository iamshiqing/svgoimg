package svg

import (
	"fmt"
	"strings"

	"github.com/iamshiqing/svgoimg/internal/model"
)

func applyStyleAttributes(base model.Style, attrs map[string]string, mode ParseMode) (model.Style, error) {
	style := base

	props := map[string]string{}
	for k, v := range attrs {
		props[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
	}
	if rawStyle, ok := props["style"]; ok {
		for k, v := range parseStyleDeclarations(rawStyle) {
			props[k] = v
		}
	}

	if v, ok := props["color"]; ok {
		clr, err := parseColorToken(v, style.CurrentColor)
		if err != nil {
			if mode != ParseIgnore {
				return style, fmt.Errorf("parse color: %w", err)
			}
		} else {
			style.CurrentColor = clr
		}
	}

	if v, ok := props["fill"]; ok {
		p, err := parsePaint(v, style.CurrentColor)
		if err != nil {
			if mode != ParseIgnore {
				return style, fmt.Errorf("parse fill: %w", err)
			}
		} else {
			style.Fill = p
		}
	}

	if v, ok := props["stroke"]; ok {
		p, err := parsePaint(v, style.CurrentColor)
		if err != nil {
			if mode != ParseIgnore {
				return style, fmt.Errorf("parse stroke: %w", err)
			}
		} else {
			style.Stroke = p
		}
	}

	if v, ok := props["stroke-width"]; ok {
		l, err := parseLength(v, 0)
		if err != nil {
			if mode != ParseIgnore {
				return style, fmt.Errorf("parse stroke-width: %w", err)
			}
		} else if l >= 0 {
			style.StrokeWidth = l
		}
	}

	if v, ok := props["opacity"]; ok {
		o, err := parseOpacity(v)
		if err != nil {
			if mode != ParseIgnore {
				return style, fmt.Errorf("parse opacity: %w", err)
			}
		} else {
			style.Opacity = o
		}
	}
	if v, ok := props["fill-opacity"]; ok {
		o, err := parseOpacity(v)
		if err != nil {
			if mode != ParseIgnore {
				return style, fmt.Errorf("parse fill-opacity: %w", err)
			}
		} else {
			style.FillOpacity = o
		}
	}
	if v, ok := props["stroke-opacity"]; ok {
		o, err := parseOpacity(v)
		if err != nil {
			if mode != ParseIgnore {
				return style, fmt.Errorf("parse stroke-opacity: %w", err)
			}
		} else {
			style.StrokeOpacity = o
		}
	}
	if v, ok := props["fill-rule"]; ok {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "evenodd":
			style.FillRule = model.FillRuleEvenOdd
		case "nonzero", "":
			style.FillRule = model.FillRuleNonZero
		default:
			if mode != ParseIgnore {
				return style, fmt.Errorf("unsupported fill-rule %q", v)
			}
		}
	}
	if v, ok := props["display"]; ok && strings.EqualFold(strings.TrimSpace(v), "none") {
		style.Visible = false
	}
	if v, ok := props["visibility"]; ok {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "hidden", "collapse":
			style.Visible = false
		case "visible", "":
			style.Visible = true
		}
	}

	return style, nil
}

func parseStyleDeclarations(raw string) map[string]string {
	out := map[string]string{}
	for _, item := range strings.Split(raw, ";") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		p := strings.SplitN(item, ":", 2)
		if len(p) != 2 {
			continue
		}
		k := strings.ToLower(strings.TrimSpace(p[0]))
		v := strings.TrimSpace(p[1])
		if k == "" {
			continue
		}
		out[k] = v
	}
	return out
}
