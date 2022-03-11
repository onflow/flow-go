package migrations

import (
	"math"
	"testing"

	"github.com/onflow/atree"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go/engine/execution/state"
	state2 "github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/utils"
)

func createAccountPayloadKey(a flow.Address, key string) ledger.Key {
	return ledger.Key{
		KeyParts: []ledger.KeyPart{
			ledger.NewKeyPart(state.KeyPartOwner, a.Bytes()),
			ledger.NewKeyPart(state.KeyPartController, []byte("")),
			ledger.NewKeyPart(state.KeyPartKey, []byte(key)),
		},
	}
}

func TestOrderedMapMigration(t *testing.T) {
	dir := t.TempDir()
	mig := OrderedMapMigration{
		Log:       zerolog.Logger{},
		OutputDir: dir,
	}

	address1 := flow.HexToAddress("0x1")
	cadenceAddress, _ := common.HexToAddress("0x1")

	_, _ = mig.initialize([]ledger.Payload{
		{Key: createAccountPayloadKey(address1, state2.KeyExists), Value: []byte{1}},
		{Key: createAccountPayloadKey(address1, state2.KeyStorageUsed), Value: utils.Uint64ToBinary(1)},
	})

	encodeValue := func(v interpreter.Value) ledger.Value {
		storable, err := v.Storable(mig.NewStorage, atree.Address(address1), math.MaxUint64)
		require.NoError(t, err)
		encodedInt, err := atree.Encode(storable, interpreter.CBOREncMode)
		require.NoError(t, err)
		return encodedInt
	}

	t.Run("sort values", func(t *testing.T) {

		one := interpreter.NewIntValueFromInt64(1)

		str := interpreter.NewStringValue("test")

		array := interpreter.NewArrayValue(
			mig.Interpreter,
			interpreter.VariableSizedStaticType{
				Type: interpreter.PrimitiveStaticTypeAnyStruct,
			},
			cadenceAddress,
			str,
			interpreter.BoolValue(true),
		)

		dict := interpreter.NewDictionaryValueWithAddress(
			mig.Interpreter,
			interpreter.DictionaryStaticType{
				KeyType:   interpreter.PrimitiveStaticTypeBool,
				ValueType: interpreter.PrimitiveStaticTypeString,
			},
			cadenceAddress,
			interpreter.BoolValue(true),
			str,
		)

		payload := []ledger.Payload{
			{Key: createAccountPayloadKey(address1, "storage\x1fFoo"), Value: encodeValue(array)},
			{Key: createAccountPayloadKey(address1, "public\x1fBar"), Value: encodeValue(one)},
			{Key: createAccountPayloadKey(address1, "public\x1fBar"), Value: encodeValue(one)},
			{Key: createAccountPayloadKey(address1, "storage\x1fBar"), Value: encodeValue(str)},
			{Key: createAccountPayloadKey(address1, "private\x1fBar"), Value: encodeValue(dict)},
		}
		migratedPayload, err := mig.migrate(payload)
		require.NoError(t, err)
		require.Equal(t, len(migratedPayload), 11)

		migrated := &OrderedMapMigration{}
		migrated.initPersistentSlabStorage(NewView(migratedPayload))
		migrated.initIntepreter()

		stored := migrated.Interpreter.ReadStored(cadenceAddress, "public", "Bar")
		require.Equal(t, stored, one)

		stored = migrated.Interpreter.ReadStored(cadenceAddress, "storage", "Foo")
		require.IsType(t, &interpreter.ArrayValue{}, stored)

		stored = migrated.Interpreter.ReadStored(cadenceAddress, "storage", "Bar")
		require.Equal(t, stored, str)

		stored = migrated.Interpreter.ReadStored(cadenceAddress, "private", "Bar")
		require.IsType(t, &interpreter.DictionaryValue{}, stored)

		stored = migrated.Interpreter.ReadStored(cadenceAddress, "storage", "Baz")
		require.Equal(t, stored, nil)
	})
}

