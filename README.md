# svgoimg

[English](./README.md) | [简体中文](./README.zh-CN.md)

Pure-Go SVG renderer that converts SVG into Go `image.Image` with built-in `defs/use` and gradient support | 纯 Go SVG 渲染库，可将 SVG 转为 Go `image.Image`，并内建支持 `defs/use` 与渐变。

## Goals

- No dependency on third-party SVG conversion libraries.
- Practical support for most icon/UI SVG files and many illustration SVGs.
- Stable public API (`Decode`, `DecodeFile`, `EncodePNG`).
- Clear project layout, examples, and bilingual docs.

## Current Support

### Elements

- `svg`, `g`, `defs`, `symbol`, `use`
- `path`
- `rect` (including rounded corners `rx`/`ry`)
- `circle`, `ellipse`
- `line`, `polyline`, `polygon`
- `linearGradient`, `radialGradient`, `stop`

### Path commands

- `M/m`, `L/l`, `H/h`, `V/v`
- `C/c`, `S/s`
- `Q/q`, `T/t`
- `A/a`
- `Z/z`

### Styling

- `fill`, `stroke`, `stroke-width`
- `opacity`, `fill-opacity`, `stroke-opacity` (supports number and percent forms like `0.5` / `50%`)
- `fill-rule` (`nonzero`, `evenodd`)
- `transform` (`matrix`, `translate`, `scale`, `rotate`, `skewX`, `skewY`)
- `style="..."`
- paint server reference: `fill="url(#id)"`, `stroke="url(#id)"`
- color formats: hex, `rgb(...)`, `rgba(...)`, `currentColor`, `transparent`, and common CSS/SVG named colors (for example `aliceblue`)

### Gradients

- `linearGradient` and `radialGradient`
- `gradientUnits` (`objectBoundingBox`, `userSpaceOnUse`)
- `gradientTransform`
- `spreadMethod` (`pad`, `repeat`, `reflect`)
- gradient inheritance via `href` / `xlink:href`
- `stop-color`, `stop-opacity`, `offset`, inline `style`

### defs/use

- Resolve reusable definitions by id.
- `use` supports `href`/`xlink:href`, `x`, `y`, `transform`.
- `symbol` + `viewBox` + `use width/height` basic mapping.
- Circular `use` references are guarded.

### Rendering options

- Output size (`Width`, `Height`)
- Fit mode (`contain`, `cover`, `stretch`)
- Optional background color
- Parse mode (`ignore`, `warn`, `strict`)
- Optional warning callback in warn mode: `Options.OnWarning func(error)`

When using `ParseWarn`, non-fatal parse issues are reported through `OnWarning` and rendering continues.

## Not Supported Yet

- `text`
- Filters (`fe*`)
- Clip/mask/composite effects
- Full CSS cascade/selectors
- Full `preserveAspectRatio` behavior for `symbol/use`
- Advanced paint servers and filter interactions

These are planned in incremental milestones.

## Installation

```bash
go get github.com/iamshiqing/svgoimg
```

## Quick Start

```go
package main

import (
	"image/color"
	"os"

	"github.com/iamshiqing/svgoimg"
)

func main() {
	img, err := svgoimg.DecodeFile("input.svg", &svgoimg.Options{
		Width:      1024,
		Height:     1024,
		Fit:        svgoimg.FitContain,
		Background: color.NRGBA{R: 255, G: 255, B: 255, A: 255},
	})
	if err != nil {
		panic(err)
	}

	out, err := os.Create("output.png")
	if err != nil {
		panic(err)
	}
	defer out.Close()

	if err := svgoimg.EncodePNG(out, img); err != nil {
		panic(err)
	}
}
```

## CLI Example

```bash
go run ./cmd/svg2img -in examples/assets/sample.svg -out examples/assets/sample.png -w 640 -h 360 -fit contain
```

## defs/use + Gradient Example

```xml
<svg viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <linearGradient id="g" x1="0%" y1="0%" x2="100%" y2="0%">
      <stop offset="0%" stop-color="#ff0000"/>
      <stop offset="100%" stop-color="#0000ff"/>
    </linearGradient>
    <symbol id="dot" viewBox="0 0 10 10">
      <circle cx="5" cy="5" r="5" fill="url(#g)"/>
    </symbol>
  </defs>
  <use href="#dot" x="10" y="10" width="30" height="30"/>
</svg>
```

## Golden Test Assets

- Input SVG folder: `testdata/svg_inputs`
- Expected PNG folder: `testdata/png_outputs`

Run golden comparison:

```bash
go test ./... -run TestGoldenSVGCases
```

Run perceptual-threshold comparison (useful when tiny anti-alias differences are acceptable):

```bash
go test ./... -run TestGoldenSVGCases -golden-compare=perceptual -golden-pixel-delta=2 -golden-max-mae=0.15 -golden-max-diff-ratio=0.001 -golden-max-channel-delta=12
```

Regenerate expected PNG outputs:

```bash
go test ./... -run TestGoldenSVGCases -update-golden
```

Run decode benchmarks:

```bash
go test . -bench BenchmarkDecode -benchmem
```

## Project Layout

```text
.
|-- cmd/svg2img/              # example CLI
|-- examples/
|   |-- assets/               # sample SVG and generated PNG
|   `-- basic/                # minimal code sample
|-- internal/
|   |-- model/                # shared scene/path/style/gradient structures
|   |-- svg/                  # XML + style + transform + defs/use + gradient parsing
|   `-- raster/               # rasterizer (fill/stroke/alpha + gradient sampling)
|-- decode.go                 # public decoding/encoding APIs
|-- options.go                # public options & enums
`-- README.md / README.zh-CN.md
```

## API Overview

- `Decode(r io.Reader, opts *Options) (*image.NRGBA, error)`
- `DecodeBytes(data []byte, opts *Options) (*image.NRGBA, error)`
- `DecodeString(svg string, opts *Options) (*image.NRGBA, error)`
- `DecodeFile(path string, opts *Options) (*image.NRGBA, error)`
- `EncodePNG(w io.Writer, img image.Image) error`
- `WritePNG(w io.Writer, svgReader io.Reader, opts *Options) error`

## Roadmap

1. Improve stroke joins/caps and anti-aliasing quality.
2. Add clip-path and mask.
3. Improve `symbol/use` viewport and `preserveAspectRatio` coverage.
4. Add filter and paint server extensions incrementally.
5. Add benchmark suite and fuzz tests for parser/rasterizer.

## License

MIT (add your preferred license text in `LICENSE` if needed).
