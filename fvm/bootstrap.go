package fvm

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-core-contracts/lib/go/contracts"
	"github.com/onflow/flow-core-contracts/lib/go/templates"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/epochs"
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

	accountCreationFee               cadence.UFix64
	transactionFee                   cadence.UFix64
	minimumStorageReservation        cadence.UFix64
	storagePerFlow                   cadence.UFix64
	restrictedAccountCreationEnabled cadence.Bool

	// config values for epoch smart-contracts
	epochConfig epochs.EpochConfig

	// list of initial network participants for whom we will create/stake flow
	// accounts and retrieve epoch-related resources
	identities flow.IdentityList
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
	value, err := cadence.NewUFix64("100.00000000")
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

func WithEpochConfig(epochConfig epochs.EpochConfig) BootstrapProcedureOption {
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

func WithIdentities(identities flow.IdentityList) BootstrapProcedureOption {
	return func(bp *BootstrapProcedure) *BootstrapProcedure {
		bp.identities = identities
		return bp
	}
}

func WithStorageMBPerFLOW(ratio cadence.UFix64) BootstrapProcedureOption {
	return func(bp *BootstrapProcedure) *BootstrapProcedure {
		bp.storagePerFlow = ratio
		return bp
	}
}

func WithRestrictedAccountCreationEnabled(enabled cadence.Bool) BootstrapProcedureOption {
	return func(bp *BootstrapProcedure) *BootstrapProcedure {
		bp.restrictedAccountCreationEnabled = enabled
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
		epochConfig:             epochs.DefaultEpochConfig(),
	}

	for _, applyOption := range opts {
		bootstrapProcedure = applyOption(bootstrapProcedure)
	}
	return bootstrapProcedure
}

func (b *BootstrapProcedure) Run(vm *VirtualMachine, ctx Context, sth *state.StateHolder, programs *programs.Programs) error {
	b.vm = vm
	b.ctx = NewContextFromParent(
		ctx,
		WithRestrictedDeployment(false))
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

	b.setupParameters(
		service,
		b.transactionFee,
		b.accountCreationFee,
		b.minimumStorageReservation,
		b.storagePerFlow,
		b.restrictedAccountCreationEnabled,
	)

	b.setupStorageForServiceAccounts(service, fungibleToken, flowToken, feeContract)

	b.createMinter(service, flowToken)

	b.deployDKG(service)

	b.deployQC(service)

	b.deployIDTableStaking(service, fungibleToken, flowToken, feeContract)

	// set the list of nodes which are allowed to stake in this network
	b.setStakingAllowlist(service, b.identities.NodeIDs())

	b.deployEpoch(service, fungibleToken, flowToken)

	// deploy staking proxy contract to the service account
	b.deployStakingProxyContract(service)

	// deploy locked tokens contract to the service account
	b.deployLockedTokensContract(service, fungibleToken, flowToken)

	// deploy staking collection contract to the service account
	b.deployStakingCollection(service, fungibleToken, flowToken)

	b.registerNodes(service, fungibleToken, flowToken)

	// deploy other contracts (ie, NonFungibleToken and Marketplace)
	nonFungibleToken := b.deployNonFungibleToken()
	b.deployExampleNFT(nonFungibleToken)

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

	txError, err := b.vm.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployFungibleTokenContractTransaction(fungibleToken),
			0),
		b.sth,
		b.programs,
	)
	panicOnMetaInvokeErrf("failed to deploy fungible token contract: %s", txError, err)
	return fungibleToken
}

func (b *BootstrapProcedure) deployNonFungibleToken() flow.Address {
	nonFungibleToken := b.createAccount()

	txError, err := b.vm.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployNonFungibleTokenTransaction(nonFungibleToken),
			0),
		b.sth,
		b.programs,
	)
	panicOnMetaInvokeErrf("failed to deploy NonFungibleToken contract: %s", txError, err)
	return nonFungibleToken
}

func (b *BootstrapProcedure) deployExampleNFT(nonFungibleToken flow.Address) flow.Address {
	exampleNFT := b.createAccount()
	txError, err := b.vm.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployExampleNFTTransaction(
				nonFungibleToken,
				exampleNFT),
			0),
		b.sth,
		b.programs,
	)
	panicOnMetaInvokeErrf("failed to deploy ExampleNFT: %s", txError, err)
	return exampleNFT
}

