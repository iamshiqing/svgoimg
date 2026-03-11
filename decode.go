package svgoimg

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"

	"github.com/iamshiqing/svgoimg/internal/raster"
	"github.com/iamshiqing/svgoimg/internal/svg"
)

func Decode(r io.Reader, opts *Options) (*image.NRGBA, error) {
	options := Options{}
	if opts != nil {
		options = *opts
	}
	options = options.withDefaults()

	scene, err := svg.Parse(r, options.toSVGOptions())
	if err != nil {
		return nil, err
	}
	img, err := raster.Render(scene, options.toRasterOptions())
	if err != nil {
		return nil, err
	}
	return img, nil
}

func DecodeBytes(data []byte, opts *Options) (*image.NRGBA, error) {
	return Decode(bytes.NewReader(data), opts)
}

func DecodeString(svgXML string, opts *Options) (*image.NRGBA, error) {
	return Decode(stringsReader(svgXML), opts)
}

func DecodeFile(path string, opts *Options) (*image.NRGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	return Decode(f, opts)
}

func EncodePNG(w io.Writer, img image.Image) error {
	if w == nil {
		return fmt.Errorf("nil writer")
	}
	if img == nil {
		return fmt.Errorf("nil image")
	}
	return png.Encode(w, img)
}

func WritePNG(w io.Writer, svgReader io.Reader, opts *Options) error {
	img, err := Decode(svgReader, opts)
	if err != nil {
		return err
	}
	return EncodePNG(w, img)
}

func stringsReader(s string) io.Reader {
	return bytes.NewBufferString(s)
}
