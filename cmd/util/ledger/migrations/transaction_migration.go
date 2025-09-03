package migrations

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/model/flow"
)

func NewTransactionBasedMigration(
	tx *flow.TransactionBody,
	chainID flow.ChainID,
	logger zerolog.Logger,
	expectedWriteAddresses map[flow.Address]struct{},
) RegistersMigration {
	return func(registersByAccount *registers.ByAccount) error {

		options := computation.DefaultFVMOptions(chainID, false, false)
		options = append(options,
			fvm.WithContractDeploymentRestricted(false),
			fvm.WithContractRemovalRestricted(false),
			fvm.WithAuthorizationChecksEnabled(false),
			fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
			fvm.WithTransactionFeesEnabled(false))
		ctx := fvm.NewContext(options...)

		storageSnapshot := registers.StorageSnapshot{
			Registers: registersByAccount,
		}

		vm := fvm.NewVirtualMachine()

		executionSnapshot, res, err := vm.Run(
			ctx,
			fvm.Transaction(tx, 0),
			storageSnapshot,
		)
		if err != nil {
			return fmt.Errorf("failed to run transaction: %w", err)
		}

		if res.Err != nil {
			return fmt.Errorf("transaction failed: %w", res.Err)
		}

		err = registers.ApplyChanges(
			registersByAccount,
			executionSnapshot.WriteSet,
			expectedWriteAddresses,
			logger,
		)
		if err != nil {
			return fmt.Errorf("failed to apply changes: %w", err)
		}

		return nil
	}
}
