package fvm

import (
	"fmt"
	"math"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-core-contracts/lib/go/contracts"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/evm/stdlib"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/storage/logical"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/epochs"
)

var (
	DefaultAccountCreationFee = mustParseUFix64(
		"account creation fee",
		"0.00100000")

	DefaultMinimumStorageReservation = mustParseUFix64(
		"minimum storage reservation",
		"0.00100000")

	DefaultStorageMBPerFLOW = mustParseUFix64(
		"storage mb per flow",
		"100.00000000")

	// DefaultTransactionFees are the default transaction fee parameters if
	// transaction fees are on. Surge factor is 1.0, inclusion effort cost is
	// 0.0001 (because the static inclusion effort is 1.0) and execution effort
	// cost is 0.0 because dynamic execution fees are off. If they are off
	// (which is the default behaviour) that means the transaction fees are 0.0.
	DefaultTransactionFees = BootstrapProcedureFeeParameters{
		SurgeFactor: mustParseUFix64("fee surge factor", "1.0"),
		InclusionEffortCost: mustParseUFix64(
			"fee inclusion effort cost",
			"0.00001"),
		ExecutionEffortCost: mustParseUFix64(
			"fee execution effort cost",
			"0.0"),
	}

	// DefaultVersionFreezePeriod is the default NodeVersionBeacon freeze period -
	// the number of blocks in the future where the version changes are frozen.
	DefaultVersionFreezePeriod = cadence.UInt64(1000)
)

func mustParseUFix64(name string, valueString string) cadence.UFix64 {
	value, err := cadence.NewUFix64(valueString)
	if err != nil {
		panic(fmt.Errorf("invalid default %s: %w", name, err))
	}
	return value
}

// A BootstrapProcedure is an invokable that can be used to bootstrap the ledger state
// with the default accounts and contracts required by the Flow virtual machine.
type BootstrapProcedure struct {
	BootstrapParams
}

