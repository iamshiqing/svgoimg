package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/iamshiqing/svgoimg"
)

func main() {
	in := filepath.Join("examples", "assets", "phase3.svg")
	out := filepath.Join("examples", "assets", "phase3.png")

	img, err := svgoimg.DecodeFile(in, &svgoimg.Options{
		Width:  900,
		Height: 600,
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
