package migrations_test

import (
	"testing"

	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/engine/execution/state"
	state2 "github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/utils"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestStorageUsedUpdateMigrationMigration(t *testing.T) {
	dir := t.TempDir()
	mig := migrations.StorageUsedUpdateMigration{
		Log:       zerolog.Logger{},
		OutputDir: dir,
	}

	address1 := flow.HexToAddress("0x1")

	t.Run("fix storage used", func(t *testing.T) {
		payload := []ledger.Payload{
			{Key: createAccountPayloadKey(address1, state2.KeyExists), Value: []byte{1}},
			{Key: createAccountPayloadKey(address1, state2.KeyStorageUsed), Value: utils.Uint64ToBinary(1)},
		}
		migratedPayload, err := mig.Migrate(payload)
		require.NoError(t, err)

		migratedSize, _, err := utils.ReadUint64(migratedPayload[1].Value)
		require.NoError(t, err)

		require.Equal(t, len(migratedPayload), len(payload))
		require.Equal(t, uint64(55), migratedSize)
	})

	t.Run("fix storage used if used to high", func(t *testing.T) {
		payload := []ledger.Payload{
			{Key: createAccountPayloadKey(address1, state2.KeyExists), Value: []byte{1}},
			{Key: createAccountPayloadKey(address1, state2.KeyStorageUsed), Value: utils.Uint64ToBinary(10000)},
		}
		migratedPayload, err := mig.Migrate(payload)
		require.NoError(t, err)

		migratedSize, _, err := utils.ReadUint64(migratedPayload[1].Value)
		require.NoError(t, err)

		require.Equal(t, len(migratedPayload), len(payload))
		require.Equal(t, uint64(55), migratedSize)
	})

	t.Run("do not fix storage used if storage used ok", func(t *testing.T) {
		payload := []ledger.Payload{
			{Key: createAccountPayloadKey(address1, state2.KeyExists), Value: []byte{1}},
			{Key: createAccountPayloadKey(address1, state2.KeyStorageUsed), Value: utils.Uint64ToBinary(55)},
		}
		migratedPayload, err := mig.Migrate(payload)
		require.NoError(t, err)

		migratedSize, _, err := utils.ReadUint64(migratedPayload[1].Value)
		require.NoError(t, err)

		require.Equal(t, len(migratedPayload), len(payload))
		require.Equal(t, uint64(55), migratedSize)
	})

	t.Run("error is storage used does not exist", func(t *testing.T) {
		payload := []ledger.Payload{
			{Key: createAccountPayloadKey(address1, state2.KeyExists), Value: []byte{1}},
		}
		_, err := mig.Migrate(payload)
		require.Error(t, err)
	})
}

func createAccountPayloadKey(a flow.Address, key string) ledger.Key {
	return ledger.Key{
		KeyParts: []ledger.KeyPart{
			ledger.NewKeyPart(state.KeyPartOwner, a.Bytes()),
			ledger.NewKeyPart(state.KeyPartController, []byte("")),
			ledger.NewKeyPart(state.KeyPartKey, []byte(key)),
		},
	}
}
