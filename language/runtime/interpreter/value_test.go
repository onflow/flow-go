package interpreter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	"github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestToExpression(t *testing.T) {

	testValue := func(expected Value) func(actual Value, err error) {
		return func(actual Value, err error) {
			require.NoError(t, err)
			assert.Equal(t, expected, actual)
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

func newTestCompositeValue(owner common.Address) *CompositeValue {
	return &CompositeValue{
		Location: utils.TestLocation,
		TypeID:   "Test",
		Kind:     common.CompositeKindStructure,
		Fields:   map[string]Value{},
		Owner:    &owner,
	}
}

func TestOwnerNewArray(t *testing.T) {
	oldOwner := common.Address{0x1}

	value := newTestCompositeValue(oldOwner)

	assert.Equal(t, &oldOwner, value.GetOwner())

	array := NewArrayValueUnownedNonCopying(value)

	assert.Nil(t, array.GetOwner())
	assert.Nil(t, value.GetOwner())
}

func TestSetOwnerArray(t *testing.T) {
	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}

	value := newTestCompositeValue(oldOwner)

	array := NewArrayValueUnownedNonCopying(value)

	array.SetOwner(&newOwner)

	assert.Equal(t, &newOwner, array.GetOwner())
	assert.Equal(t, &newOwner, value.GetOwner())
}

func TestSetOwnerArrayCopy(t *testing.T) {
	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}

	value := newTestCompositeValue(oldOwner)

	array := NewArrayValueUnownedNonCopying(value)

	array.SetOwner(&newOwner)

	arrayCopy := array.Copy().(*ArrayValue)
	valueCopy := arrayCopy.Values[0]

	assert.Nil(t, arrayCopy.GetOwner())
	assert.Nil(t, valueCopy.GetOwner())
	assert.Equal(t, &newOwner, value.GetOwner())
}

func TestSetOwnerArraySetIndex(t *testing.T) {
	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}

	value1 := newTestCompositeValue(oldOwner)
	value2 := newTestCompositeValue(oldOwner)

	array := NewArrayValueUnownedNonCopying(value1)
	array.SetOwner(&newOwner)

	assert.Equal(t, &newOwner, array.GetOwner())
	assert.Equal(t, &newOwner, value1.GetOwner())
	assert.Equal(t, &oldOwner, value2.GetOwner())

	array.Set(nil, LocationRange{}, NewIntValue(0), value2)

	assert.Equal(t, &newOwner, array.GetOwner())
	assert.Equal(t, &newOwner, value1.GetOwner())
	assert.Equal(t, &newOwner, value2.GetOwner())
}

func TestSetOwnerArrayAppend(t *testing.T) {
	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}

	value := newTestCompositeValue(oldOwner)

	array := NewArrayValueUnownedNonCopying()
	array.SetOwner(&newOwner)

	assert.Equal(t, &newOwner, array.GetOwner())
	assert.Equal(t, &oldOwner, value.GetOwner())

	array.Append(value)

	assert.Equal(t, &newOwner, array.GetOwner())
	assert.Equal(t, &newOwner, value.GetOwner())
}

func TestSetOwnerArrayInsert(t *testing.T) {
	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}

	value := newTestCompositeValue(oldOwner)

	array := NewArrayValueUnownedNonCopying()
	array.SetOwner(&newOwner)

	assert.Equal(t, &newOwner, array.GetOwner())
	assert.Equal(t, &oldOwner, value.GetOwner())

	array.Insert(0, value)

	assert.Equal(t, &newOwner, array.GetOwner())
	assert.Equal(t, &newOwner, value.GetOwner())
}

func TestOwnerNewDictionary(t *testing.T) {
	oldOwner := common.Address{0x1}

	keyValue := NewStringValue("test")
	value := newTestCompositeValue(oldOwner)

	assert.Equal(t, &oldOwner, value.GetOwner())

	dictionary := NewDictionaryValueUnownedNonCopying(keyValue, value)

	assert.Nil(t, dictionary.GetOwner())
	// NOTE: keyValue is string, has no owner
	assert.Nil(t, value.GetOwner())
}

