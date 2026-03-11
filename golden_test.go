package svgoimg

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update-golden", false, "update golden PNG outputs under testdata/png_outputs")
var goldenCompareMode = flag.String("golden-compare", "exact", "golden compare mode: exact|perceptual")
var goldenPixelDelta = flag.Int("golden-pixel-delta", 0, "per-pixel channel delta threshold (0-255) for perceptual mode")
var goldenMaxMAE = flag.Float64("golden-max-mae", 0, "max mean absolute error per channel (0-255) for perceptual mode")
var goldenMaxDiffRatio = flag.Float64("golden-max-diff-ratio", 0, "max changed-pixel ratio [0,1] for perceptual mode")
var goldenMaxChannelDelta = flag.Int("golden-max-channel-delta", 0, "max allowed single-channel delta (0-255) for perceptual mode")

func TestGoldenSVGCases(t *testing.T) {
	inputDir := filepath.Join("testdata", "svg_inputs")
	outputDir := filepath.Join("testdata", "png_outputs")
	cfg, err := compareConfigFromFlags()
	if err != nil {
		t.Fatalf("invalid compare flags: %v", err)
	}

	entries, err := os.ReadDir(inputDir)
	if err != nil {
		t.Fatalf("read input dir failed: %v", err)
	}

	var cases []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.EqualFold(filepath.Ext(name), ".svg") {
			cases = append(cases, name)
		}
	}
	sort.Strings(cases)

	if len(cases) == 0 {
		t.Fatalf("no svg cases found in %s", inputDir)
	}

	if *updateGolden {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			t.Fatalf("mkdir output dir failed: %v", err)
		}
	}

	for _, name := range cases {
		name := name
		t.Run(name, func(t *testing.T) {
			inPath := filepath.Join(inputDir, name)
			svgData, err := os.ReadFile(inPath)
			if err != nil {
				t.Fatalf("read svg failed: %v", err)
			}

			img, err := DecodeBytes(svgData, nil)
			if err != nil {
				t.Fatalf("decode svg failed: %v", err)
			}

			var buf bytes.Buffer
			if err := EncodePNG(&buf, img); err != nil {
				t.Fatalf("encode png failed: %v", err)
			}

			base := strings.TrimSuffix(name, filepath.Ext(name))
			outPath := filepath.Join(outputDir, base+".png")

			if *updateGolden {
				if err := os.WriteFile(outPath, buf.Bytes(), 0o644); err != nil {
					t.Fatalf("write golden failed: %v", err)
				}
				return
			}

			expBytes, err := os.ReadFile(outPath)
			if err != nil {
				if os.IsNotExist(err) {
					t.Fatalf("missing golden output %s; run: go test ./... -run TestGoldenSVGCases -update-golden", outPath)
				}
				t.Fatalf("read golden output failed: %v", err)
			}

			expImg, err := png.Decode(bytes.NewReader(expBytes))
			if err != nil {
				t.Fatalf("decode golden png failed: %v", err)
			}
			gotImg, err := png.Decode(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatalf("decode rendered png failed: %v", err)
			}

			if err := compareImage(expImg, gotImg, cfg); err != nil {
				t.Fatalf("image mismatch: %v", err)
			}
		})
	}
}

func compareImageExact(a, b image.Image) error {
	ab := a.Bounds()
	bb := b.Bounds()
	if ab.Dx() != bb.Dx() || ab.Dy() != bb.Dy() {
		return fmt.Errorf("bounds mismatch: got %dx%d, want %dx%d", bb.Dx(), bb.Dy(), ab.Dx(), ab.Dy())
	}

	for y := 0; y < ab.Dy(); y++ {
		for x := 0; x < ab.Dx(); x++ {
			ca := color.NRGBAModel.Convert(a.At(ab.Min.X+x, ab.Min.Y+y)).(color.NRGBA)
			cb := color.NRGBAModel.Convert(b.At(bb.Min.X+x, bb.Min.Y+y)).(color.NRGBA)
			if ca != cb {
				return fmt.Errorf("first pixel diff at (%d,%d): got=%#v want=%#v", x, y, cb, ca)
			}
		}
	}
	return nil
}

type compareConfig struct {
	mode            compareMode
	pixelDelta      uint8
	maxMAE          float64
	maxDiffRatio    float64
	maxChannelDelta uint8
}

type compareMode uint8

const (
	compareModeExact compareMode = iota
	compareModePerceptual
)

