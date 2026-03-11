package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/iamshiqing/svgoimg"
)

func main() {
	in := filepath.Join("examples", "assets", "sample.svg")
	out := filepath.Join("examples", "assets", "sample.png")

	img, err := svgoimg.DecodeFile(in, &svgoimg.Options{
		Width:  640,
		Height: 360,
		Fit:    svgoimg.FitContain,
	})
	if err != nil {
		panic(err)
	}

	f, err := os.Create(out)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	if err := svgoimg.EncodePNG(f, img); err != nil {
		panic(err)
	}

	fmt.Println("rendered:", out)
}
