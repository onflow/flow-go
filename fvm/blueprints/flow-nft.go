package blueprints

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-nft/lib/go/contracts"
)

func DeployNonFungibleTokenTransaction(nonFungibleToken flow.Address) *flow.TransactionBody {
	contract := contracts.NonFungibleToken()
	contractName := "NonFungibleToken"
	return DeployContractTransaction(
		nonFungibleToken,
		contract,
		contractName)
}

const deployExampleNFTTransactionTemplate = `
transaction {
  prepare(exampleNFT: AuthAccount) {
    exampleNFT.contracts.add(name: "ExampleNFT", code: "%s".decodeHex())
  }
}
`

func DeployExampleNFTTransaction(nonFungibleToken, exampleNFT flow.Address) *flow.TransactionBody {
	contract := contracts.ExampleNFT(nonFungibleToken.Hex())

	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(deployExampleNFTTransactionTemplate, hex.EncodeToString(contract)))).
		AddAuthorizer(exampleNFT)
}
