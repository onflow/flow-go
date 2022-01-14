package migrations

import (
	"testing"

	"github.com/onflow/cadence/runtime/common"
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

	t.Run("sort values", func(t *testing.T) {
		payload := []ledger.Payload{
			{Key: createAccountPayloadKey(address1, state2.KeyExists), Value: []byte{1}},
			{Key: createAccountPayloadKey(address1, state2.KeyStorageUsed), Value: utils.Uint64ToBinary(1)},
			{Key: createAccountPayloadKey(address1, "storage\x1fFoo"), Value: []byte{1}},
			{Key: createAccountPayloadKey(address1, "public\x1fBar"), Value: []byte{3}},
			{Key: createAccountPayloadKey(address1, "private\x1fBar"), Value: []byte{2}},
		}
		migratedPayload, err := mig.Migrate(payload)
		require.NoError(t, err)
		require.Equal(t, len(migratedPayload), 9)

		cadenceAddress, _ := common.HexToAddress("0x1")

		stored := mig.Interpreter.ReadStored(cadenceAddress, "public", "Bar")
		require.Equal(t, stored, []byte{3})
	})
}
