package blueprints

import (
	_ "embed"
	"encoding/hex"
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-core-contracts/lib/go/contracts"

	"github.com/onflow/flow-go/model/flow"
)

func DeployFungibleTokenContractTransaction(fungibleToken flow.Address) *flow.TransactionBody {
	contract := contracts.FungibleToken()
	contractName := "FungibleToken"
	return DeployContractTransaction(
		fungibleToken,
		contract,
		contractName)
}

//go:embed scripts/deployFlowTokenTransactionTemplate.cdc
var deployFlowTokenTransactionTemplate string

//go:embed scripts/createFlowTokenMinterTransactionTemplate.cdc
var createFlowTokenMinterTransactionTemplate string

//go:embed scripts/mintFlowTokenTransactionTemplate.cdc
var mintFlowTokenTransactionTemplate string

func DeployFlowTokenContractTransaction(service, fungibleToken, flowToken flow.Address) *flow.TransactionBody {
	contract := contracts.FlowToken(fungibleToken.HexWithPrefix())

	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(deployFlowTokenTransactionTemplate, hex.EncodeToString(contract)))).
		AddAuthorizer(flowToken).
		AddAuthorizer(service)
}

// CreateFlowTokenMinterTransaction returns a transaction which creates a Flow
// token Minter resource and stores it in the service account. This Minter is
// expected to be stored here by the epoch smart contracts.
func CreateFlowTokenMinterTransaction(service, flowToken flow.Address) *flow.TransactionBody {

	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(createFlowTokenMinterTransactionTemplate, flowToken.HexWithPrefix()))).
		AddAuthorizer(service)
}

func MintFlowTokenTransaction(
	fungibleToken, flowToken, service flow.Address,
	initialSupply cadence.UFix64,
) *flow.TransactionBody {
	initialSupplyArg, err := jsoncdc.Encode(initialSupply)
	if err != nil {
		panic(fmt.Sprintf("failed to encode initial token supply: %s", err.Error()))
	}

	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(mintFlowTokenTransactionTemplate, fungibleToken, flowToken))).
		AddArgument(initialSupplyArg).
		AddAuthorizer(service)
}
