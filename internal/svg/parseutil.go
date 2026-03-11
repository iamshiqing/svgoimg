package svg

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func parseFloat(raw string) (float64, error) {
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number %q: %w", raw, err)
	}
	return v, nil
}

func parseOpacity(raw string) (float64, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasSuffix(raw, "%") {
		v, err := parseFloat(strings.TrimSuffix(raw, "%"))
		if err != nil {
			return 0, err
		}
		return clamp01(v / 100.0), nil
	}
	v, err := parseFloat(raw)
	if err != nil {
		return 0, err
	}
	return clamp01(v), nil
}

func splitUnit(raw string) (num, unit string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	i := len(raw)
	for i > 0 {
		ch := raw[i-1]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '%' {
			i--
			continue
		}
		break
	}
	return strings.TrimSpace(raw[:i]), strings.ToLower(strings.TrimSpace(raw[i:]))
}

func parseLength(raw string, percentBase float64) (float64, error) {
	num, unit := splitUnit(raw)
	if num == "" {
		return 0, fmt.Errorf("empty length")
	}
	v, err := parseFloat(num)
	if err != nil {
		return 0, err
	}
	switch unit {
	case "", "px":
		return v, nil
	case "%":
		return percentBase * v / 100.0, nil
	case "in":
		return v * 96.0, nil
	case "cm":
		return v * 96.0 / 2.54, nil
	case "mm":
		return v * 96.0 / 25.4, nil
	case "pt":
		return v * 96.0 / 72.0, nil
	case "pc":
		return v * 16.0, nil
	default:
		// Unknown unit: best effort by number only.
		return v, nil
	}
}

func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
