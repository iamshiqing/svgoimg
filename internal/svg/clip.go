package svg

import (
	"fmt"
	"strings"

	"github.com/iamshiqing/svgoimg/internal/model"
)

func (p *parserState) resolveClip(attrs map[string]string, elementTransform model.Matrix) (*model.ClipPath, error) {
	raw := strings.TrimSpace(attrs["clip-path"])
	if raw == "" || strings.EqualFold(raw, "none") {
		return nil, nil
	}
	id, err := parseURLRef(raw)
	if err != nil {
		return nil, err
	}
	if id == "" {
		return nil, nil
	}
	node := p.ids[id]
	if node == nil {
		return nil, fmt.Errorf("clip-path reference %q not found", id)
	}
	if node.Name != "clippath" {
		return nil, fmt.Errorf("reference %q is not a clipPath", id)
	}

	path, rule, err := p.buildClipPath(node)
	if err != nil {
		return nil, err
	}
	path = transformPath(path, elementTransform)
	if len(path.Subpaths) == 0 {
		return nil, nil
	}
	return &model.ClipPath{
		Path: path,
		Rule: rule,
	}, nil
}

func (p *parserState) buildClipPath(node *xmlNode) (model.Path, model.FillRule, error) {
	rule := parseClipRule(node.Attrs)
	base := model.IdentityMatrix
	if raw := strings.TrimSpace(node.Attrs["transform"]); raw != "" {
		m, err := parseTransform(raw)
		if err != nil {
			return model.Path{}, model.FillRuleNonZero, fmt.Errorf("clipPath transform: %w", err)
		}
		base = m.Then(base)
	}
	out := model.Path{Subpaths: make([]model.Subpath, 0, 8)}
	for _, child := range node.Children {
		if err := p.walkClipNode(child, base, &out); err != nil {
			return model.Path{}, model.FillRuleNonZero, err
		}
	}
	return out, rule, nil
}

func (p *parserState) walkClipNode(node *xmlNode, parentTransform model.Matrix, out *model.Path) error {
	if node == nil {
		return nil
	}
	style, err := applyStyleAttributes(model.DefaultStyle(), node.Attrs, ParseIgnore)
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
		href := strings.TrimSpace(node.Attrs["href"])
		id, err := parseURLRef(href)
		if err != nil {
			return err
		}
		target := p.ids[id]
		if target == nil {
			return fmt.Errorf("clip use reference %q not found", id)
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
		useTransform := model.Translate(tx, ty).Then(transform)
		return p.walkClipNode(target, useTransform, out)
	}

	path, ok, err := elementPath(node.Name, node.Attrs, p.scene.ViewBox, p.opts.CurveTolerance)
	if err != nil {
		return err
	}
	if ok && len(path.Subpaths) > 0 {
		path = transformPath(path, transform)
		out.Subpaths = append(out.Subpaths, path.Subpaths...)
	}

	for _, child := range node.Children {
		if err := p.walkClipNode(child, transform, out); err != nil {
			return err
		}
	}
	return nil
}

func parseClipRule(attrs map[string]string) model.FillRule {
	raw := strings.TrimSpace(strings.ToLower(attrs["clip-rule"]))
	if raw == "" {
		if styleRaw := strings.TrimSpace(attrs["style"]); styleRaw != "" {
			decl := parseStyleDeclarations(styleRaw)
			raw = strings.TrimSpace(strings.ToLower(decl["clip-rule"]))
		}
	}
	if raw == "evenodd" {
		return model.FillRuleEvenOdd
	}
	return model.FillRuleNonZero
}

