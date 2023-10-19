package migrations

import (
	"bytes"
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/fxamacker/cbor/v2"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

// DeduplicateContractNamesMigration checks if the contract names have been duplicated and
// removes the duplicate ones.
type DeduplicateContractNamesMigration struct {
	log zerolog.Logger
}

func (d *DeduplicateContractNamesMigration) InitMigration(
	log zerolog.Logger,
	_ []*ledger.Payload,
	_ int,
) error {
	d.log = log.
		With().
		Str("migration", "DeduplicateContractNamesMigration").
		Logger()

	return nil
}

func (d *DeduplicateContractNamesMigration) MigrateAccount(
	ctx context.Context,
	address common.Address,
	payloads []*ledger.Payload,
) ([]*ledger.Payload, error) {
	snapshot, err := util.NewPayloadSnapshot(payloads)
	if err != nil {
		return nil, fmt.Errorf("failed to create payload snapshot: %w", err)
	}
	transactionState := state.NewTransactionState(snapshot, state.DefaultParameters())
	accounts := environment.NewAccounts(transactionState)
	flowAddress := flow.ConvertAddress(address)

	contractNames, err := accounts.GetContractNames(flowAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get contract names: %w", err)
	}
	if len(contractNames) == 0 {
		return payloads, nil
	}

	contractNamesSet := make(map[string]struct{})
	removeIndexes := make([]int, 0)
	for i, name := range contractNames {
		if _, ok := contractNamesSet[name]; ok {
			// duplicate contract name
			removeIndexes = append(removeIndexes, i)
			continue
		}

		contractNamesSet[name] = struct{}{}
	}

	if len(removeIndexes) == 0 {
		return payloads, nil
	}

	log.Info().
		Str("address", address.Hex()).
		Strs("contract_names", contractNames).
		Msg("removing duplicate contract names")

	// remove the duplicate contract names, keeping the original order
	for i := len(removeIndexes) - 1; i >= 0; i-- {
		contractNames = append(contractNames[:removeIndexes[i]], contractNames[removeIndexes[i]+1:]...)
	}

	var buf bytes.Buffer
	cborEncoder := cbor.NewEncoder(&buf)
	err = cborEncoder.Encode(contractNames)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot encode contract names: %s",
			contractNames,
		)
	}
	newContractNames := buf.Bytes()

	id := flow.ContractNamesRegisterID(flowAddress)
	err = accounts.SetValue(id, newContractNames)

	if err != nil {
		return nil, fmt.Errorf("setting value failed: %w", err)
	}

	// finalize the transaction
	result, err := transactionState.FinalizeMainTransaction()
	if err != nil {
		return nil, fmt.Errorf("failed to finalize main transaction: %w", err)
	}

	for id, value := range result.WriteSet {
		if value == nil {
			delete(snapshot.Payloads, id)
			continue
		}

		snapshot.Payloads[id] = ledger.NewPayload(
			convert.RegisterIDToLedgerKey(id),
			value,
		)
	}

	newPayloads := make([]*ledger.Payload, 0, len(snapshot.Payloads))
	for _, payload := range snapshot.Payloads {
		newPayloads = append(newPayloads, payload)
	}

	return newPayloads, nil

}

var _ AccountBasedMigration = &DeduplicateContractNamesMigration{}
