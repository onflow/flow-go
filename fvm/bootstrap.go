package fvm

import (
	"fmt"
	"math"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-core-contracts/lib/go/contracts"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/epochs"
)

// A BootstrapProcedure is an invokable that can be used to bootstrap the ledger state
// with the default accounts and contracts required by the Flow virtual machine.
type BootstrapProcedure struct {
	ctx       Context
	txnState  *state.TransactionState
	rootBlock *flow.Header

	// genesis parameters
	accountKeys        BootstrapAccountKeys
	initialTokenSupply cadence.UFix64
	accountCreator     environment.BootstrapAccountCreator

	accountCreationFee               cadence.UFix64
	minimumStorageReservation        cadence.UFix64
	storagePerFlow                   cadence.UFix64
	restrictedAccountCreationEnabled cadence.Bool

	// TODO: restrictedContractDeployment should be a bool after RestrictedDeploymentEnabled is removed from the context
	// restrictedContractDeployment of nil means that the contract deployment is taken from the fvm Context instead of from the state.
	// This can be used to mimic behaviour on chain before the restrictedContractDeployment is set with a service account transaction.
	restrictedContractDeployment *bool

	transactionFees        BootstrapProcedureFeeParameters
	executionEffortWeights meter.ExecutionEffortWeights
	executionMemoryWeights meter.ExecutionMemoryWeights
	// executionMemoryLimit of 0 means that it won't be set in the state. The FVM will use the default value from the context.
	executionMemoryLimit uint64

	// config values for epoch smart-contracts
	epochConfig epochs.EpochConfig

	// list of initial network participants for whom we will create/stake flow
	// accounts and retrieve epoch-related resources
	identities flow.IdentityList
}

type BootstrapAccountKeys struct {
	ServiceAccountPublicKeys       []flow.AccountPublicKey
	FungibleTokenAccountPublicKeys []flow.AccountPublicKey
	FlowTokenAccountPublicKeys     []flow.AccountPublicKey
	FlowFeesAccountPublicKeys      []flow.AccountPublicKey
	NodeAccountPublicKeys          []flow.AccountPublicKey
}