func TestMultipleAccounts(t *testing.T) {
	dir := t.TempDir()
	mig := OrderedMapMigration{
		Log:       zerolog.Logger{},
		OutputDir: dir,
	}

	address1 := flow.HexToAddress("0x1")
	cadenceAddress1, _ := common.HexToAddress("0x1")

	address2 := flow.HexToAddress("0x2")
	cadenceAddress2, _ := common.HexToAddress("0x2")

	address3 := flow.HexToAddress("0x3")
	cadenceAddress3, _ := common.HexToAddress("0x3")

	_, _ = mig.initialize([]ledger.Payload{
		{Key: createAccountPayloadKey(address1, state2.KeyExists), Value: []byte{1}},
		{Key: createAccountPayloadKey(address1, state2.KeyStorageUsed), Value: utils.Uint64ToBinary(1)},
		{Key: createAccountPayloadKey(address2, state2.KeyExists), Value: []byte{1}},
		{Key: createAccountPayloadKey(address2, state2.KeyStorageUsed), Value: utils.Uint64ToBinary(1)},
		{Key: createAccountPayloadKey(address3, state2.KeyExists), Value: []byte{1}},
		{Key: createAccountPayloadKey(address3, state2.KeyStorageUsed), Value: utils.Uint64ToBinary(1)},
	})

	encodeValue := func(v interpreter.Value, address flow.Address) ledger.Value {
		storable, _ := v.Storable(mig.NewStorage, atree.Address(address), math.MaxUint64)
		encodedInt, _ := atree.Encode(storable, interpreter.CBOREncMode)
		return encodedInt
	}

	t.Run("sort values", func(t *testing.T) {

		one := interpreter.NewIntValueFromInt64(1)
		two := interpreter.NewIntValueFromInt64(1)
		three := interpreter.NewIntValueFromInt64(1)
		four := interpreter.NewIntValueFromInt64(1)
		five := interpreter.NewIntValueFromInt64(1)
		six := interpreter.NewIntValueFromInt64(1)

		payload := []ledger.Payload{
			{Key: createAccountPayloadKey(address1, "storage\x1fFoo"), Value: encodeValue(one, address1)},
			{Key: createAccountPayloadKey(address2, "storage\x1fFoo"), Value: encodeValue(two, address2)},
			{Key: createAccountPayloadKey(address3, "storage\x1fFoo"), Value: encodeValue(three, address3)},
			{Key: createAccountPayloadKey(address1, "storage\x1fBar"), Value: encodeValue(four, address1)},
			{Key: createAccountPayloadKey(address1, "storage\x1fBaz"), Value: encodeValue(five, address1)},
			{Key: createAccountPayloadKey(address3, "public\x1fFoo"), Value: encodeValue(six, address3)},
		}
		migratedPayload, err := mig.migrate(payload)
		require.NoError(t, err)
		require.Equal(t, len(migratedPayload), 17)

		migrated := &OrderedMapMigration{}
		migrated.initPersistentSlabStorage(NewView(migratedPayload))
		migrated.initIntepreter()

		stored := migrated.Interpreter.ReadStored(cadenceAddress1, "storage", "Foo")
		require.Equal(t, stored, one)

		stored = migrated.Interpreter.ReadStored(cadenceAddress2, "storage", "Foo")
		require.Equal(t, stored, two)

		stored = migrated.Interpreter.ReadStored(cadenceAddress3, "storage", "Foo")
		require.Equal(t, stored, three)

		stored = migrated.Interpreter.ReadStored(cadenceAddress1, "storage", "Bar")
		require.Equal(t, stored, four)

		stored = migrated.Interpreter.ReadStored(cadenceAddress2, "storage", "Bar")
		require.Equal(t, stored, nil)

		stored = migrated.Interpreter.ReadStored(cadenceAddress1, "storage", "Baz")
		require.Equal(t, stored, five)

		stored = migrated.Interpreter.ReadStored(cadenceAddress3, "public", "Foo")
		require.Equal(t, stored, six)

		stored = migrated.Interpreter.ReadStored(cadenceAddress1, "public", "Foo")
		require.Equal(t, stored, nil)

	})
}

func TestContractValue(t *testing.T) {
	dir := t.TempDir()
	mig := OrderedMapMigration{
		Log:       zerolog.Logger{},
		OutputDir: dir,
	}

	address1 := flow.HexToAddress("0x1")
	cadenceAddress, _ := common.HexToAddress("0x1")

	_, _ = mig.initialize([]ledger.Payload{
		{Key: createAccountPayloadKey(address1, state2.KeyExists), Value: []byte{1}},
		{Key: createAccountPayloadKey(address1, state2.KeyStorageUsed), Value: utils.Uint64ToBinary(1)},
	})

	encodeValue := func(v interpreter.Value) ledger.Value {
		storable, err := v.Storable(mig.NewStorage, atree.Address(address1), math.MaxUint64)
		require.NoError(t, err)
		encodedInt, err := atree.Encode(storable, interpreter.CBOREncMode)
		require.NoError(t, err)
		return encodedInt
	}

	t.Run("migrate contract values", func(t *testing.T) {
		c := interpreter.NewCompositeValue(
			mig.Interpreter,
			common.AddressLocation{},
			"C",
			common.CompositeKindContract,
			[]interpreter.CompositeField{},
			cadenceAddress,
		)

		payload := []ledger.Payload{
			{Key: createAccountPayloadKey(address1, "contract\x1fFoo"), Value: encodeValue(c)},
		}
		migratedPayload, err := mig.migrate(payload)
		require.NoError(t, err)
		require.Equal(t, len(migratedPayload), 6)

		migrated := &OrderedMapMigration{}
		migrated.initPersistentSlabStorage(NewView(migratedPayload))
		migrated.initIntepreter()

		stored := migrated.Interpreter.ReadStored(cadenceAddress, "contract", "Foo")
		require.IsType(t, &interpreter.CompositeValue{}, stored)
		composite := stored.(*interpreter.CompositeValue)
		require.Equal(t, composite.Kind, common.CompositeKindContract)
		require.Equal(t, composite.QualifiedIdentifier, "C")
	})
}
