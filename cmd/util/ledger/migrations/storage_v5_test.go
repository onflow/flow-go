package migrations

import (
	"os"
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageFormatV5Migration_InferContainerStaticType(t *testing.T) {

	t.Parallel()

	t.Run("array, AnyStruct, no values", func(t *testing.T) {

		t.Parallel()

		array := interpreter.NewArrayValueUnownedNonCopying(nil)

		m := StorageFormatV5Migration{}

		err := m.inferContainerStaticType(
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

		m := StorageFormatV5Migration{}

		err := m.inferContainerStaticType(
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

		m := StorageFormatV5Migration{}

		err := m.inferContainerStaticType(
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

		m := StorageFormatV5Migration{}

		err := m.inferContainerStaticType(
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

		m := StorageFormatV5Migration{}

		err := m.inferContainerStaticType(
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

	t.Run("array, no type, values", func(t *testing.T) {

		t.Parallel()

		array := interpreter.NewArrayValueUnownedNonCopying(
			nil,
			interpreter.NewStringValue("one"),
			interpreter.NewStringValue("two"),
		)

		m := StorageFormatV5Migration{}

		err := m.inferContainerStaticType(
			array,
			nil,
		)
		require.NoError(t, err)

		require.Equal(t,
			interpreter.VariableSizedStaticType{
				Type: interpreter.PrimitiveStaticTypeString,
			},
			array.Type,
		)
	})

	t.Run("dictionary, AnyStruct, no values", func(t *testing.T) {

		t.Parallel()

		m := StorageFormatV5Migration{}

		dictionary := interpreter.NewDictionaryValueUnownedNonCopying(
			m.newInterpreter(),
			interpreter.DictionaryStaticType{},
		)

		err := m.inferContainerStaticType(
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

		m := StorageFormatV5Migration{}

		dictionary := interpreter.NewDictionaryValueUnownedNonCopying(
			m.newInterpreter(),
			interpreter.DictionaryStaticType{},
		)

		err := m.inferContainerStaticType(
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

		m := StorageFormatV5Migration{}

		dictionary := interpreter.NewDictionaryValueUnownedNonCopying(
			m.newInterpreter(),
			interpreter.DictionaryStaticType{},
		)

		err := m.inferContainerStaticType(
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

		m := StorageFormatV5Migration{}

		innerDictionary := interpreter.NewDictionaryValueUnownedNonCopying(
			m.newInterpreter(),
			interpreter.DictionaryStaticType{
				KeyType:   interpreter.PrimitiveStaticTypeString,
				ValueType: interpreter.PrimitiveStaticTypeBool,
			},
		)

		dictionary := interpreter.NewDictionaryValueUnownedNonCopying(
			m.newInterpreter(),
			interpreter.DictionaryStaticType{
				KeyType: interpreter.PrimitiveStaticTypeInt,
				ValueType: interpreter.DictionaryStaticType{
					KeyType:   interpreter.PrimitiveStaticTypeString,
					ValueType: interpreter.PrimitiveStaticTypeBool,
				},
			},
			interpreter.NewIntValueFromInt64(1), innerDictionary,
		)

		dictionary.Type = interpreter.DictionaryStaticType{}
		innerDictionary.Type = interpreter.DictionaryStaticType{}

		err := m.inferContainerStaticType(
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

		m := StorageFormatV5Migration{}

		innerDictionary := interpreter.NewDictionaryValueUnownedNonCopying(
			m.newInterpreter(),
			interpreter.DictionaryStaticType{},
		)
		dictionary := interpreter.NewDictionaryValueUnownedNonCopying(
			m.newInterpreter(),
			interpreter.DictionaryStaticType{},
			interpreter.NewIntValueFromInt64(1), innerDictionary,
		)

		err := m.inferContainerStaticType(
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

	t.Run("dictionary, no type, values", func(t *testing.T) {

		t.Parallel()

		m := StorageFormatV5Migration{}

		dictionary := interpreter.NewDictionaryValueUnownedNonCopying(
			m.newInterpreter(),
			interpreter.DictionaryStaticType{},
			interpreter.NewStringValue("one"), interpreter.NewIntValueFromInt64(1),
			interpreter.NewStringValue("two"), interpreter.NewIntValueFromInt64(2),
		)

		err := m.inferContainerStaticType(
			dictionary,
			nil,
		)
		require.NoError(t, err)

		require.Equal(t,
			interpreter.DictionaryStaticType{
				KeyType:   interpreter.PrimitiveStaticTypeAnyStruct,
				ValueType: interpreter.PrimitiveStaticTypeInt,
			},
			dictionary.Type,
		)
	})
}

func TestGetContractValueChildKeyContractName(t *testing.T) {

	assert.Equal(t,
		"FlowAssets",
		getContractValueChildKeyContractName([]byte("contract\x1fFlowAssets\x1fsets\x1fv\x1f12")),
	)

	assert.Equal(t,
		"",
		getContractValueChildKeyContractName([]byte("contract\x1fFlowAssets")),
	)

	assert.Equal(t,
		"",
		getContractValueChildKeyContractName([]byte("storage\x1fFlowAssets\x1fsets\x1fv\x1f12")),
	)
}

func TestStorageFormatV5Migration_AddKnownStaticType(t *testing.T) {

	key := "contract\x1fFlowIDTableStaking"
	owner, err := common.HexToAddress("dee35303492e5a0b")
	require.NoError(t, err)

	oldData := []byte{
		0xd8, 0x84, 0x85, 0xd8, 0xc0, 0x82, 0x48, 0xde, 0xe3, 0x53, 0x03, 0x49, 0x2e, 0x5a, 0x0b, 0x72,
		0x46, 0x6c, 0x6f, 0x77, 0x49, 0x44, 0x54, 0x61, 0x62, 0x6c, 0x65, 0x53, 0x74, 0x61, 0x6b, 0x69,
		0x6e, 0x67, 0xf6, 0x03, 0x94, 0x74, 0x44, 0x65, 0x6c, 0x65, 0x67, 0x61, 0x74, 0x6f, 0x72, 0x53,
		0x74, 0x6f, 0x72, 0x61, 0x67, 0x65, 0x50, 0x61, 0x74, 0x68, 0xd8, 0xc8, 0x82, 0x01, 0x74, 0x66,
		0x6c, 0x6f, 0x77, 0x53, 0x74, 0x61, 0x6b, 0x69, 0x6e, 0x67, 0x44, 0x65, 0x6c, 0x65, 0x67, 0x61,
		0x74, 0x6f, 0x72, 0x74, 0x4e, 0x6f, 0x64, 0x65, 0x53, 0x74, 0x61, 0x6b, 0x65, 0x72, 0x50, 0x75,
		0x62, 0x6c, 0x69, 0x63, 0x50, 0x61, 0x74, 0x68, 0xd8, 0xc8, 0x82, 0x03, 0x6a, 0x66, 0x6c, 0x6f,
		0x77, 0x53, 0x74, 0x61, 0x6b, 0x65, 0x72, 0x75, 0x4e, 0x6f, 0x64, 0x65, 0x53, 0x74, 0x61, 0x6b,
		0x65, 0x72, 0x53, 0x74, 0x6f, 0x72, 0x61, 0x67, 0x65, 0x50, 0x61, 0x74, 0x68, 0xd8, 0xc8, 0x82,
		0x01, 0x6a, 0x66, 0x6c, 0x6f, 0x77, 0x53, 0x74, 0x61, 0x6b, 0x65, 0x72, 0x77, 0x53, 0x74, 0x61,
		0x6b, 0x69, 0x6e, 0x67, 0x41, 0x64, 0x6d, 0x69, 0x6e, 0x53, 0x74, 0x6f, 0x72, 0x61, 0x67, 0x65,
		0x50, 0x61, 0x74, 0x68, 0xd8, 0xc8, 0x82, 0x01, 0x70, 0x66, 0x6c, 0x6f, 0x77, 0x53, 0x74, 0x61,
		0x6b, 0x69, 0x6e, 0x67, 0x41, 0x64, 0x6d, 0x69, 0x6e, 0x70, 0x65, 0x70, 0x6f, 0x63, 0x68, 0x54,
		0x6f, 0x6b, 0x65, 0x6e, 0x50, 0x61, 0x79, 0x6f, 0x75, 0x74, 0xd8, 0xbc, 0x1b, 0x00, 0x00, 0x71,
		0xaf, 0xd4, 0x98, 0xd0, 0x00, 0x74, 0x6d, 0x69, 0x6e, 0x69, 0x6d, 0x75, 0x6d, 0x53, 0x74, 0x61,
		0x6b, 0x65, 0x52, 0x65, 0x71, 0x75, 0x69, 0x72, 0x65, 0x64, 0xd8, 0x81, 0x82, 0x85, 0xd8, 0xa1,
		0x01, 0xd8, 0xa1, 0x02, 0xd8, 0xa1, 0x03, 0xd8, 0xa1, 0x04, 0xd8, 0xa1, 0x05, 0x85, 0xd8, 0xbc,
		0x1b, 0x00, 0x00, 0x16, 0xbc, 0xc4, 0x1e, 0x90, 0x00, 0xd8, 0xbc, 0x1b, 0x00, 0x00, 0x2d, 0x79,
		0x88, 0x3d, 0x20, 0x00, 0xd8, 0xbc, 0x1b, 0x00, 0x00, 0x71, 0xaf, 0xd4, 0x98, 0xd0, 0x00, 0xd8,
		0xbc, 0x1b, 0x00, 0x00, 0x0c, 0x47, 0x36, 0xb4, 0x58, 0x00, 0xd8, 0xbc, 0x00, 0x77, 0x6e, 0x6f,
		0x64, 0x65, 0x44, 0x65, 0x6c, 0x65, 0x67, 0x61, 0x74, 0x69, 0x6e, 0x67, 0x52, 0x65, 0x77, 0x61,
		0x72, 0x64, 0x43, 0x75, 0x74, 0xd8, 0xbc, 0x1a, 0x00, 0x2d, 0xc6, 0xc0, 0x65, 0x6e, 0x6f, 0x64,
		0x65, 0x73, 0xd8, 0x81, 0x82, 0x80, 0x80, 0x6c, 0x72, 0x65, 0x77, 0x61, 0x72, 0x64, 0x52, 0x61,
		0x74, 0x69, 0x6f, 0x73, 0xd8, 0x81, 0x82, 0x85, 0xd8, 0xa1, 0x01, 0xd8, 0xa1, 0x02, 0xd8, 0xa1,
		0x03, 0xd8, 0xa1, 0x04, 0xd8, 0xa1, 0x05, 0x85, 0xd8, 0xbc, 0x1a, 0x01, 0x00, 0x59, 0x00, 0xd8,
		0xbc, 0x1a, 0x03, 0x16, 0x67, 0xc0, 0xd8, 0xbc, 0x1a, 0x00, 0x77, 0x04, 0xc0, 0xd8, 0xbc, 0x1a,
		0x01, 0x68, 0x1b, 0x80, 0xd8, 0xbc, 0x00, 0x78, 0x1b, 0x74, 0x6f, 0x74, 0x61, 0x6c, 0x54, 0x6f,
		0x6b, 0x65, 0x6e, 0x73, 0x53, 0x74, 0x61, 0x6b, 0x65, 0x64, 0x42, 0x79, 0x4e, 0x6f, 0x64, 0x65,
		0x54, 0x79, 0x70, 0x65, 0xd8, 0x81, 0x82, 0x85, 0xd8, 0xa1, 0x01, 0xd8, 0xa1, 0x02, 0xd8, 0xa1,
		0x03, 0xd8, 0xa1, 0x04, 0xd8, 0xa1, 0x05, 0x85, 0xd8, 0xbc, 0x00, 0xd8, 0xbc, 0x00, 0xd8, 0xbc,
		0x00, 0xd8, 0xbc, 0x00, 0xd8, 0xbc, 0x00, 0x72, 0x46, 0x6c, 0x6f, 0x77, 0x49, 0x44, 0x54, 0x61,
		0x62, 0x6c, 0x65, 0x53, 0x74, 0x61, 0x6b, 0x69, 0x6e, 0x67,
	}

	migration := StorageFormatV5Migration{Log: log.Output(zerolog.ConsoleWriter{Out: os.Stderr})}

	_, _, err = migration.reencodeValue(
		oldData,
		owner,
		key,
		4,
	)
	require.NoError(t, err)
}
