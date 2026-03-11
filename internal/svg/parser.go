package svg

import (
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/iamshiqing/svgoimg/internal/model"
)

type context struct {
	style     model.Style
	transform model.Matrix
	inDefs    bool
}

type parserState struct {
	opts              Options
	scene             model.Scene
	rootSeen          bool
	ids               map[string]*xmlNode
	gradients         map[string]model.Gradient
	gradientResolving map[string]bool
	useDepth          map[string]int
}

func Parse(r io.Reader, opts Options) (model.Scene, error) {
	if r == nil {
		return model.Scene{}, fmt.Errorf("nil reader")
	}
	opts = opts.withDefaults()

	root, err := parseXMLTree(r)
	if err != nil {
		return model.Scene{}, err
	}

	p := &parserState{
		opts: opts,
		scene: model.Scene{
			Width:     300,
			Height:    150,
			ViewBox:   model.Rect{X: 0, Y: 0, W: 300, H: 150},
			Gradients: map[string]model.Gradient{},
		},
		ids:               map[string]*xmlNode{},
		gradients:         map[string]model.Gradient{},
		gradientResolving: map[string]bool{},
		useDepth:          map[string]int{},
	}

	p.collectIDs(root)
	err = p.walk(root, context{
		style:     model.DefaultStyle(),
		transform: model.IdentityMatrix,
		inDefs:    false,
	}, false)
	if err != nil {
		return model.Scene{}, err
	}

	p.scene.Gradients = p.gradients
	return p.scene, nil
}

func (p *parserState) collectIDs(node *xmlNode) {
	if node == nil {
		return
	}
	if id := strings.TrimSpace(node.Attrs["id"]); id != "" {
		p.ids[id] = node
	}
	for _, child := range node.Children {
		p.collectIDs(child)
	}
}

func (p *parserState) handleNonFatal(err error) error {
	if err == nil {
		return nil
	}
	if p.opts.Mode == ParseStrict {
		return err
	}
	if p.opts.Mode == ParseWarn && p.opts.OnWarning != nil {
		p.opts.OnWarning(err)
	}
	return nil
}

func (p *parserState) walk(node *xmlNode, ctx context, renderDefs bool) error {
	if node == nil {
		return nil
	}
	if node.Name == "__root__" {
		for _, child := range node.Children {
			if err := p.walk(child, ctx, renderDefs); err != nil {
				return err
			}
		}
		return nil
	}

	name := node.Name
	attrs := node.Attrs

	transform := ctx.transform
	if raw := attrs["transform"]; raw != "" {
		m, err := parseTransform(raw)
		if err != nil {
			if fatal := p.handleNonFatal(fmt.Errorf("%s transform: %w", name, err)); fatal != nil {
				return fatal
			}
		} else {
			// Child transform is applied in local coordinates, then parent.
			transform = m.Then(transform)
		}
	}

	style, err := applyStyleAttributes(ctx.style, attrs, p.opts.Mode)
	if err != nil {
		if fatal := p.handleNonFatal(fmt.Errorf("%s style: %w", name, err)); fatal != nil {
			return fatal
		}
	}

	next := context{
		style:     style,
		transform: transform,
		inDefs:    ctx.inDefs,
	}

	if name == "svg" && !p.rootSeen {
		p.rootSeen = true
		if err := parseRootSize(attrs, &p.scene); err != nil {
			if fatal := p.handleNonFatal(err); fatal != nil {
				return fatal
			}
		}
	}

	if name == "defs" {
		next.inDefs = true
	}
	if name == "symbol" && !renderDefs {
		next.inDefs = true
	}
	if name == "symbol" && renderDefs {
		next.inDefs = false
	}

	if name == "lineargradient" || name == "radialgradient" {
		if id := strings.TrimSpace(attrs["id"]); id != "" {
			_, err := p.ensureGradient(id)
			if err != nil {
				if fatal := p.handleNonFatal(err); fatal != nil {
					return fatal
				}
			}
		}
	}

	if name == "use" {
		if style.Visible && (!ctx.inDefs || renderDefs) {
			if err := p.expandUse(node, next, renderDefs); err != nil {
				if fatal := p.handleNonFatal(err); fatal != nil {
					return fatal
				}
			}
		}
		return nil
	}

	if style.Visible && (!next.inDefs || renderDefs) {
		path, ok, err := elementPath(name, attrs, p.scene.ViewBox, p.opts.CurveTolerance)
		if err != nil {
			if fatal := p.handleNonFatal(fmt.Errorf("%s parse failed: %w", name, err)); fatal != nil {
				return fatal
			}
		} else if ok {
			path = transformPath(path, transform)
			if len(path.Subpaths) > 0 {
				cmdStyle := style
				cmdStyle.StrokeWidth = style.StrokeWidth * transform.ApproxScale()
				if err := p.resolvePaintDependencies(&cmdStyle); err != nil {
					if fatal := p.handleNonFatal(err); fatal != nil {
						return fatal
					}
				}
				p.scene.Commands = append(p.scene.Commands, model.Command{
					Path:  path,
					Style: cmdStyle,
				})
			}
		}
	}

	for _, child := range node.Children {
		if err := p.walk(child, next, renderDefs); err != nil {
			return err
		}
	}
	return nil
}

