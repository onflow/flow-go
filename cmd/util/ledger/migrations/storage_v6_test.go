package migrations

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rs/zerolog"

	"github.com/onflow/atree"
	"github.com/onflow/cadence/runtime/common"
	newInter "github.com/onflow/cadence/runtime/interpreter"

	oldInter "github.com/onflow/cadence/v19/runtime/interpreter"
	"github.com/onflow/cadence/v19/runtime/tests/utils"
)

func TestValueConversion(t *testing.T) {

	t.Run("Array", func(t *testing.T) {
		oldArray := oldInter.NewArrayValueUnownedNonCopying(
			oldInter.VariableSizedStaticType{
				Type: oldInter.PrimitiveStaticTypeAnyStruct,
			},
			oldInter.NewStringValue("foo"),
			oldInter.NewStringValue("bar"),
			oldInter.BoolValue(true),
		)

		address := &common.Address{1, 2}
		oldArray.SetOwner(address)

		storage := newInter.NewInMemoryStorage()
		converter := NewValueConverter(storage)

		inter, err := oldInter.NewInterpreter(nil, nil)
		assert.NoError(t, err)

		newValue := converter.Convert(inter, oldArray)

		assert.IsType(t, &newInter.ArrayValue{}, newValue)
		array := newValue.(*newInter.ArrayValue)

		assert.Equal(t, newInter.NewStringValue("foo"), array.GetIndex(nil, 0))
		assert.Equal(t, newInter.NewStringValue("bar"), array.GetIndex(nil, 1))
	})
}

func TestEncoding(t *testing.T) {

	t.Run("Array", func(t *testing.T) {
		// Get the bytes in old format
		oldArray := oldInter.NewArrayValueUnownedNonCopying(
			oldInter.VariableSizedStaticType{
				Type: oldInter.PrimitiveStaticTypeAnyStruct,
			},
			oldInter.NewStringValue("foo"),
			oldInter.NewStringValue("bar"),
			oldInter.BoolValue(true),
		)

		encoded, _, err := oldInter.EncodeValue(oldArray, nil, false, nil)
		require.NoError(t, err)

		migration := &StorageFormatV6Migration{
			Log: zerolog.Logger{},
		}

		migration.initStorage()
		migration.migratedPayloadPaths = make(map[storagePath]bool, 0)

		address := common.Address{1, 2}

		err = migration.decodeAndConvert(encoded, address, "", oldInter.CurrentEncodingVersion)
		assert.NoError(t, err)

		migration.storage.Commit()

		encodedValues := migration.baseStorage.Payloads
		require.Len(t, encodedValues, 1)

		storageId := atree.NewStorageID(
			[8]byte(address),
			[8]byte{0, 0, 0, 0, 0, 0, 0, 1},
		)

		slab, ok, err := migration.storage.Retrieve(storageId)
		require.NoError(t, err)
		require.True(t, ok)

		newValue, err := newInter.StoredValue(slab, migration.storage)
		require.NoError(t, err)

		assert.IsType(t, &newInter.ArrayValue{}, newValue)
		array := newValue.(*newInter.ArrayValue)

		value := array.GetIndex(nil, 0)
		require.NoError(t, err)
		assert.Equal(t, newInter.NewStringValue("foo"), value)

		value = array.GetIndex(nil, 1)
		require.NoError(t, err)
		assert.Equal(t, newInter.NewStringValue("bar"), value)

		value = array.GetIndex(nil, 2)
		require.NoError(t, err)
		assert.Equal(t, newInter.BoolValue(true), value)
	})

	t.Run("Dictionary", func(t *testing.T) {
		inter, err := oldInter.NewInterpreter(nil, utils.TestLocation)
		require.NoError(t, err)

		// Get the bytes in old format
		oldArray := oldInter.NewDictionaryValueUnownedNonCopying(
			inter,
			oldInter.DictionaryStaticType{
				KeyType:   oldInter.PrimitiveStaticTypeString,
				ValueType: oldInter.PrimitiveStaticTypeAnyStruct,
			},
			oldInter.NewStringValue("key1"),
			oldInter.NewStringValue("foo"),
			oldInter.NewStringValue("key2"),
			oldInter.BoolValue(true),
		)

		encoded, _, err := oldInter.EncodeValue(oldArray, nil, false, nil)
		require.NoError(t, err)

		migration := &StorageFormatV6Migration{
			Log: zerolog.Logger{},
		}

		migration.initStorage()
		migration.migratedPayloadPaths = make(map[storagePath]bool, 0)

		address := common.Address{1, 2}

		err = migration.decodeAndConvert(encoded, address, "", oldInter.CurrentEncodingVersion)
		assert.NoError(t, err)

		migration.storage.Commit()

		encodedValues := migration.baseStorage.Payloads
		require.Len(t, encodedValues, 2)

		storageId := atree.NewStorageID(
			[8]byte(address),
			[8]byte{0, 0, 0, 0, 0, 0, 0, 1},
		)

		slab, ok, err := migration.storage.Retrieve(storageId)
		require.NoError(t, err)
		require.True(t, ok)

		newValue, err := newInter.StoredValue(slab, migration.storage)
		require.NoError(t, err)

		assert.IsType(t, &newInter.DictionaryValue{}, newValue)
		dictionary := newValue.(*newInter.DictionaryValue)

		value, _, ok := dictionary.GetKey(newInter.NewStringValue("key1"))
		require.True(t, ok)
		assert.Equal(t, newInter.NewStringValue("foo"), value)

		value, _, ok = dictionary.GetKey(newInter.NewStringValue("key2"))
		require.True(t, ok)
		assert.Equal(t, newInter.BoolValue(true), value)
	})
}
