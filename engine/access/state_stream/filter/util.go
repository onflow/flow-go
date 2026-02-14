package filter

import (
	"bytes"
	"fmt"
	"math/big"
	"strings"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/onflow/flow-go/model/flow"
)

var ErrComparisonNotSupported = fmt.Errorf("comparison not supported")

// TODO: next steps are to setup some tests to verify the behavior is correct
// and benchmark the performance is acceptable.

// numericBig is a cadence type that implements the Big() method.
type numericBig interface {
	Big() *big.Int
}

// typesEqual returns true if `actualValue` is of the provided `userType`.
// If the field value is an optional, `userType` must match the inner type of the optional type.
// If the field value is an array or dictionary, `userType` must match the element type of the array or dictionary.
func typesEqual(actualValue cadence.Value, userType cadence.Type) (bool, error) {
	if optional, ok := actualValue.(cadence.Optional); ok {
		return typesEqual(optional.Value, userType)
	}

	if array, ok := actualValue.(cadence.Array); ok {
		if array.ArrayType != nil {
			return array.ArrayType.Element().Equal(userType), nil
		}

		if len(array.Values) > 0 {
			// only traverse a single level
			return array.Values[0].Type().Equal(userType), nil
		}

		// TODO: should this return an error?
		return false, nil // cannot resolve type from array
	}

	if actualValue.Type().Equal(userType) {
		return true, nil
	}

	return false, nil
}

// compareBig compares two numericBig values.
func compareBig[T numericBig](a, b T) int {
	return a.Big().Cmp(b.Big())
}

// compare compares two values of the same type.
func compare[T ~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~float32 | ~float64 | ~string](a, b T) int {
	if a == b {
		return 0
	}
	if a < b {
		return -1
	}
	return 1
}

// cmp returns the comparison result of two values.
// If the first value is an optional, the comparison is performed on the inner value. If the inner
// value is nil, the
func cmp(a, b cadence.Value) (int, error) {
	if optional, ok := a.(*cadence.Optional); ok {
		if optional.Value == nil {
			if b == nil {
				return 0, nil
			}
			return -1, nil // TODO: document this behavior
		}
		a = optional.Value

		// TODO: can I just do this?
		// return cmp(optional.Value, b)
	}

	switch aValue := a.(type) {
	case cadence.UInt:
		return compareBig(aValue, b.(cadence.UInt)), nil
	case cadence.UInt8:
		return compare(aValue, b.(cadence.UInt8)), nil
	case cadence.UInt16:
		return compare(aValue, b.(cadence.UInt16)), nil
	case cadence.UInt32:
		return compare(aValue, b.(cadence.UInt32)), nil
	case cadence.UInt64:
		return compare(aValue, b.(cadence.UInt64)), nil
	case cadence.UInt128:
		return compareBig(aValue, b.(cadence.UInt128)), nil
	case cadence.UInt256:
		return compareBig(aValue, b.(cadence.UInt256)), nil
	case cadence.Int:
		return compareBig(aValue, b.(cadence.Int)), nil
	case cadence.Int8:
		return compare(aValue, b.(cadence.Int8)), nil
	case cadence.Int16:
		return compare(aValue, b.(cadence.Int16)), nil
	case cadence.Int32:
		return compare(aValue, b.(cadence.Int32)), nil
	case cadence.Int64:
		return compare(aValue, b.(cadence.Int64)), nil
	case cadence.Int128:
		return compareBig(aValue, b.(cadence.Int128)), nil
	case cadence.Int256:
		return compareBig(aValue, b.(cadence.Int256)), nil
	case cadence.Word8:
		return compare(aValue, b.(cadence.Word8)), nil
	case cadence.Word16:
		return compare(aValue, b.(cadence.Word16)), nil
	case cadence.Word32:
		return compare(aValue, b.(cadence.Word32)), nil
	case cadence.Word64:
		return compare(aValue, b.(cadence.Word64)), nil
	case cadence.Word128:
		return compareBig(aValue, b.(cadence.Word128)), nil
	case cadence.Word256:
		return compareBig(aValue, b.(cadence.Word256)), nil
	case cadence.Fix64:
		return compare(aValue, b.(cadence.Fix64)), nil
	case cadence.Fix128:
		aBig := new(big.Int).SetBytes(aValue.ToBigEndianBytes())
		bBig := new(big.Int).SetBytes(b.(cadence.Fix128).ToBigEndianBytes())
		return aBig.Cmp(bBig), nil
	case cadence.UFix64:
		return compare(aValue, b.(cadence.UFix64)), nil
	case cadence.UFix128:
		aBig := new(big.Int).SetBytes(aValue.ToBigEndianBytes())
		bBig := new(big.Int).SetBytes(b.(cadence.UFix128).ToBigEndianBytes())
		return aBig.Cmp(bBig), nil
	case cadence.String:
		// note: for strings, A < B
		return strings.Compare(string(aValue), string(b.(cadence.String))), nil
	case cadence.Character:
		return strings.Compare(string(aValue), string(b.(cadence.Character))), nil
	case cadence.Address:
		return bytes.Compare(aValue.Bytes(), b.(cadence.Address).Bytes()), nil
	case cadence.Bytes:
		return bytes.Compare(aValue, b.(cadence.Bytes)), nil
	default:
		return 0, fmt.Errorf("%w: compare not supported for type %T", ErrComparisonNotSupported, aValue)
	}
}

