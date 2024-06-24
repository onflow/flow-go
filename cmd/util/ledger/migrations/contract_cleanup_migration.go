package migrations

import (
	"bytes"
	"context"
	"fmt"
	"sort"

	"github.com/rs/zerolog"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/model/flow"
)

// ContractCleanupMigration normalized account's contract names and removes empty contracts.
type ContractCleanupMigration struct {
	log zerolog.Logger
}

var _ AccountBasedMigration = &ContractCleanupMigration{}

func NewContractCleanupMigration() *ContractCleanupMigration {
	return &ContractCleanupMigration{}
}

func (d *ContractCleanupMigration) InitMigration(
	log zerolog.Logger,
	_ *registers.ByAccount,
	_ int,
) error {
	d.log = log.
		With().
		Str("migration", "ContractCleanupMigration").
		Logger()

	return nil
}

func (d *ContractCleanupMigration) MigrateAccount(
	_ context.Context,
	address common.Address,
	accountRegisters *registers.AccountRegisters,
) error {

	// Normalize the account's contract names.
	// This will deduplicate and sort the contract names.

	contractNames, err := d.normalizeContractNames(accountRegisters)
	if err != nil {
		return fmt.Errorf(
			"failed to normalize contract names for %s: %w",
			address.HexWithPrefix(),
			err,
		)
	}

	// Cleanup the code for each contract.
	// If the contract code is empty, it will be removed.

	for _, contractName := range contractNames {
		err = d.cleanupContractCode(accountRegisters, contractName)
		if err != nil {
			return fmt.Errorf(
				"failed to cleanup contract code for %s: %w",
				address.HexWithPrefix(),
				err,
			)
		}
	}

	return nil
}

// normalizeContractNames deduplicates and sorts the account's contract names.
func (d *ContractCleanupMigration) normalizeContractNames(
	accountRegisters *registers.AccountRegisters,
) (
	[]string,
	error,
) {
	owner := accountRegisters.Owner()

	encodedContractNames, err := accountRegisters.Get(owner, flow.ContractNamesKey)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get contract names: %w",
			err,
		)
	}

	if len(encodedContractNames) == 0 {
		return nil, nil
	}

	contractNames, err := environment.DecodeContractNames(encodedContractNames)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to decode contract names: %w",
			err,
		)
	}

	contractNames = normalizeContractNames(contractNames)

	newEncodedContractNames, err := environment.EncodeContractNames(contractNames)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to encode contract names: %w",
			err,
		)
	}

	err = accountRegisters.Set(owner, flow.ContractNamesKey, newEncodedContractNames)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to set new contract names: %w",
			err,
		)
	}

	return contractNames, nil
}

// normalizeContractNames deduplicates and sorts the contract names.
func normalizeContractNames(names []string) []string {
	seen := make(map[string]struct{}, len(names))
	deduplicated := make([]string, 0, len(names))

	for _, name := range names {
		if _, ok := seen[name]; ok {
			continue
		}

		seen[name] = struct{}{}
		deduplicated = append(deduplicated, name)
	}

	sort.Strings(deduplicated)

	return deduplicated
}

// cleanupContractCode removes the code for the contract if it is empty.
func (d *ContractCleanupMigration) cleanupContractCode(
	accountRegisters *registers.AccountRegisters,
	contractName string,
) error {
	owner := accountRegisters.Owner()

	contractKey := flow.ContractKey(contractName)

	code, err := accountRegisters.Get(owner, contractKey)
	if err != nil {
		return fmt.Errorf(
			"failed to get contract code for %s: %w",
			contractName,
			err,
		)
	}

	if len(bytes.TrimSpace(code)) == 0 {
		err = accountRegisters.Set(owner, contractKey, nil)
		if err != nil {
			return fmt.Errorf(
				"failed to clear contract code for %s: %w",
				contractName,
				err,
			)
		}
	}

	return nil
}

func (d *ContractCleanupMigration) Close() error {
	return nil
}
