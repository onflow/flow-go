package migrations

import (
	"context"
	"fmt"

	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

func accountStatusPayload(address common.Address) *ledger.Payload {
	accountStatus := environment.NewAccountStatus()

	return ledger.NewPayload(
		convert.RegisterIDToLedgerKey(
			flow.AccountStatusRegisterID(flow.ConvertAddress(address)),
		),
		accountStatus.ToBytes(),
	)
}

func TestErrorPropagation(t *testing.T) {
	t.Parallel()

	log := zerolog.New(zerolog.NewTestWriter(t))

	address, err := common.HexToAddress("0x1")
	require.NoError(t, err)

	migrateWith := func(mig AccountBasedMigration) error {

		// at least one payload otherwise the migration will not get called
		payloads := []*ledger.Payload{
			accountStatusPayload(address),
		}

		registersByAccount, err := registers.NewByAccountFromPayloads(payloads)
		if err != nil {
			return fmt.Errorf("could not create registers by account: %w", err)
		}

		return MigrateByAccount(
			log,
			10,
			registersByAccount,
			[]AccountBasedMigration{
				mig,
			},
		)
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
				InitMigrationFN: func(
					log zerolog.Logger,
					registersByAccount *registers.ByAccount,
					nWorkers int,
				) error {
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
				MigrateAccountFN: func(
					_ context.Context,
					_ common.Address,
					_ *registers.AccountRegisters,
				) error {
					return desiredErr
				},
			},
		)
		require.ErrorIs(t, err, desiredErr)
	})
}

type testMigration struct {
	InitMigrationFN func(
		log zerolog.Logger,
		registersByAccount *registers.ByAccount,
		nWorkers int,
	) error
	MigrateAccountFN func(
		ctx context.Context,
		address common.Address,
		accountRegisters *registers.AccountRegisters,
	) error
	CloseFN func() error
}

var _ AccountBasedMigration = &testMigration{}

func (t testMigration) InitMigration(
	log zerolog.Logger,
	registersByAccount *registers.ByAccount,
	nWorkers int,
) error {
	if t.InitMigrationFN != nil {
		return t.InitMigrationFN(log, registersByAccount, nWorkers)
	}
	return nil
}

func (t testMigration) MigrateAccount(
	ctx context.Context,
	address common.Address,
	accountRegisters *registers.AccountRegisters,
) error {

	if t.MigrateAccountFN != nil {
		return t.MigrateAccountFN(ctx, address, accountRegisters)
	}
	return nil
}

func (t testMigration) Close() error {
	if t.CloseFN != nil {
		return t.CloseFN()
	}
	return nil
}
