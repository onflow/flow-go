package interpreter

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestToExpression(t *testing.T) {
	_, err := ToValue(1)
	assert.Error(t, err)

	testValue := func(expected Value) func(actual Value, err error) {
		return func(actual Value, err error) {
			assert.Nil(t, err)
			assert.Equal(t, actual, expected)
		}
	}

	testValue(Int8Value(1))(ToValue(int8(1)))
	testValue(Int16Value(2))(ToValue(int16(2)))
	testValue(Int32Value(3))(ToValue(int32(3)))
	testValue(Int64Value(4))(ToValue(int64(4)))
	testValue(UInt8Value(1))(ToValue(uint8(1)))
	testValue(UInt16Value(2))(ToValue(uint16(2)))
	testValue(UInt32Value(3))(ToValue(uint32(3)))
	testValue(UInt64Value(4))(ToValue(uint64(4)))
	testValue(BoolValue(true))(ToValue(true))
	testValue(BoolValue(false))(ToValue(false))
}
