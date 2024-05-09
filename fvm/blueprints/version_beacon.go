package blueprints

import (
	_ "embed"
	"encoding/hex"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-core-contracts/lib/go/contracts"

	"github.com/onflow/flow-go/model/flow"
)

//go:embed scripts/deployNodeVersionBeaconTransactionTemplate.cdc
var deployNodeVersionBeaconTransactionTemplate string

// DeployNodeVersionBeaconTransaction returns the transaction body for the deployment NodeVersionBeacon contract transaction
func DeployNodeVersionBeaconTransaction(
	service flow.Address,
	versionFreezePeriod cadence.UInt64,
) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(deployNodeVersionBeaconTransactionTemplate)).
		AddArgument(jsoncdc.MustEncode(cadence.String(hex.EncodeToString(contracts.NodeVersionBeacon())))).
		AddArgument(jsoncdc.MustEncode(versionFreezePeriod)).
		AddAuthorizer(service)
}
