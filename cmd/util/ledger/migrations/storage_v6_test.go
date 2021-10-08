package migrations

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fxamacker/cbor/v2"
	"github.com/onflow/atree"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/programs"
	fvmState "github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	newInter "github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"

	oldInter "github.com/onflow/cadence/v19/runtime/interpreter"
	"github.com/onflow/cadence/v19/runtime/tests/utils"
)

func TestValueConversion(t *testing.T) {

	t.Parallel()

	t.Run("Primitive", func(t *testing.T) {
		t.Parallel()

		type conversionPair struct {
			oldValue         oldInter.Value
			expectedNewValue newInter.Value
		}

		conversions := []conversionPair{
			{
				oldValue:         oldInter.NewIntValueFromInt64(math.MaxInt64),
				expectedNewValue: newInter.NewIntValueFromInt64(math.MaxInt64),
			},
			{
				oldValue:         oldInter.NewIntValueFromInt64(math.MinInt64),
				expectedNewValue: newInter.NewIntValueFromInt64(math.MinInt64),
			},
			{
				oldValue:         oldInter.Int8Value(45),
				expectedNewValue: newInter.Int8Value(45),
			},
			{
				oldValue:         oldInter.Int16Value(45),
				expectedNewValue: newInter.Int16Value(45),
			},
			{
				oldValue:         oldInter.Int32Value(45),
				expectedNewValue: newInter.Int32Value(45),
			},
			{
				oldValue:         oldInter.Int64Value(45),
				expectedNewValue: newInter.Int64Value(45),
			},
			{
				oldValue:         oldInter.NewInt128ValueFromInt64(math.MaxInt64),
				expectedNewValue: newInter.NewInt128ValueFromInt64(math.MaxInt64),
			},
			{
				oldValue:         oldInter.NewInt128ValueFromInt64(math.MinInt64),
				expectedNewValue: newInter.NewInt128ValueFromInt64(math.MinInt64),
			},
			{
				oldValue:         oldInter.NewInt256ValueFromInt64(math.MaxInt64),
				expectedNewValue: newInter.NewInt256ValueFromInt64(math.MaxInt64),
			},
			{
				oldValue:         oldInter.NewInt256ValueFromInt64(math.MinInt64),
				expectedNewValue: newInter.NewInt256ValueFromInt64(math.MinInt64),
			},
			{
				oldValue:         oldInter.NewUIntValueFromUint64(math.MaxUint64),
				expectedNewValue: newInter.NewUIntValueFromUint64(math.MaxUint64),
			},
			{
				oldValue:         oldInter.NewUIntValueFromUint64(456),
				expectedNewValue: newInter.NewUIntValueFromUint64(456),
			},
			{
				oldValue:         oldInter.UInt8Value(45),
				expectedNewValue: newInter.UInt8Value(45),
			},
			{
				oldValue:         oldInter.UInt16Value(45),
				expectedNewValue: newInter.UInt16Value(45),
			},
			{
				oldValue:         oldInter.UInt32Value(45),
				expectedNewValue: newInter.UInt32Value(45),
			},
			{
				oldValue:         oldInter.UInt64Value(45),
				expectedNewValue: newInter.UInt64Value(45),
			},
			{
				oldValue:         oldInter.NewUInt128ValueFromUint64(math.MaxUint64),
				expectedNewValue: newInter.NewUInt128ValueFromUint64(math.MaxUint64),
			},
			{
				oldValue:         oldInter.NewUInt256ValueFromUint64(math.MaxUint64),
				expectedNewValue: newInter.NewUInt256ValueFromUint64(math.MaxUint64),
			},
			{
				oldValue:         oldInter.Word8Value(45),
				expectedNewValue: newInter.Word8Value(45),
			},
			{
				oldValue:         oldInter.Word16Value(45),
				expectedNewValue: newInter.Word16Value(45),
			},
			{
				oldValue:         oldInter.Word32Value(45),
				expectedNewValue: newInter.Word32Value(45),
			},
			{
				oldValue:         oldInter.Word64Value(45),
				expectedNewValue: newInter.Word64Value(45),
			},
			{
				oldValue:         oldInter.Fix64Value(math.MaxInt64),
				expectedNewValue: newInter.Fix64Value(math.MaxInt64),
			},
			{
				oldValue:         oldInter.Fix64Value(math.MinInt64),
				expectedNewValue: newInter.Fix64Value(math.MinInt64),
			},
			{
				oldValue:         oldInter.UFix64Value(math.MaxInt64),
				expectedNewValue: newInter.UFix64Value(math.MaxInt64),
			},
			{
				oldValue:         oldInter.UFix64Value(0),
				expectedNewValue: newInter.UFix64Value(0),
			},
			{
				oldValue:         oldInter.NilValue{},
				expectedNewValue: newInter.NilValue{},
			},
			{
				oldValue:         oldInter.VoidValue{},
				expectedNewValue: newInter.VoidValue{},
			},
			{
				oldValue: oldInter.NewSomeValueOwningNonCopying(
					oldInter.NewStringValue("foo"),
				),
				expectedNewValue: newInter.NewSomeValueNonCopying(
					newInter.NewStringValue("foo"),
				),
			},
			{
				oldValue:         oldInter.AddressValue{1, 2},
				expectedNewValue: newInter.AddressValue{1, 2},
			},
			{
				oldValue: oldInter.PathValue{
					Domain:     common.PathDomainStorage,
					Identifier: "some/storage",
				},
				expectedNewValue: newInter.PathValue{
					Domain:     common.PathDomainStorage,
					Identifier: "some/storage",
				},
			},
			{
				oldValue: oldInter.CapabilityValue{
					Address: oldInter.AddressValue{1, 2},
					Path: oldInter.PathValue{
						Domain:     common.PathDomainPublic,
						Identifier: "some/path",
					},
					BorrowType: oldInter.PrimitiveStaticTypeFix64,
				},
				expectedNewValue: &newInter.CapabilityValue{
					Address: newInter.AddressValue{1, 2},
					Path: newInter.PathValue{
						Domain:     common.PathDomainPublic,
						Identifier: "some/path",
					},
					BorrowType: newInter.PrimitiveStaticTypeFix64,
				},
			},
			{
				oldValue: oldInter.LinkValue{
					TargetPath: oldInter.PathValue{
						Domain:     common.PathDomainPrivate,
						Identifier: "some/path",
					},
					Type: oldInter.PrimitiveStaticTypeSignedInteger,
				},
				expectedNewValue: newInter.LinkValue{
					TargetPath: newInter.PathValue{
						Domain:     common.PathDomainPrivate,
						Identifier: "some/path",
					},
					Type: newInter.PrimitiveStaticTypeSignedInteger,
				},
			},
			{
				oldValue: oldInter.TypeValue{
					Type: oldInter.PrimitiveStaticTypeSignedInteger,
				},
				expectedNewValue: newInter.TypeValue{
					Type: newInter.PrimitiveStaticTypeSignedInteger,
				},
			},
		}

		migration := &StorageFormatV6Migration{}
		converter := NewValueConverter(migration)

		for _, conversion := range conversions {
			newValue := converter.Convert(conversion.oldValue, nil)
			assert.Equal(t, conversion.expectedNewValue, newValue)
		}
	})

	t.Run("Array", func(t *testing.T) {
		t.Parallel()

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

		payloads := []ledger.Payload{
			{
				Key: ledger.NewKey([]ledger.KeyPart{
					ledger.NewKeyPart(state.KeyPartOwner, address[:]),
					ledger.NewKeyPart(state.KeyPartController, []byte{}),
					ledger.NewKeyPart(state.KeyPartKey, []byte(fvmState.KeyStorageUsed)),
				}),
				Value: ledger.Value(
					uint64ToBinary(
						uint64(0), // dummy value
					),
				),
			},
		}
		ledgerView := newView(payloads)

		migration := &StorageFormatV6Migration{}
		migration.initPersistentSlabStorage(ledgerView)
		migration.initNewInterpreter()
		migration.initOldInterpreter(payloads)

		converter := NewValueConverter(migration)
		newValue := converter.Convert(oldArray, nil)

		require.IsType(t, &newInter.ArrayValue{}, newValue)
		array := newValue.(*newInter.ArrayValue)

		assert.Equal(
			t,
			newInter.NewStringValue("foo"),
			array.Get(migration.newInter, nil, 0),
		)
		assert.Equal(t,
			newInter.NewStringValue("bar"),
			array.Get(migration.newInter, nil, 1),
		)
	})

	t.Run("Dictionary", func(t *testing.T) {
		t.Parallel()

		inter, err := oldInter.NewInterpreter(nil, utils.TestLocation)
		require.NoError(t, err)

		oldDictionary := oldInter.NewDictionaryValueUnownedNonCopying(
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

		address := &common.Address{1, 2}
		oldDictionary.SetOwner(address)

		payloads := []ledger.Payload{
			{
				Key: ledger.NewKey([]ledger.KeyPart{
					ledger.NewKeyPart(state.KeyPartOwner, address[:]),
					ledger.NewKeyPart(state.KeyPartController, []byte{}),
					ledger.NewKeyPart(state.KeyPartKey, []byte(fvmState.KeyStorageUsed)),
				}),
				Value: ledger.Value(
					uint64ToBinary(
						uint64(0), // dummy value
					),
				),
			},
		}
		ledgerView := newView(payloads)

		migration := &StorageFormatV6Migration{}
		migration.initPersistentSlabStorage(ledgerView)
		migration.initNewInterpreter()
		migration.initOldInterpreter(payloads)

		converter := NewValueConverter(migration)
		newValue := converter.Convert(oldDictionary, nil)

		assert.IsType(t, &newInter.DictionaryValue{}, newValue)
		dictionary := newValue.(*newInter.DictionaryValue)

		value, ok := dictionary.Get(nil, nil, newInter.NewStringValue("key1"))
		require.True(t, ok)
		assert.Equal(t, newInter.NewStringValue("foo"), value)

		value, ok = dictionary.Get(nil, nil, newInter.NewStringValue("key2"))
		require.True(t, ok)
		assert.Equal(t, newInter.BoolValue(true), value)
	})

	t.Run("Composite", func(t *testing.T) {
		t.Parallel()

		inter, err := oldInter.NewInterpreter(nil, utils.TestLocation)
		require.NoError(t, err)

		owner := common.Address{1, 2}

		oldDictionary := oldInter.NewDictionaryValueUnownedNonCopying(
			inter,
			oldInter.DictionaryStaticType{
				KeyType:   oldInter.PrimitiveStaticTypeString,
				ValueType: oldInter.PrimitiveStaticTypeAnyStruct,
			},
			oldInter.NewStringValue("key1"),
			oldInter.NewStringValue("value1"),
			oldInter.NewStringValue("key2"),
			oldInter.BoolValue(true),
		)
		oldDictionary.SetOwner(&owner)

		fields := oldInter.NewStringValueOrderedMap()
		fields.Set("foo", oldDictionary)

		oldComposite := oldInter.NewCompositeValue(
			utils.TestLocation,
			"Test",
			common.CompositeKindContract,
			fields,
			&owner,
		)

		payloads := []ledger.Payload{
			{
				Key: ledger.NewKey([]ledger.KeyPart{
					ledger.NewKeyPart(state.KeyPartOwner, owner[:]),
					ledger.NewKeyPart(state.KeyPartController, []byte{}),
					ledger.NewKeyPart(state.KeyPartKey, []byte(fvmState.KeyStorageUsed)),
				}),
				Value: ledger.Value(
					uint64ToBinary(
						uint64(0), // dummy value
					),
				),
			},
		}
		ledgerView := newView(payloads)

		migration := &StorageFormatV6Migration{}
		migration.initPersistentSlabStorage(ledgerView)
		migration.initNewInterpreter()
		migration.initOldInterpreter(payloads)

		converter := NewValueConverter(migration)
		newValue := converter.Convert(oldComposite, nil)

		assert.IsType(t, &newInter.CompositeValue{}, newValue)
		composite := newValue.(*newInter.CompositeValue)

		fieldValue := composite.GetField("foo")

		assert.IsType(t, &newInter.DictionaryValue{}, fieldValue)
		dictionary := fieldValue.(*newInter.DictionaryValue)

		value, ok := dictionary.Get(nil, nil, newInter.NewStringValue("key1"))
		require.True(t, ok)
		assert.Equal(t, newInter.NewStringValue("value1"), value)

		value, ok = dictionary.Get(nil, nil, newInter.NewStringValue("key2"))
		require.True(t, ok)
		assert.Equal(t, newInter.BoolValue(true), value)
	})

	t.Run("Negative", func(t *testing.T) {
		t.Parallel()

		t.Run("nonstorable", func(t *testing.T) {
			t.Parallel()

			type conversionPair struct {
				oldValue         oldInter.Value
				expectedNewValue newInter.Value
			}

			conversions := []conversionPair{
				{
					oldValue:         oldInter.BoundFunctionValue{},
					expectedNewValue: newInter.BoundFunctionValue{},
				},
				{
					oldValue:         &oldInter.InterpretedFunctionValue{},
					expectedNewValue: &newInter.InterpretedFunctionValue{},
				},
				{
					oldValue: oldInter.NewHostFunctionValue(
						func(invocation oldInter.Invocation) oldInter.Value {
							return oldInter.NilValue{}
						},
					),
					expectedNewValue: newInter.NewHostFunctionValue(
						func(invocation newInter.Invocation) newInter.Value {
							return newInter.NilValue{}
						},
						nil,
					),
				},
			}

			migration := &StorageFormatV6Migration{}
			converter := NewValueConverter(migration)

			check := func(oldValue oldInter.Value) {
				defer func() {
					r := recover()
					require.NotNil(t, r)
					assert.Equal(t, "value not storable", r)
				}()
				_ = converter.Convert(oldValue, nil)
			}

			for _, conversion := range conversions {
				check(conversion.oldValue)
			}
		})
	})
}

func TestEncoding(t *testing.T) {

	t.Parallel()

	t.Run("Array", func(t *testing.T) {
		t.Parallel()

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

		address := common.Address{1, 2}

		payloads := []ledger.Payload{
			{
				Key: ledger.NewKey([]ledger.KeyPart{
					ledger.NewKeyPart(state.KeyPartOwner, address[:]),
					ledger.NewKeyPart(state.KeyPartController, []byte{}),
					ledger.NewKeyPart(state.KeyPartKey, []byte(fvmState.KeyStorageUsed)),
				}),
				Value: ledger.Value(
					uint64ToBinary(
						uint64(0), // dummy value
					),
				),
			},
		}
		ledgerView := newView(payloads)

		migration := &StorageFormatV6Migration{}
		migration.initPersistentSlabStorage(ledgerView)
		migration.initNewInterpreter()
		migration.migratedPayloadPaths = make(map[storagePath]bool)
		migration.converter = NewValueConverter(migration)

		err = migration.decodeAndConvert(encoded, address, "", oldInter.CurrentEncodingVersion)
		assert.NoError(t, err)

		err = migration.storage.Commit()
		assert.NoError(t, err)

		encodedValues := ledgerView.Payloads()
		require.Len(t, encodedValues, 4)

		for _, encValue := range encodedValues {
			assert.False(t, oldInter.HasMagic(encValue.Value))
		}

		storageId := atree.NewStorageID(
			atree.Address(address),
			atree.StorageIndex{0, 0, 0, 0, 0, 0, 0, 1},
		)

		slab, ok, err := migration.storage.Retrieve(storageId)
		require.NoError(t, err)
		require.True(t, ok)

		newValue := newInter.StoredValue(slab, migration.storage)

		assert.IsType(t, &newInter.ArrayValue{}, newValue)
		array := newValue.(*newInter.ArrayValue)

		value := array.Get(migration.newInter, nil, 0)
		require.NoError(t, err)
		assert.Equal(t, newInter.NewStringValue("foo"), value)

		value = array.Get(migration.newInter, nil, 1)
		require.NoError(t, err)
		assert.Equal(t, newInter.NewStringValue("bar"), value)

		value = array.Get(migration.newInter, nil, 2)
		require.NoError(t, err)
		assert.Equal(t, newInter.BoolValue(true), value)
	})

	t.Run("Dictionary", func(t *testing.T) {
		t.Parallel()

		inter, err := oldInter.NewInterpreter(nil, utils.TestLocation)
		require.NoError(t, err)

		// Get the bytes in old format
		oldDictionary := oldInter.NewDictionaryValueUnownedNonCopying(
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

		encoded, _, err := oldInter.EncodeValue(oldDictionary, nil, false, nil)
		require.NoError(t, err)

		address := common.Address{1, 2}

		payloads := []ledger.Payload{
			{
				Key: ledger.NewKey([]ledger.KeyPart{
					ledger.NewKeyPart(state.KeyPartOwner, address[:]),
					ledger.NewKeyPart(state.KeyPartController, []byte{}),
					ledger.NewKeyPart(state.KeyPartKey, []byte(fvmState.KeyStorageUsed)),
				}),
				Value: ledger.Value(
					uint64ToBinary(
						uint64(0), // dummy value
					),
				),
			},
		}
		ledgerView := newView(payloads)

		migration := &StorageFormatV6Migration{}
		migration.initPersistentSlabStorage(ledgerView)
		migration.initNewInterpreter()
		migration.initOldInterpreter(payloads)
		migration.migratedPayloadPaths = make(map[storagePath]bool)
		migration.converter = NewValueConverter(migration)

		err = migration.decodeAndConvert(encoded, address, "", oldInter.CurrentEncodingVersion)
		assert.NoError(t, err)

		err = migration.storage.Commit()
		assert.NoError(t, err)

		encodedValues := ledgerView.Payloads()
		require.Len(t, encodedValues, 4)

		for _, encValue := range encodedValues {
			assert.False(t, oldInter.HasMagic(encValue.Value))
		}

		storageId := atree.NewStorageID(
			atree.Address(address),
			atree.StorageIndex{0, 0, 0, 0, 0, 0, 0, 1},
		)

		slab, ok, err := migration.storage.Retrieve(storageId)
		require.NoError(t, err)
		require.True(t, ok)

		newValue := newInter.StoredValue(slab, migration.storage)

		assert.IsType(t, &newInter.DictionaryValue{}, newValue)
		dictionary := newValue.(*newInter.DictionaryValue)

		value, ok := dictionary.Get(nil, nil, newInter.NewStringValue("key1"))
		require.True(t, ok)
		assert.Equal(t, newInter.NewStringValue("foo"), value)

		value, ok = dictionary.Get(nil, nil, newInter.NewStringValue("key2"))
		require.True(t, ok)
		assert.Equal(t, newInter.BoolValue(true), value)
	})

	t.Run("Composite", func(t *testing.T) {
		t.Parallel()

		inter, err := oldInter.NewInterpreter(nil, utils.TestLocation)
		require.NoError(t, err)

		owner := common.Address{1, 2}

		oldDictionary := oldInter.NewDictionaryValueUnownedNonCopying(
			inter,
			oldInter.DictionaryStaticType{
				KeyType:   oldInter.PrimitiveStaticTypeString,
				ValueType: oldInter.PrimitiveStaticTypeAnyStruct,
			},
			oldInter.NewStringValue("key1"),
			oldInter.NewStringValue("value1"),
			oldInter.NewStringValue("key2"),
			oldInter.BoolValue(true),
		)
		oldDictionary.SetOwner(&owner)

		fields := oldInter.NewStringValueOrderedMap()
		fields.Set("foo", oldDictionary)

		oldComposite := oldInter.NewCompositeValue(
			utils.TestLocation,
			"Test",
			common.CompositeKindContract,
			fields,
			&owner,
		)

		encoded, _, err := oldInter.EncodeValue(oldComposite, nil, false, nil)
		require.NoError(t, err)

		address := common.Address{1, 2}

		payloads := []ledger.Payload{
			{
				Key: ledger.NewKey([]ledger.KeyPart{
					ledger.NewKeyPart(state.KeyPartOwner, address[:]),
					ledger.NewKeyPart(state.KeyPartController, []byte{}),
					ledger.NewKeyPart(state.KeyPartKey, []byte(fvmState.KeyStorageUsed)),
				}),
				Value: ledger.Value(
					uint64ToBinary(
						uint64(0), // dummy value
					),
				),
			},
		}
		ledgerView := newView(payloads)

		migration := &StorageFormatV6Migration{}
		migration.initPersistentSlabStorage(ledgerView)
		migration.initNewInterpreter()
		migration.initOldInterpreter(payloads)
		migration.migratedPayloadPaths = make(map[storagePath]bool)
		migration.converter = NewValueConverter(migration)

		err = migration.decodeAndConvert(encoded, address, "", oldInter.CurrentEncodingVersion)
		assert.NoError(t, err)

		err = migration.storage.Commit()
		assert.NoError(t, err)

		encodedValues := ledgerView.Payloads()
		require.Len(t, encodedValues, 6)

		for _, encValue := range encodedValues {
			assert.False(t, oldInter.HasMagic(encValue.Value))
		}

		// Check composite value in storage

		storageId := atree.NewStorageID(
			atree.Address(address),
			atree.StorageIndex{0, 0, 0, 0, 0, 0, 0, 2},
		)

		slab, ok, err := migration.storage.Retrieve(storageId)
		require.NoError(t, err)
		require.True(t, ok)

		storedValue := newInter.StoredValue(slab, migration.storage)

		assert.IsType(t, &newInter.CompositeValue{}, storedValue)
		composite := storedValue.(*newInter.CompositeValue)

		fieldValue := composite.GetField("foo")

		assert.IsType(t, &newInter.DictionaryValue{}, fieldValue)
		dictionary := fieldValue.(*newInter.DictionaryValue)

		value, ok := dictionary.Get(nil, nil, newInter.NewStringValue("key1"))
		require.True(t, ok)
		assert.Equal(t, newInter.NewStringValue("value1"), value)

		value, ok = dictionary.Get(nil, nil, newInter.NewStringValue("key2"))
		require.True(t, ok)
		assert.Equal(t, newInter.BoolValue(true), value)

		// Check whether the separately stored value
		// is same as the composite's field value.

		storageId = atree.NewStorageID(
			atree.Address(address),
			atree.StorageIndex{0, 0, 0, 0, 0, 0, 0, 3},
		)

		slab, ok, err = migration.storage.Retrieve(storageId)
		require.NoError(t, err)
		require.True(t, ok)

		storedValue = newInter.StoredValue(slab, migration.storage)
		assert.Equal(t, dictionary, storedValue)
	})
}

func TestPayloadsMigration(t *testing.T) {
	t.Parallel()

	inter, err := oldInter.NewInterpreter(nil, utils.TestLocation)
	require.NoError(t, err)

	owner := common.Address{1, 2}

	oldDictionary := oldInter.NewDictionaryValueUnownedNonCopying(
		inter,
		oldInter.DictionaryStaticType{
			KeyType:   oldInter.PrimitiveStaticTypeString,
			ValueType: oldInter.PrimitiveStaticTypeAnyStruct,
		},
		oldInter.NewStringValue("key1"),
		oldInter.NewStringValue("value1"),
		oldInter.NewStringValue("key2"),
		oldInter.BoolValue(true),
	)

	fields := oldInter.NewStringValueOrderedMap()
	fields.Set("foo", oldDictionary)

	composite := oldInter.NewCompositeValue(
		utils.TestLocation,
		"Test",
		common.CompositeKindContract,
		fields,
		&owner,
	)

	encoded, _, err := oldInter.EncodeValue(composite, nil, false, nil)
	require.NoError(t, err)

	encoded = oldInter.PrependMagic(encoded, oldInter.CurrentEncodingVersion)

	keyParts := []ledger.KeyPart{
		ledger.NewKeyPart(state.KeyPartOwner, owner[:]),
		ledger.NewKeyPart(state.KeyPartController, []byte{}),
		ledger.NewKeyPart(state.KeyPartKey, []byte("Test")),
	}

	payloads := []ledger.Payload{
		{
			Key: ledger.NewKey([]ledger.KeyPart{
				ledger.NewKeyPart(state.KeyPartOwner, owner[:]),
				ledger.NewKeyPart(state.KeyPartController, []byte{}),
				ledger.NewKeyPart(state.KeyPartKey, []byte(fvmState.KeyStorageUsed)),
			}),
			Value: ledger.Value(
				uint64ToBinary(
					uint64(0), // dummy value
				),
			),
		},
		{
			Key:   ledger.NewKey(keyParts),
			Value: ledger.Value(encoded),
		},
	}

	// Check whether the query works with old ledger
	ledgerView := newView(payloads)
	value, err := ledgerView.Get(string(owner[:]), "", "Test")
	require.NoError(t, err)
	assert.NotNil(t, value)

	storageFormatV6Migration := StorageFormatV6Migration{
		Log:       zerolog.Nop(),
		OutputDir: "none",
	}

	migratedPayloads, err := storageFormatV6Migration.migrate(payloads)
	require.NoError(t, err)

	assert.Len(t, migratedPayloads, 6)

	// Check whether the query works with new ledger

	migratedLedgerView := newView(migratedPayloads)

	key := []byte{0, 0, 0, 0, 0, 0, 0, 3}
	prefixedKey := []byte(atree.LedgerBaseStorageSlabPrefix + string(key))

	migratedValue, err := migratedLedgerView.Get(string(owner[:]), "", string(prefixedKey))
	require.NoError(t, err)
	require.NotEmpty(t, migratedValue)

	assert.False(t, oldInter.HasMagic(migratedValue))
}

func TestContractValueRetrieval(t *testing.T) {

	t.Parallel()

	address := common.Address{1, 2}

	const contractName = "Test"

	location := common.AddressLocation{
		Address: address,
		Name:    contractName,
	}

	contractValue := oldInter.NewCompositeValue(
		location,
		contractName,
		common.CompositeKindContract,
		oldInter.NewStringValueOrderedMap(),
		&address,
	)

	encodeContractValue, _, err := oldInter.EncodeValue(contractValue, nil, false, nil)
	require.NoError(t, err)

	encodeContractValue = oldInter.PrependMagic(encodeContractValue, oldInter.CurrentEncodingVersion)

	contractNames := &bytes.Buffer{}
	namesEncoder := cbor.NewEncoder(contractNames)
	err = namesEncoder.Encode([]string{contractName})
	require.NoError(t, err)

	contractCode := `
        pub contract Test {
            pub fun foo(): Int { return 42 }
        }
    `

	contractValueKey := []ledger.KeyPart{
		ledger.NewKeyPart(state.KeyPartOwner, address[:]),
		ledger.NewKeyPart(state.KeyPartController, []byte{}),
		ledger.NewKeyPart(state.KeyPartKey, []byte(fmt.Sprintf("contract\x1F%s", contractName))),
	}

	contractNamesKey := []ledger.KeyPart{
		ledger.NewKeyPart(state.KeyPartOwner, address[:]),
		ledger.NewKeyPart(state.KeyPartController, address[:]),
		ledger.NewKeyPart(state.KeyPartKey, []byte(fvmState.KeyContractNames)),
	}

	contractCodeKey := []ledger.KeyPart{
		ledger.NewKeyPart(state.KeyPartOwner, address[:]),
		ledger.NewKeyPart(state.KeyPartController, address[:]),
		ledger.NewKeyPart(state.KeyPartKey, []byte("code.Test")),
	}

	storageUsedKey := []ledger.KeyPart{
		ledger.NewKeyPart(state.KeyPartOwner, address[:]),
		ledger.NewKeyPart(state.KeyPartController, []byte{}),
		ledger.NewKeyPart(state.KeyPartKey, []byte(fvmState.KeyStorageUsed)),
	}

	// old payloads
	payloads := []ledger.Payload{
		{
			Key:   ledger.NewKey(contractValueKey),
			Value: ledger.Value(encodeContractValue),
		},
		{
			Key:   ledger.NewKey(contractCodeKey),
			Value: ledger.Value(contractCode),
		},
		{
			Key:   ledger.NewKey(contractNamesKey),
			Value: ledger.Value(contractNames.Bytes()),
		},
		{
			Key: ledger.NewKey(storageUsedKey),
			Value: ledger.Value(
				uint64ToBinary(
					uint64(
						len(contractNamesKey) + len(contractCodeKey),
					),
				),
			),
		},
	}

	// Before migration

	// Call a dummy function - only need to see whether the value can be found.
	_, err = invokeContractFunction(payloads, address, contractName, "foo")

	// CBOR error means value is found, but the decoding fails due to old format.
	assert.Contains(t, err.Error(), "unsupported decoded CBOR type: CBOR uint type")

	// After migration

	migration := &StorageFormatV6Migration{}
	migratedPayloads, err := migration.migrate(payloads)
	require.NoError(t, err)

	// Must contain total of 5 payloads:
	//  - 4x FVM registers
	//      - contract code
	//      - contract_names
	//      - storage_used
	//      - storage_index
	//  - 1x account storage register
	//  - 1x slab storage register
	require.Len(t, migratedPayloads, 6)

	sort.SliceStable(migratedPayloads, func(i, j int) bool {
		a := migratedPayloads[i].Key.KeyParts[2].Value
		b := migratedPayloads[j].Key.KeyParts[2].Value
		return bytes.Compare(a, b) < 0
	})

	assert.Equal(t, []byte(atree.LedgerBaseStorageSlabPrefix+string([]byte{0, 0, 0, 0, 0, 0, 0, 1})), migratedPayloads[0].Key.KeyParts[2].Value)
	assert.Equal(t, []byte("code.Test"), migratedPayloads[1].Key.KeyParts[2].Value)
	assert.Equal(t, []byte("contract\u001FTest"), migratedPayloads[2].Key.KeyParts[2].Value)
	assert.Equal(t, []byte("contract_names"), migratedPayloads[3].Key.KeyParts[2].Value)
	assert.Equal(t, []byte("storage_index"), migratedPayloads[4].Key.KeyParts[2].Value)
	assert.Equal(t, []byte("storage_used"), migratedPayloads[5].Key.KeyParts[2].Value)

	// Call a dummy function - only need to see whether the value can be found.
	result, err := invokeContractFunction(migratedPayloads, address, contractName, "foo")
	require.NoError(t, err)
	require.Equal(t, cadence.NewInt(42), result)
}

func invokeContractFunction(
	payloads []ledger.Payload,
	address common.Address,
	contractName string,
	funcName string,
) (val cadence.Value, err error) {
	ledgerView := newView(payloads)

	stateHolder := fvmState.NewStateHolder(
		fvmState.NewState(ledgerView),
	)

	txEnv := fvm.NewTransactionEnvironment(
		fvm.NewContext(zerolog.Nop()),
		fvm.NewVirtualMachine(
			runtime.NewInterpreterRuntime(),
		),
		stateHolder,
		programs.NewEmptyPrograms(),
		flow.NewTransactionBody(),
		0,
		nil,
	)

	location := common.AddressLocation{
		Address: address,
		Name:    contractName,
	}

	predeclaredValues := make([]runtime.ValueDeclaration, 0)

	defer func() {
		if r := recover(); r != nil {
			switch typedR := r.(type) {
			case error:
				err = typedR
			case string:
				err = fmt.Errorf(typedR)
			default:
				panic(typedR)
			}
		}
	}()

	return txEnv.VM().Runtime.InvokeContractFunction(
		location,
		funcName,
		[]newInter.Value{},
		[]sema.Type{},
		runtime.Context{
			Interface:         txEnv,
			PredeclaredValues: predeclaredValues,
		},
	)
}

func uint64ToBinary(integer uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, integer)
	return b
}

func TestDeferredValues(t *testing.T) {

	var payloads []ledger.Payload

	owner, err := hex.DecodeString("000027b9a80c0152")
	require.NoError(t, err)

	data := map[string]string{
		"public_key_9":                "f847b840dac22f8bdf651e21f10d5cc96ef8ca9a55bd28b56b117dda0877b3bce76ce7e04c4fe330c48e5c81f13f76c7dc1b86ab49a33d8b2498cbbed24459f7e1e918c50201808080",
		"public\x1fflowTokenBalance":  "00cade0005d8cb82d8c882016e666c6f77546f6b656e5661756c74d8db82f4d8dc82d8d582d8c082481654653399040a6169466c6f77546f6b656e6f466c6f77546f6b656e2e5661756c7481d8d682d8c08248f233dcee88fe0abe6d46756e6769626c65546f6b656e7546756e6769626c65546f6b656e2e42616c616e6365",
		"public_key_8":                "f847b840dac22f8bdf651e21f10d5cc96ef8ca9a55bd28b56b117dda0877b3bce76ce7e04c4fe330c48e5c81f13f76c7dc1b86ab49a33d8b2498cbbed24459f7e1e918c50201808080",
		"storage\x1fMomentCollection": "00cade0005d88484d8c082480b2a3299cc857e2967546f7053686f7402846475756964d8a41a03487415696f776e65644e465473d88183d8d982d8d41830d8d582d8c082481d7e57aa55817448704e6f6e46756e6769626c65546f6b656e744e6f6e46756e6769626c65546f6b656e2e4e4654d88682d8d7d8d4183089d8a41a007f9971d8a41a00c28fa9d8a41a00d967f9d8a41a007cea68d8a41a008a43a1d8a41a0094f75ad8a41a00b7af06d8a41a00ee178ed8a41a00be2d868072546f7053686f742e436f6c6c656374696f6e",
		"storage\x1fMomentCollection\x1fownedNFTs\x1fv\x1f14247929": "00cade0005d88484d8c082480b2a3299cc857e2967546f7053686f7402866475756964d8a41a025719bd626964d8a41a00d967f96464617461d88484d8c082480b2a3299cc857e2967546f7053686f740186657365744944d8a3181a66706c61794944d8a31904cd6c73657269616c4e756d626572d8a319732d72546f7053686f742e4d6f6d656e74446174616b546f7053686f742e4e4654",
		"public\x1fMomentCollection":                                "00cade0005d8cb82d8c88201704d6f6d656e74436f6c6c656374696f6ed8db82f4d8dc82d8d40581d8d682d8c082480b2a3299cc857e2967546f7053686f74781e546f7053686f742e4d6f6d656e74436f6c6c656374696f6e5075626c6963",
		"storage\x1fMomentCollection\x1fownedNFTs\x1fv\x1f9762650":  "00cade0005d88484d8c082480b2a3299cc857e2967546f7053686f7402866475756964d8a41a01df64e6626964d8a41a0094f75a6464617461d88484d8c082480b2a3299cc857e2967546f7053686f740186657365744944d8a3181a66706c61794944d8a31904136c73657269616c4e756d626572d8a319072572546f7053686f742e4d6f6d656e74446174616b546f7053686f742e4e4654",
		"storage\x1fMomentCollection\x1fownedNFTs\x1fv\x1f15603598": "00cade0005d88484d8c082480b2a3299cc857e2967546f7053686f7402866475756964d8a41a02b37553626964d8a41a00ee178e6464617461d88484d8c082480b2a3299cc857e2967546f7053686f740186657365744944d8a3181a66706c61794944d8a31905796c73657269616c4e756d626572d8a31908e972546f7053686f742e4d6f6d656e74446174616b546f7053686f742e4e4654",
		"public_key_10":                       "f847b840dac22f8bdf651e21f10d5cc96ef8ca9a55bd28b56b117dda0877b3bce76ce7e04c4fe330c48e5c81f13f76c7dc1b86ab49a33d8b2498cbbed24459f7e1e918c50201808080",
		"storage\x1fprivateForwardingStorage": "00cade0005d88484d8c0824818eb4ee6b3c026d27818507269766174655265636569766572466f7277617264657202846475756964d8a41a0348741669726563697069656e74d8c983d8834627b9a80c0152d8c8820271666c6f77546f6b656e5265636569766572d8db82f4d8dc82d8d40581d8d682d8c08248f233dcee88fe0abe6d46756e6769626c65546f6b656e7646756e6769626c65546f6b656e2e52656365697665727822507269766174655265636569766572466f727761726465722e466f72776172646572",
		"public_key_4":                        "f847b840dac22f8bdf651e21f10d5cc96ef8ca9a55bd28b56b117dda0877b3bce76ce7e04c4fe330c48e5c81f13f76c7dc1b86ab49a33d8b2498cbbed24459f7e1e918c50201808080",
		"public_key_0":                        "f849b840dac22f8bdf651e21f10d5cc96ef8ca9a55bd28b56b117dda0877b3bce76ce7e04c4fe330c48e5c81f13f76c7dc1b86ab49a33d8b2498cbbed24459f7e1e918c502018203e88080",
		"storage\x1fMomentCollection\x1fownedNFTs\x1fv\x1f12037894": "00cade0005d88484d8c082480b2a3299cc857e2967546f7053686f7402866475756964d8a41a0227ec79626964d8a41a00b7af066464617461d88484d8c082480b2a3299cc857e2967546f7053686f740186657365744944d8a3181a66706c61794944d8a319049a6c73657269616c4e756d626572d8a3197cca72546f7053686f742e4d6f6d656e74446174616b546f7053686f742e4e4654",
		"storage\x1fdapperUtilityCoinReceiver":                      "00cade0005d88484d8c08248e544175ee0461c4b6f546f6b656e466f7277617264696e6702846475756964d8a41a0348741769726563697069656e74d8c983d88348ead892083b3e2c6cd8c8820378196461707065725574696c697479436f696e5265636569766572f67819546f6b656e466f7277617264696e672e466f72776172646572",
		"public_key_5":                                              "f847b840dac22f8bdf651e21f10d5cc96ef8ca9a55bd28b56b117dda0877b3bce76ce7e04c4fe330c48e5c81f13f76c7dc1b86ab49a33d8b2498cbbed24459f7e1e918c50201808080",
		"public\x1fprivateForwardingPublic":                         "00cade0005d8cb82d8c88201781870726976617465466f7277617264696e6753746f72616765d8db82f4d8d582d8c0824818eb4ee6b3c026d27818507269766174655265636569766572466f727761726465727822507269766174655265636569766572466f727761726465722e466f72776172646572",
		"exists":                                                    "01",
		"public\x1fdapperUtilityCoinReceiver":                       "00cade0005d8cb82d8c8820178196461707065725574696c697479436f696e5265636569766572d8db82f4d8dc82d8d582d8c08248ead892083b3e2c6c714461707065725574696c697479436f696e774461707065725574696c697479436f696e2e5661756c7481d8d682d8c08248f233dcee88fe0abe6d46756e6769626c65546f6b656e7646756e6769626c65546f6b656e2e5265636569766572",
		"storage\x1fMomentCollection\x1fownedNFTs\x1fv\x1f8186472":  "00cade0005d88484d8c082480b2a3299cc857e2967546f7053686f7402866464617461d88484d8c082480b2a3299cc857e2967546f7053686f74018666706c61794944d8a31903b16c73657269616c4e756d626572d8a3190f9a657365744944d8a3181a72546f7053686f742e4d6f6d656e7444617461626964d8a41a007cea686475756964d8a41a019776ec6b546f7053686f742e4e4654",
		"private\x1fflowTokenReceiver":                              "00cade0005d8cb82d8c882016e666c6f77546f6b656e5661756c74d8db82f4d8dc82d8d40581d8d682d8c08248f233dcee88fe0abe6d46756e6769626c65546f6b656e7646756e6769626c65546f6b656e2e5265636569766572",
		"storage\x1fMomentCollection\x1fownedNFTs\x1fv\x1f9061281":  "00cade0005d88484d8c082480b2a3299cc857e2967546f7053686f7402866464617461d88484d8c082480b2a3299cc857e2967546f7053686f74018666706c61794944d8a31903e96c73657269616c4e756d626572d8a3194a6a657365744944d8a3181a72546f7053686f742e4d6f6d656e7444617461626964d8a41a008a43a16475756964d8a41a01b2beca6b546f7053686f742e4e4654",
		"public_key_3":                                              "f847b840dac22f8bdf651e21f10d5cc96ef8ca9a55bd28b56b117dda0877b3bce76ce7e04c4fe330c48e5c81f13f76c7dc1b86ab49a33d8b2498cbbed24459f7e1e918c50201808080",
		"public_key_7":                                              "f847b840dac22f8bdf651e21f10d5cc96ef8ca9a55bd28b56b117dda0877b3bce76ce7e04c4fe330c48e5c81f13f76c7dc1b86ab49a33d8b2498cbbed24459f7e1e918c50201808080",
		"public_key_1":                                              "f847b840dac22f8bdf651e21f10d5cc96ef8ca9a55bd28b56b117dda0877b3bce76ce7e04c4fe330c48e5c81f13f76c7dc1b86ab49a33d8b2498cbbed24459f7e1e918c50201808080",
		"storage\x1fMomentCollection\x1fownedNFTs\x1fv\x1f12750761": "00cade0005d88484d8c082480b2a3299cc857e2967546f7053686f7402866475756964d8a41a023f0e94626964d8a41a00c28fa96464617461d88484d8c082480b2a3299cc857e2967546f7053686f740186657365744944d8a3181a66706c61794944d8a31902786c73657269616c4e756d626572d8a3198d9872546f7053686f742e4d6f6d656e74446174616b546f7053686f742e4e4654",
		"storage_used":                                              "0000000000001292",
		"public_key_2":                                              "f847b840dac22f8bdf651e21f10d5cc96ef8ca9a55bd28b56b117dda0877b3bce76ce7e04c4fe330c48e5c81f13f76c7dc1b86ab49a33d8b2498cbbed24459f7e1e918c50201808080",
		"storage\x1fflowTokenVault":                                 "00cade0005d88484d8c082481654653399040a6169466c6f77546f6b656e02846475756964d8a41a034874146762616c616e6365d8bc1a000186a06f466c6f77546f6b656e2e5661756c74",
		"storage\x1fMomentCollection\x1fownedNFTs\x1fv\x1f8362353":  "00cade0005d88484d8c082480b2a3299cc857e2967546f7053686f7402866464617461d88484d8c082480b2a3299cc857e2967546f7053686f74018666706c61794944d8a31903b86c73657269616c4e756d626572d8a3192af8657365744944d8a3181a72546f7053686f742e4d6f6d656e7444617461626964d8a41a007f99716475756964d8a41a019a33b16b546f7053686f742e4e4654",
		"public_key_count":                                          "0b",
		"public_key_6":                                              "f847b840dac22f8bdf651e21f10d5cc96ef8ca9a55bd28b56b117dda0877b3bce76ce7e04c4fe330c48e5c81f13f76c7dc1b86ab49a33d8b2498cbbed24459f7e1e918c50201808080",
		"storage\x1fMomentCollection\x1fownedNFTs\x1fv\x1f12463494": "00cade0005d88484d8c082480b2a3299cc857e2967546f7053686f7402866475756964d8a41a023a2c6c626964d8a41a00be2d866464617461d88484d8c082480b2a3299cc857e2967546f7053686f740186657365744944d8a3181a66706c61794944d8a31903bc6c73657269616c4e756d626572d8a319984572546f7053686f742e4d6f6d656e74446174616b546f7053686f742e4e4654",
	}

	for key, hexValue := range data {

		value, err := hex.DecodeString(hexValue)
		require.NoError(t, err)

		var controller []byte

		if key == fvmState.KeyPublicKeyCount ||
			bytes.HasPrefix([]byte(key), []byte("public_key_")) ||
			key == fvmState.KeyContractNames ||
			bytes.HasPrefix([]byte(key), []byte(fvmState.KeyCode)) {

			controller = owner
		}

		payloads = append(payloads, ledger.Payload{
			Key: ledger.NewKey([]ledger.KeyPart{
				ledger.NewKeyPart(state.KeyPartOwner, owner),
				ledger.NewKeyPart(state.KeyPartController, controller),
				ledger.NewKeyPart(state.KeyPartKey, []byte(key)),
			}),
			Value: value,
		})
	}

	ledgerView := newView(payloads)

	migration := &StorageFormatV6Migration{}
	migration.initPersistentSlabStorage(ledgerView)
	migration.initNewInterpreter()
	migration.initOldInterpreter(payloads)

	nftAddress, err := common.HexToAddress("1d7e57aa55817448")
	require.NoError(t, err)

	nftLocation := common.AddressLocation{
		Address: nftAddress,
		Name:    "NonFungibleToken",
	}
	nftType := &sema.CompositeType{
		Location:   nftLocation,
		Identifier: "NonFungibleToken.NFT",
		Kind:       common.CompositeKindResource,
	}

	nftElaboration := sema.NewElaboration()
	nftElaboration.CompositeTypes[nftType.ID()] = nftType

	topshotAddress, err := common.HexToAddress("0b2a3299cc857e29")
	require.NoError(t, err)

	topshotLocation := common.AddressLocation{
		Address: topshotAddress,
		Name:    "TopShot",
	}
	topshotType := &sema.CompositeType{
		Location:   topshotLocation,
		Identifier: "TopShot.NFT",
		Kind:       common.CompositeKindResource,
		ImplicitTypeRequirementConformances: []*sema.CompositeType{
			nftType,
		},
	}

	topshotElaboration := sema.NewElaboration()
	topshotElaboration.CompositeTypes[topshotType.ID()] = topshotType

	migration.programs = programs.NewEmptyPrograms()
	migration.programs.Set(
		nftLocation,
		&newInter.Program{
			Program:     &ast.Program{},
			Elaboration: nftElaboration,
		},
		nil,
	)
	migration.programs.Set(
		topshotLocation,
		&newInter.Program{
			Program:     &ast.Program{},
			Elaboration: topshotElaboration,
		},
		nil,
	)

	_, err = migration.Migrate(payloads)
	require.NoError(t, err)

	var result newInter.Value = migration.newInter.ReadStored(
		common.BytesToAddress(owner),
		"storage\u001FMomentCollection",
	)

	require.IsType(t, result, &newInter.SomeValue{})
	result = result.(*newInter.SomeValue).Value

	require.IsType(t, &newInter.CompositeValue{}, result)
	composite := result.(*newInter.CompositeValue)

	ownedNFTS := composite.GetField("ownedNFTs")
	require.IsType(t, &newInter.DictionaryValue{}, ownedNFTS)
	dictionary := ownedNFTS.(*newInter.DictionaryValue)

	require.Equal(t, 9, dictionary.Count())

	for _, id := range []newInter.UInt64Value{
		15603598,
		12750761,
		8186472,
		8362353,
		9061281,
		12037894,
		9762650,
		14247929,
		12463494,
	} {
		result := dictionary.GetKey(migration.newInter, newInter.ReturnEmptyLocationRange, id)
		require.NotNil(t, result)
	}
}