// equal returns true if the two values are equal.
func equal(a, b cadence.Value) (bool, error) {
	if optional, ok := a.(cadence.Optional); ok {
		return equal(optional.Value, b)
	}

	switch aValue := a.(type) {
	case cadence.Array:
		return false, fmt.Errorf("%w: equal comparison not supported for array type", ErrComparisonNotSupported)
	case cadence.Bytes:
		return bytes.Equal([]byte(aValue), []byte(b.(cadence.Bytes))), nil
	default:
		return a == b, nil
	}
}

// contains returns true if the provided `set` contains the provided `value`.
func contains(set cadence.Value, value cadence.Value) (bool, error) {
	if optional, ok := set.(cadence.Optional); ok {
		set = optional.Value
	}

	switch v := set.(type) {
	case cadence.String:
		return strings.Contains(string(v), string(value.(cadence.String))), nil

	case cadence.Array:
		for _, item := range v.Values {
			if item == value {
				return true, nil
			}
		}
		return false, nil

	default:
		return false, fmt.Errorf("contains not supported for type %T", v)
	}
}

// in returns true if `eventValue` is contained in the list `allowedValues`.
func in(eventValue cadence.Value, allowedValues []cadence.Value) (bool, error) {
	if optional, ok := eventValue.(cadence.Optional); ok {
		eventValue = optional.Value
	}

	for _, allowedValue := range allowedValues {
		ok, err := equal(eventValue, allowedValue)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}

	return false, nil
}

// getEventFields extracts field values and field names from the payload of a flow event.
// It decodes the event payload into a Cadence event, retrieves the field values and fields, and returns them.
// Parameters:
// - event: The Flow event to extract field values and field names from.
// Returns:
// - map[string]cadence.Value: A map containing name and value for each field extracted from the event payload.
// - error: An error, if any, encountered during event decoding or if the fields are empty.
func getEventFields(event *flow.Event) (map[string]cadence.Value, error) {
	data, err := ccf.Decode(nil, event.Payload)
	if err != nil {
		return nil, err
	}

	cdcEvent, ok := data.(cadence.Event)
	if !ok {
		return nil, err
	}

	fields := cadence.FieldsMappedByName(cdcEvent)
	if fields == nil {
		return nil, fmt.Errorf("fields are empty")
	}
	return fields, nil
}

func isAllowedFilterType(value cadence.Value) bool {
	switch value.(type) {
	case cadence.UInt, cadence.UInt8, cadence.UInt16, cadence.UInt32, cadence.UInt64, cadence.UInt128, cadence.UInt256,
		cadence.Int, cadence.Int8, cadence.Int16, cadence.Int32, cadence.Int64, cadence.Int128, cadence.Int256,
		cadence.Word8, cadence.Word16, cadence.Word32, cadence.Word64, cadence.Word128, cadence.Word256,
		cadence.Fix64, cadence.Fix128, cadence.UFix64, cadence.UFix128,
		cadence.String, cadence.Character, cadence.Bytes, cadence.Bool,
		cadence.Address, cadence.Path:
		return true
	default:
		return false
	}
}
