package shared

import "reflect"

// SetParamField marks a cloudflare-go param.Field as present without importing
// the SDK root package, which eagerly imports every generated service.
func SetParamField(field any, value any) {
	fieldValue := reflect.ValueOf(field)
	if fieldValue.Kind() != reflect.Pointer || fieldValue.IsNil() {
		panic("SetParamField requires a non-nil pointer")
	}

	elem := fieldValue.Elem()
	valueField := elem.FieldByName("Value")
	presentField := elem.FieldByName("Present")
	if !valueField.IsValid() || !presentField.IsValid() {
		panic("SetParamField requires a cloudflare-go param.Field")
	}

	valueValue := reflect.ValueOf(value)
	switch {
	case !valueValue.IsValid():
		valueField.SetZero()
	case valueValue.Type().AssignableTo(valueField.Type()):
		valueField.Set(valueValue)
	case valueValue.Type().ConvertibleTo(valueField.Type()):
		valueField.Set(valueValue.Convert(valueField.Type()))
	default:
		panic("SetParamField value is not assignable to field")
	}

	presentField.SetBool(true)
}
