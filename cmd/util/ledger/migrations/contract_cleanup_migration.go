package migrations

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/rs/zerolog"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/model/flow"
)

// ContractCleanupMigration normalized account's contract names and removes empty contracts.
type ContractCleanupMigration struct {
	log      zerolog.Logger
	reporter reporters.ReportWriter
}

const contractCleanupReporterName = "contract-cleanup"

var _ AccountBasedMigration = &ContractCleanupMigration{}

func NewContractCleanupMigration(rwf reporters.ReportWriterFactory) *ContractCleanupMigration {
	return &ContractCleanupMigration{
		reporter: rwf.ReportWriter(contractCleanupReporterName),
	}
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

	// Get the set of all contract names for the account.

	oldContractNames, err := d.getContractNames(accountRegisters)
	if err != nil {
		return fmt.Errorf(
			"failed to get contract names for %s: %w",
			address.HexWithPrefix(),
			err,
		)
	}

	contractNameSet := make(map[string]struct{})
	for _, contractName := range oldContractNames {
		contractNameSet[contractName] = struct{}{}
	}

	contractNamesSorted := make([]string, 0, len(contractNameSet))
	for contractName := range contractNameSet {
		contractNamesSorted = append(contractNamesSorted, contractName)
	}
	sort.Strings(contractNamesSorted)

	// Cleanup the code for each contract in the account.
	// If the contract code is empty, the contract code register will be removed,
	// and the contract name will be removed from the account's contract names.

	for _, contractName := range contractNamesSorted {
		removed, err := d.cleanupContractCode(
			address,
			accountRegisters,
			contractName,
		)
		if err != nil {
			return fmt.Errorf(
				"failed to cleanup contract code for %s: %w",
				address.HexWithPrefix(),
				err,
			)
		}

		if removed {
			delete(contractNameSet, contractName)
		}
	}

	// Sort the contract names and set them back to the account.

	newContractNames := make([]string, 0, len(contractNameSet))
	for contractName := range contractNameSet {
		newContractNames = append(newContractNames, contractName)
	}

	sort.Strings(newContractNames)

	// NOTE: Always set the contract names back to the account,
	// even if there are no contract names.
	// This effectively clears the contract names register.

	err = d.setContractNames(accountRegisters, newContractNames)
	if err != nil {
		return fmt.Errorf(
			"failed to set contract names for %s: %w",
			address.HexWithPrefix(),
			err,
		)
	}

	if !stringSlicesEqual(newContractNames, oldContractNames) {
		d.reporter.Write(contractNamesChanged{
			AccountAddress: address,
			Old:            oldContractNames,
			New:            newContractNames,
		})
	}

	return nil
}

func (d *ContractCleanupMigration) getContractNames(
	accountRegisters *registers.AccountRegisters,
) ([]string, error) {
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

	return contractNames, nil
}

func (d *ContractCleanupMigration) setContractNames(
	accountRegisters *registers.AccountRegisters,
	contractNames []string,
) error {
	owner := accountRegisters.Owner()

	var newEncodedContractNames []byte
	var err error

	// Encode the new contract names, if there are any.

	if len(contractNames) > 0 {
		newEncodedContractNames, err = environment.EncodeContractNames(contractNames)
		if err != nil {
			return fmt.Errorf(
				"failed to encode contract names: %w",
				err,
			)
		}
	}

	// NOTE: always set the contract names register, even if there are not contract names.
	// This effectively clears the contract names register.

	err = accountRegisters.Set(owner, flow.ContractNamesKey, newEncodedContractNames)
	if err != nil {
		return fmt.Errorf(
			"failed to set contract names: %w",
			err,
		)
	}

	return nil
}

// cleanupContractCode removes the code for the contract if it is empty.
// Returns true if the contract code was removed.
func (d *ContractCleanupMigration) cleanupContractCode(
	address common.Address,
	accountRegisters *registers.AccountRegisters,
	contractName string,
) (removed bool, err error) {
	owner := accountRegisters.Owner()

	contractKey := flow.ContractKey(contractName)

	code, err := accountRegisters.Get(owner, contractKey)
	if err != nil {
		return false, fmt.Errorf(
			"failed to get contract code for %s: %w",
			contractName,
			err,
		)
	}

	// If the contract code is empty, remove the contract code register.

	if len(bytes.TrimSpace(code)) == 0 {
		err = accountRegisters.Set(owner, contractKey, nil)
		if err != nil {
			return false, fmt.Errorf(
				"failed to clear contract code for %s: %w",
				contractName,
				err,
			)
		}

		d.reporter.Write(emptyContractRemoved{
			AccountAddress: address,
			ContractName:   contractName,
		})

		removed = true
	}

	return removed, nil
}

func (d *ContractCleanupMigration) Close() error {
	d.reporter.Close()

	return nil
}

type emptyContractRemoved struct {
	AccountAddress common.Address
	ContractName   string
}

var _ json.Marshaler = emptyContractRemoved{}

func (e emptyContractRemoved) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind           string `json:"kind"`
		AccountAddress string `json:"address"`
		ContractName   string `json:"name"`
	}{
		Kind:           "empty-contract-removed",
		AccountAddress: e.AccountAddress.HexWithPrefix(),
		ContractName:   e.ContractName,
	})
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

type contractNamesChanged struct {
	AccountAddress common.Address `json:"address"`
	Old            []string       `json:"old"`
	New            []string       `json:"new"`
}

var _ json.Marshaler = contractNamesChanged{}

func (e contractNamesChanged) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind           string   `json:"kind"`
		AccountAddress string   `json:"address"`
		Old            []string `json:"old"`
		New            []string `json:"new"`
	}{
		Kind:           "contract-names-changed",
		AccountAddress: e.AccountAddress.HexWithPrefix(),
		Old:            e.Old,
		New:            e.New,
	})
}
