package migrations

import (
	"github.com/rs/zerolog"

	migrationSnapshot "github.com/onflow/flow-go/cmd/util/ledger/util/snapshot"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

func NewTransactionBasedMigration(
	tx *flow.TransactionBody,
	chainID flow.ChainID,
	logger zerolog.Logger,
	expectedWriteAddresses map[flow.Address]struct{},
) ledger.Migration {
	return func(payloads []*ledger.Payload) ([]*ledger.Payload, error) {

		options := computation.DefaultFVMOptions(chainID, false, false)
		options = append(options,
			fvm.WithContractDeploymentRestricted(false),
			fvm.WithContractRemovalRestricted(false),
			fvm.WithAuthorizationChecksEnabled(false),
			fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
			fvm.WithTransactionFeesEnabled(false))
		ctx := fvm.NewContext(options...)

		snapshot, err := migrationSnapshot.NewPayloadSnapshot(zerolog.Nop(), payloads, migrationSnapshot.SmallChangeSetSnapshot, 1)
		if err != nil {
			return nil, err
		}

		vm := fvm.NewVirtualMachine()

		executionSnapshot, res, err := vm.Run(
			ctx,
			fvm.Transaction(tx, 0),
			snapshot,
		)

		if err != nil {
			return nil, err
		}

		if res.Err != nil {
			return nil, res.Err
		}

		return snapshot.ApplyChangesAndGetNewPayloads(
			executionSnapshot.WriteSet,
			expectedWriteAddresses,
			logger,
		)
	}
}
