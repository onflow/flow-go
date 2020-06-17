package fvm

import (
	"encoding/hex"
	"fmt"

	"github.com/dapperlabs/flow-core-contracts/contracts"
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/dapperlabs/flow-go/model/flow"
)

type bootstrap struct {
	metaCtx                 Context
	ledger                  Ledger
	serviceAccountPublicKey flow.AccountPublicKey
	initialTokenSupply      uint64
}

func Bootstrap(
	servicePublicKey flow.AccountPublicKey,
	initialTokenSupply uint64,
) Invokable {
	return &bootstrap{
		serviceAccountPublicKey: servicePublicKey,
		initialTokenSupply:      initialTokenSupply,
	}
}

func (b *bootstrap) Parse(ctx Context, ledger Ledger) (Invokable, error) {
	// Bootstrapping invocation does not support pre-parsing
	return b, nil
}

func (b *bootstrap) Invoke(ctx Context, ledger Ledger) (*InvocationResult, error) {
	b.metaCtx = ctx.NewChild(
		WithSignatureVerification(false),
		WithFeePayments(false),
		WithRestrictedDeployment(false),
	)

	b.ledger = ledger

	// initialize the account addressing state
	setAddressState(ledger, flow.NewAddressGenerator())

	service := b.createServiceAccount(b.serviceAccountPublicKey)

	fungibleToken := b.deployFungibleToken()
	flowToken := b.deployFlowToken(service, fungibleToken)
	feeContract := b.deployFlowFees(service, fungibleToken, flowToken)

	if b.initialTokenSupply > 0 {
		b.mintInitialTokens(service, fungibleToken, flowToken, b.initialTokenSupply)
	}

	b.deployServiceAccount(service, fungibleToken, flowToken, feeContract)

	return nil, nil
}

func (b *bootstrap) createAccount() flow.Address {
	address, err := createAccount(b.ledger, nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create account: %s", err))
	}

	return address
}

func (b *bootstrap) createServiceAccount(accountKey flow.AccountPublicKey) flow.Address {
	address, err := createAccount(b.ledger, []flow.AccountPublicKey{accountKey})
	if err != nil {
		panic(fmt.Sprintf("failed to create service account: %s", err))
	}

	return address
}

func (b *bootstrap) deployFungibleToken() flow.Address {
	fungibleToken := b.createAccount()

	result := b.mustInvoke(deployContractTransaction(fungibleToken, contracts.FungibleToken()))
	if result.Error != nil {
		panic(fmt.Sprintf("failed to deploy fungible token contract: %s", result.Error.ErrorMessage()))
	}

	return fungibleToken
}

func (b *bootstrap) deployFlowToken(service, fungibleToken flow.Address) flow.Address {
	flowToken := b.createAccount()

	contract := contracts.FlowToken(fungibleToken.Hex())

	result := b.mustInvoke(deployFlowTokenTransaction(flowToken, service, contract))
	if result.Error != nil {
		panic(fmt.Sprintf("failed to deploy Flow token contract: %s", result.Error.ErrorMessage()))
	}

	return flowToken
}

func (b *bootstrap) deployFlowFees(service, fungibleToken, flowToken flow.Address) flow.Address {
	flowFees := b.createAccount()

	contract := contracts.FlowFees(fungibleToken.Hex(), flowToken.Hex())

	result := b.mustInvoke(deployFlowFeesTransaction(flowFees, service, contract))
	if result.Error != nil {
		panic(fmt.Sprintf("failed to deploy fees contract: %s", result.Error.ErrorMessage()))
	}

	return flowFees
}

func (b *bootstrap) deployServiceAccount(service, fungibleToken, flowToken, feeContract flow.Address) {
	contract := contracts.FlowServiceAccount(fungibleToken.Hex(), flowToken.Hex(), feeContract.Hex())

	result := b.mustInvoke(deployContractTransaction(service, contract))
	if result.Error != nil {
		panic(fmt.Sprintf("failed to deploy service account contract: %s", result.Error.ErrorMessage()))
	}
}

func (b *bootstrap) mintInitialTokens(service, fungibleToken, flowToken flow.Address, initialSupply uint64) {
	result := b.mustInvoke(mintFlowTokenTransaction(fungibleToken, flowToken, service, initialSupply))
	if result.Error != nil {
		panic(fmt.Sprintf("failed to mint initial token supply: %s", result.Error.ErrorMessage()))
	}
}

func (b *bootstrap) deployContractToServiceAccount(service flow.Address, contract []byte) {
	result := b.mustInvoke(deployContractTransaction(service, contract))
	if result.Error != nil {
		panic(fmt.Sprintf("failed to deploy service account contract: %s", result.Error.ErrorMessage()))
	}
}

func (b *bootstrap) mustInvoke(i Invokable) *InvocationResult {
	result, err := b.metaCtx.Invoke(i, b.ledger)
	if err != nil {
		panic(err)
	}

	if result.Error != nil {
		panic(result.Error.ErrorMessage())
	}

	return result
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

func deployContractTransaction(addressess flow.Address, contract []byte) Invokable {
	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(deployContractTransactionTemplate, hex.EncodeToString(contract)))).
			AddAuthorizer(addressess),
	)
}

func deployFlowTokenTransaction(flowToken, service flow.Address, contract []byte) Invokable {
	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(deployFlowTokenTransactionTemplate, hex.EncodeToString(contract)))).
			AddAuthorizer(flowToken).
			AddAuthorizer(service),
	)
}

func deployFlowFeesTransaction(flowFees, service flow.Address, contract []byte) Invokable {
	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(deployFlowFeesTransactionTemplate, hex.EncodeToString(contract)))).
			AddAuthorizer(flowFees).
			AddAuthorizer(service),
	)
}

func mintFlowTokenTransaction(fungibleToken, flowToken, service flow.Address, initialSupply uint64) Invokable {
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

func FungibleTokenAddress() flow.Address {
	addressess, _ := flow.AddressAtIndex(2)
	return addressess
}

func FlowTokenAddress() flow.Address {
	addressess, _ := flow.AddressAtIndex(3)
	return addressess
}
