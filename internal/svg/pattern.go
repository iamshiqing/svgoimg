package svg

import (
	"fmt"
	"strings"

	"github.com/iamshiqing/svgoimg/internal/model"
)

func (p *parserState) ensurePattern(id string) (bool, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return false, nil
	}
	if _, ok := p.patterns[id]; ok {
		return true, nil
	}
	if p.patternResolving[id] {
		return false, fmt.Errorf("pattern reference cycle at %q", id)
	}

	node := p.ids[id]
	if node == nil {
		return false, nil
	}
	if node.Name != "pattern" {
		return false, nil
	}

	p.patternResolving[id] = true
	pat, err := p.parsePatternNode(node, id)
	delete(p.patternResolving, id)
	if err != nil {
		return false, err
	}
	p.patterns[id] = pat
	return true, nil
}

func (p *parserState) parsePatternNode(node *xmlNode, id string) (model.Pattern, error) {
	pat := model.Pattern{
		ID:        id,
		Units:     model.PatternUnitsObjectBoundingBox,
		Transform: model.IdentityMatrix,
		X:         0,
		Y:         0,
		W:         1,
		H:         1,
		Commands:  nil,
	}

	if href := refID(node.Attrs["href"]); href != "" && href != id {
		ok, err := p.ensurePattern(href)
		if err != nil {
			return model.Pattern{}, err
		}
		if ok {
			base := p.patterns[href]
			pat = base
			pat.ID = id
		}
	}

	if v := strings.TrimSpace(strings.ToLower(node.Attrs["patternunits"])); v != "" {
		switch v {
		case "userspaceonuse":
			pat.Units = model.PatternUnitsUserSpaceOnUse
		case "objectboundingbox":
			pat.Units = model.PatternUnitsObjectBoundingBox
		default:
			if p.opts.Mode != ParseIgnore {
				return model.Pattern{}, fmt.Errorf("unsupported patternUnits %q", v)
			}
		}
	}

	if raw := strings.TrimSpace(node.Attrs["patterntransform"]); raw != "" {
		m, err := parseTransform(raw)
		if err != nil {
			if p.opts.Mode != ParseIgnore {
				return model.Pattern{}, fmt.Errorf("patternTransform: %w", err)
			}
		} else {
			pat.Transform = m
		}
	}

	if err := p.applyPatternCoordinates(&pat, node.Attrs); err != nil {
		if p.opts.Mode != ParseIgnore {
			return model.Pattern{}, err
		}
	}

	commands := make([]model.Command, 0, 8)
	for _, child := range node.Children {
		if err := p.walkPatternNode(child, model.DefaultStyle(), model.IdentityMatrix, &commands); err != nil {
			return model.Pattern{}, err
		}
	}
	if len(commands) > 0 {
		pat.Commands = commands
	}
	return pat, nil
}

func (p *parserState) applyPatternCoordinates(pat *model.Pattern, attrs map[string]string) error {
	vw := p.scene.ViewBox.W
	vh := p.scene.ViewBox.H
	if vw <= 0 {
		vw = 300
	}
	if vh <= 0 {
		vh = 150
	}

	x, err := parsePatternCoord(attrs["x"], pat.X, pat.Units, vw)
	if err != nil {
		return fmt.Errorf("pattern x: %w", err)
	}
	y, err := parsePatternCoord(attrs["y"], pat.Y, pat.Units, vh)
	if err != nil {
		return fmt.Errorf("pattern y: %w", err)
	}
	w, err := parsePatternCoord(attrs["width"], pat.W, pat.Units, vw)
	if err != nil {
		return fmt.Errorf("pattern width: %w", err)
	}
	h, err := parsePatternCoord(attrs["height"], pat.H, pat.Units, vh)
	if err != nil {
		return fmt.Errorf("pattern height: %w", err)
	}
	pat.X, pat.Y, pat.W, pat.H = x, y, w, h
	return nil
}

func parsePatternCoord(raw string, defaultValue float64, units model.PatternUnits, percentBase float64) (float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultValue, nil
	}
	if units == model.PatternUnitsUserSpaceOnUse {
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

func (p *parserState) walkPatternNode(node *xmlNode, inherited model.Style, parentTransform model.Matrix, out *[]model.Command) error {
	if node == nil {
		return nil
	}
	style, err := applyStyleAttributes(inherited, node.Attrs, p.opts.Mode)
	if err != nil {
		return err
	}
	if !style.Visible {
		return nil
	}
	transform := parentTransform
	if raw := strings.TrimSpace(node.Attrs["transform"]); raw != "" {
		m, err := parseTransform(raw)
		if err != nil {
			return err
		}
		transform = m.Then(transform)
	}

	if node.Name == "use" {
		id, err := parseURLRef(node.Attrs["href"])
		if err != nil {
			return err
		}
		target := p.ids[id]
		if target == nil {
			return fmt.Errorf("pattern use reference %q not found", id)
		}
		vw := p.scene.ViewBox.W
		if vw <= 0 {
			vw = 300
		}
		vh := p.scene.ViewBox.H
		if vh <= 0 {
			vh = 150
		}
		tx, _ := parseAttrLength(node.Attrs, "x", vw, 0)
		ty, _ := parseAttrLength(node.Attrs, "y", vh, 0)
		return p.walkPatternNode(target, style, model.Translate(tx, ty).Then(transform), out)
	}

	path, ok, err := elementPath(node.Name, node.Attrs, p.scene.ViewBox, p.opts.CurveTolerance)
	if err != nil {
		return err
	}
	if ok && len(path.Subpaths) > 0 {
		path = transformPath(path, transform)
		cmdStyle := style
		scaleStrokeStyle(&cmdStyle, transform.ApproxScale())
		if err := p.resolvePaintDependencies(&cmdStyle); err != nil {
			if p.opts.Mode == ParseStrict {
				return err
			}
		}
		*out = append(*out, model.Command{
			Path:  path,
			Style: cmdStyle,
		})
	}
	for _, child := range node.Children {
		if err := p.walkPatternNode(child, style, transform, out); err != nil {
			return err
		}
	}
	return nil
}
