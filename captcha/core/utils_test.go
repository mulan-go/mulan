package core

import (
	"math"
	"math/rand"
	"testing"
)

func BitNum(bit int) int64 {
	x := math.Pow10(bit - 1)
	return rand.Int63n(int64(math.Pow10(bit)-x)) + int64(x)
}

func BenchmarkNBitNum(b *testing.B) {
	b.Run("BitNum", func(b *testing.B) {
		for i := 1; b.Loop(); i++ {
			BitNum(8)
		}
	})
	b.Run("BitNumInt63n", func(b *testing.B) {
		int63n := BitNumInt63n(8)
		for i := 1; b.Loop(); i++ {
			int63n()
		}
	})
}

func TestNBitBun(t *testing.T) {
	for i := 1; i < 8; i++ {
		t.Log(BitNum(i))
	}
}
