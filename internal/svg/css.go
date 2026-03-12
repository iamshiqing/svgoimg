package svg

import (
	"fmt"
	"sort"
	"strings"
)

type cssSelectorKind uint8

const (
	cssSelectorElement cssSelectorKind = iota
	cssSelectorClass
	cssSelectorID
)

type cssSelector struct {
	Kind  cssSelectorKind
	Value string
}

type cssRule struct {
	Selector    cssSelector
	Props       map[string]string
	Specificity int
	Order       int
}

func (p *parserState) collectCSS(node *xmlNode) {
	if node == nil {
		return
	}
	if node.Name == "style" {
		raw := strings.TrimSpace(node.Text)
		if raw != "" {
			p.cssRules = append(p.cssRules, parseCSSRules(raw)...)
		}
	}
	for _, child := range node.Children {
		p.collectCSS(child)
	}
}

func parseCSSRules(raw string) []cssRule {
	raw = stripCSSComments(raw)
	out := make([]cssRule, 0, 16)
	order := 0

	for _, block := range strings.Split(raw, "}") {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		parts := strings.SplitN(block, "{", 2)
		if len(parts) != 2 {
			continue
		}
		selectorsPart := strings.TrimSpace(parts[0])
		declPart := strings.TrimSpace(parts[1])
		if selectorsPart == "" || declPart == "" {
			continue
		}
		props := parseStyleDeclarations(declPart)
		if len(props) == 0 {
			continue
		}
		selectors := strings.Split(selectorsPart, ",")
		for _, selRaw := range selectors {
			selRaw = strings.TrimSpace(selRaw)
			sel, spec, ok := parseSimpleSelector(selRaw)
			if !ok {
				continue
			}
			cp := make(map[string]string, len(props))
			for k, v := range props {
				cp[k] = v
			}
			out = append(out, cssRule{
				Selector:    sel,
				Props:       cp,
				Specificity: spec,
				Order:       order,
			})
			order++
		}
	}
	return out
}

func stripCSSComments(raw string) string {
	var b strings.Builder
	i := 0
	for i < len(raw) {
		if i+1 < len(raw) && raw[i] == '/' && raw[i+1] == '*' {
			i += 2
			for i+1 < len(raw) && !(raw[i] == '*' && raw[i+1] == '/') {
				i++
			}
			if i+1 < len(raw) {
				i += 2
			}
			continue
		}
		b.WriteByte(raw[i])
		i++
	}
	return b.String()
}

func parseSimpleSelector(raw string) (cssSelector, int, bool) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return cssSelector{}, 0, false
	}
	if strings.ContainsAny(raw, " >+~:[") || strings.Contains(raw, " ") {
		return cssSelector{}, 0, false
	}
	if strings.HasPrefix(raw, "#") {
		id := strings.TrimSpace(raw[1:])
		if id == "" {
			return cssSelector{}, 0, false
		}
		return cssSelector{Kind: cssSelectorID, Value: id}, 100, true
	}
	if strings.HasPrefix(raw, ".") {
		className := strings.TrimSpace(raw[1:])
		if className == "" {
			return cssSelector{}, 0, false
		}
		return cssSelector{Kind: cssSelectorClass, Value: className}, 10, true
	}
	return cssSelector{Kind: cssSelectorElement, Value: raw}, 1, true
}

func (p *parserState) mergedAttrs(node *xmlNode) map[string]string {
	out := make(map[string]string, len(node.Attrs)+8)
	for k, v := range node.Attrs {
		out[k] = v
	}
	if len(p.cssRules) > 0 {
		matched := make([]cssRule, 0, 8)
		for _, rule := range p.cssRules {
			if cssRuleMatches(rule.Selector, node) {
				matched = append(matched, rule)
			}
		}
		sort.SliceStable(matched, func(i, j int) bool {
			if matched[i].Specificity != matched[j].Specificity {
				return matched[i].Specificity < matched[j].Specificity
			}
			return matched[i].Order < matched[j].Order
		})
		for _, rule := range matched {
			for k, v := range rule.Props {
				out[k] = v
			}
		}
	}
	return out
}

func cssRuleMatches(sel cssSelector, node *xmlNode) bool {
	switch sel.Kind {
	case cssSelectorElement:
		return strings.EqualFold(node.Name, sel.Value)
	case cssSelectorID:
		return strings.EqualFold(strings.TrimSpace(node.Attrs["id"]), sel.Value)
	case cssSelectorClass:
		classAttr := strings.TrimSpace(node.Attrs["class"])
		if classAttr == "" {
			return false
		}
		for _, c := range strings.Fields(classAttr) {
			if strings.EqualFold(strings.TrimSpace(c), sel.Value) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func (p *parserState) warnCSSUnsupported() error {
	return fmt.Errorf("some CSS selectors are ignored (only tag/.class/#id are supported)")
}
