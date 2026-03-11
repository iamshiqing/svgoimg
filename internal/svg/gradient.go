package svg

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/iamshiqing/svgoimg/internal/model"
)

func (p *parserState) ensureGradient(id string) (bool, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return false, nil
	}
	if _, ok := p.gradients[id]; ok {
		return true, nil
	}
	if p.gradientResolving[id] {
		return false, fmt.Errorf("gradient reference cycle at %q", id)
	}

	node := p.ids[id]
	if node == nil {
		return false, nil
	}
	if node.Name != "lineargradient" && node.Name != "radialgradient" {
		return false, nil
	}

	p.gradientResolving[id] = true
	g, err := p.parseGradientNode(node, id)
	delete(p.gradientResolving, id)
	if err != nil {
		return false, err
	}
	p.gradients[id] = g
	return true, nil
}

func (p *parserState) parseGradientNode(node *xmlNode, id string) (model.Gradient, error) {
	kind := model.GradientKindLinear
	if node.Name == "radialgradient" {
		kind = model.GradientKindRadial
	}
	g := defaultGradient(kind)
	g.ID = id

	var inheritedStops []model.GradientStop

	if href := refID(node.Attrs["href"]); href != "" && href != id {
		ok, err := p.ensureGradient(href)
		if err != nil {
			return model.Gradient{}, err
		}
		if ok {
			base := p.gradients[href]
			inheritedStops = copyStops(base.Stops)
			if base.Kind == g.Kind {
				g = base
				g.ID = id
			}
		}
	}

	if v := strings.ToLower(strings.TrimSpace(node.Attrs["gradientunits"])); v != "" {
		switch v {
		case "userspaceonuse":
			g.Units = model.GradientUnitsUserSpaceOnUse
		case "objectboundingbox":
			g.Units = model.GradientUnitsObjectBoundingBox
		default:
			if p.opts.Mode != ParseIgnore {
				return model.Gradient{}, fmt.Errorf("unsupported gradientUnits %q", v)
			}
		}
	}

	if v := strings.ToLower(strings.TrimSpace(node.Attrs["spreadmethod"])); v != "" {
		switch v {
		case "pad":
			g.Spread = model.GradientSpreadPad
		case "repeat":
			g.Spread = model.GradientSpreadRepeat
		case "reflect":
			g.Spread = model.GradientSpreadReflect
		default:
			if p.opts.Mode != ParseIgnore {
				return model.Gradient{}, fmt.Errorf("unsupported spreadMethod %q", v)
			}
		}
	}

	if raw := strings.TrimSpace(node.Attrs["gradienttransform"]); raw != "" {
		m, err := parseTransform(raw)
		if err != nil {
			if p.opts.Mode != ParseIgnore {
				return model.Gradient{}, fmt.Errorf("gradientTransform: %w", err)
			}
		} else {
			g.Transform = m
		}
	}

	if err := p.applyGradientCoordinates(&g, node.Attrs); err != nil {
		if p.opts.Mode != ParseIgnore {
			return model.Gradient{}, err
		}
	}

	stops, err := parseGradientStops(node.Children, p.opts.Mode)
	if err != nil {
		return model.Gradient{}, err
	}
	if len(stops) == 0 && len(inheritedStops) > 0 {
		stops = inheritedStops
	}
	if len(stops) == 0 {
		stops = []model.GradientStop{
			{Offset: 0, Color: color.NRGBA{R: 0, G: 0, B: 0, A: 255}},
			{Offset: 1, Color: color.NRGBA{R: 0, G: 0, B: 0, A: 255}},
		}
	}

	// Keep author order and clamp non-monotonic offsets per SVG behavior.
	stops = normalizeGradientStops(stops)
	if stops[0].Offset > 0 {
		stops = append([]model.GradientStop{{Offset: 0, Color: stops[0].Color}}, stops...)
	}
	last := stops[len(stops)-1]
	if last.Offset < 1 {
		stops = append(stops, model.GradientStop{Offset: 1, Color: last.Color})
	}
	g.Stops = stops
	return g, nil
}

func defaultGradient(kind model.GradientKind) model.Gradient {
	g := model.Gradient{
		Kind:      kind,
		Units:     model.GradientUnitsObjectBoundingBox,
		Spread:    model.GradientSpreadPad,
		Transform: model.IdentityMatrix,
	}
	if kind == model.GradientKindLinear {
		g.X1 = 0
		g.Y1 = 0
		g.X2 = 1
		g.Y2 = 0
	} else {
		g.CX = 0.5
		g.CY = 0.5
		g.R = 0.5
		g.FX = 0.5
		g.FY = 0.5
	}
	return g
}

