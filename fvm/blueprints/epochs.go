package blueprints

import (
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-core-contracts/lib/go/templates"

	"github.com/onflow/flow-go/model/flow"
)

func SetStakingAllowlistTransaction(idTableStakingAddr flow.Address, allowedNodeIDs []flow.Identifier) *flow.TransactionBody {
	env := templates.Environment{
		IDTableAddress: idTableStakingAddr.HexWithPrefix(),
	}

	cdcNodeIDs := make([]cadence.Value, 0, len(allowedNodeIDs))
	for _, id := range allowedNodeIDs {
		cdcNodeID := cadence.NewString(id.String())
		cdcNodeIDs = append(cdcNodeIDs, cdcNodeID)
	}

	return flow.NewTransactionBody().
		SetScript(templates.GenerateSetApprovedNodesScript(env)).
		AddArgument(jsoncdc.MustEncode(cadence.NewArray(cdcNodeIDs))).
		AddAuthorizer(idTableStakingAddr)
}
