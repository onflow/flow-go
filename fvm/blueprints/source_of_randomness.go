package blueprints

import (
	_ "embed"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-core-contracts/lib/go/contracts"

	"github.com/onflow/flow-go/model/flow"
)

//go:embed scripts/deployRandomBeaconHistoryTransactionTemplate.cdc
var deployRandomBeaconHistoryTransactionTemplate string

// DeployRandomBeaconHistoryTransaction returns the transaction body for the deployment
// of the RandomBeaconHistory contract transaction
func DeployRandomBeaconHistoryTransaction(
	service flow.Address,
) (*flow.TransactionBody, error) {
	return flow.NewTransactionBodyBuilder().
		SetScript([]byte(deployRandomBeaconHistoryTransactionTemplate)).
		SetPayer(service).
		AddArgument(jsoncdc.MustEncode(cadence.String(contracts.RandomBeaconHistory()))).
		AddAuthorizer(service).
		Build()
}
