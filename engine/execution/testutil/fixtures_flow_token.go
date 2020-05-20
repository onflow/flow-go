package testutil

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

func CreateDeployFlowTokenContractTransaction(authorizer flow.Address, fungibleTokenAdr flow.Address) flow.TransactionBody {
	return CreateContractDeploymentTransaction(fmt.Sprintf(`
	import FungibleToken from 0x%s
	pub contract FlowToken: FungibleToken {
		pub var totalSupply: UFix64
		pub event FungibleTokenInitialized(initialSupply: UFix64)
		pub event Withdraw(amount: UFix64, from: Address?)
		pub event Deposit(amount: UFix64, to: Address?)
		pub event Mint(amount: UFix64)
		pub event Burn(amount: UFix64)
		pub event MinterCreated(allowedAmount: UFix64)
		pub resource Vault: FungibleToken.Provider, FungibleToken.Receiver, FungibleToken.Balance {
			pub var balance: UFix64
			init(balance: UFix64) {
				self.balance = balance
			}
			pub fun withdraw(amount: UFix64): @FungibleToken.Vault {
				self.balance = self.balance - amount
				emit Withdraw(amount: amount, from: self.owner?.address)
				return <-create Vault(balance: amount)
			}
			pub fun deposit(from: @FungibleToken.Vault) {
				let vault <- from as! @Vault
				self.balance = self.balance + vault.balance
				emit Deposit(amount: vault.balance, to: self.owner?.address)
				vault.balance = 0.0
				destroy vault
			}
			destroy() {
				FlowToken.totalSupply = FlowToken.totalSupply - self.balance
			}
		}
		pub fun createEmptyVault(): @FungibleToken.Vault {
			return <-create Vault(balance: 0.0)
		}
		pub resource MintAndBurn {
			pub var allowedAmount: UFix64
			pub fun mintTokens(amount: UFix64): @FlowToken.Vault {
				pre {
					amount > UFix64(0): "Amount minted must be greater than zero"
					amount <= self.allowedAmount: "Amount minted must be less than the allowed amount"
				}
				FlowToken.totalSupply = FlowToken.totalSupply + amount
				self.allowedAmount = self.allowedAmount - amount
				emit Mint(amount: amount)
				return <-create Vault(balance: amount)
			}
			pub fun burnTokens(from: @Vault) {
				let vault <- from as! @FlowToken.Vault
				let amount = vault.balance
				destroy vault
				emit Burn(amount: amount)
			}
			pub fun createNewMinter(allowedAmount: UFix64): @MintAndBurn {
				emit MinterCreated(allowedAmount: allowedAmount)
				return <-create MintAndBurn(allowedAmount: allowedAmount)
			}
			init(allowedAmount: UFix64) {
				self.allowedAmount = allowedAmount
			}
		}
		init() {
			self.totalSupply = 1000.0
			let vault <- create Vault(balance: self.totalSupply)
			self.account.save(<-vault, to: /storage/flowTokenVault)
			self.account.link<&{FungibleToken.Receiver}>(
				/public/flowTokenReceiver,
				target: /storage/flowTokenVault
			)
			self.account.link<&{FungibleToken.Balance}>(
				/public/flowTokenBalance,
				target: /storage/flowTokenVault
			)
			let mintAndBurn <- create MintAndBurn(allowedAmount: 100.0)
			self.account.save(<-mintAndBurn, to: /storage/flowTokenMintAndBurn)
			emit FungibleTokenInitialized(initialSupply: self.totalSupply)
		}
	}`, fungibleTokenAdr.Hex()), authorizer)
}