func TestSetOwnerDictionary(t *testing.T) {
	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}

	keyValue := NewStringValue("test")
	value := newTestCompositeValue(oldOwner)

	dictionary := NewDictionaryValueUnownedNonCopying(keyValue, value)

	dictionary.SetOwner(&newOwner)

	assert.Equal(t, &newOwner, dictionary.GetOwner())
	assert.Equal(t, &newOwner, value.GetOwner())
}

func TestSetOwnerDictionaryCopy(t *testing.T) {
	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}

	keyValue := NewStringValue("test")
	value := newTestCompositeValue(oldOwner)

	dictionary := NewDictionaryValueUnownedNonCopying(keyValue, value)
	dictionary.SetOwner(&newOwner)

	dictionaryCopy := dictionary.Copy().(*DictionaryValue)
	valueCopy := dictionaryCopy.Entries[keyValue.KeyString()]

	assert.Nil(t, dictionaryCopy.GetOwner())
	assert.Nil(t, valueCopy.GetOwner())
	assert.Equal(t, &newOwner, value.GetOwner())
}

func TestSetOwnerDictionarySetIndex(t *testing.T) {
	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}

	keyValue := NewStringValue("test")
	value := newTestCompositeValue(oldOwner)

	dictionary := NewDictionaryValueUnownedNonCopying()
	dictionary.SetOwner(&newOwner)

	assert.Equal(t, &newOwner, dictionary.GetOwner())
	assert.Equal(t, &oldOwner, value.GetOwner())

	dictionary.Set(
		nil,
		LocationRange{},
		keyValue,
		NewSomeValueOwningNonCopying(value),
	)

	assert.Equal(t, &newOwner, dictionary.GetOwner())
	assert.Equal(t, &newOwner, value.GetOwner())
}

func TestSetOwnerDictionaryInsert(t *testing.T) {
	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}

	keyValue := NewStringValue("test")
	value := newTestCompositeValue(oldOwner)

	dictionary := NewDictionaryValueUnownedNonCopying()
	dictionary.SetOwner(&newOwner)

	assert.Equal(t, &newOwner, dictionary.GetOwner())
	assert.Equal(t, &oldOwner, value.GetOwner())

	dictionary.Insert(keyValue, value)

	assert.Equal(t, &newOwner, dictionary.GetOwner())
	assert.Equal(t, &newOwner, value.GetOwner())
}

func TestOwnerNewAny(t *testing.T) {
	oldOwner := common.Address{0x1}

	value := newTestCompositeValue(oldOwner)
	valueType := &sema.CompositeType{
		Location:   value.Location,
		Identifier: string(value.TypeID),
		Kind:       value.Kind,
	}

	assert.Equal(t, &oldOwner, value.GetOwner())

	any := NewAnyValueOwningNonCopying(value, valueType)

	assert.Equal(t, &oldOwner, any.GetOwner())
	assert.Equal(t, &oldOwner, value.GetOwner())
}

func TestSetOwnerAny(t *testing.T) {
	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}

	value := newTestCompositeValue(oldOwner)
	valueType := &sema.CompositeType{
		Location:   value.Location,
		Identifier: string(value.TypeID),
		Kind:       value.Kind,
	}

	assert.Equal(t, &oldOwner, value.GetOwner())

	any := NewAnyValueOwningNonCopying(value, valueType)

	any.SetOwner(&newOwner)

	assert.Equal(t, &newOwner, any.GetOwner())
	assert.Equal(t, &newOwner, value.GetOwner())
}

func TestSetOwnerAnyCopy(t *testing.T) {
	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}

	value := newTestCompositeValue(oldOwner)
	valueType := &sema.CompositeType{
		Location:   value.Location,
		Identifier: string(value.TypeID),
		Kind:       value.Kind,
	}

	assert.Equal(t, &oldOwner, value.GetOwner())

	any := NewAnyValueOwningNonCopying(value, valueType)
	any.SetOwner(&newOwner)

	anyCopy := any.Copy().(*AnyValue)
	valueCopy := anyCopy.Value

	assert.Nil(t, anyCopy.GetOwner())
	assert.Nil(t, valueCopy.GetOwner())
	assert.Equal(t, &newOwner, value.GetOwner())
}

