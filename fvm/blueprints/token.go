package blueprints

import (
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

const deployFlowTokenTransactionTemplate = `
transaction {
  prepare(flowTokenAccount: AuthAccount, serviceAccount: AuthAccount) {
    let adminAccount = serviceAccount
    flowTokenAccount.contracts.add(name: "FlowToken", code: "%s".decodeHex(), adminAccount: adminAccount)
  }
}
`

func DeployFlowTokenContractTransaction(service, fungibleToken, flowToken flow.Address) *flow.TransactionBody {
	contract := contracts.FlowToken(fungibleToken.HexWithPrefix())

	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(deployFlowTokenTransactionTemplate, hex.EncodeToString(contract)))).
		AddAuthorizer(flowToken).
		AddAuthorizer(service)
}

const createFlowTokenMinterTransactionTemplate = `
import FlowToken from %s

transaction {
	prepare(serviceAccount: AuthAccount) {
    /// Borrow a reference to the Flow Token Admin in the account storage
    let flowTokenAdmin = serviceAccount.borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin)
        ?? panic("Could not borrow a reference to the Flow Token Admin resource")

    /// Create a flowTokenMinterResource
    let flowTokenMinter <- flowTokenAdmin.createNewMinter(allowedAmount: 1000000000.0)

    serviceAccount.save(<-flowTokenMinter, to: /storage/flowTokenMinter)
	}
}
`

// CreateFlowTokenMinterTransaction returns a transaction which creates a Flow
// token Minter resource and stores it in the service account. This Minter is
// expected to be stored here by the epoch smart contracts.
func CreateFlowTokenMinterTransaction(service, flowToken flow.Address) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(createFlowTokenMinterTransactionTemplate, flowToken.HexWithPrefix()))).
		AddAuthorizer(service)
}

const mintFlowTokenTransactionTemplate = `
import FungibleToken from 0x%s
import FlowToken from 0x%s

transaction(amount: UFix64) {

  let tokenAdmin: &FlowToken.Administrator
  let tokenReceiver: &FlowToken.Vault{FungibleToken.Receiver}

  prepare(signer: AuthAccount) {
	self.tokenAdmin = signer
	  .borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin)
	  ?? panic("Signer is not the token admin")

	self.tokenReceiver = signer
	  .getCapability(/public/flowTokenReceiver)
	  .borrow<&FlowToken.Vault{FungibleToken.Receiver}>()
	  ?? panic("Unable to borrow receiver reference for recipient")
  }

  execute {
	let minter <- self.tokenAdmin.createNewMinter(allowedAmount: amount)
	let mintedVault <- minter.mintTokens(amount: amount)

	self.tokenReceiver.deposit(from: <-mintedVault)

	destroy minter
  }
}
`

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
