# svgoimg

[English](./README.md) | [简体中文](./README.zh-CN.md)

Pure-Go SVG renderer that converts SVG into Go `image.Image` with built-in `defs/use` and gradient support | 纯 Go SVG 渲染库，可将 SVG 转为 Go `image.Image`，并内建支持 `defs/use` 与渐变。

## 项目目标

- 不使用第三方 SVG 转换库。
- 优先覆盖大多数图标/UI 场景和常见插画 SVG。
- 提供稳定、简单的库接口。
- 提供合理项目布局、示例和中英文文档。

## 当前支持（第 3 阶段基础）

### 元素

- `svg`, `g`, `defs`, `symbol`, `use`
- `path`
- `rect`（含圆角 `rx`/`ry`）
- `circle`, `ellipse`
- `line`, `polyline`, `polygon`
- `image`（支持 data URI 的 PNG/JPEG/GIF）
- `clipPath`, `mask`（基础几何掩膜管线）
- `marker`, `pattern`（第 3 阶段基础覆盖）
- `linearGradient`, `radialGradient`, `stop`
- `style`（基础选择器支持）

### 路径命令

- `M/m`, `L/l`, `H/h`, `V/v`
- `C/c`, `S/s`
- `Q/q`, `T/t`
- `A/a`
- `Z/z`

### 样式与变换

- `fill`, `stroke`, `stroke-width`
- `stroke-linecap`, `stroke-linejoin`, `stroke-miterlimit`
- `stroke-dasharray`, `stroke-dashoffset`
- `opacity`, `fill-opacity`, `stroke-opacity`（支持数字与百分比写法，如 `0.5` / `50%`）
- `fill-rule`（`nonzero`, `evenodd`）
- `transform`（`matrix`, `translate`, `scale`, `rotate`, `skewX`, `skewY`）
- `style="..."`
- marker 引用：`marker`, `marker-start`, `marker-mid`, `marker-end`
- 裁剪/掩膜引用：`clip-path`, `mask`
- 支持 paint server 引用：`fill="url(#id)"`、`stroke="url(#id)"`
- 颜色支持：hex、`rgb(...)`、`rgba(...)`、`currentColor`、`transparent`，以及常见 CSS/SVG 命名色（如 `aliceblue`）

### CSS

- 支持 `<style>` 的基础选择器：
- 元素选择器（如 `rect`）
- 类选择器（如 `.btn`）
- id 选择器（如 `#logo`）
- 基础优先级（`element < class < id`，且内联属性仍优先）

### 渐变

- `linearGradient` 与 `radialGradient`
- `gradientUnits`（`objectBoundingBox`, `userSpaceOnUse`）
- `gradientTransform`
- `spreadMethod`（`pad`, `repeat`, `reflect`）
- 渐变 `href` / `xlink:href` 继承
- `stop-color`、`stop-opacity`、`offset`、`style`

### defs/use / clip / mask / marker / pattern

- 通过 `id` 解析可复用定义。
- `use` 支持 `href`/`xlink:href`、`x`、`y`、`transform`。
- 支持 `symbol + viewBox + use width/height` 的基础映射。
- 增加循环引用保护，避免 `use` 无限递归。
- 支持 `clipPath` 引用并按命令应用裁剪。
- 支持 `mask` 引用（当前为几何掩膜基础实现）。
- 支持 `marker` 的 `start/mid/end` 展开。
- 支持 `pattern` 作为填充/描边画刷（含 userSpace/OBB 平铺）。

### 输出选项

- 输出尺寸（`Width`, `Height`）
- 适配模式（`contain`, `cover`, `stretch`）
- 可选背景色
- 解析模式（`ignore`, `warn`, `strict`）
- `warn` 模式可选告警回调：`Options.OnWarning func(error)`

使用 `ParseWarn` 时，非致命解析问题会通过 `OnWarning` 回调上报，并继续渲染。

## 暂未支持

- `text`
- 过滤器（`fe*`）
- 完整 CSS 级联/复杂选择器/伪类
- `symbol/use` 的完整 `preserveAspectRatio` 行为
- 完整 SVG mask 复合模型（当前是几何掩膜基线）
- 更高级的 paint server 与 filter 交互及完整规范一致性

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

执行解码性能基准：

```bash
go test . -bench BenchmarkDecode -benchmem
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

1. 继续完善 `symbol/use` 与 `preserveAspectRatio` 的规范一致性。
2. 将当前几何掩膜升级为完整 SVG mask 复合模型。
3. 扩展 CSS（后代选择器、组合选择器、更完整级联规则）。
4. 增加 `text` 渲染能力。
5. 逐步补齐 `fe*` filter 能力。
6. 增加 parser/rasterizer 的 fuzz 测试与更大规模基准集。

## 许可证

建议使用 MIT，可在 `LICENSE` 中补充正式文本。
