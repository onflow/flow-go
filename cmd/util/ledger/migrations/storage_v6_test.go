package migrations

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fxamacker/cbor/v2"
	"github.com/onflow/atree"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/programs"
	fvmState "github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	newInter "github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"

	oldInter "github.com/onflow/cadence/v19/runtime/interpreter"
	"github.com/onflow/cadence/v19/runtime/tests/utils"
)

func TestValueConversion(t *testing.T) {

	t.Parallel()

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
					ledger.NewKeyPart(state.KeyPartOwner, address.Bytes()),
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
		newValue := converter.Convert(oldArray)

		assert.IsType(t, &newInter.ArrayValue{}, newValue)
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
					ledger.NewKeyPart(state.KeyPartOwner, address.Bytes()),
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
		newValue := converter.Convert(oldDictionary)

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
					ledger.NewKeyPart(state.KeyPartOwner, owner.Bytes()),
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
		newValue := converter.Convert(oldComposite)

		assert.IsType(t, &newInter.CompositeValue{}, newValue)
		composite := newValue.(*newInter.CompositeValue)

		fieldValue := composite.GetField(nil, nil, "foo")

		assert.IsType(t, &newInter.DictionaryValue{}, fieldValue)
		dictionary := fieldValue.(*newInter.DictionaryValue)

		value, ok := dictionary.Get(nil, nil, newInter.NewStringValue("key1"))
		require.True(t, ok)
		assert.Equal(t, newInter.NewStringValue("value1"), value)

		value, ok = dictionary.Get(nil, nil, newInter.NewStringValue("key2"))
		require.True(t, ok)
		assert.Equal(t, newInter.BoolValue(true), value)
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
					ledger.NewKeyPart(state.KeyPartOwner, address.Bytes()),
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
		migration.migratedPayloadPaths = make(map[storagePath]bool, 0)
		migration.converter = NewValueConverter(migration)

		err = migration.decodeAndConvert(encoded, address, "", oldInter.CurrentEncodingVersion)
		assert.NoError(t, err)

		err = migration.storage.Commit()
		assert.NoError(t, err)

		encodedValues := ledgerView.Payloads()
		require.Len(t, encodedValues, 3)

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
					ledger.NewKeyPart(state.KeyPartOwner, address.Bytes()),
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
		migration.migratedPayloadPaths = make(map[storagePath]bool, 0)
		migration.converter = NewValueConverter(migration)

		err = migration.decodeAndConvert(encoded, address, "", oldInter.CurrentEncodingVersion)
		assert.NoError(t, err)

		err = migration.storage.Commit()
		assert.NoError(t, err)

		encodedValues := ledgerView.Payloads()
		require.Len(t, encodedValues, 3)

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
					ledger.NewKeyPart(state.KeyPartOwner, address.Bytes()),
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
		migration.migratedPayloadPaths = make(map[storagePath]bool, 0)
		migration.converter = NewValueConverter(migration)

		err = migration.decodeAndConvert(encoded, address, "", oldInter.CurrentEncodingVersion)
		assert.NoError(t, err)

		err = migration.storage.Commit()
		assert.NoError(t, err)

		encodedValues := ledgerView.Payloads()
		require.Len(t, encodedValues, 5)

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

		fieldValue := composite.GetField(nil, nil, "foo")

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
		ledger.NewKeyPart(state.KeyPartOwner, owner.Bytes()),
		ledger.NewKeyPart(state.KeyPartController, []byte{}),
		ledger.NewKeyPart(state.KeyPartKey, []byte("Test")),
	}

	payloads := []ledger.Payload{
		{
			Key: ledger.NewKey([]ledger.KeyPart{
				ledger.NewKeyPart(state.KeyPartOwner, owner.Bytes()),
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
	value, err := ledgerView.Get(string(owner.Bytes()), "", "Test")
	require.NoError(t, err)
	assert.NotNil(t, value)

	storageFormatV6Migration := StorageFormatV6Migration{
		Log:       zerolog.Nop(),
		OutputDir: "none",
	}

	migratedPayloads, err := storageFormatV6Migration.migrate(payloads)
	require.NoError(t, err)

	assert.Len(t, migratedPayloads, 5)

	// Check whether the query works with new ledger

	migratedLedgerView := newView(migratedPayloads)

	key := []byte{0, 0, 0, 0, 0, 0, 0, 3}
	prefixedKey := []byte(atree.LedgerBaseStorageSlabPrefix + string(key))

	migratedValue, err := migratedLedgerView.Get(string(owner.Bytes()), "", string(prefixedKey))
	require.NoError(t, err)
	require.NotEmpty(t, migratedValue)

	assert.False(t, oldInter.HasMagic(migratedValue))
}

func TestContractValueRetrieval(t *testing.T) {

	t.Parallel()

	address := common.Address{1, 2}

	const contractName = "Test"

	contractValue := oldInter.NewCompositeValue(
		utils.TestLocation,
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
        }
    `

	contractValueKey := []ledger.KeyPart{
		ledger.NewKeyPart(state.KeyPartOwner, address.Bytes()),
		ledger.NewKeyPart(state.KeyPartController, []byte{}),
		ledger.NewKeyPart(state.KeyPartKey, []byte(fmt.Sprintf("contract\x1F%s", contractName))),
	}

	contractNamesKey := []ledger.KeyPart{
		ledger.NewKeyPart(state.KeyPartOwner, address.Bytes()),
		ledger.NewKeyPart(state.KeyPartController, address.Bytes()),
		ledger.NewKeyPart(state.KeyPartKey, []byte(fvmState.KeyContractNames)),
	}

	contractCodeKey := []ledger.KeyPart{
		ledger.NewKeyPart(state.KeyPartOwner, address.Bytes()),
		ledger.NewKeyPart(state.KeyPartController, address.Bytes()),
		ledger.NewKeyPart(state.KeyPartKey, []byte("code.Test")),
	}

	storageUsedKey := []ledger.KeyPart{
		ledger.NewKeyPart(state.KeyPartOwner, address.Bytes()),
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

	assert.Equal(t, []byte("/slab/"+string([]byte{0, 0, 0, 0, 0, 0, 0, 1})), migratedPayloads[0].Key.KeyParts[2].Value)
	assert.Equal(t, []byte("code.Test"), migratedPayloads[1].Key.KeyParts[2].Value)
	assert.Equal(t, []byte("contract\u001FTest"), migratedPayloads[2].Key.KeyParts[2].Value)
	assert.Equal(t, []byte("contract_names"), migratedPayloads[3].Key.KeyParts[2].Value)
	assert.Equal(t, []byte("storage_index"), migratedPayloads[4].Key.KeyParts[2].Value)
	assert.Equal(t, []byte("storage_used"), migratedPayloads[5].Key.KeyParts[2].Value)

	// Call a dummy function - only need to see whether the value can be found.
	_, err = invokeContractFunction(migratedPayloads, address, contractName, "foo")
	assert.NoError(t, err)
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
