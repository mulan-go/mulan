package req

import (
	"reflect"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	v "github.com/go-playground/validator/v10"
)

type defaultValidator struct{ validate *v.Validate }

func init() {
	validate := v.New()
	validate.SetTagName("binding")
	binding.Validator = &defaultValidator{validate: validate}
}

func (v *defaultValidator) Engine() any { return v.validate }

func (v *defaultValidator) ValidateStruct(obj any) error {
	if obj == nil {
		return nil
	}
	value := reflect.ValueOf(obj)
	switch value.Kind() {
	case reflect.Pointer:
		return v.ValidateStruct(value.Elem().Interface())
	case reflect.Slice, reflect.Array:
		validateRet := make(binding.SliceValidationError, 0)
		for i := range value.Len() {
			if err := v.ValidateStruct(value.Index(i).Interface()); err != nil {
				validateRet = append(validateRet, err)
			}
		}
		if len(validateRet) == 0 {
			return nil
		}
		return validateRet
	case reflect.Struct:
		return v.validate.Struct(obj)
	default:
		return nil
	}
}

func Bind(c *gin.Context, obj any) (err error) {
	// Bind Path Params
	if len(c.Params) > 0 {
		m := make(map[string][]string, len(c.Params))
		for _, v := range c.Params {
			m[v.Key] = []string{v.Value}
		}
		if err := binding.Uri.BindUri(m, obj); err != nil {
			return c.Error(err)
		}
	}
	// Bind Body
	err = binding.Default(c.Request.Method, c.ContentType()).Bind(c.Request, obj)
	if err != nil {
		return c.Error(err)
	}
	// Bind Query
	err = binding.Query.Bind(c.Request, obj)
	if err != nil {
		return c.Error(err)
	}
	if err := ApplyDefaults(obj); err != nil {
		return c.Error(err)
	}
	return binding.Validator.ValidateStruct(obj)
}

// ShouldBind 从多个来源(path/body/query)绑定数据到同一个结构体
// 设计意图：尽力绑定模式，忽略各来源的绑定错误，最终只返回验证结果
// 适用场景：同一个 Req 结构体需要同时接收路径参数、请求体和查询参数
func ShouldBind(c *gin.Context, obj any) error {
	// Bind Path Params (忽略错误，允许来源为空)
	if len(c.Params) > 0 {
		m := make(map[string][]string, len(c.Params))
		for _, v := range c.Params {
			m[v.Key] = []string{v.Value}
		}
		_ = binding.Uri.BindUri(m, obj)
	}
	// Bind Body (忽略错误，允许无 body)
	_ = binding.Default(c.Request.Method, c.ContentType()).Bind(c.Request, obj)
	// Bind Query (忽略错误，允许无 query)
	_ = binding.Query.Bind(c.Request, obj)
	if err := ApplyDefaults(obj); err != nil {
		return err
	}
	// 只返回验证错误
	return binding.Validator.ValidateStruct(obj)
}