type BootstrapParams struct {
	rootBlock *flow.Header

	// genesis parameters
	accountKeys        BootstrapAccountKeys
	initialTokenSupply cadence.UFix64

	accountCreationFee               cadence.UFix64
	minimumStorageReservation        cadence.UFix64
	storagePerFlow                   cadence.UFix64
	restrictedAccountCreationEnabled cadence.Bool

	// `setupEVMEnabled` == true && `evmAbiOnly` == true will enable the ABI-only EVM
	// `setupEVMEnabled` == true && `evmAbiOnly` == false will enable the full EVM functionality
	// `setupEVMEnabled` == false will disable EVM
	// This will allow to quickly disable the ABI-only EVM, in case there's a bug or something.
	setupEVMEnabled cadence.Bool
	evmAbiOnly      cadence.Bool

	// versionFreezePeriod is the number of blocks in the future where the version
	// changes are frozen. The Node version beacon manages the freeze period,
	// but this is the value used when first deploying the contract, during the
	// bootstrap procedure.
	versionFreezePeriod cadence.UInt64

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

func WithSetupEVMEnabled(enabled cadence.Bool) BootstrapProcedureOption {
	return func(bp *BootstrapProcedure) *BootstrapProcedure {
		bp.setupEVMEnabled = enabled
		return bp
	}
}

func WithEVMABIOnly(evmAbiOnly cadence.Bool) BootstrapProcedureOption {
	return func(bp *BootstrapProcedure) *BootstrapProcedure {
		bp.evmAbiOnly = evmAbiOnly
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
		BootstrapParams: BootstrapParams{
			accountKeys: BootstrapAccountKeys{
				ServiceAccountPublicKeys:       []flow.AccountPublicKey{serviceAccountPublicKey},
				FungibleTokenAccountPublicKeys: []flow.AccountPublicKey{serviceAccountPublicKey},
				FlowTokenAccountPublicKeys:     []flow.AccountPublicKey{serviceAccountPublicKey},
				NodeAccountPublicKeys:          []flow.AccountPublicKey{serviceAccountPublicKey},
			},
			transactionFees:     BootstrapProcedureFeeParameters{0, 0, 0},
			epochConfig:         epochs.DefaultEpochConfig(),
			versionFreezePeriod: DefaultVersionFreezePeriod,
		},
	}

	for _, applyOption := range opts {
		bootstrapProcedure = applyOption(bootstrapProcedure)
	}
	return bootstrapProcedure
}

func (b *BootstrapProcedure) NewExecutor(
	ctx Context,
	txnState storage.TransactionPreparer,
) ProcedureExecutor {
	return newBootstrapExecutor(b.BootstrapParams, ctx, txnState)
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

func (proc *BootstrapProcedure) ExecutionTime() logical.Time {
	return 0
}

type bootstrapExecutor struct {
	BootstrapParams

	ctx      Context
	txnState storage.TransactionPreparer

	accountCreator environment.BootstrapAccountCreator
}

func newBootstrapExecutor(
	params BootstrapParams,
	ctx Context,
	txnState storage.TransactionPreparer,
) *bootstrapExecutor {
	return &bootstrapExecutor{
		BootstrapParams: params,
		ctx: NewContextFromParent(
			ctx,
			WithContractDeploymentRestricted(false)),
		txnState: txnState,
	}
}

func (b *bootstrapExecutor) Cleanup() {
	// Do nothing.
}

func (b *bootstrapExecutor) Output() ProcedureOutput {
	return ProcedureOutput{}
}

func (b *bootstrapExecutor) Preprocess() error {
	// Do nothing.
	return nil
}

func (b *bootstrapExecutor) Execute() error {
	b.rootBlock = flow.Genesis(flow.ChainID(b.ctx.Chain.String())).Header

	// initialize the account addressing state
	b.accountCreator = environment.NewBootstrapAccountCreator(
		b.txnState,
		b.ctx.Chain,
		environment.NewAccounts(b.txnState))

	service := b.createServiceAccount()

	b.deployViewResolver(service)
	b.deployBurner(service)

	fungibleToken := b.deployFungibleToken(service, service)
	nonFungibleToken := b.deployNonFungibleToken(service, service)

	b.deployMetadataViews(fungibleToken, nonFungibleToken, service)

	flowToken := b.deployFlowToken(service, fungibleToken, nonFungibleToken)
	storageFees := b.deployStorageFees(service, fungibleToken, flowToken)
	feeContract := b.deployFlowFees(service, fungibleToken, flowToken, storageFees)

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

	b.deployIDTableStaking(service, fungibleToken, flowToken, feeContract, service)

	b.deployEpoch(service, fungibleToken, flowToken, feeContract)

	b.deployVersionBeacon(service, b.versionFreezePeriod)

	b.deployRandomBeaconHistory(service)

	// deploy staking proxy contract to the service account
	b.deployStakingProxyContract(service)

	// deploy locked tokens contract to the service account
	b.deployLockedTokensContract(service, fungibleToken, flowToken)

	// deploy staking collection contract to the service account
	b.deployStakingCollection(service, fungibleToken, flowToken)

	b.registerNodes(service, fungibleToken, flowToken)

	// set the list of nodes which are allowed to stake in this network
	b.setStakingAllowlist(service, b.identities.NodeIDs())

	// sets up the EVM environment
	b.setupEVM(service, fungibleToken, flowToken)

	return nil
}

func (b *bootstrapExecutor) createAccount(publicKeys []flow.AccountPublicKey) flow.Address {
	address, err := b.accountCreator.CreateBootstrapAccount(publicKeys)
	if err != nil {
		panic(fmt.Sprintf("failed to create account: %s", err))
	}

	return address
}

func (b *bootstrapExecutor) createServiceAccount() flow.Address {
	address, err := b.accountCreator.CreateBootstrapAccount(
		b.accountKeys.ServiceAccountPublicKeys)
	if err != nil {
		panic(fmt.Sprintf("failed to create service account: %s", err))
	}

	return address
}

func (b *bootstrapExecutor) deployFungibleToken(viewResolver, burner flow.Address) flow.Address {
	fungibleToken := b.createAccount(b.accountKeys.FungibleTokenAccountPublicKeys)

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployFungibleTokenContractTransaction(fungibleToken, viewResolver, burner),
			0),
	)
	panicOnMetaInvokeErrf("failed to deploy fungible token contract: %s", txError, err)
	return fungibleToken
}

