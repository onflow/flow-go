package interpreter

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/tests/utils"
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

func TestOwnerNewArray(t *testing.T) {

	oldOwner := "1"

	value := &CompositeValue{
		Location:   utils.TestLocation,
		Identifier: "Test",
		Kind:       common.CompositeKindStructure,
		Owner:      oldOwner,
	}

	assert.Equal(t, oldOwner, value.GetOwner())

	array := NewArrayValueUnownedNonCopying(value)

	assert.Equal(t, "", value.GetOwner())
	assert.Equal(t, "", array.GetOwner())
}

func TestSetOwnerArray(t *testing.T) {

	oldOwner := "1"
	newOwner := "2"

	value := &CompositeValue{
		Location:   utils.TestLocation,
		Identifier: "Test",
		Kind:       common.CompositeKindStructure,
		Owner:      oldOwner,
	}

	array := NewArrayValueUnownedNonCopying(value)

	array.SetOwner(newOwner)

	assert.Equal(t, array.GetOwner(), newOwner)
	assert.Equal(t, value.GetOwner(), newOwner)
}