func (b *BootstrapProcedure) deployFlowToken(service, fungibleToken flow.Address) flow.Address {
	flowToken := b.createAccount()
	txError, err := b.vm.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployFlowTokenContractTransaction(
				service,
				fungibleToken,
				flowToken),
			0),
		b.sth,
		b.programs,
	)
	panicOnMetaInvokeErrf("failed to deploy Flow token contract: %s", txError, err)
	return flowToken
}

func (b *BootstrapProcedure) deployFlowFees(service, fungibleToken, flowToken flow.Address) flow.Address {
	flowFees := b.createAccount()

	txError, err := b.vm.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployTxFeesContractTransaction(
				service,
				fungibleToken,
				flowToken,
				flowFees,
			),
			0),
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
	txError, err := b.vm.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployStorageFeesContractTransaction(
				service,
				contract),
			0),
		b.sth,
		b.programs,
	)
	panicOnMetaInvokeErrf("failed to deploy storage fees contract: %s", txError, err)
}

func (b *BootstrapProcedure) createMinter(service, flowToken flow.Address) {
	txError, err := b.vm.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.CreateFlowTokenMinterTransaction(
				service,
				flowToken),
			0),
		b.sth,
		b.programs,
	)
	panicOnMetaInvokeErrf("failed to create flow token minter: %s", txError, err)
}

func (b *BootstrapProcedure) deployDKG(service flow.Address) {
	contract := contracts.FlowDKG()
	txError, err := b.vm.invokeMetaTransaction(
		b.ctx,
		deployContractTransaction(service, contract, "FlowDKG"),
		b.sth,
		b.programs,
	)
	panicOnMetaInvokeErrf("failed to deploy DKG contract: %s", txError, err)
}

func (b *BootstrapProcedure) deployQC(service flow.Address) {
	contract := contracts.FlowQC()
	txError, err := b.vm.invokeMetaTransaction(
		b.ctx,
		deployContractTransaction(service, contract, "FlowClusterQC"),
		b.sth,
		b.programs,
	)
	panicOnMetaInvokeErrf("failed to deploy QC contract: %s", txError, err)
}

func (b *BootstrapProcedure) deployIDTableStaking(service, fungibleToken, flowToken, flowFees flow.Address) {

	contract := contracts.FlowIDTableStaking(
		fungibleToken.HexWithPrefix(),
		flowToken.HexWithPrefix(),
		flowFees.HexWithPrefix(),
		true)

	txError, err := b.vm.invokeMetaTransaction(
		b.ctx,
		deployIDTableStakingTransaction(service,
			contract,
			b.epochConfig.EpochTokenPayout,
			b.epochConfig.RewardCut),
		b.sth,
		b.programs,
	)
	panicOnMetaInvokeErrf("failed to deploy IDTableStaking contract: %s", txError, err)
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

	txError, err := b.vm.invokeMetaTransaction(
		context,
		deployEpochTransaction(service, contract, b.epochConfig),
		b.sth,
		b.programs,
	)
	panicOnMetaInvokeErrf("failed to deploy Epoch contract: %s", txError, err)
}

func (b *BootstrapProcedure) deployServiceAccount(service, fungibleToken, flowToken, feeContract flow.Address) {
	contract := contracts.FlowServiceAccount(
		fungibleToken.HexWithPrefix(),
		flowToken.HexWithPrefix(),
		feeContract.HexWithPrefix(),
		service.HexWithPrefix(),
	)

	txError, err := b.vm.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployContractTransaction(
				service,
				contract,
				"FlowServiceAccount"),
			0),
		b.sth,
		b.programs,
	)
	panicOnMetaInvokeErrf("failed to deploy service account contract: %s", txError, err)
}

func (b *BootstrapProcedure) mintInitialTokens(
	service, fungibleToken, flowToken flow.Address,
	initialSupply cadence.UFix64,
) {
	txError, err := b.vm.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.MintFlowTokenTransaction(
				fungibleToken,
				flowToken,
				service,
				initialSupply),
			0),
		b.sth,
		b.programs,
	)
	panicOnMetaInvokeErrf("failed to mint initial token supply: %s", txError, err)
}

func (b *BootstrapProcedure) setupParameters(
	service flow.Address,
	transactionFee,
	addressCreationFee,
	minimumStorageReservation,
	storagePerFlow cadence.UFix64,
	restrictedAccountCreationEnabled cadence.Bool,
) {
	txError, err := b.vm.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.SetupFeesTransaction(
				service,
				transactionFee,
				addressCreationFee,
				minimumStorageReservation,
				storagePerFlow,
				restrictedAccountCreationEnabled,
			),
			0),
		b.sth,
		b.programs,
	)
	panicOnMetaInvokeErrf("failed to setup fees: %s", txError, err)
}

