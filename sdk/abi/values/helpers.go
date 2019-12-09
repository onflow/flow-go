package values

import "fmt"

//This package contains convenience functions to help convert data between Go and Values
func NewValue(value interface{}) (Value, error) {

	switch v := value.(type) {
	case string:
		ret := String(v)
		return &ret, nil
	case int:
		ret := NewInt(v)
		return &ret, nil
	case int8:
		ret := Int8(v)
		return &ret, nil
	case int16:
		ret := Int16(v)
		return &ret, nil
	case int32:
		ret := Int32(v)
		return &ret, nil
	case int64:
		ret := Int64(v)
		return &ret, nil
	case uint8:
		ret := UInt8(v)
		return &ret, nil
	case uint16:
		ret := UInt16(v)
		return &ret, nil
	case uint32:
		ret := UInt32(v)
		return &ret, nil
	case uint64:
		ret := UInt64(v)
		return &ret, nil
	case []interface{}:
		values := make([]Value, len(v))
		for i, v := range v {
			t, err := NewValue(v)
			if err != nil {
				return nil, err
			}
			values[i] = t
		}
		ret := VariableSizedArray(values)
		return &ret, nil
	case nil:
		ret := Nil{}
		return &ret, nil

	}

	return nil, fmt.Errorf("value type %T cannot be converted to ABI Value type", value)
}

// NewValueOrPanic is convenience function when failure is really unexpected
// like generated Go code
func NewValueOrPanic(value interface{}) Value {
	ret, err := NewValue(value)
	if err != nil {
		panic(err)
	}
	return ret
}

func CastToString(value Value) (string, error) {

	casted, ok := value.(String)
	if !ok {
		return "", fmt.Errorf("%T is not a values.String", value)
	}

	goValue := casted.ToGoValue()

	str, ok := goValue.(string)
	if !ok {
		return "", fmt.Errorf("%T is not a string", goValue)
	}
	return str, nil
}

func CastToUInt8(value Value) (uint8, error) {

	casted, ok := value.(UInt8)
	if !ok {
		return 0, fmt.Errorf("%T is not a values.UInt8", value)
	}

	goValue := casted.ToGoValue()

	u, ok := goValue.(uint8)
	if !ok {
		return 0, fmt.Errorf("%T is not a uint8", value)
	}
	return u, nil
}

func CastToUInt16(value Value) (uint16, error) {

	casted, ok := value.(UInt16)
	if !ok {
		return 0, fmt.Errorf("%T is not a values.UInt16", value)
	}

	goValue := casted.ToGoValue()

	u, ok := goValue.(uint16)
	if !ok {
		return 0, fmt.Errorf("%T is not a uint16", value)
	}
	return u, nil
}

func CastToArray(value Value) ([]interface{}, error) {

	casted, ok := value.(VariableSizedArray)
	if !ok {
		return nil, fmt.Errorf("%T is not a values.VariableSizedArray", value)
	}

	goValue := casted.ToGoValue()

	u, ok := goValue.([]interface{})
	if !ok {
		return nil, fmt.Errorf("%T is not a []interface{}]", value)
	}
	return u, nil
}

func CastToInt(value Value) (int, error) {

	casted, ok := value.(Int)
	if !ok {
		return 0, fmt.Errorf("%T is not a values.Int", value)
	}

	goValue := casted.ToGoValue()

	u, ok := goValue.(int)
	if !ok {
		return 0, fmt.Errorf("%T %v is not a int", value, value)
	}
	return u, nil
}

func CastToComposite(value Value) (Composite, error) {
	u, ok := value.(Composite)
	if !ok {
		return Composite{}, fmt.Errorf("%T is not a Composite", value)
	}
	return u, nil
}
