# svgoimg

[English](./README.md) | [简体中文](./README.zh-CN.md)

Pure-Go SVG renderer that converts SVG into Go `image.Image` with built-in `defs/use` and gradient support | 纯 Go SVG 渲染库，可将 SVG 转为 Go `image.Image`，并内建支持 `defs/use` 与渐变。

## 项目目标

- 不使用第三方 SVG 转换库。
- 优先覆盖大多数图标/UI 场景和常见插画 SVG。
- 提供稳定、简单的库接口。
- 提供合理项目布局、示例和中英文文档。

## 当前支持（第 2 阶段）

### 元素

- `svg`, `g`, `defs`, `symbol`, `use`
- `path`
- `rect`（含圆角 `rx`/`ry`）
- `circle`, `ellipse`
- `line`, `polyline`, `polygon`
- `linearGradient`, `radialGradient`, `stop`

### 路径命令

- `M/m`, `L/l`, `H/h`, `V/v`
- `C/c`, `S/s`
- `Q/q`, `T/t`
- `A/a`
- `Z/z`

### 样式与变换

- `fill`, `stroke`, `stroke-width`
- `opacity`, `fill-opacity`, `stroke-opacity`
- `fill-rule`（`nonzero`, `evenodd`）
- `transform`（`matrix`, `translate`, `scale`, `rotate`, `skewX`, `skewY`）
- `style="..."`
- 支持 paint server 引用：`fill="url(#id)"`、`stroke="url(#id)"`

### 渐变

- `linearGradient` 与 `radialGradient`
- `gradientUnits`（`objectBoundingBox`, `userSpaceOnUse`）
- `gradientTransform`
- `spreadMethod`（`pad`, `repeat`, `reflect`）
- 渐变 `href` / `xlink:href` 继承
- `stop-color`、`stop-opacity`、`offset`、`style`

### defs/use

- 通过 `id` 解析可复用定义。
- `use` 支持 `href`/`xlink:href`、`x`、`y`、`transform`。
- 支持 `symbol + viewBox + use width/height` 的基础映射。
- 增加循环引用保护，避免 `use` 无限递归。

### 输出选项

- 输出尺寸（`Width`, `Height`）
- 适配模式（`contain`, `cover`, `stretch`）
- 可选背景色
- 解析模式（`ignore`, `warn`, `strict`）

## 暂未支持

- `text`
- 过滤器（`fe*`）
- `clipPath` / `mask` / 复合混合
- 完整 CSS 选择器级联
- `symbol/use` 的完整 `preserveAspectRatio` 行为
- 更高级的 paint server 与 filter 交互

这些能力会按里程碑持续补齐。

## 安装

```bash
go get github.com/iamshiqing/svgoimg
```

## 快速使用

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

## CLI 示例

```bash
go run ./cmd/svg2img -in examples/assets/sample.svg -out examples/assets/sample.png -w 640 -h 360 -fit contain
```

## defs/use + 渐变示例

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

## Golden 测试资源

- 输入 SVG 目录：`testdata/svg_inputs`
- 期望 PNG 目录：`testdata/png_outputs`

执行 golden 对比：

```bash
go test ./... -run TestGoldenSVGCases
```

执行感知阈值对比（适合允许轻微抗锯齿差异的场景）：

```bash
go test ./... -run TestGoldenSVGCases -golden-compare=perceptual -golden-pixel-delta=2 -golden-max-mae=0.15 -golden-max-diff-ratio=0.001 -golden-max-channel-delta=12
```

重新生成期望输出图片：

```bash
go test ./... -run TestGoldenSVGCases -update-golden
```

## 项目结构

```text
.
|-- cmd/svg2img/              # 示例命令行工具
|-- examples/
|   |-- assets/               # 示例 SVG 与输出 PNG
|   `-- basic/                # 最小调用示例
|-- internal/
|   |-- model/                # 场景/路径/样式/渐变数据结构
|   |-- svg/                  # XML + 样式 + 变换 + defs/use + 渐变解析
|   `-- raster/               # 栅格化内核（填充/描边/渐变采样/混合）
|-- decode.go                 # 对外解码/编码 API
|-- options.go                # 对外参数与枚举
`-- README.md / README.zh-CN.md
```

## 对外 API

- `Decode(r io.Reader, opts *Options) (*image.NRGBA, error)`
- `DecodeBytes(data []byte, opts *Options) (*image.NRGBA, error)`
- `DecodeString(svg string, opts *Options) (*image.NRGBA, error)`
- `DecodeFile(path string, opts *Options) (*image.NRGBA, error)`
- `EncodePNG(w io.Writer, img image.Image) error`
- `WritePNG(w io.Writer, svgReader io.Reader, opts *Options) error`

## 后续计划

1. 改进描边拐角/端点与抗锯齿质量。
2. 增加 `clipPath` 与 `mask`。
3. 完善 `symbol/use` 视口与 `preserveAspectRatio` 覆盖。
4. 持续扩展 filter 与 paint server 能力。
5. 增加 parser/rasterizer 的 benchmark 与 fuzz 测试。

## 许可证

建议使用 MIT，可在 `LICENSE` 中补充正式文本。
