package fvm

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-core-contracts/lib/go/contracts"
	"github.com/onflow/flow-core-contracts/lib/go/templates"

	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

// A BootstrapProcedure is an invokable that can be used to bootstrap the ledger state
// with the default accounts and contracts required by the Flow virtual machine.
type BootstrapProcedure struct {
	vm        *VirtualMachine
	ctx       Context
	sth       *state.StateHolder
	programs  *programs.Programs
	accounts  *state.Accounts
	rootBlock *flow.Header

	// genesis parameters
	serviceAccountPublicKey flow.AccountPublicKey
	initialTokenSupply      cadence.UFix64
	addressGenerator        flow.AddressGenerator

	accountCreationFee        cadence.UFix64
	transactionFee            cadence.UFix64
	minimumStorageReservation cadence.UFix64

	epochConfig flow.EpochConfig
}

type BootstrapProcedureOption func(*BootstrapProcedure) *BootstrapProcedure

func WithInitialTokenSupply(supply cadence.UFix64) BootstrapProcedureOption {
	return func(bp *BootstrapProcedure) *BootstrapProcedure {
		bp.initialTokenSupply = supply
		return bp
	}
}

var DefaultAccountCreationFee = func() cadence.UFix64 {
	value, err := cadence.NewUFix64("0.10000000")
	if err != nil {
		panic(fmt.Errorf("invalid default account creation fee: %w", err))
	}
	return value
}()