func (p *parserState) resolveMask(attrs map[string]string, elementTransform model.Matrix) (*model.MaskRef, error) {
	raw := strings.TrimSpace(attrs["mask"])
	if raw == "" || strings.EqualFold(raw, "none") {
		return nil, nil
	}
	id, err := parseURLRef(raw)
	if err != nil {
		return nil, err
	}
	if id == "" {
		return nil, nil
	}
	node := p.ids[id]
	if node == nil {
		return nil, fmt.Errorf("mask reference %q not found", id)
	}
	if node.Name != "mask" {
		return nil, fmt.Errorf("reference %q is not a mask", id)
	}

	commands, err := p.buildMaskCommands(node, elementTransform)
	if err != nil {
		return nil, err
	}
	luminance := true
	maskType := strings.TrimSpace(strings.ToLower(node.Attrs["mask-type"]))
	if maskType == "alpha" {
		luminance = false
	}
	return &model.MaskRef{
		Commands:  commands,
		Luminance: luminance,
	}, nil
}

func parseURLRef(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if strings.HasPrefix(strings.ToLower(raw), "url(") {
		closeIdx := strings.IndexByte(raw, ')')
		if closeIdx < 0 {
			return "", fmt.Errorf("invalid url reference %q", raw)
		}
		ref := strings.TrimSpace(raw[len("url("):closeIdx])
		ref = trimQuoted(ref)
		if strings.HasPrefix(ref, "#") {
			return strings.TrimSpace(ref[1:]), nil
		}
		if i := strings.IndexByte(ref, '#'); i >= 0 && i < len(ref)-1 {
			return strings.TrimSpace(ref[i+1:]), nil
		}
		return "", fmt.Errorf("url reference must point to #id, got %q", raw)
	}
	if strings.HasPrefix(raw, "#") {
		return strings.TrimSpace(raw[1:]), nil
	}
	return refID(raw), nil
}

func (p *parserState) buildMaskCommands(node *xmlNode, elementTransform model.Matrix) ([]model.Command, error) {
	base := elementTransform
	if raw := strings.TrimSpace(node.Attrs["transform"]); raw != "" {
		m, err := parseTransform(raw)
		if err != nil {
			return nil, fmt.Errorf("mask transform: %w", err)
		}
		base = m.Then(base)
	}
	out := make([]model.Command, 0, 8)
	for _, child := range node.Children {
		if err := p.walkMaskNode(child, model.DefaultStyle(), base, &out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (p *parserState) walkMaskNode(node *xmlNode, inherited model.Style, parentTransform model.Matrix, out *[]model.Command) error {
	if node == nil {
		return nil
	}
	attrs := p.mergedAttrs(node)
	style, err := applyStyleAttributes(inherited, attrs, p.opts.Mode)
	if err != nil {
		return err
	}
	if !style.Visible {
		return nil
	}

	transform := parentTransform
	if raw := strings.TrimSpace(attrs["transform"]); raw != "" {
		m, err := parseTransform(raw)
		if err != nil {
			return err
		}
		transform = m.Then(transform)
	}

	if node.Name == "use" {
		id, err := parseURLRef(attrs["href"])
		if err != nil {
			return err
		}
		target := p.ids[id]
		if target == nil {
			return fmt.Errorf("mask use reference %q not found", id)
		}
		vw := p.scene.ViewBox.W
		if vw <= 0 {
			vw = 300
		}
		vh := p.scene.ViewBox.H
		if vh <= 0 {
			vh = 150
		}
		tx, _ := parseAttrLength(attrs, "x", vw, 0)
		ty, _ := parseAttrLength(attrs, "y", vh, 0)
		return p.walkMaskNode(target, style, model.Translate(tx, ty).Then(transform), out)
	}

	path, ok, err := elementPath(node.Name, attrs, p.scene.ViewBox, p.opts.CurveTolerance)
	if err != nil {
		return err
	}
	if ok && len(path.Subpaths) > 0 {
		path = transformPath(path, transform)
		cmdStyle := style
		scaleStrokeStyle(&cmdStyle, transform.ApproxScale())
		if err := p.resolvePaintDependencies(&cmdStyle); err != nil && p.opts.Mode == ParseStrict {
			return err
		}
		*out = append(*out, model.Command{
			Path:  path,
			Style: cmdStyle,
		})
	}

	for _, child := range node.Children {
		if err := p.walkMaskNode(child, style, transform, out); err != nil {
			return err
		}
	}
	return nil
}
