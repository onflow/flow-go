package runnables

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-core-contracts/lib/go/contracts"

	"github.com/onflow/flow-go/fvm/context"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

// A BootstrapProcedure is an invokable that can be used to bootstrap the ledger state
// with the default accounts and contracts required by the Flow virtual machine.
type BootstrapProcedure struct {
	vm       context.VirtualMachine
	ctx      context.Context
	sth      *state.StateHolder
	programs *programs.Programs
	accounts *state.Accounts

	// genesis parameters
	serviceAccountPublicKey flow.AccountPublicKey
	initialTokenSupply      cadence.UFix64
	addressGenerator        flow.AddressGenerator

	accountCreationFee        cadence.UFix64
	transactionFee            cadence.UFix64
	minimumStorageReservation cadence.UFix64
	storagePerFlow            cadence.UFix64
}

type BootstrapProcedureOption func(*BootstrapProcedure) *BootstrapProcedure

func WithInitialTokenSupply(supply cadence.UFix64) BootstrapProcedureOption {
	return func(bp *BootstrapProcedure) *BootstrapProcedure {
		bp.initialTokenSupply = supply
		return bp
	}
}

var DefaultAccountCreationFee = func() cadence.UFix64 {
	value, err := cadence.NewUFix64("0.00100000")
	if err != nil {
		panic(fmt.Errorf("invalid default account creation fee: %w", err))
	}
	return value
}()

var DefaultMinimumStorageReservation = func() cadence.UFix64 {
	value, err := cadence.NewUFix64("0.00100000")
	if err != nil {
		panic(fmt.Errorf("invalid default minimum storage reservation: %w", err))
	}
	return value
}()

var DefaultStorageMBPerFLOW = func() cadence.UFix64 {
	value, err := cadence.NewUFix64("10.00000000")
	if err != nil {
		panic(fmt.Errorf("invalid default minimum storage reservation: %w", err))
	}
	return value
}()

// DefaultTransactionFees are the default transaction fees if transaction fees are on.
// If they are off (which is the default behaviour) that means the transaction fees are 0.0.
var DefaultTransactionFees = func() cadence.UFix64 {
	value, err := cadence.NewUFix64("0.0001")
	if err != nil {
		panic(fmt.Errorf("invalid default transaction fees: %w", err))
	}
	return value
}()

func WithAccountCreationFee(fee cadence.UFix64) BootstrapProcedureOption {
	return func(bp *BootstrapProcedure) *BootstrapProcedure {
		bp.accountCreationFee = fee
		return bp
	}
}

func WithTransactionFee(fee cadence.UFix64) BootstrapProcedureOption {
	return func(bp *BootstrapProcedure) *BootstrapProcedure {
		bp.transactionFee = fee
		return bp
	}
}

func WithMinimumStorageReservation(reservation cadence.UFix64) BootstrapProcedureOption {
	return func(bp *BootstrapProcedure) *BootstrapProcedure {
		bp.minimumStorageReservation = reservation
		return bp
	}
}

func WithStorageMBPerFLOW(ratio cadence.UFix64) BootstrapProcedureOption {
	return func(bp *BootstrapProcedure) *BootstrapProcedure {
		bp.storagePerFlow = ratio
		return bp
	}
}

// Bootstrap returns a new BootstrapProcedure instance configured with the provided
// genesis parameters.
func Bootstrap(
	serviceAccountPublicKey flow.AccountPublicKey,
	opts ...BootstrapProcedureOption,
) *BootstrapProcedure {
	bootstrapProcedure := &BootstrapProcedure{
		serviceAccountPublicKey: serviceAccountPublicKey,
		transactionFee:          0,
	}

	for _, applyOption := range opts {
		bootstrapProcedure = applyOption(bootstrapProcedure)
	}
	return bootstrapProcedure
}

