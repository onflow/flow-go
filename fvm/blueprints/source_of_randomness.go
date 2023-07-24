package blueprints

import (
	_ "embed"

	"encoding/hex"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-core-contracts/lib/go/contracts"

	"github.com/onflow/flow-go/model/flow"
)

//go:embed scripts/deploySourceOfRandomnessTransactionTemplate.cdc
var deploySourceOfRandomnessTransactionTemplate string

// DeploySourceOfRandomnessTransaction returns the transaction body for the deployment
// of the SourceOfRandomness contract transaction
func DeploySourceOfRandomnessTransaction(
	service flow.Address,
) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(deploySourceOfRandomnessTransactionTemplate)).
		AddArgument(jsoncdc.MustEncode(cadence.String(hex.EncodeToString(contracts.SourceOfRandomness())))).
		AddAuthorizer(service)
}
