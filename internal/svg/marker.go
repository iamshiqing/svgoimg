package svg

import (
	"fmt"
	"math"
	"strings"

	"github.com/iamshiqing/svgoimg/internal/model"
)

type markerDef struct {
	commands    []model.Command
	baseMap     model.Matrix
	refX        float64
	refY        float64
	unitsStroke bool
	orientAuto  bool
	orientRad   float64
}

func (p *parserState) expandMarkers(attrs map[string]string, path model.Path, style model.Style) ([]model.Command, error) {
	startID, midID, endID := markerRefs(attrs)
	if startID == "" && midID == "" && endID == "" {
		return nil, nil
	}
	if style.StrokeWidth <= 0 {
		return nil, nil
	}

	out := make([]model.Command, 0, 8)
	for _, sp := range path.Subpaths {
		n := len(sp.Points)
		if n < 2 {
			continue
		}

		if startID != "" {
			angle := edgeAngle(sp.Points[0], sp.Points[1])
			cmds, err := p.placeMarker(startID, sp.Points[0], angle, style.StrokeWidth)
			if err != nil {
				return nil, err
			}
			out = append(out, cmds...)
		}

		if midID != "" {
			for i := 1; i < n-1; i++ {
				angle := edgeAngle(sp.Points[i-1], sp.Points[i+1])
				cmds, err := p.placeMarker(midID, sp.Points[i], angle, style.StrokeWidth)
				if err != nil {
					return nil, err
				}
				out = append(out, cmds...)
			}
		}

		if endID != "" {
			angle := edgeAngle(sp.Points[n-2], sp.Points[n-1])
			cmds, err := p.placeMarker(endID, sp.Points[n-1], angle, style.StrokeWidth)
			if err != nil {
				return nil, err
			}
			out = append(out, cmds...)
		}
	}
	return out, nil
}

func markerRefs(attrs map[string]string) (startID, midID, endID string) {
	all, _ := parseURLRef(attrs["marker"])
	startID, _ = parseURLRef(attrs["marker-start"])
	midID, _ = parseURLRef(attrs["marker-mid"])
	endID, _ = parseURLRef(attrs["marker-end"])
	if startID == "" {
		startID = all
	}
	if midID == "" {
		midID = all
	}
	if endID == "" {
		endID = all
	}
	return startID, midID, endID
}

func edgeAngle(a, b model.Point) float64 {
	return math.Atan2(b.Y-a.Y, b.X-a.X)
}

func (p *parserState) placeMarker(id string, anchor model.Point, angle, strokeWidth float64) ([]model.Command, error) {
	def, ok, err := p.ensureMarker(id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}

	rot := def.orientRad
	if def.orientAuto {
		rot = angle
	}
	m := model.Translate(-def.refX, -def.refY).Then(def.baseMap)
	if def.unitsStroke && strokeWidth > 0 {
		m = m.Then(model.Scale(strokeWidth, strokeWidth))
	}
	m = m.Then(model.Rotate(rot)).Then(model.Translate(anchor.X, anchor.Y))

	out := make([]model.Command, 0, len(def.commands))
	for _, c := range def.commands {
		path := transformPath(c.Path, m)
		if len(path.Subpaths) == 0 {
			continue
		}
		style := c.Style
		scaleStrokeStyle(&style, m.ApproxScale())
		out = append(out, model.Command{
			Path:  path,
			Style: style,
		})
	}
	return out, nil
}

func (p *parserState) ensureMarker(id string) (markerDef, bool, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return markerDef{}, false, nil
	}
	if def, ok := p.markerCache[id]; ok {
		return def, true, nil
	}
	node := p.ids[id]
	if node == nil {
		return markerDef{}, false, nil
	}
	if node.Name != "marker" {
		return markerDef{}, false, nil
	}
	def, err := p.parseMarkerNode(node, id)
	if err != nil {
		return markerDef{}, false, err
	}
	p.markerCache[id] = def
	return def, true, nil
}

func (p *parserState) parseMarkerNode(node *xmlNode, id string) (markerDef, error) {
	vw := p.scene.ViewBox.W
	if vw <= 0 {
		vw = 300
	}
	vh := p.scene.ViewBox.H
	if vh <= 0 {
		vh = 150
	}

	markerW, _ := parseAttrLength(node.Attrs, "markerwidth", vw, 3)
	markerH, _ := parseAttrLength(node.Attrs, "markerheight", vh, 3)
	if markerW <= 0 {
		markerW = 3
	}
	if markerH <= 0 {
		markerH = 3
	}

	base := model.IdentityMatrix
	vb, hasVB := parseViewBox(node.Attrs["viewbox"])
	if hasVB && vb.W > 0 && vb.H > 0 {
		base = model.Translate(-vb.X, -vb.Y).Then(model.Scale(markerW/vb.W, markerH/vb.H))
	}

	refBaseX := markerW
	refBaseY := markerH
	if hasVB && vb.W > 0 && vb.H > 0 {
		refBaseX = vb.W
		refBaseY = vb.H
	}
	refX, _ := parseAttrLength(node.Attrs, "refx", refBaseX, 0)
	refY, _ := parseAttrLength(node.Attrs, "refy", refBaseY, 0)

	unitsStroke := true
	if v := strings.TrimSpace(strings.ToLower(node.Attrs["markerunits"])); v == "userspaceonuse" {
		unitsStroke = false
	}
	orientAuto, orientRad, err := parseMarkerOrient(node.Attrs["orient"])
	if err != nil {
		return markerDef{}, fmt.Errorf("marker %q orient: %w", id, err)
	}

	out := markerDef{
		commands:    make([]model.Command, 0, 8),
		baseMap:     base,
		refX:        refX,
		refY:        refY,
		unitsStroke: unitsStroke,
		orientAuto:  orientAuto,
		orientRad:   orientRad,
	}
	for _, child := range node.Children {
		if err := p.walkMarkerNode(child, model.DefaultStyle(), model.IdentityMatrix, &out.commands); err != nil {
			return markerDef{}, fmt.Errorf("marker %q: %w", id, err)
		}
	}
	return out, nil
}

func parseMarkerOrient(raw string) (auto bool, rad float64, err error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" || raw == "auto" {
		return true, 0, nil
	}
	deg, err := parseFloat(raw)
	if err != nil {
		return false, 0, err
	}
	return false, deg * math.Pi / 180.0, nil
}

func (p *parserState) walkMarkerNode(node *xmlNode, inherited model.Style, parentTransform model.Matrix, out *[]model.Command) error {
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
			return fmt.Errorf("marker use reference %q not found", id)
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
		return p.walkMarkerNode(target, style, model.Translate(tx, ty).Then(transform), out)
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
		if err := p.walkMarkerNode(child, style, transform, out); err != nil {
			return err
		}
	}
	return nil
}
