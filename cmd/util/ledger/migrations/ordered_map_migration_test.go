package migrations

import (
	"math"
	"testing"

	"github.com/onflow/atree"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
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
	cadenceAddress, _ := common.HexToAddress("0x1")

	mig.initialize([]ledger.Payload{
		{Key: createAccountPayloadKey(address1, state2.KeyExists), Value: []byte{1}},
		{Key: createAccountPayloadKey(address1, state2.KeyStorageUsed), Value: utils.Uint64ToBinary(1)},
	})

	encodeValue := func(v interpreter.Value) ledger.Value {
		storable, _ := v.Storable(mig.newStorage, atree.Address(address1), math.MaxUint64)
		encodedInt, _ := atree.Encode(storable, interpreter.CBOREncMode)
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

		payload := []ledger.Payload{
			{Key: createAccountPayloadKey(address1, "storage\x1fFoo"), Value: encodeValue(array)},
			{Key: createAccountPayloadKey(address1, "public\x1fBar"), Value: encodeValue(one)},
			{Key: createAccountPayloadKey(address1, "storage\x1fBar"), Value: encodeValue(str)},
		}
		migratedPayload, err := mig.migrate(payload)
		require.NoError(t, err)
		require.Equal(t, len(migratedPayload), 8)

		migrated := &OrderedMapMigration{}
		migrated.initPersistentSlabStorage(NewView(migratedPayload))
		migrated.initIntepreter()

		stored := migrated.Interpreter.ReadStored(cadenceAddress, "public", "Bar")
		require.Equal(t, stored, one)

		stored = migrated.Interpreter.ReadStored(cadenceAddress, "storage", "Foo")
		require.IsType(t, &interpreter.ArrayValue{}, stored)

		stored = migrated.Interpreter.ReadStored(cadenceAddress, "storage", "Bar")
		require.Equal(t, stored, str)
	})
}