func (b *BootstrapProcedure) Run(vm context.VirtualMachine, ctx context.Context, sth *state.StateHolder, programs *programs.Programs) error {
	b.vm = vm
	b.ctx = context.NewContextFromParent(ctx, context.WithRestrictedDeployment(false))
	b.sth = sth
	b.programs = programs

	// initialize the account addressing state
	b.accounts = state.NewAccounts(b.sth)
	addressGenerator := state.NewStateBoundAddressGenerator(b.sth, ctx.Chain)
	b.addressGenerator = addressGenerator

	service := b.createServiceAccount(b.serviceAccountPublicKey)

	fungibleToken := b.deployFungibleToken()
	flowToken := b.deployFlowToken(service, fungibleToken)
	feeContract := b.deployFlowFees(service, fungibleToken, flowToken)
	b.deployStorageFees(service, fungibleToken, flowToken)

	if b.initialTokenSupply > 0 {
		b.mintInitialTokens(service, fungibleToken, flowToken, b.initialTokenSupply)
	}
	b.deployServiceAccount(service, fungibleToken, flowToken, feeContract)

	b.setupFees(service, b.transactionFee, b.accountCreationFee, b.minimumStorageReservation, b.storagePerFlow)

	b.setupStorageForServiceAccounts(service, fungibleToken, flowToken, feeContract)
	return nil
}

func (b *BootstrapProcedure) createAccount() flow.Address {
	address, err := b.addressGenerator.NextAddress()
	if err != nil {
		panic(fmt.Sprintf("failed to generate address: %s", err))
	}

	err = b.accounts.Create(nil, address)
	if err != nil {
		panic(fmt.Sprintf("failed to create account: %s", err))
	}

	return address
}

func (b *BootstrapProcedure) createServiceAccount(accountKey flow.AccountPublicKey) flow.Address {
	address, err := b.addressGenerator.NextAddress()
	if err != nil {
		panic(fmt.Sprintf("failed to generate address: %s", err))
	}

	err = b.accounts.Create([]flow.AccountPublicKey{accountKey}, address)
	if err != nil {
		panic(fmt.Sprintf("failed to create service account: %s", err))
	}

	return address
}

func (b *BootstrapProcedure) deployFungibleToken() flow.Address {
	fungibleToken := b.createAccount()

	txError, err := b.vm.InvokeMetaTransaction(
		b.ctx,
		deployContractTransaction(fungibleToken, contracts.FungibleToken(), "FungibleToken"),
		b.sth,
		b.programs,
	)
	panicOnMetaInvokeErrf("failed to deploy fungible token contract: %s", txError, err)
	return fungibleToken
}

func (b *BootstrapProcedure) deployFlowToken(service, fungibleToken flow.Address) flow.Address {
	flowToken := b.createAccount()

	contract := contracts.FlowToken(fungibleToken.HexWithPrefix())

	txError, err := b.vm.InvokeMetaTransaction(
		b.ctx,
		deployFlowTokenTransaction(flowToken, service, contract),
		b.sth,
		b.programs,
	)
	panicOnMetaInvokeErrf("failed to deploy Flow token contract: %s", txError, err)
	return flowToken
}

func (b *BootstrapProcedure) deployFlowFees(service, fungibleToken, flowToken flow.Address) flow.Address {
	flowFees := b.createAccount()

	contract := contracts.FlowFees(
		fungibleToken.HexWithPrefix(),
		flowToken.HexWithPrefix(),
	)

	txError, err := b.vm.InvokeMetaTransaction(
		b.ctx,
		deployFlowFeesTransaction(flowFees, service, contract),
		b.sth,
		b.programs,
	)
	panicOnMetaInvokeErrf("failed to deploy fees contract: %s", txError, err)
	return flowFees
}

func (b *BootstrapProcedure) deployStorageFees(service, fungibleToken, flowToken flow.Address) {
	contract := contracts.FlowStorageFees(
		fungibleToken.HexWithPrefix(),
		flowToken.HexWithPrefix(),
	)

	// deploy storage fees contract on the service account
	txError, err := b.vm.InvokeMetaTransaction(
		b.ctx,
		deployStorageFeesTransaction(service, contract),
		b.sth,
		b.programs,
	)
	panicOnMetaInvokeErrf("failed to deploy storage fees contract: %s", txError, err)
}

func (b *BootstrapProcedure) deployServiceAccount(service, fungibleToken, flowToken, feeContract flow.Address) {
	contract := contracts.FlowServiceAccount(
		fungibleToken.HexWithPrefix(),
		flowToken.HexWithPrefix(),
		feeContract.HexWithPrefix(),
		service.HexWithPrefix(),
	)

	txError, err := b.vm.InvokeMetaTransaction(
		b.ctx,
		deployContractTransaction(service, contract, "FlowServiceAccount"),
		b.sth,
		b.programs,
	)
	panicOnMetaInvokeErrf("failed to deploy service account contract: %s", txError, err)
}