var DefaultMinimumStorageReservation = func() cadence.UFix64 {
	value, err := cadence.NewUFix64("0.10000000")
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

func WithEpochConfig(epochConfig flow.EpochConfig) BootstrapProcedureOption {
	return func(bp *BootstrapProcedure) *BootstrapProcedure {
		bp.epochConfig = epochConfig
		return bp
	}
}

func WithRootBlock(rootBlock *flow.Header) BootstrapProcedureOption {
	return func(bp *BootstrapProcedure) *BootstrapProcedure {
		bp.rootBlock = rootBlock
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
		epochConfig:             flow.DefaultEpochConfig(),
	}

	for _, applyOption := range opts {
		bootstrapProcedure = applyOption(bootstrapProcedure)
	}
	return bootstrapProcedure
}

func (b *BootstrapProcedure) Run(vm *VirtualMachine, ctx Context, sth *state.StateHolder, programs *programs.Programs) error {
	b.vm = vm
	b.ctx = NewContextFromParent(ctx, WithRestrictedDeployment(false))
	b.rootBlock = flow.Genesis(flow.ChainID(ctx.Chain.String())).Header
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

	b.setupFees(service, b.transactionFee, b.accountCreationFee, b.minimumStorageReservation)

	b.setupStorageForServiceAccounts(service, fungibleToken, flowToken, feeContract)

	b.deployDKG(service)

	b.deployQC(service)

	b.deployIDTableStaking(service,
		fungibleToken,
		flowToken)

	b.deployEpoch(service, fungibleToken, flowToken)

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

	err := b.vm.invokeMetaTransaction(
		b.ctx,
		deployContractTransaction(fungibleToken, contracts.FungibleToken(), "FungibleToken"),
		b.sth,
		b.programs,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to deploy fungible token contract: %s", err.Error()))
	}
	return fungibleToken
}

func (b *BootstrapProcedure) deployFlowToken(service, fungibleToken flow.Address) flow.Address {
	flowToken := b.createAccount()

	contract := contracts.FlowToken(fungibleToken.HexWithPrefix())

	err := b.vm.invokeMetaTransaction(
		b.ctx,
		deployFlowTokenTransaction(flowToken, service, contract),
		b.sth,
		b.programs,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to deploy Flow token contract: %s", err.Error()))
	}
	return flowToken
}

func (b *BootstrapProcedure) deployFlowFees(service, fungibleToken, flowToken flow.Address) flow.Address {
	flowFees := b.createAccount()

	contract := contracts.FlowFees(
		fungibleToken.HexWithPrefix(),
		flowToken.HexWithPrefix(),
	)

	err := b.vm.invokeMetaTransaction(
		b.ctx,
		deployFlowFeesTransaction(flowFees, service, contract),
		b.sth,
		b.programs,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to deploy fees contract: %s", err.Error()))
	}
	return flowFees
}

func (b *BootstrapProcedure) deployStorageFees(service, fungibleToken, flowToken flow.Address) {
	contract := contracts.FlowStorageFees(
		fungibleToken.HexWithPrefix(),
		flowToken.HexWithPrefix(),
	)

	// deploy storage fees contract on the service account
	err := b.vm.invokeMetaTransaction(
		b.ctx,
		deployStorageFeesTransaction(service, contract),
		b.sth,
		b.programs,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to deploy storage fees contract: %s", err.Error()))
	}
}

func (b *BootstrapProcedure) deployDKG(service flow.Address) {
	contract := contracts.FlowDKG()
	err := b.vm.invokeMetaTransaction(
		b.ctx,
		deployContractTransaction(service, contract, "FlowDKG"),
		b.sth,
		b.programs,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to deploy DKG contract: %s", err.Error()))
	}
}

func (b *BootstrapProcedure) deployQC(service flow.Address) {
	contract := contracts.FlowQC()
	err := b.vm.invokeMetaTransaction(
		b.ctx,
		deployContractTransaction(service, contract, "FlowEpochClusterQC"),
		b.sth,
		b.programs,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to deploy QC contract: %s", err.Error()))
	}
}

func (b *BootstrapProcedure) deployIDTableStaking(
	service, fungibleToken,
	flowToken flow.Address) {

	contract := contracts.FlowIDTableStaking(
		fungibleToken.HexWithPrefix(),
		flowToken.HexWithPrefix(),
		true)

	err := b.vm.invokeMetaTransaction(
		b.ctx,
		deployIDTableStakingTransaction(service,
			contract,
			b.epochConfig.EpochTokenPayout,
			b.epochConfig.RewardCut),
		b.sth,
		b.programs,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to deploy IDTableStaking contract: %s", err.Error()))
	}
}

func (b *BootstrapProcedure) deployEpoch(service, fungibleToken, flowToken flow.Address) {

	contract := contracts.FlowEpoch(
		fungibleToken.HexWithPrefix(),
		flowToken.HexWithPrefix(),
		service.HexWithPrefix(),
		service.HexWithPrefix(),
		service.HexWithPrefix(),
	)

	context := NewContextFromParent(b.ctx,
		WithBlockHeader(b.rootBlock),
		WithBlocks(&NoopBlockFinder{}),
	)

	err := b.vm.invokeMetaTransaction(
		context,
		deployEpochTransaction(service, contract, b.epochConfig),
		b.sth,
		b.programs,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to deploy Epoch contract: %s", err.Error()))
	}
}

func (b *BootstrapProcedure) deployServiceAccount(service, fungibleToken, flowToken, feeContract flow.Address) {
	contract := contracts.FlowServiceAccount(
		fungibleToken.HexWithPrefix(),
		flowToken.HexWithPrefix(),
		feeContract.HexWithPrefix(),
		service.HexWithPrefix(),
	)

	err := b.vm.invokeMetaTransaction(
		b.ctx,
		deployContractTransaction(service, contract, "FlowServiceAccount"),
		b.sth,
		b.programs,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to deploy service account contract: %s", err.Error()))
	}
}

func (b *BootstrapProcedure) mintInitialTokens(
	service, fungibleToken, flowToken flow.Address,
	initialSupply cadence.UFix64,
) {
	err := b.vm.invokeMetaTransaction(
		b.ctx,
		mintFlowTokenTransaction(fungibleToken, flowToken, service, initialSupply),
		b.sth,
		b.programs,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to mint initial token supply: %s", err.Error()))
	}
}

func (b *BootstrapProcedure) setupFees(
	service flow.Address,
	transactionFee,
	addressCreationFee,
	minimumStorageReservation cadence.UFix64,
) {
	err := b.vm.invokeMetaTransaction(
		b.ctx,
		setupFeesTransaction(service, transactionFee, addressCreationFee, minimumStorageReservation),
		b.sth,
		b.programs,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to setup fees: %s", err.Error()))
	}
}

func (b *BootstrapProcedure) setupStorageForServiceAccounts(
	service, fungibleToken, flowToken, feeContract flow.Address,
) {
	err := b.vm.invokeMetaTransaction(
		b.ctx,
		setupStorageForServiceAccountsTransaction(service, fungibleToken, flowToken, feeContract),
		b.sth,
		b.programs,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to setup storage for service accounts: %s", err.Error()))
	}
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

const deployIDTableStakingTransactionTemplate = `
transaction {
  prepare(serviceAccount: AuthAccount) {
	serviceAccount.contracts.add(name: "FlowIDTableStaking", code: "%s".decodeHex(), epochTokenPayout: UFix64(%d), rewardCut: UFix64(%d))
  }
}
`

const deployEpochTransactionTemplate = `
transaction {
  prepare(serviceAccount: AuthAccount) {
	serviceAccount.contracts.add(
		name: "FlowEpoch",
		code: "%s".decodeHex(),
		currentEpochCounter: UInt64(%d),
		numViewsInEpoch: UInt64(%d),
		numViewsInStakingAuction: UInt64(%d),
		numViewsInDKGPhase: UInt64(%d),
		numCollectorClusters: UInt16(%d),
		FLOWsupplyIncreasePercentage: UFix64(%d),
		randomSource: %s,
		collectorClusters: %s,
		clusterQCs: %s,
		dkgPubKeys: %s,
	)
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

transaction(transactionFee: UFix64, accountCreationFee: UFix64, minimumStorageReservation: UFix64) {
    prepare(service: AuthAccount) {
        let serviceAdmin = service.borrow<&FlowServiceAccount.Administrator>(from: /storage/flowServiceAdmin)
            ?? panic("Could not borrow reference to the flow service admin!");

        let storageAdmin = service.borrow<&FlowStorageFees.Administrator>(from: /storage/storageFeesAdmin)
            ?? panic("Could not borrow reference to the flow storage fees admin!");

        serviceAdmin.setTransactionFee(transactionFee)
        serviceAdmin.setAccountCreationFee(accountCreationFee)
        storageAdmin.setMinimumStorageReservation(minimumStorageReservation)
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

func deployContractTransaction(address flow.Address, contract []byte, contractName string) *TransactionProcedure {
	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(deployContractTransactionTemplate, contractName, hex.EncodeToString(contract)))).
			AddAuthorizer(address),
		0,
	)
}

func deployFlowTokenTransaction(flowToken, service flow.Address, contract []byte) *TransactionProcedure {
	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(deployFlowTokenTransactionTemplate, hex.EncodeToString(contract)))).
			AddAuthorizer(flowToken).
			AddAuthorizer(service),
		0,
	)
}

func deployFlowFeesTransaction(flowFees, service flow.Address, contract []byte) *TransactionProcedure {
	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(deployFlowFeesTransactionTemplate, hex.EncodeToString(contract)))).
			AddAuthorizer(flowFees).
			AddAuthorizer(service),
		0,
	)
}

func deployStorageFeesTransaction(service flow.Address, contract []byte) *TransactionProcedure {
	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(deployStorageFeesTransactionTemplate, hex.EncodeToString(contract)))).
			AddAuthorizer(service),
		0,
	)
}

func deployIDTableStakingTransaction(service flow.Address, contract []byte, epochTokenPayout cadence.UFix64, rewardCut cadence.UFix64) *TransactionProcedure {
	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(
				deployIDTableStakingTransactionTemplate,
				hex.EncodeToString(contract),
				epochTokenPayout,
				rewardCut))).
			AddAuthorizer(service),
		0,
	)
}

func deployEpochTransaction(service flow.Address, contract []byte, epochConfig flow.EpochConfig) *TransactionProcedure {

	tx := Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(
				deployEpochTransactionTemplate,
				hex.EncodeToString(contract),
				epochConfig.CurrentEpochCounter,
				epochConfig.NumViewsInEpoch,
				epochConfig.NumViewsInStakingAuction,
				epochConfig.NumViewsInDKGPhase,
				epochConfig.NumCollectorClusters,
				epochConfig.FLOWsupplyIncreasePercentage,
				epochConfig.RandomSource,
				encodeClusterAssignments(epochConfig.CollectorClusters, service),
				cadence.Array{},
				cadence.Array{},
			))).
			AddAuthorizer(service),
		0,
	)
	return tx
}

func encodeClusterAssignments(clusterAssignments flow.AssignmentList, service flow.Address) cadence.Array {
	collectorClusterValues := []cadence.Value{}

	for i, cluster := range clusterAssignments {
		clusterIndex := cadence.UInt16(i)

		weightsByNodeID := []cadence.KeyValuePair{}
		for _, id := range cluster {
			kvp := cadence.KeyValuePair{
				Key:   cadence.NewString(id.String()),
				Value: cadence.NewUInt64(0),
			}
			weightsByNodeID = append(weightsByNodeID, kvp)
		}

		fields := []cadence.Value{
			clusterIndex,
			cadence.NewDictionary(weightsByNodeID),
		}

		clusterStruct := cadence.NewStruct(fields).
			WithType(&cadence.StructType{
				Location: common.AddressLocation{
					Address: common.BytesToAddress(service.Bytes()),
					Name:    "Service",
				},
				QualifiedIdentifier: "FlowEpochClusterQC.Cluster",
				Fields: []cadence.Field{
					{
						Identifier: "index",
						Type:       cadence.UInt16Type{},
					},
					{
						Identifier: "nodeWeights",
						Type: cadence.DictionaryType{
							KeyType:     cadence.StringType{},
							ElementType: cadence.UInt64Type{},
						},
					},
				},
			})

		collectorClusterValues = append(collectorClusterValues, clusterStruct)
	}

	collectorClusters := cadence.NewArray(collectorClusterValues)

	fmt.Printf("XXX collectorCluster: %#v\n", collectorClusters)

	return collectorClusters
}

func mintFlowTokenTransaction(
	fungibleToken, flowToken, service flow.Address,
	initialSupply cadence.UFix64,
) *TransactionProcedure {
	initialSupplyArg, err := jsoncdc.Encode(initialSupply)
	if err != nil {
		panic(fmt.Sprintf("failed to encode initial token supply: %s", err.Error()))
	}

	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(mintFlowTokenTransactionTemplate, fungibleToken, flowToken))).
			AddArgument(initialSupplyArg).
			AddAuthorizer(service),
		0,
	)
}

func setupFeesTransaction(
	service flow.Address,
	transactionFee,
	addressCreationFee,
	minimumStorageReservation cadence.UFix64,
) *TransactionProcedure {
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

	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(setupFeesTransactionTemplate, service))).
			AddArgument(transactionFeeArg).
			AddArgument(addressCreationFeeArg).
			AddArgument(minimumStorageReservationArg).
			AddAuthorizer(service),
		0,
	)
}

func setupStorageForServiceAccountsTransaction(
	service, fungibleToken, flowToken, feeContract flow.Address,
) *TransactionProcedure {
	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(setupStorageForServiceAccountsTemplate, service, service, fungibleToken, flowToken))).
			AddAuthorizer(service).
			AddAuthorizer(fungibleToken).
			AddAuthorizer(flowToken).
			AddAuthorizer(feeContract),
		0,
	)
}

func setupDKGAdminTransaction(service, dkg flow.Address) *TransactionProcedure {
	env := templates.Environment{
		DkgAddress: dkg.Hex(),
	}
	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(templates.GeneratePublishDKGParticipantScript(env))).
			AddAuthorizer(service),
		0,
	)
}

func setupQCAdminTransaction(service, qc flow.Address) *TransactionProcedure {
	env := templates.Environment{
		QuorumCertificateAddress: qc.Hex(),
	}
	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(templates.GeneratePublishVoterScript(env))).
			AddAuthorizer(service),
		0,
	)
}

const (
	fungibleTokenAccountIndex = 2
	flowTokenAccountIndex     = 3
	flowFeesAccountIndex      = 4
)

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