type BootstrapProcedureFeeParameters struct {
	SurgeFactor         cadence.UFix64
	InclusionEffortCost cadence.UFix64
	ExecutionEffortCost cadence.UFix64
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

// DefaultTransactionFees are the default transaction fee parameters if transaction fees are on.
// surge factor is 1.0, inclusion effort cost is 0.0001 (because the static inclusion effort is 1.0) and
// execution effort cost is 0.0 because dynamic execution fees are off
// If they are off (which is the default behaviour) that means the transaction fees are 0.0.
var DefaultTransactionFees = func() BootstrapProcedureFeeParameters {
	surgeFactor, err := cadence.NewUFix64("1.0")
	if err != nil {
		panic(fmt.Errorf("invalid default fee surge factor: %w", err))
	}
	inclusionEffortCost, err := cadence.NewUFix64("0.00001")
	if err != nil {
		panic(fmt.Errorf("invalid default fee effort cost: %w", err))
	}
	executionEffortCost, err := cadence.NewUFix64("0.0")
	if err != nil {
		panic(fmt.Errorf("invalid default fee effort cost: %w", err))
	}
	return BootstrapProcedureFeeParameters{
		SurgeFactor:         surgeFactor,
		InclusionEffortCost: inclusionEffortCost,
		ExecutionEffortCost: executionEffortCost,
	}
}()

// WithBootstrapAccountKeys sets the public keys of the accounts that will be created during bootstrapping
// by default all accounts are created with the ServiceAccountPublicKey specified when calling `Bootstrap`.
func WithBootstrapAccountKeys(keys BootstrapAccountKeys) BootstrapProcedureOption {
	return func(bp *BootstrapProcedure) *BootstrapProcedure {
		bp.accountKeys = keys
		return bp
	}
}

func WithAccountCreationFee(fee cadence.UFix64) BootstrapProcedureOption {
	return func(bp *BootstrapProcedure) *BootstrapProcedure {
		bp.accountCreationFee = fee
		return bp
	}
}

func WithTransactionFee(fees BootstrapProcedureFeeParameters) BootstrapProcedureOption {
	return func(bp *BootstrapProcedure) *BootstrapProcedure {
		bp.transactionFees = fees
		return bp
	}
}

func WithExecutionEffortWeights(weights meter.ExecutionEffortWeights) BootstrapProcedureOption {
	return func(bp *BootstrapProcedure) *BootstrapProcedure {
		bp.executionEffortWeights = weights
		return bp
	}
}

func WithExecutionMemoryWeights(weights meter.ExecutionMemoryWeights) BootstrapProcedureOption {
	return func(bp *BootstrapProcedure) *BootstrapProcedure {
		bp.executionMemoryWeights = weights
		return bp
	}
}

func WithExecutionMemoryLimit(limit uint64) BootstrapProcedureOption {
	return func(bp *BootstrapProcedure) *BootstrapProcedure {
		bp.executionMemoryLimit = limit
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

func WithRestrictedContractDeployment(restricted *bool) BootstrapProcedureOption {
	return func(bp *BootstrapProcedure) *BootstrapProcedure {
		bp.restrictedContractDeployment = restricted
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
		transactionFees: BootstrapProcedureFeeParameters{0, 0, 0},
		epochConfig:     epochs.DefaultEpochConfig(),
	}
	bootstrapProcedure.accountKeys = BootstrapAccountKeys{
		ServiceAccountPublicKeys:       []flow.AccountPublicKey{serviceAccountPublicKey},
		FungibleTokenAccountPublicKeys: []flow.AccountPublicKey{serviceAccountPublicKey},
		FlowTokenAccountPublicKeys:     []flow.AccountPublicKey{serviceAccountPublicKey},
		NodeAccountPublicKeys:          []flow.AccountPublicKey{serviceAccountPublicKey},
	}

	for _, applyOption := range opts {
		bootstrapProcedure = applyOption(bootstrapProcedure)
	}
	return bootstrapProcedure
}

func (b *BootstrapProcedure) Run(
	ctx Context,
	txnState *state.TransactionState,
	_ *programs.TransactionPrograms,
) error {
	b.ctx = NewContextFromParent(
		ctx,
		WithContractDeploymentRestricted(false))
	b.rootBlock = flow.Genesis(flow.ChainID(ctx.Chain.String())).Header
	b.txnState = txnState

	// initialize the account addressing state
	b.accountCreator = environment.NewBootstrapAccountCreator(
		b.txnState,
		ctx.Chain,
		environment.NewAccounts(b.txnState))

	service := b.createServiceAccount()

	b.deployContractAuditVouchers(service)
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
		b.accountCreationFee,
		b.minimumStorageReservation,
		b.storagePerFlow,
		b.restrictedAccountCreationEnabled,
	)

	b.setupFees(
		service,
		feeContract,
		b.transactionFees.SurgeFactor,
		b.transactionFees.InclusionEffortCost,
		b.transactionFees.ExecutionEffortCost,
	)

	b.setContractDeploymentRestrictions(service, b.restrictedContractDeployment)

	b.setupExecutionWeights(service)

	b.setupStorageForServiceAccounts(service, fungibleToken, flowToken, feeContract)

	b.createMinter(service, flowToken)

	b.deployDKG(service)

	b.deployQC(service)

	b.deployIDTableStaking(service, fungibleToken, flowToken, feeContract)

	// set the list of nodes which are allowed to stake in this network
	b.setStakingAllowlist(service, b.identities.NodeIDs())

	b.deployEpoch(service, fungibleToken, flowToken, feeContract)

	// deploy staking proxy contract to the service account
	b.deployStakingProxyContract(service)

	// deploy locked tokens contract to the service account
	b.deployLockedTokensContract(service, fungibleToken, flowToken)

	// deploy staking collection contract to the service account
	b.deployStakingCollection(service, fungibleToken, flowToken)

	b.registerNodes(service, fungibleToken, flowToken)

	return nil
}

func (proc *BootstrapProcedure) ComputationLimit(_ Context) uint64 {
	return math.MaxUint64
}

func (proc *BootstrapProcedure) MemoryLimit(_ Context) uint64 {
	return math.MaxUint64
}

func (proc *BootstrapProcedure) ShouldDisableMemoryAndInteractionLimits(_ Context) bool {
	return true
}

func (BootstrapProcedure) Type() ProcedureType {
	return BootstrapProcedureType
}

func (proc *BootstrapProcedure) InitialSnapshotTime() programs.LogicalTime {
	return 0
}

func (proc *BootstrapProcedure) ExecutionTime() programs.LogicalTime {
	return 0
}

func (b *BootstrapProcedure) createAccount(publicKeys []flow.AccountPublicKey) flow.Address {
	address, err := b.accountCreator.CreateBootstrapAccount(publicKeys)
	if err != nil {
		panic(fmt.Sprintf("failed to create account: %s", err))
	}

	return address
}

func (b *BootstrapProcedure) createServiceAccount() flow.Address {
	address, err := b.accountCreator.CreateBootstrapAccount(
		b.accountKeys.ServiceAccountPublicKeys)
	if err != nil {
		panic(fmt.Sprintf("failed to create service account: %s", err))
	}

	return address
}

func (b *BootstrapProcedure) deployFungibleToken() flow.Address {
	fungibleToken := b.createAccount(b.accountKeys.FungibleTokenAccountPublicKeys)

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployFungibleTokenContractTransaction(fungibleToken),
			0),
	)
	panicOnMetaInvokeErrf("failed to deploy fungible token contract: %s", txError, err)
	return fungibleToken
}

