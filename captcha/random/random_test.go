package random

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestRandomDraw(t *testing.T) {
	c := NewRandom(4, LetterDigits|LetterLower|LetterUpper)
	_, d, _ := c.Draw()
	fmt.Printf("%+v\n", d)
	_, d, _ = c.Draw()
	fmt.Printf("%+v\n", d)
	_, d, _ = c.Draw()
	fmt.Printf("%+v\n", d)
	_, d, _ = c.Draw()
	fmt.Printf("%+v\n", d)
	_, d, _ = c.Draw()
	fmt.Printf("%+v\n", d)
	_, d, _ = c.Draw()
	fmt.Printf("%+v\n", d)
	_, d, _ = c.Draw()
	fmt.Printf("%+v\n", d)
}

func TestRandom(t *testing.T) {
	ce := NewRandom(6, LetterDigits|LetterLower|LetterUpper)
	id, cd, img := ce.Draw()
	fmt.Printf("id: %s, result: %s, data: %+v\n", id, cd.Result, cd)
	// 输出文件
	f, err := os.OpenFile(filepath.Join(t.TempDir(), "test.png"), os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0600)
	if err != nil {
		t.Error(err)
	}
	defer f.Close()
	err = png.Encode(f, img)
	if err != nil {
		t.Error(err)
	}
}

func BenchmarkCreateRandom(b *testing.B) {
	ce := NewRandom(6, LetterDigits|LetterLower|LetterUpper)
	for b.Loop() {
		ce.Create()
	}
}
