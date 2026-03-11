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

func TestGoldenSVGCases(t *testing.T) {
	inputDir := filepath.Join("testdata", "svg_inputs")
	outputDir := filepath.Join("testdata", "png_outputs")

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

			if err := compareImageExact(expImg, gotImg); err != nil {
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
