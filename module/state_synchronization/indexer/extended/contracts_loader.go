package extended

import (
	"fmt"
	"time"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/utils/slices"
	"github.com/rs/zerolog"
)

// deployedContractsLoader is a helper for loading deployed contracts from storage at the given height.
// It scans all code registers at the given height and returns a [access.ContractDeployment] placeholder record
// for each deployed contract.
//
// All loaded contracts have their IsPlaceholder flag set to true, their BlockHeight set to
// the scanned height, and the TransactionID, TxIndex, and EventIndex fields are undefined (0).
//
// CAUTION: Not safe for concurrent use.
type deployedContractsLoader struct {
	log            zerolog.Logger
	scriptExecutor snapshotProvider
	registers      registerScanner
}

func newDeployedContractsLoader(
	log zerolog.Logger,
	scriptExecutor snapshotProvider,
	registers registerScanner,
) *deployedContractsLoader {
	return &deployedContractsLoader{
		log:            log,
		scriptExecutor: scriptExecutor,
		registers:      registers,
	}
}

// load scans all code registers at the given height and returns a
// access.ContractDeployment placeholder record for each deployed contract not already
// covered by seenContracts.
//
// All loaded contracts have their IsPlaceholder flag set to true, their BlockHeight set to
// the scanned height, and the TransactionID, TxIndex, and EventIndex fields are undefined (0).
//
// CAUTION: Not safe for concurrent use.
//
// No error returns are expected during normal operation.
func (c *deployedContractsLoader) Load(height uint64, seenContracts map[string]bool) ([]access.ContractDeployment, error) {
	start := time.Now()

	var deployments []access.ContractDeployment

	// log progress since this may be a long operation
	progress := util.LogProgress(c.log,
		// use the upper byte of the address as the progress total
		util.DefaultLogProgressConfig("loading contracts", 255),
	)

	lastAddressByte := 0
	logProgress := func(addressByte byte) {
		current := int(addressByte)
		if diff := current - lastAddressByte; diff > 0 {
			progress(diff)
			lastAddressByte = current
		}
	}

	// loadedContracts maps contracts in initial scan
	//   [address] → [contract name] → [index in deployments slice]
	// Used after the scan to mark deleted contracts and verify all expected contracts are present.
	loadedContracts := make(map[flow.Address]map[string]int)

	// Scan all contract code registers. This could take a long time, so interrupt the scan every
	// [contractsBootstrapMaxIterationDuration] and release the iterator to avoid blocking compaction
	// for too long.
	var lastRegisterID *flow.RegisterID
	for {
		batchStart := time.Now()
		timedOut := false

		for item, err := range c.registers.ByKeyPrefix(flow.CodeKeyPrefix, height, lastRegisterID) {
			if err != nil {
				return nil, fmt.Errorf("error scanning contract code registers: %w", err)
			}

			registerID := item.Cursor()
			deployment, isNew, err := c.parseContractRegister(registerID, item.Value, seenContracts, height)
			if err != nil {
				return nil, fmt.Errorf("error processing contract: %w", err)
			}

			if isNew {
				deployments = append(deployments, deployment)

				if _, ok := loadedContracts[deployment.Address]; !ok {
					loadedContracts[deployment.Address] = make(map[string]int)
				}
				loadedContracts[deployment.Address][deployment.ContractName] = len(deployments) - 1
			}

			logProgress(registerID.Owner[0])

			// If we've held the iterator open too long, record the lastRegisterID and release it so
			// compaction can proceed before we resume.
			if time.Since(batchStart) >= contractsBootstrapMaxIterationDuration {
				lastRegisterID = &registerID
				timedOut = true
				break
			}
		}

		if !timedOut {
			break
		}
	}

	// For each address with loaded contracts, fetch the names register once to:
	// 1. Set IsDeleted for contracts whose names are absent from the register
	// 2. Verify every name in the register has a corresponding deployment (loaded here or already
	//    seen in the current block via seenContracts).
	deletedCount := 0
	for address, byName := range loadedContracts {
		registerValue, err := c.scriptExecutor.RegisterValue(flow.ContractNamesRegisterID(address), height)
		if err != nil {
			return nil, fmt.Errorf("error getting contract names for %s: %w", address, err)
		}

		contractNames, err := environment.DecodeContractNames(registerValue)
		if err != nil {
			return nil, fmt.Errorf("error decoding contract names for %s: %w", address, err)
		}

		registeredNames := slices.ToMap(contractNames)

		// when a contract is deleted, its name is removed from the contract names register. In the
		// current implementation, its code is also set to nil. However, there are contracts on mainnet
		// where the content is not nil, but the name is removed. Handle both cases by using the
		// contract names register as the source of truth.
		for name, idx := range byName {
			if _, ok := registeredNames[name]; !ok {
				deployments[idx].IsDeleted = true
				deletedCount++
			}
		}

		// verify that all contracts listed in the contract names register are present in the loaded contracts.
		for name := range registeredNames {
			if _, ok := byName[name]; !ok {
				if !seenContracts[access.ContractID(address, name)] {
					return nil, fmt.Errorf("contract %q not found in initial load for %s", name, address)
				}
			}
		}
	}

	logProgress(255) // log to 100%

	c.log.Info().
		Uint64("height", height).
		Int("contracts", len(deployments)).
		Int("skipped_updated_in_block", len(seenContracts)).
		Int("deleted", deletedCount).
		Str("duration_ms", time.Since(start).String()).
		Msg("loaded contracts during bootstrap")

	return deployments, nil
}

// parseContractRegister parses a contract register and returns a [access.ContractDeployment] placeholder record.
// Returns false and an empty deployment if the contract was already seen.
// The returned deployment has BlockHeight set to height.
//
// No error returns are expected during normal operation.
func (c *deployedContractsLoader) parseContractRegister(
	reg flow.RegisterID,
	getRegistryValue func() (flow.RegisterValue, error),
	seenContracts map[string]bool,
	height uint64,
) (access.ContractDeployment, bool, error) {
	if reg.Owner == "" {
		return access.ContractDeployment{}, false, fmt.Errorf("found contract with empty owner: %q", reg)
	}

	address := flow.BytesToAddress([]byte(reg.Owner))
	contractName := flow.KeyContractName(reg.Key)
	contractID := access.ContractID(address, contractName)

	if seenContracts[contractID] {
		return access.ContractDeployment{}, false, nil
	}

	code, err := getRegistryValue()
	if err != nil {
		return access.ContractDeployment{}, false, fmt.Errorf("error reading contract code for %s: %w", contractID, err)
	}

	return access.ContractDeployment{
		ContractName:  contractName,
		Address:       address,
		BlockHeight:   height,
		Code:          code,
		CodeHash:      access.CadenceCodeHash(code),
		IsPlaceholder: true,
	}, true, nil
}
