package migrations

import (
	"context"
	"fmt"

	"github.com/fxamacker/cbor/v2"
	"github.com/rs/zerolog"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/model/flow"
)

// DeduplicateContractNamesMigration checks if the contract names have been duplicated and
// removes the duplicate ones.
//
// This migration de-syncs storage used, so it should be run before the StorageUsedMigration.
type DeduplicateContractNamesMigration struct {
	log zerolog.Logger
}

func (d *DeduplicateContractNamesMigration) Close() error {
	return nil
}

func (d *DeduplicateContractNamesMigration) InitMigration(
	log zerolog.Logger,
	_ *registers.ByAccount,
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
	accountRegisters *registers.AccountRegisters,
) error {
	flowAddress := flow.ConvertAddress(address)
	owner := flow.AddressToRegisterOwner(flowAddress)

	value, err := accountRegisters.Get(owner, flow.ContractNamesKey)
	if err != nil {
		return err
	}

	if len(value) == 0 {
		// Remove the empty payload if exists
		return accountRegisters.Set(owner, flow.ContractNamesKey, nil)
	}

	var contractNames []string
	err = cbor.Unmarshal(value, &contractNames)
	if err != nil {
		return fmt.Errorf("failed to get contract names: %w", err)
	}

	var foundDuplicate bool
	i := 1
	for i < len(contractNames) {
		if contractNames[i-1] != contractNames[i] {

			if contractNames[i-1] > contractNames[i] {
				// this is not a valid state and we should fail.
				// Contract names must be sorted by definition.
				return fmt.Errorf(
					"contract names for account %s are not sorted: %s",
					address.Hex(),
					contractNames,
				)
			}

			i++
			continue
		}
		// Found duplicate (contactNames[i-1] == contactNames[i])
		// Remove contractNames[i]
		copy(contractNames[i:], contractNames[i+1:])
		contractNames = contractNames[:len(contractNames)-1]
		foundDuplicate = true
	}

	if !foundDuplicate {
		return nil
	}

	d.log.Info().
		Str("address", address.Hex()).
		Strs("contract_names", contractNames).
		Msg("removing duplicate contract names")

	newContractNames, err := cbor.Marshal(contractNames)
	if err != nil {
		return fmt.Errorf(
			"cannot encode contract names: %s",
			contractNames,
		)
	}

	// Set deduplicated contract names
	return accountRegisters.Set(owner, flow.ContractNamesKey, newContractNames)
}

var _ AccountBasedMigration = &DeduplicateContractNamesMigration{}
