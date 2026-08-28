package captcha_test

import (
	"fmt"

	"github.com/mulan-go/mulan/captcha"
	"github.com/mulan-go/mulan/captcha/equation"
)

func init() {
	captcha.SetGlobal(equation.NewEquation(2))
}

func ExampleCreateB64() {
	id, result, data, err := captcha.CreateB64()
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(id)
	fmt.Println(data)
	if ok, _ := captcha.Check(id, "xxxx"); ok {
		fmt.Println("test check")
	} else if ok, _ := captcha.Check(id, result); ok {
		fmt.Println("ok")
	} else {
		fmt.Println("fail")
	}
}