func (b *BootstrapProcedure) mintInitialTokens(
	service, fungibleToken, flowToken flow.Address,
	initialSupply cadence.UFix64,
) {
	txError, err := b.vm.InvokeMetaTransaction(
		b.ctx,
		mintFlowTokenTransaction(fungibleToken, flowToken, service, initialSupply),
		b.sth,
		b.programs,
	)
	panicOnMetaInvokeErrf("failed to mint initial token supply: %s", txError, err)
}

func (b *BootstrapProcedure) setupFees(
	service flow.Address,
	transactionFee,
	addressCreationFee,
	minimumStorageReservation,
	storagePerFlow cadence.UFix64,
) {
	txError, err := b.vm.InvokeMetaTransaction(
		b.ctx,
		setupFeesTransaction(service, transactionFee, addressCreationFee, minimumStorageReservation, storagePerFlow),
		b.sth,
		b.programs,
	)
	panicOnMetaInvokeErrf("failed to setup fees: %s", txError, err)
}

func (b *BootstrapProcedure) setupStorageForServiceAccounts(
	service, fungibleToken, flowToken, feeContract flow.Address,
) {
	txError, err := b.vm.InvokeMetaTransaction(
		b.ctx,
		setupStorageForServiceAccountsTransaction(service, fungibleToken, flowToken, feeContract),
		b.sth,
		b.programs,
	)
	panicOnMetaInvokeErrf("failed to setup storage for service accounts: %s", txError, err)
}

const deployContractTransactionTemplate = `
transaction {
  prepare(signer: AuthAccount) {
    signer.contracts.add(name: "%s", code: "%s".decodeHex())
  }
}
`

const deployFlowTokenTransactionTemplate = `
transaction {
  prepare(flowTokenAccount: AuthAccount, serviceAccount: AuthAccount) {
    let adminAccount = serviceAccount
    flowTokenAccount.contracts.add(name: "FlowToken", code: "%s".decodeHex(), adminAccount: adminAccount)
  }
}
`

const deployFlowFeesTransactionTemplate = `
transaction {
  prepare(flowFeesAccount: AuthAccount, serviceAccount: AuthAccount) {
    let adminAccount = serviceAccount
    flowFeesAccount.contracts.add(name: "FlowFees", code: "%s".decodeHex(), adminAccount: adminAccount)
  }
}
`

const deployStorageFeesTransactionTemplate = `
transaction {
  prepare(serviceAccount: AuthAccount) {
    serviceAccount.contracts.add(name: "FlowStorageFees", code: "%s".decodeHex())
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

const setupFeesTransactionTemplate = `
import FlowStorageFees, FlowServiceAccount from 0x%s

transaction(transactionFee: UFix64, accountCreationFee: UFix64, minimumStorageReservation: UFix64, storageMegaBytesPerReservedFLOW: UFix64) {
    prepare(service: AuthAccount) {
        let serviceAdmin = service.borrow<&FlowServiceAccount.Administrator>(from: /storage/flowServiceAdmin)
            ?? panic("Could not borrow reference to the flow service admin!");

        let storageAdmin = service.borrow<&FlowStorageFees.Administrator>(from: /storage/storageFeesAdmin)
            ?? panic("Could not borrow reference to the flow storage fees admin!");

        serviceAdmin.setTransactionFee(transactionFee)
        serviceAdmin.setAccountCreationFee(accountCreationFee)
        storageAdmin.setMinimumStorageReservation(minimumStorageReservation)
        storageAdmin.setStorageMegaBytesPerReservedFLOW(storageMegaBytesPerReservedFLOW)
    }
}
`

const setupStorageForServiceAccountsTemplate = `
import FlowServiceAccount from 0x%s
import FlowStorageFees from 0x%s
import FungibleToken from 0x%s
import FlowToken from 0x%s