func compareConfigFromFlags() (compareConfig, error) {
	modeRaw := strings.ToLower(strings.TrimSpace(*goldenCompareMode))
	cfg := compareConfig{
		mode: compareModeExact,
	}
	switch modeRaw {
	case "", "exact":
		cfg.mode = compareModeExact
	case "perceptual":
		cfg.mode = compareModePerceptual
	default:
		return compareConfig{}, fmt.Errorf("unsupported -golden-compare value %q", *goldenCompareMode)
	}

	if *goldenPixelDelta < 0 || *goldenPixelDelta > 255 {
		return compareConfig{}, fmt.Errorf("-golden-pixel-delta must be in [0,255]")
	}
	if *goldenMaxChannelDelta < 0 || *goldenMaxChannelDelta > 255 {
		return compareConfig{}, fmt.Errorf("-golden-max-channel-delta must be in [0,255]")
	}
	if *goldenMaxMAE < 0 || *goldenMaxMAE > 255 {
		return compareConfig{}, fmt.Errorf("-golden-max-mae must be in [0,255]")
	}
	if *goldenMaxDiffRatio < 0 || *goldenMaxDiffRatio > 1 {
		return compareConfig{}, fmt.Errorf("-golden-max-diff-ratio must be in [0,1]")
	}
	cfg.pixelDelta = uint8(*goldenPixelDelta)
	cfg.maxChannelDelta = uint8(*goldenMaxChannelDelta)
	cfg.maxMAE = *goldenMaxMAE
	cfg.maxDiffRatio = *goldenMaxDiffRatio
	return cfg, nil
}

func compareImage(a, b image.Image, cfg compareConfig) error {
	switch cfg.mode {
	case compareModeExact:
		return compareImageExact(a, b)
	case compareModePerceptual:
		return compareImagePerceptual(a, b, cfg)
	default:
		return fmt.Errorf("invalid compare mode")
	}
}

type perceptualStats struct {
	diffPixels  int
	totalPixels int
	sumAbs      int64
	maxDelta    uint8
	firstX      int
	firstY      int
	firstA      color.NRGBA
	firstB      color.NRGBA
	hasFirst    bool
}

func compareImagePerceptual(a, b image.Image, cfg compareConfig) error {
	ab := a.Bounds()
	bb := b.Bounds()
	if ab.Dx() != bb.Dx() || ab.Dy() != bb.Dy() {
		return fmt.Errorf("bounds mismatch: got %dx%d, want %dx%d", bb.Dx(), bb.Dy(), ab.Dx(), ab.Dy())
	}

	stats := perceptualStats{
		totalPixels: ab.Dx() * ab.Dy(),
	}
	for y := 0; y < ab.Dy(); y++ {
		for x := 0; x < ab.Dx(); x++ {
			ca := color.NRGBAModel.Convert(a.At(ab.Min.X+x, ab.Min.Y+y)).(color.NRGBA)
			cb := color.NRGBAModel.Convert(b.At(bb.Min.X+x, bb.Min.Y+y)).(color.NRGBA)

			dr := absInt(int(ca.R) - int(cb.R))
			dg := absInt(int(ca.G) - int(cb.G))
			db := absInt(int(ca.B) - int(cb.B))
			da := absInt(int(ca.A) - int(cb.A))
			pixelMax := max4(dr, dg, db, da)

			stats.sumAbs += int64(dr + dg + db + da)
			if pixelMax > int(stats.maxDelta) {
				stats.maxDelta = uint8(pixelMax)
			}
			if pixelMax > int(cfg.pixelDelta) {
				stats.diffPixels++
				if !stats.hasFirst {
					stats.hasFirst = true
					stats.firstX = x
					stats.firstY = y
					stats.firstA = ca
					stats.firstB = cb
				}
			}
		}
	}

	totalChannels := stats.totalPixels * 4
	mae := 0.0
	if totalChannels > 0 {
		mae = float64(stats.sumAbs) / float64(totalChannels)
	}
	diffRatio := 0.0
	if stats.totalPixels > 0 {
		diffRatio = float64(stats.diffPixels) / float64(stats.totalPixels)
	}

	if stats.maxDelta > cfg.maxChannelDelta || mae > cfg.maxMAE || diffRatio > cfg.maxDiffRatio {
		first := ""
		if stats.hasFirst {
			first = fmt.Sprintf(", first diff at (%d,%d): got=%#v want=%#v", stats.firstX, stats.firstY, stats.firstB, stats.firstA)
		}
		return fmt.Errorf(
			"perceptual thresholds exceeded: maxDelta=%d (limit=%d), MAE=%.4f (limit=%.4f), diffRatio=%.6f (limit=%.6f), diffPixels=%d/%d%s",
			stats.maxDelta, cfg.maxChannelDelta,
			mae, cfg.maxMAE,
			diffRatio, cfg.maxDiffRatio,
			stats.diffPixels, stats.totalPixels,
			first,
		)
	}
	return nil
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func max4(a, b, c, d int) int {
	m := a
	if b > m {
		m = b
	}
	if c > m {
		m = c
	}
	if d > m {
		m = d
	}
	return m
}