func (p *parserState) resolvePaintDependencies(style *model.Style) error {
	if style == nil {
		return nil
	}
	resolve := func(paint *model.Paint) error {
		if paint == nil || paint.None || paint.Kind != model.PaintKindGradient || paint.GradientID == "" {
			return nil
		}
		ok, err := p.ensureGradient(paint.GradientID)
		if err != nil {
			return err
		}
		if !ok && !paint.HasFallback {
			paint.None = true
		}
		return nil
	}
	if err := resolve(&style.Fill); err != nil {
		return err
	}
	if err := resolve(&style.Stroke); err != nil {
		return err
	}
	return nil
}

func (p *parserState) expandUse(node *xmlNode, ctx context, renderDefs bool) error {
	href := strings.TrimSpace(node.Attrs["href"])
	if href == "" {
		return fmt.Errorf("use element missing href")
	}

	id := refID(href)
	if id == "" {
		return fmt.Errorf("use href must reference #id, got %q", href)
	}

	target := p.ids[id]
	if target == nil {
		return fmt.Errorf("use reference %q not found", id)
	}

	if p.useDepth[id] >= 16 {
		return fmt.Errorf("use reference cycle detected at %q", id)
	}

	vw, vh := p.scene.ViewBox.W, p.scene.ViewBox.H
	if vw <= 0 {
		vw = 300
	}
	if vh <= 0 {
		vh = 150
	}

	tx, _ := parseAttrLength(node.Attrs, "x", vw, 0)
	ty, _ := parseAttrLength(node.Attrs, "y", vh, 0)

	useCtx := ctx
	useMap := model.IdentityMatrix
	if target.Name == "symbol" {
		if m, ok := symbolUseTransform(target, node.Attrs, p.scene.ViewBox); ok {
			useMap = useMap.Then(m)
		}
	}
	useMap = useMap.Then(model.Translate(tx, ty))
	// Referenced content applies use mapping first, then inherited parent transform.
	useCtx.transform = useMap.Then(useCtx.transform)

	p.useDepth[id]++
	err := p.walk(target, useCtx, true)
	p.useDepth[id]--
	return err
}

func refID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "#") {
		return strings.TrimSpace(raw[1:])
	}
	if i := strings.IndexByte(raw, '#'); i >= 0 && i < len(raw)-1 {
		return strings.TrimSpace(raw[i+1:])
	}
	return ""
}

func symbolUseTransform(symbol *xmlNode, useAttrs map[string]string, rootViewBox model.Rect) (model.Matrix, bool) {
	vb, ok := parseViewBox(symbol.Attrs["viewbox"])
	if !ok || vb.W <= 0 || vb.H <= 0 {
		return model.IdentityMatrix, false
	}

	vw, vh := rootViewBox.W, rootViewBox.H
	if vw <= 0 {
		vw = 300
	}
	if vh <= 0 {
		vh = 150
	}

	w, _ := parseAttrLength(useAttrs, "width", vw, vb.W)
	h, _ := parseAttrLength(useAttrs, "height", vh, vb.H)
	if w <= 0 {
		w = vb.W
	}
	if h <= 0 {
		h = vb.H
	}

	sx := w / vb.W
	sy := h / vb.H
	return model.Translate(-vb.X, -vb.Y).Then(model.Scale(sx, sy)), true
}

func parseViewBox(raw string) (model.Rect, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return model.Rect{}, false
	}
	nums, err := parseNumberList(raw)
	if err != nil || len(nums) != 4 {
		return model.Rect{}, false
	}
	return model.Rect{
		X: nums[0],
		Y: nums[1],
		W: nums[2],
		H: nums[3],
	}, nums[2] > 0 && nums[3] > 0
}