// This transaction sets up storage on any auth accounts that were created before the storage fees.
// This is used during bootstrapping a local environment 
transaction() {
    prepare(service: AuthAccount, fungibleToken: AuthAccount, flowToken: AuthAccount, feeContract: AuthAccount) {
        let authAccounts = [service, fungibleToken, flowToken, feeContract]

        // take all the funds from the service account
        let tokenVault = service.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)
            ?? panic("Unable to borrow reference to the default token vault")
        
        for account in authAccounts {
            let storageReservation <- tokenVault.withdraw(amount: FlowStorageFees.minimumStorageReservation) as! @FlowToken.Vault
            let hasReceiver = account.getCapability(/public/flowTokenReceiver)!.check<&{FungibleToken.Receiver}>()
            if !hasReceiver {
                FlowServiceAccount.initDefaultToken(account)
            }
            let receiver = account.getCapability(/public/flowTokenReceiver)!.borrow<&{FungibleToken.Receiver}>()
                ?? panic("Could not borrow receiver reference to the recipient's Vault")

            receiver.deposit(from: <-storageReservation)
        }
    }
}
`

func deployContractTransaction(address flow.Address, contract []byte, contractName string) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(deployContractTransactionTemplate, contractName, hex.EncodeToString(contract)))).
		AddAuthorizer(address)
}

func deployFlowTokenTransaction(flowToken, service flow.Address, contract []byte) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(deployFlowTokenTransactionTemplate, hex.EncodeToString(contract)))).
		AddAuthorizer(flowToken).
		AddAuthorizer(service)
}

func deployFlowFeesTransaction(flowFees, service flow.Address, contract []byte) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(deployFlowFeesTransactionTemplate, hex.EncodeToString(contract)))).
		AddAuthorizer(flowFees).
		AddAuthorizer(service)
}

func deployStorageFeesTransaction(service flow.Address, contract []byte) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(deployStorageFeesTransactionTemplate, hex.EncodeToString(contract)))).
		AddAuthorizer(service)
}

func mintFlowTokenTransaction(
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

func setupFeesTransaction(
	service flow.Address,
	transactionFee,
	addressCreationFee,
	minimumStorageReservation,
	storagePerFlow cadence.UFix64,
) *flow.TransactionBody {
	transactionFeeArg, err := jsoncdc.Encode(transactionFee)
	if err != nil {
		panic(fmt.Sprintf("failed to encode transaction fee: %s", err.Error()))
	}
	addressCreationFeeArg, err := jsoncdc.Encode(addressCreationFee)
	if err != nil {
		panic(fmt.Sprintf("failed to encode address creation fee: %s", err.Error()))
	}
	minimumStorageReservationArg, err := jsoncdc.Encode(minimumStorageReservation)
	if err != nil {
		panic(fmt.Sprintf("failed to encode minimum storage reservation: %s", err.Error()))
	}
	storagePerFlowArg, err := jsoncdc.Encode(storagePerFlow)
	if err != nil {
		panic(fmt.Sprintf("failed to encode storage ratio: %s", err.Error()))
	}

	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(setupFeesTransactionTemplate, service))).
		AddArgument(transactionFeeArg).
		AddArgument(addressCreationFeeArg).
		AddArgument(minimumStorageReservationArg).
		AddArgument(storagePerFlowArg).
		AddAuthorizer(service)
}

func setupStorageForServiceAccountsTransaction(
	service, fungibleToken, flowToken, feeContract flow.Address,
) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(setupStorageForServiceAccountsTemplate, service, service, fungibleToken, flowToken))).
		AddAuthorizer(service).
		AddAuthorizer(fungibleToken).
		AddAuthorizer(flowToken).
		AddAuthorizer(feeContract)
}

const (
	fungibleTokenAccountIndex = 2
	flowTokenAccountIndex     = 3
	flowFeesAccountIndex      = 4
)

func panicOnMetaInvokeErrf(msg string, txError errors.Error, err error) {
	if txError != nil {
		panic(fmt.Sprintf(msg, txError.Error()))
	}
	if err != nil {
		panic(fmt.Sprintf(msg, err.Error()))
	}
}

func FungibleTokenAddress(chain flow.Chain) flow.Address {
	address, _ := chain.AddressAtIndex(fungibleTokenAccountIndex)
	return address
}

func FlowTokenAddress(chain flow.Chain) flow.Address {
	address, _ := chain.AddressAtIndex(flowTokenAccountIndex)
	return address
}

func FlowFeesAddress(chain flow.Chain) flow.Address {
	address, _ := chain.AddressAtIndex(flowFeesAccountIndex)
	return address
}
