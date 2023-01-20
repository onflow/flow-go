package blueprints

import (
	_ "embed"
	"encoding/hex"
	"fmt"
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-core-contracts/lib/go/contracts"

	"github.com/onflow/flow-core-contracts/lib/go/templates"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

const SystemChunkTransactionGasLimit = 100_000_000

// TODO (Ramtin) after changes to this method are merged into master move them here.

//go:embed scripts/systemChunkTransactionTemplate.cdc
var systemChunkTransactionTemplate string

// SystemChunkTransaction creates and returns the transaction corresponding to the system chunk
// for the given chain.
func SystemChunkTransaction(chain flow.Chain) (*flow.TransactionBody, error) {

	contracts, err := systemcontracts.SystemContractsForChain(chain.ChainID())
	if err != nil {
		return nil, fmt.Errorf("could not get system contracts for chain: %w", err)
	}

	tx := flow.NewTransactionBody().
		SetScript([]byte(templates.ReplaceAddresses(systemChunkTransactionTemplate,
			templates.Environment{
				EpochAddress:             contracts.Epoch.Address.Hex(),
				NodeVersionBeaconAddress: contracts.VersionBeacon.Address.Hex(),
			})),
		).
		AddAuthorizer(contracts.Epoch.Address).
		SetGasLimit(SystemChunkTransactionGasLimit)

	return tx, nil
}

// DeployNodeVersionBeaconTransaction returns the transaction body for the deployment NodeVersionBeacon contract transaction
func DeployNodeVersionBeaconTransaction(service flow.Address, versionThreshold cadence.UInt64) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(deployNodeVersionBeaconTransactionTemplate)).
		AddArgument(jsoncdc.MustEncode(cadence.String(hex.EncodeToString(contracts.NodeVersionBeacon())))).
		AddArgument(jsoncdc.MustEncode(versionThreshold)).
		AddAuthorizer(service)
}
