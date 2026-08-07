package migrations

import (
	"context"
	"testing"

	"github.com/onflow/atree"
	"github.com/onflow/cadence/common"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

func TestAccountStatusMigration(t *testing.T) {
	t.Parallel()

	log := zerolog.New(zerolog.NewTestWriter(t))
	rwf := &testReportWriterFactory{}

	migration := NewAccountUsageMigration(rwf)
	err := migration.InitMigration(log, nil, 0)
	require.NoError(t, err)

	addressString := "0x1"
	address, err := common.HexToAddress(addressString)
	require.NoError(t, err)
	ownerKey := ledger.KeyPart{Type: 0, Value: address.Bytes()}

	sizeOfTheStatusPayload := uint64(
		environment.RegisterSize(
			flow.AccountStatusRegisterID(flow.Address(address)),
			environment.NewAccountStatus().ToBytes(),
		),
	)

	migrate := func(oldPayloads []*ledger.Payload) ([]*ledger.Payload, error) {

		registersByAccount, err := registers.NewByAccountFromPayloads(oldPayloads)
		require.NoError(t, err)

		err = migration.InitMigration(log, registersByAccount, 1)
		require.NoError(t, err)

		accountRegisters := registersByAccount.AccountRegisters(string(address[:]))

		err = migration.MigrateAccount(
			context.Background(),
			address,
			accountRegisters,
		)
		if err != nil {
			return nil, err
		}

		err = migration.Close()
		require.NoError(t, err)

		newPayloads := registersByAccount.DestructIntoPayloads(1)

		return newPayloads, nil
	}

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		_, err := migrate([]*ledger.Payload{})
		require.Error(t, err)
	})

	t.Run("status register v3", func(t *testing.T) {
		t.Parallel()

		accountPublicKeyCount := uint32(5)
		statusPayloadAndSequenceNubmerSize := sizeOfTheStatusPayload +
			predefinedSequenceNumberPayloadSizes(string(address[:]), 1, accountPublicKeyCount)

		payloads := []*ledger.Payload{
			ledger.NewPayload(
				ledger.NewKey([]ledger.KeyPart{ownerKey, {Type: 2, Value: []byte(flow.AccountStatusKey)}}),
				[]byte{
					0,                                                 // flags
					0, 0, 0, 0, 0, 0, 0, byte(sizeOfTheStatusPayload), // storage used
					0, 0, 0, 0, 0, 0, 0, 6, // storage index
					0, 0, 0, 5, // public key counts
					0, 0, 0, 0, 0, 0, 0, 3, // account id counter
				},
			),
		}

		migrated, err := migrate(payloads)
		require.NoError(t, err)
		require.Len(t, migrated, 1)

		accountStatus, err := environment.AccountStatusFromBytes(migrated[0].Value())
		require.NoError(t, err)

		require.Equal(t, statusPayloadAndSequenceNubmerSize, accountStatus.StorageUsed())
		require.Equal(t, atree.SlabIndex{0, 0, 0, 0, 0, 0, 0, 6}, accountStatus.SlabIndex())
		require.Equal(t, accountPublicKeyCount, accountStatus.AccountPublicKeyCount())
		require.Equal(t, uint64(3), accountStatus.AccountIdCounter())
	})

	t.Run("data registers", func(t *testing.T) {
		t.Parallel()

		accountPublicKeyCount := uint32(5)
		statusPayloadAndSequenceNubmerSize := sizeOfTheStatusPayload +
			predefinedSequenceNumberPayloadSizes(string(address[:]), 1, accountPublicKeyCount)

		payloads := []*ledger.Payload{
			ledger.NewPayload(
				ledger.NewKey([]ledger.KeyPart{ownerKey, {Type: 2, Value: []byte(flow.AccountStatusKey)}}),
				[]byte{
					0,                       // flags
					0, 0, 0, 0, 0, 0, 0, 15, // storage used
					0, 0, 0, 0, 0, 0, 0, 6, // storage index
					0, 0, 0, 5, // public key counts
					0, 0, 0, 0, 0, 0, 0, 3, // account id counter
				},
			),
			ledger.NewPayload(
				ledger.NewKey([]ledger.KeyPart{ownerKey, {Type: 2, Value: []byte("1")}}),
				make([]byte, 100),
			),
		}

		migrated, err := migrate(payloads)
		require.NoError(t, err)
		require.Len(t, migrated, 2)

		var accountStatus *environment.AccountStatus
		for _, payload := range migrated {
			key, err := payload.Key()
			require.NoError(t, err)
			if string(key.KeyParts[1].Value) != flow.AccountStatusKey {
				continue
			}

			accountStatus, err = environment.AccountStatusFromBytes(payload.Value())
			require.NoError(t, err)
		}

		dataRegisterSize := uint64(environment.RegisterSize(
			flow.RegisterID{
				Owner: string(flow.Address(address).Bytes()),
				Key:   "1",
			},
			make([]byte, 100),
		))

		require.Equal(t, statusPayloadAndSequenceNubmerSize+dataRegisterSize, accountStatus.StorageUsed())
		require.Equal(t, atree.SlabIndex{0, 0, 0, 0, 0, 0, 0, 6}, accountStatus.SlabIndex())
		require.Equal(t, accountPublicKeyCount, accountStatus.AccountPublicKeyCount())
		require.Equal(t, uint64(3), accountStatus.AccountIdCounter())
	})
}
