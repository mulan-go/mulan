package equation

import (
	"fmt"
	"image/png"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/mulan-go/mulan/captcha/core"
)

func TestEquationDraw(t *testing.T) {
	c := NewEquation(2)
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

func TestEquationMethodAdd(t *testing.T) {
	bit := 2
	minNum := int64(math.Pow10(bit - 1))
	// left + right = result
	int63n := core.BitNumInt63n(bit)
	for range 20 {
		resultNum := int63n()
		for resultNum < minNum*2 {
			resultNum = int63n()
		}
		var left, right int64
		for left < minNum || right < minNum {
			left = rand.Int63n(resultNum-minNum) + minNum
			right = resultNum - left
		}
		fmt.Printf("%d + %d = %d\n", left, right, resultNum)
	}
}

func TestEquationMethodSub(t *testing.T) {
	bit := 2
	minNum := int64(math.Pow10(bit - 1))
	var resultNum int64
	int63n := core.BitNumInt63n(bit)
	// left - right = result
	for range 20 {
		var left, right int64
		for left < minNum*2 {
			left = int63n()
		}
		for resultNum < minNum || right < minNum {
			resultNum = rand.Int63n(left-minNum) + minNum
			right = left - resultNum
		}
		fmt.Printf("%d - %d = %d\n", left, right, resultNum)
	}
}

func TestEquationMethodMul(t *testing.T) {
	bit := 2
	var resultNum int64
	int63n := core.BitNumInt63n(bit)
	// left * right = result
	for range 20 {
		var left, right = int63n(), int63n()
		resultNum = left * right
		fmt.Printf("%d * %d = %d\n", left, right, resultNum)
	}
}

func TestEquation(t *testing.T) {
	ce := NewEquation(2)
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

func TestCreateB64(t *testing.T) {
	ce := NewEquation(2)
	id, result, _, err := ce.CreateB64()
	if err != nil {
		t.Error(err)
	}
	fmt.Println(id)
	fmt.Println(result)
	if ok, _ := ce.Check(id, result); !ok {
		t.Fail()
	}
}

func BenchmarkCreateEquation(b *testing.B) {
	ce := NewEquation(2)
	for b.Loop() {
		ce.Create()
	}
}