func (b *BootstrapProcedure) deployFlowToken(service, fungibleToken flow.Address) flow.Address {
	flowToken := b.createAccount(b.accountKeys.FlowTokenAccountPublicKeys)
	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployFlowTokenContractTransaction(
				service,
				fungibleToken,
				flowToken),
			0),
	)
	panicOnMetaInvokeErrf("failed to deploy Flow token contract: %s", txError, err)
	return flowToken
}

func (b *BootstrapProcedure) deployFlowFees(service, fungibleToken, flowToken flow.Address) flow.Address {
	flowFees := b.createAccount(b.accountKeys.FlowFeesAccountPublicKeys)

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployTxFeesContractTransaction(
				service,
				fungibleToken,
				flowToken,
				flowFees,
			),
			0),
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
	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployStorageFeesContractTransaction(
				service,
				contract),
			0),
	)
	panicOnMetaInvokeErrf("failed to deploy storage fees contract: %s", txError, err)
}

// deployContractAuditVouchers deploys audit vouchers contract to the service account
func (b *BootstrapProcedure) deployContractAuditVouchers(service flow.Address) {
	contract := contracts.FlowContractAudits()

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployContractTransaction(
				service,
				contract,
				"FlowContractAudits"),
			0),
	)
	panicOnMetaInvokeErrf("failed to deploy contract audit vouchers contract: %s", txError, err)
}

func (b *BootstrapProcedure) createMinter(service, flowToken flow.Address) {
	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.CreateFlowTokenMinterTransaction(
				service,
				flowToken),
			0),
	)
	panicOnMetaInvokeErrf("failed to create flow token minter: %s", txError, err)
}

func (b *BootstrapProcedure) deployDKG(service flow.Address) {
	contract := contracts.FlowDKG()
	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployContractTransaction(service, contract, "FlowDKG"),
			0,
		),
	)
	panicOnMetaInvokeErrf("failed to deploy DKG contract: %s", txError, err)
}

func (b *BootstrapProcedure) deployQC(service flow.Address) {
	contract := contracts.FlowQC()
	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployContractTransaction(service, contract, "FlowClusterQC"),
			0,
		),
	)
	panicOnMetaInvokeErrf("failed to deploy QC contract: %s", txError, err)
}

func (b *BootstrapProcedure) deployIDTableStaking(service, fungibleToken, flowToken, flowFees flow.Address) {

	contract := contracts.FlowIDTableStaking(
		fungibleToken.HexWithPrefix(),
		flowToken.HexWithPrefix(),
		flowFees.HexWithPrefix(),
		true)

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployIDTableStakingTransaction(service,
				contract,
				b.epochConfig.EpochTokenPayout,
				b.epochConfig.RewardCut),
			0,
		),
	)
	panicOnMetaInvokeErrf("failed to deploy IDTableStaking contract: %s", txError, err)
}

func (b *BootstrapProcedure) deployEpoch(service, fungibleToken, flowToken, flowFees flow.Address) {

	contract := contracts.FlowEpoch(
		fungibleToken.HexWithPrefix(),
		flowToken.HexWithPrefix(),
		service.HexWithPrefix(),
		service.HexWithPrefix(),
		service.HexWithPrefix(),
		flowFees.HexWithPrefix(),
	)

	context := NewContextFromParent(b.ctx,
		WithBlockHeader(b.rootBlock),
		WithBlocks(&environment.NoopBlockFinder{}),
	)

	txError, err := b.invokeMetaTransaction(
		context,
		Transaction(
			blueprints.DeployEpochTransaction(service, contract, b.epochConfig),
			0,
		),
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

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployContractTransaction(
				service,
				contract,
				"FlowServiceAccount"),
			0),
	)
	panicOnMetaInvokeErrf("failed to deploy service account contract: %s", txError, err)
}

