package svgoimg

import (
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkDecode_ComplexLandscape(b *testing.B) {
	data, err := os.ReadFile(filepath.Join("testdata", "svg_inputs", "020-complex-landscape.svg"))
	if err != nil {
		b.Fatalf("read benchmark svg failed: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := DecodeBytes(data, nil); err != nil {
			b.Fatalf("DecodeBytes failed: %v", err)
		}
	}
}

func BenchmarkDecode_NestedUseChain(b *testing.B) {
	data, err := os.ReadFile(filepath.Join("testdata", "svg_inputs", "017-nested-use-chain.svg"))
	if err != nil {
		b.Fatalf("read benchmark svg failed: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := DecodeBytes(data, nil); err != nil {
			b.Fatalf("DecodeBytes failed: %v", err)
		}
	}
}
