package svg

import (
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/iamshiqing/svgoimg/internal/model"
)

type context struct {
	style     model.Style
	transform model.Matrix
}

func Parse(r io.Reader, opts Options) (model.Scene, error) {
	if r == nil {
		return model.Scene{}, fmt.Errorf("nil reader")
	}
	opts = opts.withDefaults()

	scene := model.Scene{
		Width:   300,
		Height:  150,
		ViewBox: model.Rect{X: 0, Y: 0, W: 300, H: 150},
	}

	stack := []context{{
		style:     model.DefaultStyle(),
		transform: model.IdentityMatrix,
	}}
	rootSeen := false
	defsDepth := 0

	dec := xml.NewDecoder(r)
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return model.Scene{}, fmt.Errorf("xml parse failed: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			name := strings.ToLower(t.Name.Local)
			attrs := attrsMap(t.Attr)
			parent := stack[len(stack)-1]

			transform := parent.transform
			if raw := attrs["transform"]; raw != "" {
				m, err := parseTransform(raw)
				if err != nil {
					if opts.Mode == ParseStrict {
						return model.Scene{}, fmt.Errorf("%s transform: %w", name, err)
					}
				} else {
					transform = transform.Then(m)
				}
			}

			style, err := applyStyleAttributes(parent.style, attrs, opts.Mode)
			if err != nil {
				return model.Scene{}, fmt.Errorf("%s style: %w", name, err)
			}

			stack = append(stack, context{
				style:     style,
				transform: transform,
			})

			if name == "defs" {
				defsDepth++
				continue
			}
			if name == "svg" && !rootSeen {
				rootSeen = true
				if err := parseRootSize(attrs, &scene); err != nil && opts.Mode == ParseStrict {
					return model.Scene{}, err
				}
				continue
			}
			if defsDepth > 0 {
				continue
			}

			if !style.Visible {
				continue
			}
			path, ok, err := elementPath(name, attrs, scene.ViewBox, opts.CurveTolerance)
			if err != nil {
				if opts.Mode == ParseStrict {
					return model.Scene{}, fmt.Errorf("%s parse failed: %w", name, err)
				}
				continue
			}
			if !ok {
				continue
			}
			path = transformPath(path, transform)
			if len(path.Subpaths) == 0 {
				continue
			}

			cmdStyle := style
			cmdStyle.StrokeWidth = style.StrokeWidth * transform.ApproxScale()
			scene.Commands = append(scene.Commands, model.Command{
				Path:  path,
				Style: cmdStyle,
			})

		case xml.EndElement:
			name := strings.ToLower(t.Name.Local)
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
			if name == "defs" && defsDepth > 0 {
				defsDepth--
			}
		}
	}

	return scene, nil
}

func attrsMap(attrs []xml.Attr) map[string]string {
	out := make(map[string]string, len(attrs))
	for _, a := range attrs {
		k := strings.ToLower(strings.TrimSpace(a.Name.Local))
		if k == "" {
			continue
		}
		out[k] = strings.TrimSpace(a.Value)
	}
	return out
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
