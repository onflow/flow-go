package testutil

import (
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

func CreateTokenTransferTransaction(chain flow.Chain, amount int, to flow.Address, signer flow.Address) *flow.TransactionBody {
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(`
		import FungibleToken from 0x%s
		import FlowToken from 0x%s

		transaction(amount: UFix64, to: Address) {
			let sentVault: @{FungibleToken.Vault}

			prepare(signer: auth(BorrowValue) &Account) {
				let vaultRef = signer.storage.borrow<auth(FungibleToken.Withdrawable) &FlowToken.Vault>(from: /storage/flowTokenVault)
					?? panic("Could not borrow reference to the owner's Vault!")
				self.sentVault <- vaultRef.withdraw(amount: amount)
			}

			execute {
				let receiverRef = getAccount(to)
					.capabilities.borrow<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)
					?? panic("Could not borrow receiver reference to the recipient's Vault")
				receiverRef.deposit(from: <-self.sentVault)
			}
		}`, sc.FungibleToken.Address.Hex(), sc.FlowToken.Address.Hex()))).
		AddArgument(jsoncdc.MustEncode(cadence.UFix64(amount))).
		AddArgument(jsoncdc.MustEncode(cadence.NewAddress(to))).
		AddAuthorizer(signer)
}
