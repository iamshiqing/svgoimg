package raster

import "image/color"

type FitMode uint8

const (
	FitContain FitMode = iota
	FitCover
	FitStretch
)

type Options struct {
	Width      int
	Height     int
	Fit        FitMode
	Background color.Color
}

func (o Options) withDefaults() Options {
	if o.Fit > FitStretch {
		o.Fit = FitContain
	}
	return o
}