func (b *BootstrapProcedure) setupStorageForServiceAccounts(
	service, fungibleToken, flowToken, feeContract flow.Address,
) {
	txError, err := b.vm.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.SetupStorageForServiceAccountsTransaction(
				service,
				fungibleToken,
				flowToken,
				feeContract),
			0),
		b.sth,
		b.programs,
	)
	panicOnMetaInvokeErrf("failed to setup storage for service accounts: %s", txError, err)
}

func (b *BootstrapProcedure) setStakingAllowlist(service flow.Address, allowedIDs []flow.Identifier) {

	txError, err := b.vm.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.SetStakingAllowlistTransaction(
				service,
				allowedIDs,
			),
			0),
		b.sth,
		b.programs,
	)
	panicOnMetaInvokeErrf("failed to set staking allow-list: %s", txError, err)
}

func (b *BootstrapProcedure) registerNodes(service, fungibleToken, flowToken flow.Address) {
	for _, id := range b.identities {

		// create a staking account for the node
		nodeAddress := b.createAccount()

		// give a vault resource to the staking account
		txError, err := b.vm.invokeMetaTransaction(
			b.ctx,
			setupAccountTransaction(
				fungibleToken,
				flowToken,
				nodeAddress,
			),
			b.sth,
			b.programs,
		)
		panicOnMetaInvokeErrf("failed to setup machine account: %s", txError, err)

		// fund the staking account
		txError, err = b.vm.invokeMetaTransaction(
			b.ctx,
			fundAccountTransaction(service,
				fungibleToken,
				flowToken,
				nodeAddress),
			b.sth,
			b.programs,
		)
		panicOnMetaInvokeErrf("failed to fund node staking account: %s", txError, err)

		// register the node
		// for collection/consensus nodes this will also create the machine account
		// and set it up with the QC/DKG participant resource
		txError, err = b.vm.invokeMetaTransaction(
			b.ctx,
			registerNodeTransaction(service,
				flowToken,
				nodeAddress,
				id),
			b.sth,
			b.programs,
		)
		panicOnMetaInvokeErrf("failed to register node: %s", txError, err)
	}
}

func (b *BootstrapProcedure) deployStakingProxyContract(service flow.Address) {
	contract := contracts.FlowStakingProxy()
	txError, err := b.vm.invokeMetaTransaction(
		b.ctx,
		deployContractTransaction(service, contract, "StakingProxy"),
		b.sth,
		b.programs,
	)
	panicOnMetaInvokeErrf("failed to deploy StakingProxy contract: %s", txError, err)
}

func (b *BootstrapProcedure) deployLockedTokensContract(service flow.Address, fungibleTokenAddress,
	flowTokenAddress flow.Address) {

	publicKeys := make([]cadence.Value, 1)
	encodedPublicKey, err := flow.EncodeRuntimeAccountPublicKey(b.serviceAccountPublicKey)
	if err != nil {
		panic(err)
	}
	publicKeys[0] = bytesToCadenceArray(encodedPublicKey)

	contract := contracts.FlowLockedTokens(
		fungibleTokenAddress.Hex(),
		flowTokenAddress.Hex(),
		service.Hex(),
		service.Hex(),
		service.Hex())

	txError, err := b.vm.invokeMetaTransaction(
		b.ctx,
		deployLockedTokensTransaction(service, contract, publicKeys),
		b.sth,
		b.programs,
	)

	panicOnMetaInvokeErrf("failed to deploy LockedTokens contract: %s", txError, err)
}

func (b *BootstrapProcedure) deployStakingCollection(service flow.Address, fungibleTokenAddress, flowTokenAddress flow.Address) {
	contract := contracts.FlowStakingCollection(
		fungibleTokenAddress.Hex(),
		flowTokenAddress.Hex(),
		service.Hex(),
		service.Hex(),
		service.Hex(),
		service.Hex(),
		service.Hex(),
		service.Hex(),
		service.Hex())
	txError, err := b.vm.invokeMetaTransaction(
		b.ctx,
		deployContractTransaction(service, contract, "FlowStakingCollection"),
		b.sth,
		b.programs,
	)
	panicOnMetaInvokeErrf("failed to deploy FlowStakingCollection contract: %s", txError, err)
}

