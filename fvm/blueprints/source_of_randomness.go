package blueprints

import (
	_ "embed"

	"encoding/hex"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-core-contracts/lib/go/contracts"

	"github.com/onflow/flow-go/model/flow"
)

//go:embed scripts/deploySourceOfRandomnessHistoryTransactionTemplate.cdc
var deploySourceOfRandomnessHistoryTransactionTemplate string

// DeploySourceOfRandomnessHistoryTransaction returns the transaction body for the deployment
// of the SourceOfRandomness contract transaction
func DeploySourceOfRandomnessHistoryTransaction(
	service flow.Address,
) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(deploySourceOfRandomnessHistoryTransactionTemplate)).
		AddArgument(jsoncdc.MustEncode(cadence.String(hex.EncodeToString(contracts.SourceOfRandomnessHistory())))).
		AddAuthorizer(service)
}
