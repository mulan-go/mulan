package req

import (
	"fmt"
	"reflect"
	"strconv"
)

// ApplyDefaults 根据结构体字段的 `default` tag 为零值字段填充默认值。
// 在 Bind / ShouldBind 中于校验前自动调用；也可在手动绑定后单独调用。
func ApplyDefaults(obj any) error {
	if obj == nil {
		return nil
	}
	v := reflect.ValueOf(obj)
	return applyDefaults(v)
}

func applyDefaults(v reflect.Value) error {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}
	t := v.Type()
	for i := range v.NumField() {
		sf := t.Field(i)
		field := v.Field(i)
		if !field.CanSet() {
			continue
		}
		tag := sf.Tag.Get("default")
		if tag == "" {
			switch field.Kind() {
			case reflect.Struct:
				if err := applyDefaults(field); err != nil {
					return err
				}
			case reflect.Pointer:
				if !field.IsNil() && field.Elem().Kind() == reflect.Struct {
					if err := applyDefaults(field); err != nil {
						return err
					}
				}
			}
			continue
		}
		if err := setDefaultTaggedField(field, tag); err != nil {
			return fmt.Errorf("%s: %w", sf.Name, err)
		}
	}
	return nil
}

func setDefaultTaggedField(field reflect.Value, tag string) error {
	switch field.Kind() {
	case reflect.Pointer:
		elemType := field.Type().Elem()
		if elemType.Kind() == reflect.Struct {
			return nil
		}
		need := field.IsNil()
		if !need {
			elem := field.Elem()
			need = elem.IsZero()
		}
		if !need {
			return nil
		}
		if field.IsNil() {
			field.Set(reflect.New(elemType))
		}
		return setScalarDefault(field.Elem(), tag)
	default:
		if !field.IsZero() {
			return nil
		}
		return setScalarDefault(field, tag)
	}
}

func setScalarDefault(field reflect.Value, tag string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(tag)
	case reflect.Bool:
		b, err := strconv.ParseBool(tag)
		if err != nil {
			return err
		}
		field.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(tag, 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(tag, 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetUint(n)
	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(tag, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetFloat(n)
	default:
		return fmt.Errorf("unsupported kind %s for default tag", field.Kind())
	}
	return nil
}