const deployContractTransactionTemplate = `
transaction {
  prepare(signer: AuthAccount) {
    signer.contracts.add(name: "%s", code: "%s".decodeHex())
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
import FlowClusterQC from 0x%s

transaction(clusterWeights: [{String: UInt64}]) {
  prepare(serviceAccount: AuthAccount)	{

    // first, construct Cluster objects from cluster weights
    let clusters: [FlowClusterQC.Cluster] = []
    var clusterIndex: UInt16 = 0
    for weightMapping in clusterWeights {
      let cluster = FlowClusterQC.Cluster(clusterIndex, weightMapping)
      clusterIndex = clusterIndex + 1
    }

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
		collectorClusters: clusters,
        // NOTE: clusterQCs and dkgPubKeys are empty because these initial values are not used
		clusterQCs: [],
		dkgPubKeys: [],
	)
  }
}
`

const setupAccountTemplate = `
// This transaction is a template for a transaction
// to add a Vault resource to their account
// so that they can use the flowToken

import FungibleToken from 0x%s
import FlowToken from 0x%s

transaction {

    prepare(signer: AuthAccount) {

        if signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault) == nil {
            // Create a new flowToken Vault and put it in storage
            signer.save(<-FlowToken.createEmptyVault(), to: /storage/flowTokenVault)

            // Create a public capability to the Vault that only exposes
            // the deposit function through the Receiver interface
            signer.link<&FlowToken.Vault{FungibleToken.Receiver}>(
                /public/flowTokenReceiver,
                target: /storage/flowTokenVault
            )

            // Create a public capability to the Vault that only exposes
            // the balance field through the Balance interface
            signer.link<&FlowToken.Vault{FungibleToken.Balance}>(
                /public/flowTokenBalance,
                target: /storage/flowTokenVault
            )
        }
    }
}
`

const fundAccountTemplate = `
import FungibleToken from 0x%s
import FlowToken from 0x%s

transaction(amount: UFix64, recipient: Address) {
	let sentVault: @FungibleToken.Vault
	prepare(signer: AuthAccount) {
	let vaultRef = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)
		?? panic("failed to borrow reference to sender vault")
	self.sentVault <- vaultRef.withdraw(amount: amount)
	}
	execute {
	let receiverRef =  getAccount(recipient)
		.getCapability(/public/flowTokenReceiver)
		.borrow<&{FungibleToken.Receiver}>()
		?? panic("failed to borrow reference to recipient vault")
	receiverRef.deposit(from: <-self.sentVault)
	}
}
`

const deployLockedTokensTemplate = `
transaction(publicKeys: [[UInt8]]) {
    
    prepare(admin: AuthAccount) {
        admin.contracts.add(name: "LockedTokens", code: "%s".decodeHex(), admin)

    }
}
`

func deployLockedTokensTransaction(service flow.Address, contract []byte, publicKeys []cadence.Value) *TransactionProcedure {
	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(
				deployLockedTokensTemplate,
				hex.EncodeToString(contract),
			))).
			AddArgument(jsoncdc.MustEncode(cadence.NewArray(publicKeys))).
			AddAuthorizer(service),
		0,
	)
}

func deployContractTransaction(address flow.Address, contract []byte, contractName string) *TransactionProcedure {
	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(deployContractTransactionTemplate, contractName, hex.EncodeToString(contract)))).
			AddAuthorizer(address),
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

func deployEpochTransaction(service flow.Address, contract []byte, epochConfig epochs.EpochConfig) *TransactionProcedure {
	tx := Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(
				deployEpochTransactionTemplate,
				service,
				hex.EncodeToString(contract),
				epochConfig.CurrentEpochCounter,
				epochConfig.NumViewsInEpoch,
				epochConfig.NumViewsInStakingAuction,
				epochConfig.NumViewsInDKGPhase,
				epochConfig.NumCollectorClusters,
				epochConfig.FLOWsupplyIncreasePercentage,
				epochConfig.RandomSource,
			))).
			AddArgument(epochs.EncodeClusterAssignments(epochConfig.CollectorClusters)).
			AddAuthorizer(service),
		0,
	)
	return tx
}

func setupAccountTransaction(
	fungibleToken flow.Address,
	flowToken flow.Address,
	accountAddress flow.Address,
) *TransactionProcedure {
	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(setupAccountTemplate, fungibleToken, flowToken))).
			AddAuthorizer(accountAddress),
		0,
	)
}

