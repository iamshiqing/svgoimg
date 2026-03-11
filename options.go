package svgoimg

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/iamshiqing/svgoimg/internal/raster"
	"github.com/iamshiqing/svgoimg/internal/svg"
)

type FitMode uint8

const (
	// FitContain keeps aspect ratio and fully fits inside target size.
	FitContain FitMode = iota
	// FitCover keeps aspect ratio and fully covers target size (may crop).
	FitCover
	// FitStretch stretches to target size and may distort aspect ratio.
	FitStretch
)

func (m FitMode) String() string {
	switch m {
	case FitContain:
		return "contain"
	case FitCover:
		return "cover"
	case FitStretch:
		return "stretch"
	default:
		return "contain"
	}
}

func ParseFitMode(raw string) (FitMode, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "contain":
		return FitContain, nil
	case "cover":
		return FitCover, nil
	case "stretch":
		return FitStretch, nil
	default:
		return FitContain, fmt.Errorf("unknown fit mode %q", raw)
	}
}

type ParseMode uint8

const (
	ParseIgnore ParseMode = iota
	ParseWarn
	ParseStrict
)

type Options struct {
	Width          int
	Height         int
	Fit            FitMode
	Background     color.Color
	ParseMode      ParseMode
	OnWarning      func(error)
	CurveTolerance float64
}

func (o Options) withDefaults() Options {
	if o.Fit > FitStretch {
		o.Fit = FitContain
	}
	if o.CurveTolerance <= 0 {
		o.CurveTolerance = 0.6
	}
	return o
}

func (o Options) toSVGOptions() svg.Options {
	return svg.Options{
		Mode:           svg.ParseMode(o.ParseMode),
		OnWarning:      o.OnWarning,
		CurveTolerance: o.CurveTolerance,
	}
}

func (o Options) toRasterOptions() raster.Options {
	return raster.Options{
		Width:      o.Width,
		Height:     o.Height,
		Fit:        raster.FitMode(o.Fit),
		Background: o.Background,
	}
}