func (p *parserState) applyGradientCoordinates(g *model.Gradient, attrs map[string]string) error {
	vw := p.scene.ViewBox.W
	vh := p.scene.ViewBox.H
	if vw <= 0 {
		vw = 300
	}
	if vh <= 0 {
		vh = 150
	}
	minWH := vw
	if vh < minWH {
		minWH = vh
	}
	if minWH <= 0 {
		minWH = 100
	}

	if g.Kind == model.GradientKindLinear {
		x1, err := parseGradientCoord(attrs["x1"], g.X1, g.Units, vw)
		if err != nil {
			return fmt.Errorf("x1: %w", err)
		}
		y1, err := parseGradientCoord(attrs["y1"], g.Y1, g.Units, vh)
		if err != nil {
			return fmt.Errorf("y1: %w", err)
		}
		x2, err := parseGradientCoord(attrs["x2"], g.X2, g.Units, vw)
		if err != nil {
			return fmt.Errorf("x2: %w", err)
		}
		y2, err := parseGradientCoord(attrs["y2"], g.Y2, g.Units, vh)
		if err != nil {
			return fmt.Errorf("y2: %w", err)
		}
		g.X1, g.Y1, g.X2, g.Y2 = x1, y1, x2, y2
		return nil
	}

	cx, err := parseGradientCoord(attrs["cx"], g.CX, g.Units, vw)
	if err != nil {
		return fmt.Errorf("cx: %w", err)
	}
	cy, err := parseGradientCoord(attrs["cy"], g.CY, g.Units, vh)
	if err != nil {
		return fmt.Errorf("cy: %w", err)
	}
	r, err := parseGradientCoord(attrs["r"], g.R, g.Units, minWH)
	if err != nil {
		return fmt.Errorf("r: %w", err)
	}
	fx, err := parseGradientCoord(attrs["fx"], cx, g.Units, vw)
	if err != nil {
		return fmt.Errorf("fx: %w", err)
	}
	fy, err := parseGradientCoord(attrs["fy"], cy, g.Units, vh)
	if err != nil {
		return fmt.Errorf("fy: %w", err)
	}
	g.CX, g.CY, g.R, g.FX, g.FY = cx, cy, r, fx, fy
	return nil
}

func parseGradientCoord(raw string, defaultValue float64, units model.GradientUnits, percentBase float64) (float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultValue, nil
	}
	if units == model.GradientUnitsUserSpaceOnUse {
		return parseLength(raw, percentBase)
	}

	num, unit := splitUnit(raw)
	if num == "" {
		return 0, fmt.Errorf("empty coordinate")
	}
	v, err := parseFloat(num)
	if err != nil {
		return 0, err
	}
	if unit == "%" {
		return v / 100.0, nil
	}
	return v, nil
}

func parseGradientStops(children []*xmlNode, mode ParseMode) ([]model.GradientStop, error) {
	out := make([]model.GradientStop, 0, 8)
	for _, child := range children {
		if child.Name != "stop" {
			continue
		}
		stop, ok, err := parseGradientStop(child, mode)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, stop)
		}
	}
	return out, nil
}

func parseGradientStop(node *xmlNode, mode ParseMode) (model.GradientStop, bool, error) {
	props := map[string]string{}
	for k, v := range node.Attrs {
		props[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
	}
	if rawStyle, ok := props["style"]; ok {
		for k, v := range parseStyleDeclarations(rawStyle) {
			props[k] = v
		}
	}

	offset := 0.0
	if raw := props["offset"]; strings.TrimSpace(raw) != "" {
		raw = strings.TrimSpace(raw)
		if strings.HasSuffix(raw, "%") {
			v, err := parseFloat(strings.TrimSuffix(raw, "%"))
			if err != nil {
				if mode != ParseIgnore {
					return model.GradientStop{}, false, fmt.Errorf("stop offset: %w", err)
				}
				return model.GradientStop{}, false, nil
			}
			offset = v / 100.0
		} else {
			v, err := parseFloat(raw)
			if err != nil {
				if mode != ParseIgnore {
					return model.GradientStop{}, false, fmt.Errorf("stop offset: %w", err)
				}
				return model.GradientStop{}, false, nil
			}
			offset = v
		}
	}
	if offset < 0 {
		offset = 0
	}
	if offset > 1 {
		offset = 1
	}

	stopColor := color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	if raw := props["stop-color"]; strings.TrimSpace(raw) != "" {
		c, err := parseColorToken(raw, stopColor)
		if err != nil {
			if mode != ParseIgnore {
				return model.GradientStop{}, false, fmt.Errorf("stop-color: %w", err)
			}
		} else {
			stopColor = c
		}
	}
	alpha := 1.0
	if raw := props["stop-opacity"]; strings.TrimSpace(raw) != "" {
		v, err := parseOpacity(raw)
		if err != nil {
			if mode != ParseIgnore {
				return model.GradientStop{}, false, fmt.Errorf("stop-opacity: %w", err)
			}
		} else {
			alpha = v
		}
	}
	stopColor.A = uint8(float64(stopColor.A)*alpha + 0.5)

	return model.GradientStop{
		Offset: offset,
		Color:  stopColor,
	}, true, nil
}

func copyStops(stops []model.GradientStop) []model.GradientStop {
	if len(stops) == 0 {
		return nil
	}
	out := make([]model.GradientStop, len(stops))
	copy(out, stops)
	return out
}

func normalizeGradientStops(stops []model.GradientStop) []model.GradientStop {
	if len(stops) == 0 {
		return nil
	}
	out := make([]model.GradientStop, len(stops))
	copy(out, stops)
	prev := 0.0
	for i := range out {
		if out[i].Offset < prev {
			out[i].Offset = prev
		}
		prev = out[i].Offset
	}
	return out
}