func fundAccountTransaction(
	service flow.Address,
	fungibleToken flow.Address,
	flowToken flow.Address,
	nodeAddress flow.Address,
) *TransactionProcedure {

	cdcAmount, err := cadence.NewUFix64(fmt.Sprintf("%d.0", 2_000_000))
	if err != nil {
		panic(err)
	}

	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(fundAccountTemplate, fungibleToken, flowToken))).
			AddArgument(jsoncdc.MustEncode(cdcAmount)).
			AddArgument(jsoncdc.MustEncode(cadence.NewAddress(nodeAddress))).
			AddAuthorizer(service),
		0,
	)
}

// registerNodeTransaction creates a new node struct object.
// Then, if the node is a collector node, creates a new account and adds a QC object to it
// If the node is a consensus node, it creates a new account and adds a DKG object to it
func registerNodeTransaction(
	service flow.Address,
	flowTokenAddress flow.Address,
	nodeAddress flow.Address,
	id *flow.Identity,
) *TransactionProcedure {

	env := templates.Environment{
		FlowTokenAddress:         flowTokenAddress.HexWithPrefix(),
		IDTableAddress:           service.HexWithPrefix(),
		QuorumCertificateAddress: service.HexWithPrefix(),
		DkgAddress:               service.HexWithPrefix(),
		EpochAddress:             service.HexWithPrefix(),
	}

	// Use NetworkingKey as the public key of the machine account.
	// We do this for tests/localnet but normally it should be a separate key.
	accountKey := flow.AccountPublicKey{
		PublicKey: id.NetworkPubKey,
		SignAlgo:  id.NetworkPubKey.Algorithm(),
		HashAlgo:  bootstrap.DefaultMachineAccountHashAlgo,
		Weight:    1000,
	}

	encAccountKey, err := flow.EncodeRuntimeAccountPublicKey(accountKey)
	if err != nil {
		panic(err)
	}

	cadencePublicKeys := cadence.NewArray(
		[]cadence.Value{
			bytesToCadenceArray(encAccountKey),
		},
	)

	cdcAmount, err := cadence.NewUFix64(fmt.Sprintf("%d.0", id.Stake))
	if err != nil {
		panic(err)
	}

	cdcNodeID, err := cadence.NewString(id.NodeID.String())
	if err != nil {
		panic(err)
	}

	cdcAddress, err := cadence.NewString(id.Address)
	if err != nil {
		panic(err)
	}

	cdcNetworkPubKey, err := cadence.NewString(id.NetworkPubKey.String()[2:])
	if err != nil {
		panic(err)
	}

	cdcStakingPubKey, err := cadence.NewString(id.StakingPubKey.String()[2:])
	if err != nil {
		panic(err)
	}

	// register node
	return Transaction(
		flow.NewTransactionBody().
			SetScript(templates.GenerateEpochRegisterNodeScript(env)).
			AddArgument(jsoncdc.MustEncode(cdcNodeID)).
			AddArgument(jsoncdc.MustEncode(cadence.NewUInt8(uint8(id.Role)))).
			AddArgument(jsoncdc.MustEncode(cdcAddress)).
			AddArgument(jsoncdc.MustEncode(cdcNetworkPubKey)).
			AddArgument(jsoncdc.MustEncode(cdcStakingPubKey)).
			AddArgument(jsoncdc.MustEncode(cdcAmount)).
			AddArgument(jsoncdc.MustEncode(cadencePublicKeys)).
			AddAuthorizer(nodeAddress),
		0,
	)
}

func bytesToCadenceArray(b []byte) cadence.Array {
	values := make([]cadence.Value, len(b))
	for i, v := range b {
		values[i] = cadence.NewUInt8(v)
	}
	return cadence.NewArray(values)
}

const (
	fungibleTokenAccountIndex    = 2
	flowTokenAccountIndex        = 3
	flowFeesAccountIndex         = 4
	nonFungibleTokenAccountIndex = 5
	exampleNFTAccountIndex       = 6
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

func NonFungibleTokenAddress(chain flow.Chain) flow.Address {
	address, _ := chain.AddressAtIndex(nonFungibleTokenAccountIndex)
	return address
}

func ExampleNFTAddress(chain flow.Chain) flow.Address {
	address, _ := chain.AddressAtIndex(exampleNFTAccountIndex)
	return address
}
