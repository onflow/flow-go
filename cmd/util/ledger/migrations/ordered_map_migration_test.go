package migrations

import (
	"math"
	"testing"

	"github.com/onflow/atree"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	testUtils "github.com/onflow/cadence/runtime/tests/utils"
	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/engine/execution/state"
	state2 "github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
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

	encodeValue := func(v interpreter.Value) ledger.Value {
		storable, _ := v.Storable(interpreter.NewInMemoryStorage(), atree.Address(address1), math.MaxUint64)
		encodedInt, _ := atree.Encode(storable, interpreter.CBOREncMode)
		return encodedInt
	}

	testInter, _ := interpreter.NewInterpreter(
		nil,
		testUtils.TestLocation,
		interpreter.WithStorage(interpreter.NewInMemoryStorage()),
	)

	t.Run("sort values", func(t *testing.T) {

		one := interpreter.NewIntValueFromInt64(1)

		str := interpreter.NewStringValue("test")

		array := interpreter.NewArrayValue(
			testInter,
			interpreter.VariableSizedStaticType{
				Type: interpreter.PrimitiveStaticTypeAnyStruct,
			},
			common.Address{},
			str,
			interpreter.BoolValue(true),
		)

		payload := []ledger.Payload{
			{Key: createAccountPayloadKey(address1, state2.KeyExists), Value: []byte{1}},
			{Key: createAccountPayloadKey(address1, state2.KeyStorageUsed), Value: utils.Uint64ToBinary(1)},
			{Key: createAccountPayloadKey(address1, "storage\x1fFoo"), Value: encodeValue(array)},
			{Key: createAccountPayloadKey(address1, "public\x1fBar"), Value: encodeValue(one)},
			{Key: createAccountPayloadKey(address1, "storage\x1fBar"), Value: encodeValue(str)},
		}
		migratedPayload, err := mig.Migrate(payload)
		require.NoError(t, err)
		require.Equal(t, len(migratedPayload), 7)

		migrated := &OrderedMapMigration{}
		migrated.initPersistentSlabStorage(NewView(migratedPayload))
		migrated.initIntepreter()

		cadenceAddress, _ := common.HexToAddress("0x1")

		stored := migrated.Interpreter.ReadStored(cadenceAddress, "public", "Bar")
		require.Equal(t, stored, one)

		//stored = migrated.Interpreter.ReadStored(cadenceAddress, "storage", "Foo")
		//require.Equal(t, stored, array)

		stored = migrated.Interpreter.ReadStored(cadenceAddress, "storage", "Bar")
		require.Equal(t, stored, str)
	})
}
