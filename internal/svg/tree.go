package svg

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type xmlNode struct {
	Name     string
	Attrs    map[string]string
	Text     string
	Children []*xmlNode
}

func parseXMLTree(r io.Reader) (*xmlNode, error) {
	root := &xmlNode{
		Name:  "__root__",
		Attrs: map[string]string{},
	}
	stack := []*xmlNode{root}

	dec := xml.NewDecoder(r)
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("xml parse failed: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			n := &xmlNode{
				Name:  strings.ToLower(strings.TrimSpace(t.Name.Local)),
				Attrs: attrsMap(t.Attr),
			}
			parent := stack[len(stack)-1]
			parent.Children = append(parent.Children, n)
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			if len(stack) == 0 {
				continue
			}
			stack[len(stack)-1].Text += string(t)
		}
	}
	return root, nil
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
