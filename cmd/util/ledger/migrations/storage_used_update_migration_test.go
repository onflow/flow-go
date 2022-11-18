package migrations_test

import (
	"testing"

	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/fvm/environment"
	state2 "github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"

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
		status := environment.NewAccountStatus()
		status.SetStorageUsed(1)
		payload := []ledger.Payload{
			// TODO (ramtin) add more registers
			*ledger.NewPayload(
				createAccountPayloadKey(address1, state2.AccountStatusKey),
				status.ToBytes(),
			),
		}
		migratedPayload, err := mig.Migrate(payload)
		require.NoError(t, err)

		migratedStatus, err := environment.AccountStatusFromBytes(migratedPayload[0].Value())
		require.NoError(t, err)

		require.Equal(t, len(migratedPayload), len(payload))
		require.Equal(t, uint64(40), migratedStatus.StorageUsed())
	})

	t.Run("fix storage used if used to high", func(t *testing.T) {
		status := environment.NewAccountStatus()
		status.SetStorageUsed(10000)
		payload := []ledger.Payload{
			*ledger.NewPayload(
				createAccountPayloadKey(address1, state2.AccountStatusKey),
				status.ToBytes(),
			),
		}
		migratedPayload, err := mig.Migrate(payload)
		require.NoError(t, err)

		migratedStatus, err := environment.AccountStatusFromBytes(migratedPayload[0].Value())
		require.NoError(t, err)

		require.Equal(t, len(migratedPayload), len(payload))
		require.Equal(t, uint64(40), migratedStatus.StorageUsed())
	})

	t.Run("do not fix storage used if storage used ok", func(t *testing.T) {
		status := environment.NewAccountStatus()
		status.SetStorageUsed(40)
		payload := []ledger.Payload{
			*ledger.NewPayload(
				createAccountPayloadKey(address1, state2.AccountStatusKey),
				status.ToBytes(),
			),
		}
		migratedPayload, err := mig.Migrate(payload)
		require.NoError(t, err)

		migratedStatus, err := environment.AccountStatusFromBytes(migratedPayload[0].Value())
		require.NoError(t, err)

		require.Equal(t, len(migratedPayload), len(payload))
		require.Equal(t, uint64(40), migratedStatus.StorageUsed())
	})

	t.Run("error is storage used does not exist", func(t *testing.T) {
		payload := []ledger.Payload{
			*ledger.NewPayload(
				createAccountPayloadKey(address1, state2.AccountStatusKey),
				[]byte{1},
			),
		}
		_, err := mig.Migrate(payload)
		require.Error(t, err)
	})
}

func createAccountPayloadKey(a flow.Address, key string) ledger.Key {
	return ledger.Key{
		KeyParts: []ledger.KeyPart{
			ledger.NewKeyPart(state.KeyPartOwner, a.Bytes()),
			ledger.NewKeyPart(state.KeyPartKey, []byte(key)),
		},
	}
}