func (b *bootstrapExecutor) deployNonFungibleToken(deployTo, viewResolver flow.Address) flow.Address {

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployNonFungibleTokenContractTransaction(deployTo, viewResolver),
			0),
	)
	panicOnMetaInvokeErrf("failed to deploy non-fungible token contract: %s", txError, err)
	return deployTo
}

func (b *bootstrapExecutor) deployViewResolver(deployTo flow.Address) {

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployViewResolverContractTransaction(deployTo),
			0),
	)
	panicOnMetaInvokeErrf("failed to deploy view resolver contract: %s", txError, err)
}

func (b *bootstrapExecutor) deployBurner(deployTo flow.Address) {

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployBurnerContractTransaction(deployTo),
			0),
	)
	panicOnMetaInvokeErrf("failed to deploy burner contract: %s", txError, err)
}

func (b *bootstrapExecutor) deployMetadataViews(fungibleToken, nonFungibleToken, viewResolver flow.Address) {

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployMetadataViewsContractTransaction(fungibleToken, nonFungibleToken, viewResolver),
			0),
	)
	panicOnMetaInvokeErrf("failed to deploy metadata views contract: %s", txError, err)

	txError, err = b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployFungibleTokenMetadataViewsContractTransaction(fungibleToken, nonFungibleToken, viewResolver),
			0),
	)
	panicOnMetaInvokeErrf("failed to deploy fungible token metadata views contract: %s", txError, err)
}

func (b *bootstrapExecutor) deployFlowToken(service, fungibleToken, metadataViews flow.Address) flow.Address {
	flowToken := b.createAccount(b.accountKeys.FlowTokenAccountPublicKeys)
	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployFlowTokenContractTransaction(
				service,
				fungibleToken,
				metadataViews,
				flowToken),
			0),
	)
	panicOnMetaInvokeErrf("failed to deploy Flow token contract: %s", txError, err)
	return flowToken
}

func (b *bootstrapExecutor) deployFlowFees(service, fungibleToken, flowToken, storageFees flow.Address) flow.Address {
	flowFees := b.createAccount(b.accountKeys.FlowFeesAccountPublicKeys)

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployTxFeesContractTransaction(
				service,
				fungibleToken,
				flowToken,
				storageFees,
				flowFees,
			),
			0),
	)
	panicOnMetaInvokeErrf("failed to deploy fees contract: %s", txError, err)
	return flowFees
}

func (b *bootstrapExecutor) deployStorageFees(service, fungibleToken, flowToken flow.Address) flow.Address {
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
	return service
}

