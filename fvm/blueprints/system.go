package blueprints

import (
	_ "embed"
	"fmt"

	"github.com/onflow/flow-core-contracts/lib/go/templates"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

const SystemChunkTransactionGasLimit = 100_000_000

// TODO (Ramtin) after changes to this method are merged into master move them here.

// systemChunkTransactionTemplate looks for the epoch and version beacon heartbeat resources
// and calls them.
//
//go:embed scripts/systemChunkTransactionTemplate.cdc
var systemChunkTransactionTemplate string

// SystemChunkTransaction creates and returns the transaction corresponding to the
// system chunk for the given chain.
func SystemChunkTransaction(chain flow.Chain) (*flow.TransactionBody, error) {
	contracts, err := systemcontracts.SystemContractsForChain(chain.ChainID())
	if err != nil {
		return nil, fmt.Errorf("could not get system contracts for chain: %w", err)
	}

	// this is only true for testnet, sandboxnet and mainnet.
	if contracts.Epoch.Address != chain.ServiceAddress() {
		// Temporary workaround because the heartbeat resources need to be moved
		// to the service account:
		//  - the system chunk will attempt to load both Epoch and VersionBeacon
		//      resources from either the service account or the staking account
		//  - the service account committee can then safely move the resources
		//      at any time
		//  - once the resources are moved, this workaround should be removed
		//      after version v0.31.0
		return systemChunkTransactionDualAuthorizers(chain, contracts)
	}

	tx := flow.NewTransactionBody().
		SetScript(
			[]byte(templates.ReplaceAddresses(
				systemChunkTransactionTemplate,
				templates.Environment{
					EpochAddress:             contracts.Epoch.Address.Hex(),
					NodeVersionBeaconAddress: contracts.NodeVersionBeacon.Address.Hex(),
				},
			)),
		).
		AddAuthorizer(contracts.Epoch.Address).
		SetGasLimit(SystemChunkTransactionGasLimit)

	return tx, nil
}

// systemChunkTransactionTemplateDualAuthorizer is the same as systemChunkTransactionTemplate
// but it looks for the heartbeat resources on two different accounts.
//
//go:embed scripts/systemChunkTransactionTemplateDualAuthorizer.cdc
var systemChunkTransactionTemplateDualAuthorizer string

func systemChunkTransactionDualAuthorizers(
	chain flow.Chain,
	contracts *systemcontracts.SystemContracts,
) (*flow.TransactionBody, error) {

	tx := flow.NewTransactionBody().
		SetScript(
			[]byte(templates.ReplaceAddresses(
				systemChunkTransactionTemplateDualAuthorizer,
				templates.Environment{
					EpochAddress:             contracts.Epoch.Address.Hex(),
					NodeVersionBeaconAddress: contracts.NodeVersionBeacon.Address.Hex(),
				},
			)),
		).
		AddAuthorizer(chain.ServiceAddress()).
		AddAuthorizer(contracts.Epoch.Address).
		SetGasLimit(SystemChunkTransactionGasLimit)

	return tx, nil
}
