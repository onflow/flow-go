package migrations_test

import (
	"context"
	"fmt"

	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/ledger"
)

func TestErrorPropagation(t *testing.T) {
	t.Parallel()

	log := zerolog.New(zerolog.NewTestWriter(t))

	address, err := common.HexToAddress("0x1")
	require.NoError(t, err)

	migrateWith := func(mig migrations.AccountBasedMigration) error {
		_, err := migrations.MigrateByAccount(
			log,
			10,
			[]*ledger.Payload{
				// at least one payload otherwise the migration will not get called
				accountStatusPayload(address),
			},
			[]migrations.AccountBasedMigration{
				mig,
			},
		)
		return err
	}

	t.Run("no err", func(t *testing.T) {
		err := migrateWith(
			testMigration{},
		)
		require.NoError(t, err)
	})

	t.Run("err on close", func(t *testing.T) {

		desiredErr := fmt.Errorf("test close error")
		err := migrateWith(
			testMigration{
				CloseFN: func() error {
					return desiredErr
				},
			},
		)
		require.ErrorIs(t, err, desiredErr)
	})

	t.Run("err on init", func(t *testing.T) {
		desiredErr := fmt.Errorf("test init error")
		err := migrateWith(
			testMigration{
				InitMigrationFN: func(log zerolog.Logger, allPayloads []*ledger.Payload, nWorkers int) error {
					return desiredErr
				},
			},
		)
		require.ErrorIs(t, err, desiredErr)
	})

	t.Run("err on migrate", func(t *testing.T) {
		desiredErr := fmt.Errorf("test migrate error")
		err := migrateWith(
			testMigration{
				MigrateAccountFN: func(ctx context.Context, address common.Address, payloads []*ledger.Payload) ([]*ledger.Payload, error) {
					return nil, desiredErr
				},
			},
		)
		require.ErrorIs(t, err, desiredErr)
	})
}

type testMigration struct {
	InitMigrationFN  func(log zerolog.Logger, allPayloads []*ledger.Payload, nWorkers int) error
	MigrateAccountFN func(ctx context.Context, address common.Address, payloads []*ledger.Payload) ([]*ledger.Payload, error)
	CloseFN          func() error
}

func (t testMigration) InitMigration(log zerolog.Logger, allPayloads []*ledger.Payload, nWorkers int) error {
	if t.InitMigrationFN != nil {
		return t.InitMigrationFN(log, allPayloads, nWorkers)
	}
	return nil
}

func (t testMigration) MigrateAccount(ctx context.Context, address common.Address, payloads []*ledger.Payload) ([]*ledger.Payload, error) {

	if t.MigrateAccountFN != nil {
		return t.MigrateAccountFN(ctx, address, payloads)
	}
	return payloads, nil
}

func (t testMigration) Close() error {
	if t.CloseFN != nil {
		return t.CloseFN()
	}
	return nil
}

var _ migrations.AccountBasedMigration = &testMigration{}
