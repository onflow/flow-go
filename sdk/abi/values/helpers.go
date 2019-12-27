package values

import "fmt"

func NewValue(value interface{}) (Value, error) {
	switch v := value.(type) {
	case string:
		return NewString(v), nil
	case int:
		return NewInt(v), nil
	case int8:
		return NewInt8(v), nil
	case int16:
		return NewInt16(v), nil
	case int32:
		return NewInt32(v), nil
	case int64:
		return NewInt64(v), nil
	case uint8:
		return NewUInt8(v), nil
	case uint16:
		return NewUInt16(v), nil
	case uint32:
		return NewUInt32(v), nil
	case uint64:
		return NewUInt64(v), nil
	case []interface{}:
		values := make([]Value, len(v))

		for i, v := range v {
			t, err := NewValue(v)
			if err != nil {
				return nil, err
			}

			values[i] = t
		}

		return NewVariableSizedArray(values), nil
	case nil:
		return NewNil(), nil
	}

	return nil, fmt.Errorf("value type %T cannot be converted to ABI value type", value)
}

// MustConvertValue converts a Go value to an ABI value or panics if the value
// cannot be converted.
func MustConvertValue(value interface{}) Value {
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

func CastToVariableSizedArray(value Value) ([]interface{}, error) {
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