func parseRootSize(attrs map[string]string, scene *model.Scene) error {
	viewBoxRaw := attrs["viewbox"]
	hasViewBox := false
	if viewBoxRaw != "" {
		nums, err := parseNumberList(viewBoxRaw)
		if err != nil {
			return fmt.Errorf("invalid viewBox: %w", err)
		}
		if len(nums) != 4 {
			return fmt.Errorf("viewBox must have 4 numbers")
		}
		if nums[2] > 0 && nums[3] > 0 {
			scene.ViewBox = model.Rect{X: nums[0], Y: nums[1], W: nums[2], H: nums[3]}
			hasViewBox = true
		}
	}

	width := scene.Width
	height := scene.Height
	if raw := attrs["width"]; raw != "" {
		base := scene.ViewBox.W
		if !hasViewBox {
			base = width
		}
		if v, err := parseLength(raw, base); err == nil && v > 0 {
			width = v
		}
	}
	if raw := attrs["height"]; raw != "" {
		base := scene.ViewBox.H
		if !hasViewBox {
			base = height
		}
		if v, err := parseLength(raw, base); err == nil && v > 0 {
			height = v
		}
	}

	if hasViewBox {
		switch {
		case width <= 0 && height <= 0:
			width, height = scene.ViewBox.W, scene.ViewBox.H
		case width <= 0 && height > 0:
			width = height * scene.ViewBox.W / scene.ViewBox.H
		case height <= 0 && width > 0:
			height = width * scene.ViewBox.H / scene.ViewBox.W
		}
	}
	if width <= 0 {
		width = 300
	}
	if height <= 0 {
		height = 150
	}
	if !hasViewBox {
		scene.ViewBox = model.Rect{X: 0, Y: 0, W: width, H: height}
	}

	scene.Width = width
	scene.Height = height
	return nil
}

func elementPath(name string, attrs map[string]string, viewBox model.Rect, tolerance float64) (model.Path, bool, error) {
	vw, vh := viewBox.W, viewBox.H
	if vw <= 0 {
		vw = 300
	}
	if vh <= 0 {
		vh = 150
	}
	minWH := math.Min(vw, vh)
	if minWH <= 0 {
		minWH = 100
	}

	switch name {
	case "path":
		d := attrs["d"]
		if d == "" {
			return model.Path{}, false, fmt.Errorf("missing d attribute")
		}
		p, err := parsePathData(d, tolerance)
		return p, true, err

	case "rect":
		x, _ := parseAttrLength(attrs, "x", vw, 0)
		y, _ := parseAttrLength(attrs, "y", vh, 0)
		w, _ := parseAttrLength(attrs, "width", vw, 0)
		h, _ := parseAttrLength(attrs, "height", vh, 0)
		rx, _ := parseAttrLength(attrs, "rx", minWH, 0)
		ry, _ := parseAttrLength(attrs, "ry", minWH, 0)
		return rectPath(x, y, w, h, rx, ry, tolerance), true, nil

	case "circle":
		cx, _ := parseAttrLength(attrs, "cx", vw, 0)
		cy, _ := parseAttrLength(attrs, "cy", vh, 0)
		r, _ := parseAttrLength(attrs, "r", minWH, 0)
		return circlePath(cx, cy, r, tolerance), true, nil

	case "ellipse":
		cx, _ := parseAttrLength(attrs, "cx", vw, 0)
		cy, _ := parseAttrLength(attrs, "cy", vh, 0)
		rx, _ := parseAttrLength(attrs, "rx", vw, 0)
		ry, _ := parseAttrLength(attrs, "ry", vh, 0)
		return ellipsePath(cx, cy, rx, ry, tolerance), true, nil

	case "line":
		x1, _ := parseAttrLength(attrs, "x1", vw, 0)
		y1, _ := parseAttrLength(attrs, "y1", vh, 0)
		x2, _ := parseAttrLength(attrs, "x2", vw, 0)
		y2, _ := parseAttrLength(attrs, "y2", vh, 0)
		return linePath(x1, y1, x2, y2), true, nil

	case "polyline":
		points, err := parsePoints(attrs["points"])
		if err != nil {
			return model.Path{}, false, err
		}
		return polylinePath(points, false), true, nil

	case "polygon":
		points, err := parsePoints(attrs["points"])
		if err != nil {
			return model.Path{}, false, err
		}
		return polylinePath(points, true), true, nil
	}

	return model.Path{}, false, nil
}

func parseAttrLength(attrs map[string]string, key string, base float64, defaultValue float64) (float64, error) {
	raw, ok := attrs[key]
	if !ok || strings.TrimSpace(raw) == "" {
		return defaultValue, nil
	}
	return parseLength(raw, base)
}