func (b *bootstrapExecutor) createMinter(service, flowToken flow.Address) {
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

func (b *bootstrapExecutor) deployDKG(service flow.Address) {
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

func (b *bootstrapExecutor) deployQC(service flow.Address) {
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

func (b *bootstrapExecutor) deployIDTableStaking(service, fungibleToken, flowToken, flowFees, burner flow.Address) {

	contract := contracts.FlowIDTableStaking(
		fungibleToken.HexWithPrefix(),
		flowToken.HexWithPrefix(),
		flowFees.HexWithPrefix(),
		burner.HexWithPrefix(),
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

func (b *bootstrapExecutor) deployEpoch(service, fungibleToken, flowToken, flowFees flow.Address) {

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

func (b *bootstrapExecutor) deployServiceAccount(service, fungibleToken, flowToken, feeContract flow.Address) {
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

func (b *bootstrapExecutor) mintInitialTokens(
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

func (b *bootstrapExecutor) setupParameters(
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

func (b *bootstrapExecutor) setupFees(
	service, flowFees flow.Address,
	surgeFactor, inclusionEffortCost, executionEffortCost cadence.UFix64,
) {
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

func (b *bootstrapExecutor) setupExecutionWeights(service flow.Address) {
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

func (b *bootstrapExecutor) setupExecutionEffortWeights(service flow.Address) {
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

func (b *bootstrapExecutor) setupExecutionMemoryWeights(service flow.Address) {
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

func (b *bootstrapExecutor) setExecutionMemoryLimitTransaction(service flow.Address) {

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

func (b *bootstrapExecutor) setupStorageForServiceAccounts(
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

func (b *bootstrapExecutor) setupStorageForAccount(
	account, service, fungibleToken, flowToken flow.Address,
) {
	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.SetupStorageForAccountTransaction(
				account,
				service,
				fungibleToken,
				flowToken),
			0),
	)
	panicOnMetaInvokeErrf("failed to setup storage for service accounts: %s", txError, err)
}

func (b *bootstrapExecutor) setStakingAllowlist(
	service flow.Address,
	allowedIDs []flow.Identifier,
) {

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

func (b *bootstrapExecutor) setupEVM(serviceAddress, fungibleTokenAddress, flowTokenAddress flow.Address) {
	if b.setupEVMEnabled {
		evmAcc := b.createAccount(nil) // account for storage
		tx := blueprints.DeployContractTransaction(
			serviceAddress,
			stdlib.ContractCode(flowTokenAddress, bool(b.evmAbiOnly)),
			stdlib.ContractName,
		)
		// WithEVMEnabled should only be used after we create an account for storage
		txError, err := b.invokeMetaTransaction(
			NewContextFromParent(b.ctx, WithEVMEnabled(true)),
			Transaction(tx, 0),
		)
		panicOnMetaInvokeErrf("failed to deploy EVM contract: %s", txError, err)

		b.setupStorageForAccount(evmAcc, serviceAddress, fungibleTokenAddress, flowTokenAddress)
	}
}

func (b *bootstrapExecutor) registerNodes(service, fungibleToken, flowToken flow.Address) {
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
				fungibleToken,
				nodeAddress,
				id),
				0),
		)
		panicOnMetaInvokeErrf("failed to register node: %s", txError, err)
	}
}

func (b *bootstrapExecutor) deployStakingProxyContract(service flow.Address) {
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

func (b *bootstrapExecutor) deployVersionBeacon(
	service flow.Address,
	versionFreezePeriod cadence.UInt64,
) {
	tx := blueprints.DeployNodeVersionBeaconTransaction(service, versionFreezePeriod)
	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			tx,
			0,
		),
	)
	panicOnMetaInvokeErrf("failed to deploy NodeVersionBeacon contract: %s", txError, err)
}

func (b *bootstrapExecutor) deployRandomBeaconHistory(
	service flow.Address,
) {
	tx := blueprints.DeployRandomBeaconHistoryTransaction(service)
	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			tx,
			0,
		),
	)
	panicOnMetaInvokeErrf("failed to deploy RandomBeaconHistory history contract: %s", txError, err)
}

func (b *bootstrapExecutor) deployLockedTokensContract(
	service flow.Address, fungibleTokenAddress,
	flowTokenAddress flow.Address,
) {

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

func (b *bootstrapExecutor) deployStakingCollection(
	service flow.Address,
	fungibleTokenAddress, flowTokenAddress flow.Address,
) {
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

func (b *bootstrapExecutor) setContractDeploymentRestrictions(
	service flow.Address,
	deployment *bool,
) {
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

// invokeMetaTransaction invokes a meta transaction inside the context of an
// outer transaction.
//
// Errors that occur in a meta transaction are propagated as a single error
// that can be captured by the Cadence runtime and eventually disambiguated by
// the parent context.
func (b *bootstrapExecutor) invokeMetaTransaction(
	parentCtx Context,
	tx *TransactionProcedure,
) (
	errors.CodedError,
	error,
) {
	// do not deduct fees or check storage in meta transactions
	ctx := NewContextFromParent(parentCtx,
		WithAccountStorageLimit(false),
		WithTransactionFeesEnabled(false),
		WithAuthorizationChecksEnabled(false),
		WithSequenceNumberCheckAndIncrementEnabled(false),

		// disable interaction and computation limits for bootstrapping
		WithMemoryAndInteractionLimitsDisabled(),
		WithComputationLimit(math.MaxUint64),
	)

	executor := tx.NewExecutor(ctx, b.txnState)
	err := Run(executor)

	return executor.Output().Err, err
}
