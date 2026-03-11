package main

import (
	"flag"
	"fmt"
	"image/color"
	"os"
	"strings"

	"github.com/iamshiqing/svgoimg"
)

func main() {
	var (
		inPath  string
		outPath string
		width   int
		height  int
		fit     string
		bg      string
		strict  bool
		tol     float64
	)

	flag.StringVar(&inPath, "in", "", "input svg file path")
	flag.StringVar(&outPath, "out", "", "output png file path")
	flag.IntVar(&width, "w", 0, "output width, 0 means auto")
	flag.IntVar(&height, "h", 0, "output height, 0 means auto")
	flag.StringVar(&fit, "fit", "contain", "fit mode: contain|cover|stretch")
	flag.StringVar(&bg, "bg", "", "background color in #RRGGBB or #RRGGBBAA")
	flag.BoolVar(&strict, "strict", false, "strict svg parsing mode")
	flag.Float64Var(&tol, "tol", 0.6, "curve flatten tolerance")
	flag.Parse()

	if inPath == "" || outPath == "" {
		fmt.Fprintln(os.Stderr, "usage: svg2img -in input.svg -out output.png [-w 512 -h 512 -fit contain]")
		os.Exit(2)
	}

	fitMode, err := svgoimg.ParseFitMode(fit)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	var bgColor color.Color
	if strings.TrimSpace(bg) != "" {
		c, err := parseHexColor(bg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid bg color: %v\n", err)
			os.Exit(2)
		}
		bgColor = c
	}

	parseMode := svgoimg.ParseIgnore
	if strict {
		parseMode = svgoimg.ParseStrict
	}

	img, err := svgoimg.DecodeFile(inPath, &svgoimg.Options{
		Width:          width,
		Height:         height,
		Fit:            fitMode,
		Background:     bgColor,
		ParseMode:      parseMode,
		CurveTolerance: tol,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "render failed:", err)
		os.Exit(1)
	}

	out, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "create output failed:", err)
		os.Exit(1)
	}
	defer out.Close()

	if err := svgoimg.EncodePNG(out, img); err != nil {
		fmt.Fprintln(os.Stderr, "write png failed:", err)
		os.Exit(1)
	}
}

func parseHexColor(raw string) (color.NRGBA, error) {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "#"))
	switch len(raw) {
	case 6:
		var c color.NRGBA
		_, err := fmt.Sscanf(raw, "%02x%02x%02x", &c.R, &c.G, &c.B)
		c.A = 255
		return c, err
	case 8:
		var c color.NRGBA
		_, err := fmt.Sscanf(raw, "%02x%02x%02x%02x", &c.R, &c.G, &c.B, &c.A)
		return c, err
	default:
		return color.NRGBA{}, fmt.Errorf("expect #RRGGBB or #RRGGBBAA")
	}
}