func (b *BootstrapProcedure) mintInitialTokens(
	service, fungibleToken, flowToken flow.Address,
	initialSupply cadence.UFix64,
) {
	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.MintFlowTokenTransaction(
				fungibleToken,
				flowToken,
				service,
				initialSupply),
			0),
	)
	panicOnMetaInvokeErrf("failed to mint initial token supply: %s", txError, err)
}

func (b *BootstrapProcedure) setupParameters(
	service flow.Address,
	addressCreationFee,
	minimumStorageReservation,
	storagePerFlow cadence.UFix64,
	restrictedAccountCreationEnabled cadence.Bool,
) {
	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.SetupParametersTransaction(
				service,
				addressCreationFee,
				minimumStorageReservation,
				storagePerFlow,
				restrictedAccountCreationEnabled,
			),
			0),
	)
	panicOnMetaInvokeErrf("failed to setup parameters: %s", txError, err)
}

func (b *BootstrapProcedure) setupFees(service, flowFees flow.Address, surgeFactor, inclusionEffortCost, executionEffortCost cadence.UFix64) {
	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.SetupFeesTransaction(
				service,
				flowFees,
				surgeFactor,
				inclusionEffortCost,
				executionEffortCost,
			),
			0),
	)
	panicOnMetaInvokeErrf("failed to setup fees: %s", txError, err)
}

func (b *BootstrapProcedure) setupExecutionWeights(service flow.Address) {
	// if executionEffortWeights were not set skip this part and just use the defaults.
	if b.executionEffortWeights != nil {
		b.setupExecutionEffortWeights(service)
	}
	// if executionMemoryWeights were not set skip this part and just use the defaults.
	if b.executionMemoryWeights != nil {
		b.setupExecutionMemoryWeights(service)
	}
	if b.executionMemoryLimit != 0 {
		b.setExecutionMemoryLimitTransaction(service)
	}
}

func (b *BootstrapProcedure) setupExecutionEffortWeights(service flow.Address) {
	weights := b.executionEffortWeights

	uintWeights := make(map[uint]uint64, len(weights))
	for i, weight := range weights {
		uintWeights[uint(i)] = weight
	}

	tb, err := blueprints.SetExecutionEffortWeightsTransaction(service, uintWeights)
	if err != nil {
		panic(fmt.Sprintf("failed to setup execution effort weights %s", err.Error()))
	}

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			tb,
			0),
	)
	panicOnMetaInvokeErrf("failed to setup execution effort weights: %s", txError, err)
}

func (b *BootstrapProcedure) setupExecutionMemoryWeights(service flow.Address) {
	weights := b.executionMemoryWeights

	uintWeights := make(map[uint]uint64, len(weights))
	for i, weight := range weights {
		uintWeights[uint(i)] = weight
	}

	tb, err := blueprints.SetExecutionMemoryWeightsTransaction(service, uintWeights)
	if err != nil {
		panic(fmt.Sprintf("failed to setup execution memory weights %s", err.Error()))
	}

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			tb,
			0),
	)
	panicOnMetaInvokeErrf("failed to setup execution memory weights: %s", txError, err)
}

func (b *BootstrapProcedure) setExecutionMemoryLimitTransaction(service flow.Address) {

	tb, err := blueprints.SetExecutionMemoryLimitTransaction(service, b.executionMemoryLimit)
	if err != nil {
		panic(fmt.Sprintf("failed to setup execution memory limit %s", err.Error()))
	}

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			tb,
			0),
	)
	panicOnMetaInvokeErrf("failed to setup execution memory limit: %s", txError, err)
}

func (b *BootstrapProcedure) setupStorageForServiceAccounts(
	service, fungibleToken, flowToken, feeContract flow.Address,
) {
	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.SetupStorageForServiceAccountsTransaction(
				service,
				fungibleToken,
				flowToken,
				feeContract),
			0),
	)
	panicOnMetaInvokeErrf("failed to setup storage for service accounts: %s", txError, err)
}

func (b *BootstrapProcedure) setStakingAllowlist(service flow.Address, allowedIDs []flow.Identifier) {

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.SetStakingAllowlistTransaction(
				service,
				allowedIDs,
			),
			0),
	)
	panicOnMetaInvokeErrf("failed to set staking allow-list: %s", txError, err)
}