func TestOwnerNewSome(t *testing.T) {
	oldOwner := common.Address{0x1}

	value := newTestCompositeValue(oldOwner)

	assert.Equal(t, &oldOwner, value.GetOwner())

	any := NewSomeValueOwningNonCopying(value)

	assert.Equal(t, &oldOwner, any.GetOwner())
	assert.Equal(t, &oldOwner, value.GetOwner())
}

func TestSetOwnerSome(t *testing.T) {
	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}

	value := newTestCompositeValue(oldOwner)

	assert.Equal(t, &oldOwner, value.GetOwner())

	any := NewSomeValueOwningNonCopying(value)

	any.SetOwner(&newOwner)

	assert.Equal(t, &newOwner, any.GetOwner())
	assert.Equal(t, &newOwner, value.GetOwner())
}

func TestSetOwnerSomeCopy(t *testing.T) {
	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}

	value := newTestCompositeValue(oldOwner)

	assert.Equal(t, &oldOwner, value.GetOwner())

	some := NewSomeValueOwningNonCopying(value)
	some.SetOwner(&newOwner)

	someCopy := some.Copy().(*SomeValue)
	valueCopy := someCopy.Value

	assert.Nil(t, someCopy.GetOwner())
	assert.Nil(t, valueCopy.GetOwner())
	assert.Equal(t, &newOwner, value.GetOwner())
}

func TestSetOwnerReference(t *testing.T) {
	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}
	targetAddress := common.Address{0x3}

	reference := &StorageReferenceValue{
		TargetStorageAddress: targetAddress,
		TargetKey:            "Test",
		Owner:                &oldOwner,
	}

	assert.Equal(t, &oldOwner, reference.GetOwner())

	reference.SetOwner(&newOwner)

	assert.Equal(t, &newOwner, reference.GetOwner())
}

func TestSetOwnerReferenceCopy(t *testing.T) {
	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}
	targetAddress := common.Address{0x3}

	reference := &StorageReferenceValue{
		TargetStorageAddress: targetAddress,
		TargetKey:            "Test",
		Owner:                &oldOwner,
	}

	assert.Equal(t, &oldOwner, reference.GetOwner())

	reference.SetOwner(&newOwner)

	referenceCopy := reference.Copy().(*StorageReferenceValue)

	assert.Nil(t, referenceCopy.GetOwner())
	assert.Equal(t, &newOwner, reference.GetOwner())
}

func TestOwnerNewComposite(t *testing.T) {
	oldOwner := common.Address{0x1}

	composite := newTestCompositeValue(oldOwner)

	assert.Equal(t, &oldOwner, composite.GetOwner())
}

func TestSetOwnerComposite(t *testing.T) {
	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}

	value := newTestCompositeValue(oldOwner)
	composite := newTestCompositeValue(oldOwner)

	const fieldName = "test"

	composite.Fields[fieldName] = value

	composite.SetOwner(&newOwner)

	assert.Equal(t, &newOwner, composite.GetOwner())
	assert.Equal(t, &newOwner, value.GetOwner())
}

func TestSetOwnerCompositeCopy(t *testing.T) {
	oldOwner := common.Address{0x1}

	value := newTestCompositeValue(oldOwner)
	composite := newTestCompositeValue(oldOwner)

	const fieldName = "test"

	composite.Fields[fieldName] = value

	compositeCopy := composite.Copy().(*CompositeValue)
	valueCopy := compositeCopy.Fields[fieldName]

	assert.Nil(t, compositeCopy.GetOwner())
	assert.Nil(t, valueCopy.GetOwner())
	assert.Equal(t, &oldOwner, value.GetOwner())
}

func TestSetOwnerCompositeSetMember(t *testing.T) {
	oldOwner := common.Address{0x1}
	newOwner := common.Address{0x2}

	value := newTestCompositeValue(oldOwner)
	composite := newTestCompositeValue(oldOwner)

	const fieldName = "test"

	composite.SetOwner(&newOwner)

	assert.Equal(t, &newOwner, composite.GetOwner())
	assert.Equal(t, &oldOwner, value.GetOwner())

	composite.SetMember(
		nil,
		LocationRange{},
		fieldName,
		value,
	)

	assert.Equal(t, &newOwner, composite.GetOwner())
	assert.Equal(t, &newOwner, value.GetOwner())
}
