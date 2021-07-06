package migrations

import (
	"testing"

	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/stretchr/testify/require"
)

func TestStorageFormatV5Migration_InferContainerStaticType(t *testing.T) {

	t.Parallel()

	t.Run("array, AnyStruct, no values", func(t *testing.T) {

		t.Parallel()

		array := interpreter.NewArrayValueUnownedNonCopying(nil)

		err := inferContainerStaticType(
			array,
			interpreter.PrimitiveStaticTypeAnyStruct,
		)
		require.NoError(t, err)

		require.Equal(t,
			interpreter.VariableSizedStaticType{
				Type: interpreter.PrimitiveStaticTypeAnyStruct,
			},
			array.Type,
		)
	})

	t.Run("array, AnyResource, no values", func(t *testing.T) {

		t.Parallel()

		array := interpreter.NewArrayValueUnownedNonCopying(nil)

		err := inferContainerStaticType(
			array,
			interpreter.PrimitiveStaticTypeAnyResource,
		)
		require.NoError(t, err)

		require.Equal(t,
			interpreter.VariableSizedStaticType{
				Type: interpreter.PrimitiveStaticTypeAnyResource,
			},
			array.Type,
		)
	})

	t.Run("array, [Int], no values", func(t *testing.T) {

		t.Parallel()

		array := interpreter.NewArrayValueUnownedNonCopying(nil)

		err := inferContainerStaticType(
			array,
			interpreter.VariableSizedStaticType{
				Type: interpreter.PrimitiveStaticTypeInt,
			},
		)
		require.NoError(t, err)

		require.Equal(t,
			interpreter.VariableSizedStaticType{
				Type: interpreter.PrimitiveStaticTypeInt,
			},
			array.Type,
		)
	})

	t.Run("array, [Int; 3], no values", func(t *testing.T) {

		t.Parallel()

		array := interpreter.NewArrayValueUnownedNonCopying(nil)

		err := inferContainerStaticType(
			array,
			interpreter.ConstantSizedStaticType{
				Type: interpreter.PrimitiveStaticTypeInt,
				Size: 3,
			},
		)
		require.NoError(t, err)

		require.Equal(t,
			interpreter.ConstantSizedStaticType{
				Type: interpreter.PrimitiveStaticTypeInt,
				Size: 3,
			},
			array.Type,
		)
	})

	t.Run("array, AnyStruct, nested", func(t *testing.T) {

		t.Parallel()

		innerArray := interpreter.NewArrayValueUnownedNonCopying(nil)
		array := interpreter.NewArrayValueUnownedNonCopying(nil, innerArray)

		err := inferContainerStaticType(
			array,
			interpreter.PrimitiveStaticTypeAnyStruct,
		)
		require.NoError(t, err)

		require.Equal(t,
			interpreter.VariableSizedStaticType{
				Type: interpreter.PrimitiveStaticTypeAnyStruct,
			},
			array.Type,
		)

		require.Equal(t,
			interpreter.VariableSizedStaticType{
				Type: interpreter.PrimitiveStaticTypeAnyStruct,
			},
			innerArray.Type,
		)
	})

	t.Run("dictionary, AnyStruct, no values", func(t *testing.T) {

		t.Parallel()

		dictionary := interpreter.NewDictionaryValueUnownedNonCopying(
			interpreter.DictionaryStaticType{},
		)

		err := inferContainerStaticType(
			dictionary,
			interpreter.PrimitiveStaticTypeAnyStruct,
		)
		require.NoError(t, err)

		require.Equal(t,
			interpreter.DictionaryStaticType{
				KeyType:   interpreter.PrimitiveStaticTypeAnyStruct,
				ValueType: interpreter.PrimitiveStaticTypeAnyStruct,
			},
			dictionary.Type,
		)
	})

	t.Run("dictionary, AnyResource, no values", func(t *testing.T) {

		t.Parallel()

		dictionary := interpreter.NewDictionaryValueUnownedNonCopying(
			interpreter.DictionaryStaticType{},
		)

		err := inferContainerStaticType(
			dictionary,
			interpreter.PrimitiveStaticTypeAnyResource,
		)
		require.NoError(t, err)

		require.Equal(t,
			interpreter.DictionaryStaticType{
				KeyType:   interpreter.PrimitiveStaticTypeAnyStruct,
				ValueType: interpreter.PrimitiveStaticTypeAnyResource,
			},
			dictionary.Type,
		)
	})

	t.Run("dictionary, {Int: String}, no values", func(t *testing.T) {

		t.Parallel()

		dictionary := interpreter.NewDictionaryValueUnownedNonCopying(interpreter.DictionaryStaticType{})

		err := inferContainerStaticType(
			dictionary,
			interpreter.DictionaryStaticType{
				KeyType:   interpreter.PrimitiveStaticTypeInt,
				ValueType: interpreter.PrimitiveStaticTypeString,
			},
		)
		require.NoError(t, err)

		require.Equal(t,
			interpreter.DictionaryStaticType{
				KeyType:   interpreter.PrimitiveStaticTypeInt,
				ValueType: interpreter.PrimitiveStaticTypeString,
			},
			dictionary.Type,
		)
	})

	t.Run("dictionary, {Int: {String: Bool}}, nested", func(t *testing.T) {

		t.Parallel()

		innerDictionary := interpreter.NewDictionaryValueUnownedNonCopying(
			interpreter.DictionaryStaticType{},
		)
		dictionary := interpreter.NewDictionaryValueUnownedNonCopying(
			interpreter.DictionaryStaticType{},
			interpreter.NewIntValueFromInt64(1), innerDictionary,
		)

		err := inferContainerStaticType(
			dictionary,
			interpreter.DictionaryStaticType{
				KeyType: interpreter.PrimitiveStaticTypeInt,
				ValueType: interpreter.DictionaryStaticType{
					KeyType:   interpreter.PrimitiveStaticTypeString,
					ValueType: interpreter.PrimitiveStaticTypeBool,
				},
			},
		)
		require.NoError(t, err)

		require.Equal(t,
			interpreter.DictionaryStaticType{
				KeyType: interpreter.PrimitiveStaticTypeInt,
				ValueType: interpreter.DictionaryStaticType{
					KeyType:   interpreter.PrimitiveStaticTypeString,
					ValueType: interpreter.PrimitiveStaticTypeBool,
				},
			},
			dictionary.Type,
		)

		require.Equal(t,
			interpreter.DictionaryStaticType{
				KeyType:   interpreter.PrimitiveStaticTypeString,
				ValueType: interpreter.PrimitiveStaticTypeBool,
			},
			innerDictionary.Type,
		)
	})

	t.Run("dictionary, AnyResource, nested", func(t *testing.T) {

		t.Parallel()

		innerDictionary := interpreter.NewDictionaryValueUnownedNonCopying(
			interpreter.DictionaryStaticType{},
		)
		dictionary := interpreter.NewDictionaryValueUnownedNonCopying(
			interpreter.DictionaryStaticType{},
			interpreter.NewIntValueFromInt64(1), innerDictionary,
		)

		err := inferContainerStaticType(
			dictionary,
			interpreter.PrimitiveStaticTypeAnyResource,
		)
		require.NoError(t, err)

		require.Equal(t,
			interpreter.DictionaryStaticType{
				KeyType:   interpreter.PrimitiveStaticTypeAnyStruct,
				ValueType: interpreter.PrimitiveStaticTypeAnyResource,
			},
			dictionary.Type,
		)

		require.Equal(t,
			interpreter.DictionaryStaticType{
				KeyType:   interpreter.PrimitiveStaticTypeAnyStruct,
				ValueType: interpreter.PrimitiveStaticTypeAnyResource,
			},
			innerDictionary.Type,
		)
	})
}
