package svg

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/url"
	"strings"

	"github.com/iamshiqing/svgoimg/internal/model"
)

func (p *parserState) elementImage(attrs map[string]string, transform model.Matrix, style model.Style) (model.ImageDraw, bool, error) {
	href := strings.TrimSpace(attrs["href"])
	if href == "" {
		return model.ImageDraw{}, false, fmt.Errorf("image missing href")
	}
	src, err := decodeImageHref(href)
	if err != nil {
		return model.ImageDraw{}, false, err
	}
	sb := src.Bounds()
	if sb.Dx() <= 0 || sb.Dy() <= 0 {
		return model.ImageDraw{}, false, fmt.Errorf("image has invalid bounds")
	}

	vw, vh := p.scene.ViewBox.W, p.scene.ViewBox.H
	if vw <= 0 {
		vw = 300
	}
	if vh <= 0 {
		vh = 150
	}
	x, _ := parseAttrLength(attrs, "x", vw, 0)
	y, _ := parseAttrLength(attrs, "y", vh, 0)
	w, _ := parseAttrLength(attrs, "width", vw, 0)
	h, _ := parseAttrLength(attrs, "height", vh, 0)
	if w <= 0 || h <= 0 {
		return model.ImageDraw{}, false, fmt.Errorf("image width/height must be > 0")
	}

	dx, dy, dw, dh, err := resolveImageContentRect(
		x, y, w, h,
		float64(sb.Dx()), float64(sb.Dy()),
		attrs["preserveaspectratio"],
	)
	if err != nil {
		return model.ImageDraw{}, false, err
	}

	p0 := transform.Apply(model.Point{X: dx, Y: dy})
	p1 := transform.Apply(model.Point{X: dx + dw, Y: dy})
	p3 := transform.Apply(model.Point{X: dx, Y: dy + dh})
	c0 := transform.Apply(model.Point{X: x, Y: y})
	c1 := transform.Apply(model.Point{X: x + w, Y: y})
	c3 := transform.Apply(model.Point{X: x, Y: y + h})

	return model.ImageDraw{
		Img:     src,
		Opacity: style.Opacity,
		P0:      p0,
		P1:      p1,
		P3:      p3,
		C0:      c0,
		C1:      c1,
		C3:      c3,
	}, true, nil
}

func decodeImageHref(raw string) (image.Image, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty image href")
	}
	if strings.HasPrefix(strings.ToLower(raw), "data:") {
		return decodeDataURIImage(raw)
	}
	return nil, fmt.Errorf("unsupported image href %q (only data URI is supported)", raw)
}

func decodeDataURIImage(uri string) (image.Image, error) {
	comma := strings.IndexByte(uri, ',')
	if comma < 0 {
		return nil, fmt.Errorf("invalid data URI")
	}
	meta := strings.TrimSpace(uri[5:comma])
	data := uri[comma+1:]
	metaParts := strings.Split(meta, ";")
	mime := "text/plain"
	if len(metaParts) > 0 && strings.TrimSpace(metaParts[0]) != "" {
		mime = strings.ToLower(strings.TrimSpace(metaParts[0]))
	}
	if !strings.HasPrefix(mime, "image/") {
		return nil, fmt.Errorf("unsupported data URI mime type %q", mime)
	}
	isBase64 := false
	for _, p := range metaParts[1:] {
		if strings.EqualFold(strings.TrimSpace(p), "base64") {
			isBase64 = true
			break
		}
	}

	var payload []byte
	if isBase64 {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(data))
		if err != nil {
			decoded, err = base64.RawStdEncoding.DecodeString(strings.TrimSpace(data))
			if err != nil {
				return nil, fmt.Errorf("decode base64 image data: %w", err)
			}
		}
		payload = decoded
	} else {
		decoded, err := url.PathUnescape(data)
		if err != nil {
			return nil, fmt.Errorf("decode escaped image data: %w", err)
		}
		payload = []byte(decoded)
	}

	img, _, err := image.Decode(bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("decode image payload: %w", err)
	}
	return img, nil
}

func resolveImageContentRect(vx, vy, vw, vh, srcW, srcH float64, preserveAspectRaw string) (x, y, w, h float64, err error) {
	if vw <= 0 || vh <= 0 || srcW <= 0 || srcH <= 0 {
		return vx, vy, vw, vh, nil
	}
	raw := strings.TrimSpace(strings.ToLower(preserveAspectRaw))
	if raw == "" {
		raw = "xmidymid meet"
	}
	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return vx, vy, vw, vh, nil
	}
	if parts[0] == "defer" {
		parts = parts[1:]
		if len(parts) == 0 {
			return vx, vy, vw, vh, nil
		}
	}
	align := parts[0]
	mode := "meet"
	if len(parts) > 1 {
		if parts[1] == "slice" {
			mode = "slice"
		} else if parts[1] == "meet" {
			mode = "meet"
		} else {
			return 0, 0, 0, 0, fmt.Errorf("unsupported preserveAspectRatio mode %q", parts[1])
		}
	}
	if align == "none" {
		return vx, vy, vw, vh, nil
	}
	alignX, alignY, ok := parseAspectAlign(align)
	if !ok {
		return 0, 0, 0, 0, fmt.Errorf("unsupported preserveAspectRatio align %q", align)
	}

	sx := vw / srcW
	sy := vh / srcH
	scale := sx
	if mode == "meet" {
		if sy < sx {
			scale = sy
		}
	} else {
		if sy > sx {
			scale = sy
		}
	}
	dw := srcW * scale
	dh := srcH * scale
	dx := vx + (vw-dw)*alignX
	dy := vy + (vh-dh)*alignY
	return dx, dy, dw, dh, nil
}

func parseAspectAlign(raw string) (ax, ay float64, ok bool) {
	switch raw {
	case "xminymin":
		return 0, 0, true
	case "xmidymin":
		return 0.5, 0, true
	case "xmaxymin":
		return 1, 0, true
	case "xminymid":
		return 0, 0.5, true
	case "xmidymid":
		return 0.5, 0.5, true
	case "xmaxymid":
		return 1, 0.5, true
	case "xminymax":
		return 0, 1, true
	case "xmidymax":
		return 0.5, 1, true
	case "xmaxymax":
		return 1, 1, true
	default:
		return 0, 0, false
	}
}
