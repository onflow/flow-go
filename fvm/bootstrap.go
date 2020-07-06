package fvm

import (
	"encoding/hex"
	"fmt"

	"github.com/dapperlabs/flow-core-contracts/contracts"
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/dapperlabs/flow-go/fvm/state"
	"github.com/dapperlabs/flow-go/model/flow"
)

// A BootstrapProcedure is an invokable that can be used to bootstrap the ledger state
// with the default accounts and contracts required by the Flow virtual machine.
type BootstrapProcedure struct {
	vm       *VirtualMachine
	ctx      Context
	ledger   state.Ledger
	accounts *state.Accounts

	// genesis parameters
	serviceAccountPublicKey flow.AccountPublicKey
	initialTokenSupply      uint64
}

// Bootstrap returns a new BootstrapProcedure instance configured with the provided
// genesis parameters.
func Bootstrap(
	servicePublicKey flow.AccountPublicKey,
	initialTokenSupply uint64,
) *BootstrapProcedure {
	return &BootstrapProcedure{
		serviceAccountPublicKey: servicePublicKey,
		initialTokenSupply:      initialTokenSupply,
	}
}

func (b *BootstrapProcedure) Run(vm *VirtualMachine, ctx Context, ledger state.Ledger) error {
	b.vm = vm
	b.ctx = NewContextFromParent(ctx, WithRestrictedDeployment(false))
	b.ledger = ledger

	// initialize the account addressing state
	addresses := state.NewAddresses(ledger, ctx.Chain)
	addresses.InitGeneratorState()

	b.accounts = state.NewAccounts(ledger, addresses)

	service := b.createServiceAccount(b.serviceAccountPublicKey)

	fungibleToken := b.deployFungibleToken()
	flowToken := b.deployFlowToken(service, fungibleToken)
	feeContract := b.deployFlowFees(service, fungibleToken, flowToken)

	if b.initialTokenSupply > 0 {
		b.mintInitialTokens(service, fungibleToken, flowToken, b.initialTokenSupply)
	}

	b.deployServiceAccount(service, fungibleToken, flowToken, feeContract)

	return nil
}

func (b *BootstrapProcedure) createAccount() flow.Address {
	address, err := b.accounts.Create(nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create account: %s", err))
	}

	return address
}

func (b *BootstrapProcedure) createServiceAccount(accountKey flow.AccountPublicKey) flow.Address {
	address, err := b.accounts.Create([]flow.AccountPublicKey{accountKey})
	if err != nil {
		panic(fmt.Sprintf("failed to create service account: %s", err))
	}

	return address
}

func (b *BootstrapProcedure) deployFungibleToken() flow.Address {
	fungibleToken := b.createAccount()

	err := b.vm.invokeMetaTransaction(
		b.ctx,
		deployContractTransaction(fungibleToken, contracts.FungibleToken()),
		b.ledger,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to deploy fungible token contract: %s", err.Error()))
	}

	return fungibleToken
}

func (b *BootstrapProcedure) deployFlowToken(service, fungibleToken flow.Address) flow.Address {
	flowToken := b.createAccount()

	contract := contracts.FlowToken(fungibleToken.Hex())

	err := b.vm.invokeMetaTransaction(
		b.ctx,
		deployFlowTokenTransaction(flowToken, service, contract),
		b.ledger,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to deploy Flow token contract: %s", err.Error()))
	}

	return flowToken
}

func (b *BootstrapProcedure) deployFlowFees(service, fungibleToken, flowToken flow.Address) flow.Address {
	flowFees := b.createAccount()

	contract := contracts.FlowFees(fungibleToken.Hex(), flowToken.Hex())

	err := b.vm.invokeMetaTransaction(
		b.ctx,
		deployFlowFeesTransaction(flowFees, service, contract),
		b.ledger,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to deploy fees contract: %s", err.Error()))
	}

	return flowFees
}

func (b *BootstrapProcedure) deployServiceAccount(service, fungibleToken, flowToken, feeContract flow.Address) {
	contract := contracts.FlowServiceAccount(fungibleToken.Hex(), flowToken.Hex(), feeContract.Hex())

	err := b.vm.invokeMetaTransaction(
		b.ctx,
		deployContractTransaction(service, contract),
		b.ledger,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to deploy service account contract: %s", err.Error()))
	}
}

func (b *BootstrapProcedure) mintInitialTokens(service, fungibleToken, flowToken flow.Address, initialSupply uint64) {
	err := b.vm.invokeMetaTransaction(
		b.ctx,
		mintFlowTokenTransaction(fungibleToken, flowToken, service, initialSupply),
		b.ledger,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to mint initial token supply: %s", err.Error()))
	}
}

const deployContractTransactionTemplate = `
transaction {
  prepare(signer: AuthAccount) {
    signer.setCode("%s".decodeHex())
  }
}
`

const deployFlowTokenTransactionTemplate = `
transaction {
  prepare(flowTokenAccount: AuthAccount, serviceAccount: AuthAccount) {
    let adminAccount = serviceAccount
    flowTokenAccount.setCode("%s".decodeHex(), adminAccount)
  }
}
`

const deployFlowFeesTransactionTemplate = `
transaction {
  prepare(flowFeesAccount: AuthAccount, serviceAccount: AuthAccount) {
    let adminAccount = serviceAccount
    flowFeesAccount.setCode("%s".decodeHex(), adminAccount)
  }
}
`

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
	  .getCapability(/public/flowTokenReceiver)!
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

func deployContractTransaction(address flow.Address, contract []byte) *TransactionProcedure {
	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(deployContractTransactionTemplate, hex.EncodeToString(contract)))).
			AddAuthorizer(address),
	)
}

func deployFlowTokenTransaction(flowToken, service flow.Address, contract []byte) *TransactionProcedure {
	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(deployFlowTokenTransactionTemplate, hex.EncodeToString(contract)))).
			AddAuthorizer(flowToken).
			AddAuthorizer(service),
	)
}

func deployFlowFeesTransaction(flowFees, service flow.Address, contract []byte) *TransactionProcedure {
	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(deployFlowFeesTransactionTemplate, hex.EncodeToString(contract)))).
			AddAuthorizer(flowFees).
			AddAuthorizer(service),
	)
}

func mintFlowTokenTransaction(
	fungibleToken, flowToken, service flow.Address,
	initialSupply uint64,
) *TransactionProcedure {
	initialSupplyArg, err := jsoncdc.Encode(cadence.NewUFix64(initialSupply))
	if err != nil {
		panic(fmt.Sprintf("failed to encode initial token supply: %s", err.Error()))
	}

	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(mintFlowTokenTransactionTemplate, fungibleToken, flowToken))).
			AddArgument(initialSupplyArg).
			AddAuthorizer(service),
	)
}

const (
	fungibleTokenAccountIndex = 2
	flowTokenAccountIndex     = 3
)

func FungibleTokenAddress(chain flow.Chain) flow.Address {
	address, _ := chain.AddressAtIndex(fungibleTokenAccountIndex)
	return address
}

func FlowTokenAddress(chain flow.Chain) flow.Address {
	address, _ := chain.AddressAtIndex(flowTokenAccountIndex)
	return address
}