func (b *BootstrapProcedure) registerNodes(service, fungibleToken, flowToken flow.Address) {
	for _, id := range b.identities {

		// create a staking account for the node
		nodeAddress := b.createAccount(b.accountKeys.NodeAccountPublicKeys)

		// give a vault resource to the staking account
		txError, err := b.invokeMetaTransaction(
			b.ctx,
			Transaction(
				blueprints.SetupAccountTransaction(fungibleToken,
					flowToken,
					nodeAddress),
				0,
			),
		)
		panicOnMetaInvokeErrf("failed to setup machine account: %s", txError, err)

		// fund the staking account
		txError, err = b.invokeMetaTransaction(
			b.ctx,
			Transaction(blueprints.FundAccountTransaction(service,
				fungibleToken,
				flowToken,
				nodeAddress),
				0),
		)
		panicOnMetaInvokeErrf("failed to fund node staking account: %s", txError, err)

		// register the node
		// for collection/consensus nodes this will also create the machine account
		// and set it up with the QC/DKG participant resource
		txError, err = b.invokeMetaTransaction(
			b.ctx,
			Transaction(blueprints.RegisterNodeTransaction(service,
				flowToken,
				nodeAddress,
				id),
				0),
		)
		panicOnMetaInvokeErrf("failed to register node: %s", txError, err)
	}
}

func (b *BootstrapProcedure) deployStakingProxyContract(service flow.Address) {
	contract := contracts.FlowStakingProxy()
	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployContractTransaction(service, contract, "StakingProxy"),
			0,
		),
	)
	panicOnMetaInvokeErrf("failed to deploy StakingProxy contract: %s", txError, err)
}

func (b *BootstrapProcedure) deployLockedTokensContract(service flow.Address, fungibleTokenAddress,
	flowTokenAddress flow.Address) {

	publicKeys, err := flow.EncodeRuntimeAccountPublicKeys(b.accountKeys.ServiceAccountPublicKeys)
	if err != nil {
		panic(err)
	}

	contract := contracts.FlowLockedTokens(
		fungibleTokenAddress.Hex(),
		flowTokenAddress.Hex(),
		service.Hex(),
		service.Hex(),
		service.Hex())

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployLockedTokensTransaction(service, contract, publicKeys),
			0,
		),
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
	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployContractTransaction(service, contract, "FlowStakingCollection"),
			0,
		),
	)
	panicOnMetaInvokeErrf("failed to deploy FlowStakingCollection contract: %s", txError, err)
}

func (b *BootstrapProcedure) setContractDeploymentRestrictions(service flow.Address, deployment *bool) {
	if deployment == nil {
		return
	}

	txBody, err := blueprints.SetIsContractDeploymentRestrictedTransaction(service, *deployment)
	if err != nil {
		panic(err)
	}
	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			txBody,
			0,
		),
	)
	panicOnMetaInvokeErrf("failed to deploy FlowStakingCollection contract: %s", txError, err)
}

func panicOnMetaInvokeErrf(msg string, txError errors.CodedError, err error) {
	if txError != nil {
		panic(fmt.Sprintf(msg, txError.Error()))
	}
	if err != nil {
		panic(fmt.Sprintf(msg, err.Error()))
	}
}

func FungibleTokenAddress(chain flow.Chain) flow.Address {
	address, _ := chain.AddressAtIndex(environment.FungibleTokenAccountIndex)
	return address
}

func FlowTokenAddress(chain flow.Chain) flow.Address {
	address, _ := chain.AddressAtIndex(environment.FlowTokenAccountIndex)
	return address
}

// invokeMetaTransaction invokes a meta transaction inside the context of an
// outer transaction.
//
// Errors that occur in a meta transaction are propagated as a single error
// that can be captured by the Cadence runtime and eventually disambiguated by
// the parent context.
func (b *BootstrapProcedure) invokeMetaTransaction(
	parentCtx Context,
	tx *TransactionProcedure,
) (
	errors.CodedError,
	error,
) {
	invoker := NewTransactionInvoker()

	// do not deduct fees or check storage in meta transactions
	ctx := NewContextFromParent(parentCtx,
		WithAccountStorageLimit(false),
		WithTransactionFeesEnabled(false),
	)

	// use new programs for each meta transaction.
	// It's not necessary to use a program cache during bootstrapping and most transactions are contract deploys anyway.
	prog, err := programs.
		NewEmptyBlockPrograms().
		NewTransactionPrograms(0, 0)

	if err != nil {
		return nil, err
	}

	err = invoker.Process(ctx, tx, b.txnState, prog)
	txErr, fatalErr := errors.SplitErrorTypes(err)

	return txErr, fatalErr
}
