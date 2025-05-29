package fvm

import (
	"fmt"
	"math"
	"strings"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/onflow/flow-core-contracts/lib/go/contracts"
	"github.com/onflow/flow-core-contracts/lib/go/templates"

	bridge "github.com/onflow/flow-evm-bridge"
	storefront "github.com/onflow/nft-storefront/lib/go/contracts"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/evm/stdlib"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/migration"
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/storage/logical"
	"github.com/onflow/flow-go/fvm/systemcontracts"
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
	setupEVMEnabled                  cadence.Bool
	setupVMBridgeEnabled             cadence.Bool

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

// Option to deploy and setup the Flow VM bridge during bootstrapping
// so that assets can be bridged between Flow-Cadence and Flow-EVM
func WithSetupVMBridgeEnabled(enabled cadence.Bool) BootstrapProcedureOption {
	return func(bp *BootstrapProcedure) *BootstrapProcedure {
		bp.setupVMBridgeEnabled = enabled
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
			setupEVMEnabled:     true,
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
	if b.rootBlock == nil {
		b.rootBlock = flow.Genesis(b.ctx.Chain.ChainID()).Header
	}

	// initialize the account addressing state
	b.accountCreator = environment.NewBootstrapAccountCreator(
		b.txnState,
		b.ctx.Chain,
		environment.NewAccounts(b.txnState))

	expectAccounts := func(n uint64) error {
		ag := environment.NewAddressGenerator(b.txnState, b.ctx.Chain)
		currentAddresses := ag.AddressCount()
		if currentAddresses != n {
			return fmt.Errorf("expected %d accounts, got %d", n, currentAddresses)
		}
		return nil
	}

	service := b.createServiceAccount()

	err := expectAccounts(1)
	if err != nil {
		return err
	}

	env := templates.Environment{
		ServiceAccountAddress: service.String(),
	}

	b.deployViewResolver(service, &env)
	b.deployBurner(service, &env)
	b.deployCrypto(service, &env)

	err = expectAccounts(1)
	if err != nil {
		return err
	}

	fungibleToken := b.deployFungibleToken(&env)

	err = expectAccounts(systemcontracts.FungibleTokenAccountIndex)
	if err != nil {
		return err
	}

	nonFungibleToken := b.deployNonFungibleToken(service, &env)

	b.deployMetadataViews(fungibleToken, nonFungibleToken, &env)
	b.deployFungibleTokenSwitchboard(fungibleToken, &env)

	// deploys the NFTStorefrontV2 contract
	b.deployNFTStorefrontV2(nonFungibleToken, &env)

	flowToken := b.deployFlowToken(service, &env)
	err = expectAccounts(systemcontracts.FlowTokenAccountIndex)
	if err != nil {
		return err
	}

	b.deployStorageFees(service, &env)
	feeContract := b.deployFlowFees(service, &env)
	err = expectAccounts(systemcontracts.FlowFeesAccountIndex)
	if err != nil {
		return err
	}

	if b.initialTokenSupply > 0 {
		b.mintInitialTokens(service, fungibleToken, flowToken, b.initialTokenSupply)
	}

	b.deployExecutionParameters(fungibleToken, &env)
	b.setupExecutionWeights(fungibleToken)
	b.deployServiceAccount(service, &env)

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

	b.setupStorageForServiceAccounts(service, fungibleToken, flowToken, feeContract)

	b.createMinter(service, flowToken)

	b.deployDKG(service, &env)

	b.deployQC(service, &env)

	b.deployIDTableStaking(service, &env)

	b.deployEpoch(service, &env)

	b.deployVersionBeacon(service, b.versionFreezePeriod, &env)

	b.deployRandomBeaconHistory(service, &env)

	// deploy staking proxy contract to the service account
	b.deployStakingProxyContract(service, &env)

	// deploy locked tokens contract to the service account
	b.deployLockedTokensContract(service, &env)

	// deploy staking collection contract to the service account
	b.deployStakingCollection(service, &env)

	// sets up the EVM environment
	b.setupEVM(service, nonFungibleToken, fungibleToken, flowToken, &env)
	b.setupVMBridge(service, &env)

	b.deployCrossVMMetadataViews(nonFungibleToken, &env)

	err = expectAccounts(systemcontracts.EVMStorageAccountIndex)
	if err != nil {
		return err
	}

	b.registerNodes(service, fungibleToken, flowToken)

	// set the list of nodes which are allowed to stake in this network
	b.setStakingAllowlist(service, b.identities.NodeIDs())

	b.deployMigrationContract(service)

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

func (b *bootstrapExecutor) deployFungibleToken(env *templates.Environment) flow.Address {
	fungibleToken := b.createAccount(b.accountKeys.FungibleTokenAccountPublicKeys)

	contract := contracts.FungibleToken(*env)

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployFungibleTokenContractTransaction(fungibleToken, contract),
			0),
	)
	env.FungibleTokenAddress = fungibleToken.String()
	panicOnMetaInvokeErrf("failed to deploy fungible token contract: %s", txError, err)
	return fungibleToken
}

func (b *bootstrapExecutor) deployNonFungibleToken(deployTo flow.Address, env *templates.Environment) flow.Address {

	contract := contracts.NonFungibleToken(*env)

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployNonFungibleTokenContractTransaction(deployTo, contract),
			0),
	)
	env.NonFungibleTokenAddress = deployTo.String()
	panicOnMetaInvokeErrf("failed to deploy non-fungible token contract: %s", txError, err)
	return deployTo
}

func (b *bootstrapExecutor) deployViewResolver(deployTo flow.Address, env *templates.Environment) {

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployViewResolverContractTransaction(deployTo),
			0),
	)
	env.ViewResolverAddress = deployTo.String()
	panicOnMetaInvokeErrf("failed to deploy view resolver contract: %s", txError, err)
}

func (b *bootstrapExecutor) deployBurner(deployTo flow.Address, env *templates.Environment) {

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployBurnerContractTransaction(deployTo),
			0),
	)
	env.BurnerAddress = deployTo.String()
	panicOnMetaInvokeErrf("failed to deploy burner contract: %s", txError, err)
}

func (b *bootstrapExecutor) deployCrypto(deployTo flow.Address, env *templates.Environment) {
	contract := contracts.Crypto()

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployContractTransaction(
				deployTo,
				contract,
				"Crypto"),
			0),
	)
	env.CryptoAddress = deployTo.String()
	panicOnMetaInvokeErrf("failed to deploy crypto contract: %s", txError, err)
}

func (b *bootstrapExecutor) deployMetadataViews(fungibleToken, nonFungibleToken flow.Address, env *templates.Environment) {

	mvContract := contracts.MetadataViews(*env)

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployMetadataViewsContractTransaction(nonFungibleToken, mvContract),
			0),
	)
	env.MetadataViewsAddress = nonFungibleToken.String()
	panicOnMetaInvokeErrf("failed to deploy metadata views contract: %s", txError, err)

	ftmvContract := contracts.FungibleTokenMetadataViews(*env)

	txError, err = b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployFungibleTokenMetadataViewsContractTransaction(fungibleToken, ftmvContract),
			0),
	)
	env.FungibleTokenMetadataViewsAddress = fungibleToken.String()
	panicOnMetaInvokeErrf("failed to deploy fungible token metadata views contract: %s", txError, err)
}

func (b *bootstrapExecutor) deployCrossVMMetadataViews(nonFungibleToken flow.Address, env *templates.Environment) {
	if !bool(b.setupEVMEnabled) ||
		!bool(b.setupVMBridgeEnabled) ||
		!b.ctx.Chain.ChainID().Transient() {
		return
	}

	crossVMMVContract := contracts.CrossVMMetadataViews(*env)

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployCrossVMMetadataViewsContractTransaction(nonFungibleToken, crossVMMVContract),
			0),
	)
	env.CrossVMMetadataViewsAddress = nonFungibleToken.String()
	panicOnMetaInvokeErrf("failed to deploy cross VM metadata views contract: %s", txError, err)
}

func (b *bootstrapExecutor) deployFungibleTokenSwitchboard(deployTo flow.Address, env *templates.Environment) {

	contract := contracts.FungibleTokenSwitchboard(*env)

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployFungibleTokenSwitchboardContractTransaction(deployTo, contract),
			0),
	)
	env.FungibleTokenSwitchboardAddress = deployTo.String()
	panicOnMetaInvokeErrf("failed to deploy fungible token switchboard contract: %s", txError, err)
}

func (b *bootstrapExecutor) deployFlowToken(service flow.Address, env *templates.Environment) flow.Address {
	flowToken := b.createAccount(b.accountKeys.FlowTokenAccountPublicKeys)

	contract := contracts.FlowToken(*env)

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployFlowTokenContractTransaction(
				service,
				flowToken,
				contract),
			0),
	)
	env.FlowTokenAddress = flowToken.String()
	panicOnMetaInvokeErrf("failed to deploy Flow token contract: %s", txError, err)
	return flowToken
}

func (b *bootstrapExecutor) deployFlowFees(service flow.Address, env *templates.Environment) flow.Address {
	flowFees := b.createAccount(b.accountKeys.FlowFeesAccountPublicKeys)

	contract := contracts.FlowFees(
		*env,
	)

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployTxFeesContractTransaction(
				flowFees,
				service,
				contract,
			),
			0),
	)
	env.FlowFeesAddress = flowFees.String()
	panicOnMetaInvokeErrf("failed to deploy fees contract: %s", txError, err)
	return flowFees
}

func (b *bootstrapExecutor) deployStorageFees(deployTo flow.Address, env *templates.Environment) {
	contract := contracts.FlowStorageFees(*env)

	// deploy storage fees contract on the service account
	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployStorageFeesContractTransaction(
				deployTo,
				contract),
			0),
	)
	env.StorageFeesAddress = deployTo.String()
	panicOnMetaInvokeErrf("failed to deploy storage fees contract: %s", txError, err)
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

func (b *bootstrapExecutor) deployDKG(deployTo flow.Address, env *templates.Environment) {
	contract := contracts.FlowDKG()
	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployContractTransaction(deployTo, contract, "FlowDKG"),
			0,
		),
	)
	env.DkgAddress = deployTo.String()
	panicOnMetaInvokeErrf("failed to deploy DKG contract: %s", txError, err)
}

func (b *bootstrapExecutor) deployQC(deployTo flow.Address, env *templates.Environment) {
	contract := contracts.FlowQC()
	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployContractTransaction(deployTo, contract, "FlowClusterQC"),
			0,
		),
	)
	env.QuorumCertificateAddress = deployTo.String()
	panicOnMetaInvokeErrf("failed to deploy QC contract: %s", txError, err)
}

func (b *bootstrapExecutor) deployIDTableStaking(deployTo flow.Address, env *templates.Environment) {

	contract := contracts.FlowIDTableStaking(
		*env,
	)

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployIDTableStakingTransaction(deployTo,
				contract,
				b.epochConfig.EpochTokenPayout,
				b.epochConfig.RewardCut),
			0,
		),
	)
	env.IDTableAddress = deployTo.String()
	panicOnMetaInvokeErrf("failed to deploy IDTableStaking contract: %s", txError, err)
}

func (b *bootstrapExecutor) deployEpoch(deployTo flow.Address, env *templates.Environment) {

	contract := contracts.FlowEpoch(*env)

	context := NewContextFromParent(b.ctx,
		WithBlockHeader(b.rootBlock),
		WithBlocks(&environment.NoopBlockFinder{}),
	)

	txError, err := b.invokeMetaTransaction(
		context,
		Transaction(
			blueprints.DeployEpochTransaction(deployTo, contract, b.epochConfig),
			0,
		),
	)
	env.EpochAddress = deployTo.String()
	panicOnMetaInvokeErrf("failed to deploy Epoch contract: %s", txError, err)
}

func (b *bootstrapExecutor) deployExecutionParameters(deployTo flow.Address, env *templates.Environment) {
	contract := contracts.FlowExecutionParameters(*env)
	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployContractTransaction(deployTo, contract, "FlowExecutionParameters"),
			0,
		),
	)
	env.FlowExecutionParametersAddress = deployTo.String()
	panicOnMetaInvokeErrf("failed to deploy FlowExecutionParameters contract: %s", txError, err)
}

func (b *bootstrapExecutor) deployServiceAccount(deployTo flow.Address, env *templates.Environment) {
	contract := contracts.FlowServiceAccount(
		*env,
	)

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployContractTransaction(
				deployTo,
				contract,
				"FlowServiceAccount"),
			0),
	)
	panicOnMetaInvokeErrf("failed to deploy service account contract: %s", txError, err)
}

func (b *bootstrapExecutor) deployNFTStorefrontV2(deployTo flow.Address, env *templates.Environment) {

	contract := storefront.NFTStorefrontV2(
		env.FungibleTokenAddress,
		env.NonFungibleTokenAddress)

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployContractTransaction(
				deployTo,
				contract,
				"NFTStorefrontV2"),
			0),
	)
	panicOnMetaInvokeErrf("failed to deploy NFTStorefrontV2 contract: %s", txError, err)
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

func (b *bootstrapExecutor) setupExecutionWeights(parametersAccount flow.Address) {
	// if executionEffortWeights were not set skip this part and just use the defaults.
	if b.executionEffortWeights != nil {
		b.setupExecutionEffortWeights(parametersAccount)
	}
	// if executionMemoryWeights were not set skip this part and just use the defaults.
	if b.executionMemoryWeights != nil {
		b.setupExecutionMemoryWeights(parametersAccount)
	}
	if b.executionMemoryLimit != 0 {
		b.setExecutionMemoryLimitTransaction(parametersAccount)
	}
}

func (b *bootstrapExecutor) setupExecutionEffortWeights(parametersAccount flow.Address) {
	weights := b.executionEffortWeights

	uintWeights := make(map[uint]uint64, len(weights))
	for i, weight := range weights {
		uintWeights[uint(i)] = weight
	}

	tb, err := blueprints.SetExecutionEffortWeightsTransaction(parametersAccount, uintWeights)
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

func (b *bootstrapExecutor) setupExecutionMemoryWeights(parametersAccount flow.Address) {
	weights := b.executionMemoryWeights

	uintWeights := make(map[uint]uint64, len(weights))
	for i, weight := range weights {
		uintWeights[uint(i)] = weight
	}

	tb, err := blueprints.SetExecutionMemoryWeightsTransaction(parametersAccount, uintWeights)
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

func (b *bootstrapExecutor) setExecutionMemoryLimitTransaction(parametersAccount flow.Address) {

	tb, err := blueprints.SetExecutionMemoryLimitTransaction(parametersAccount, b.executionMemoryLimit)
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

func (b *bootstrapExecutor) setupEVM(serviceAddress, nonFungibleTokenAddress, fungibleTokenAddress, flowTokenAddress flow.Address, env *templates.Environment) {
	if b.setupEVMEnabled {
		// account for storage
		// we dont need to deploy anything to this account, but it needs to exist
		// so that we can store the EVM state on it
		evmAcc := b.createAccount(nil)
		b.setupStorageForAccount(evmAcc, serviceAddress, fungibleTokenAddress, flowTokenAddress)

		// deploy the EVM contract to the service account
		tx := blueprints.DeployContractTransaction(
			serviceAddress,
			stdlib.ContractCode(nonFungibleTokenAddress, fungibleTokenAddress, flowTokenAddress),
			stdlib.ContractName,
		)
		// WithEVMEnabled should only be used after we create an account for storage
		txError, err := b.invokeMetaTransaction(
			NewContextFromParent(b.ctx, WithEVMEnabled(true)),
			Transaction(tx, 0),
		)
		panicOnMetaInvokeErrf("failed to deploy EVM contract: %s", txError, err)

		env.EVMAddress = env.ServiceAccountAddress
	}
}

type stubEntropyProvider struct{}

func (stubEntropyProvider) RandomSource() ([]byte, error) {
	return []byte{0}, nil
}

func (b *bootstrapExecutor) setupVMBridge(serviceAddress flow.Address, env *templates.Environment) {
	// only setup VM bridge for transient networks
	// this is because the evm storage account for testnet and mainnet do not exist yet after boostrapping
	if !bool(b.setupEVMEnabled) ||
		!bool(b.setupVMBridgeEnabled) ||
		!b.ctx.Chain.ChainID().Transient() {
		return
	}

	bridgeEnv := bridge.Environment{
		CrossVMNFTAddress:                     env.ServiceAccountAddress,
		CrossVMTokenAddress:                   env.ServiceAccountAddress,
		FlowEVMBridgeHandlerInterfacesAddress: env.ServiceAccountAddress,
		IBridgePermissionsAddress:             env.ServiceAccountAddress,
		ICrossVMAddress:                       env.ServiceAccountAddress,
		ICrossVMAssetAddress:                  env.ServiceAccountAddress,
		IEVMBridgeNFTMinterAddress:            env.ServiceAccountAddress,
		IEVMBridgeTokenMinterAddress:          env.ServiceAccountAddress,
		IFlowEVMNFTBridgeAddress:              env.ServiceAccountAddress,
		IFlowEVMTokenBridgeAddress:            env.ServiceAccountAddress,
		FlowEVMBridgeAddress:                  env.ServiceAccountAddress,
		FlowEVMBridgeAccessorAddress:          env.ServiceAccountAddress,
		FlowEVMBridgeConfigAddress:            env.ServiceAccountAddress,
		FlowEVMBridgeHandlersAddress:          env.ServiceAccountAddress,
		FlowEVMBridgeNFTEscrowAddress:         env.ServiceAccountAddress,
		FlowEVMBridgeResolverAddress:          env.ServiceAccountAddress,
		FlowEVMBridgeTemplatesAddress:         env.ServiceAccountAddress,
		FlowEVMBridgeTokenEscrowAddress:       env.ServiceAccountAddress,
		FlowEVMBridgeUtilsAddress:             env.ServiceAccountAddress,
		ArrayUtilsAddress:                     env.ServiceAccountAddress,
		ScopedFTProvidersAddress:              env.ServiceAccountAddress,
		SerializeAddress:                      env.ServiceAccountAddress,
		SerializeMetadataAddress:              env.ServiceAccountAddress,
		StringUtilsAddress:                    env.ServiceAccountAddress,
	}

	ctx := NewContextFromParent(b.ctx,
		WithBlockHeader(b.rootBlock),
		WithEntropyProvider(stubEntropyProvider{}),
		WithEVMEnabled(true),
	)

	run := func(tx *flow.TransactionBody, failMSG string) {
		txError, err := b.invokeMetaTransaction(
			ctx,
			Transaction(tx, 0),
		)

		panicOnMetaInvokeErrf(failMSG, txError, err)
	}

	runAndReturn := func(tx *flow.TransactionBody, failMSG string) ProcedureOutput {
		txOutput, err := b.runMetaTransaction(
			ctx,
			Transaction(tx, 0),
		)

		if err != nil {
			panic(fmt.Sprintf(failMSG, err.Error()))
		}
		if txOutput.Err != nil {
			panic(fmt.Sprintf(failMSG, txOutput.Err.Error()))
		}

		return txOutput
	}

	// Create a COA in the bridge account
	run(blueprints.CreateCOATransaction(*env, bridgeEnv, serviceAddress), "failed to create COA in Service Account: %s")

	// Arbitrary high gas limit that can be used for all the
	// EVM transactions to ensure none of them run out of gas
	gasLimit := 15000000
	deploymentValue := 0.0

	// Retrieve the factory bytecode from the JSON args
	factoryBytecode := bridge.GetBytecodeFromArgsJSON("cadence/args/deploy-factory-args.json")

	// deploy the Solidity Factory contract to the service account's COA
	txOutput := runAndReturn(blueprints.DeployEVMContractTransaction(*env, bridgeEnv, serviceAddress, factoryBytecode, gasLimit, deploymentValue), "failed to deploy the Factory in the Service Account COA: %s")

	factoryAddress, err := getContractAddressFromEVMEvent(txOutput)
	if err != nil {
		panic(fmt.Sprintf("failed to deploy Solidity Factory contract: %s", err))
	}

	// Retrieve the registry bytecode from the JSON args
	registryBytecode := bridge.GetBytecodeFromArgsJSON("cadence/args/deploy-deployment-registry-args.json")

	// deploy the Solidity Registry contract to the service account's COA
	txOutput = runAndReturn(blueprints.DeployEVMContractTransaction(*env, bridgeEnv, serviceAddress, registryBytecode, gasLimit, deploymentValue), "failed to deploy the Registry in the Service Account COA: %s")

	registryAddress, err := getContractAddressFromEVMEvent(txOutput)
	if err != nil {
		panic(fmt.Sprintf("failed to deploy Solidity Registry contract: %s", err))
	}

	// Retrieve the erc20Deployer bytecode from the JSON args
	erc20DeployerBytecode := bridge.GetBytecodeFromArgsJSON("cadence/args/deploy-erc20-deployer-args.json")

	// deploy the Solidity ERC20 Deployer contract to the service account's COA
	txOutput = runAndReturn(blueprints.DeployEVMContractTransaction(*env, bridgeEnv, serviceAddress, erc20DeployerBytecode, gasLimit, deploymentValue), "failed to deploy the ERC20 Deployer in the Service Account COA: %s")

	erc20DeployerAddress, err := getContractAddressFromEVMEvent(txOutput)
	if err != nil {
		panic(fmt.Sprintf("failed to deploy ERC20 deployer contract: %s", err))
	}

	erc721DeployerBytecode := bridge.GetBytecodeFromArgsJSON("cadence/args/deploy-erc721-deployer-args.json")

	// deploy the ERC721 deployer contract to the service account's COA
	txOutput = runAndReturn(blueprints.DeployEVMContractTransaction(*env, bridgeEnv, serviceAddress, erc721DeployerBytecode, gasLimit, deploymentValue), "failed to deploy the ERC721 Deployer in the Service Account COA: %s")

	erc721DeployerAddress, err := getContractAddressFromEVMEvent(txOutput)
	if err != nil {
		panic(fmt.Sprintf("failed to deploy ERC 721 deployer contract: %s", err))
	}

	for _, path := range blueprints.BridgeContracts {

		contract, _ := bridge.GetCadenceContractCode(path, bridgeEnv, *env)

		slashSplit := strings.Split(path, "/")
		nameWithCDC := slashSplit[len(slashSplit)-1]
		name := nameWithCDC[:len(nameWithCDC)-4]

		if name == "FlowEVMBridgeUtils" {
			txError, err := b.invokeMetaTransaction(
				ctx,
				Transaction(
					blueprints.DeployFlowEVMBridgeUtilsContractTransaction(*env, &bridgeEnv, serviceAddress, contract, name, factoryAddress),
					0),
			)
			panicOnMetaInvokeErrf("failed to deploy FlowEVMBridgeUtils contract: %s", txError, err)
		} else {
			txError, err := b.invokeMetaTransaction(
				ctx,
				Transaction(
					blueprints.DeployContractTransaction(serviceAddress, contract, name),
					0),
			)
			panicOnMetaInvokeErrf("failed to deploy "+name+" contract: %s", txError, err)
		}
	}

	// Pause the bridge for setup
	run(blueprints.PauseBridgeTransaction(*env, bridgeEnv, serviceAddress, true),
		"failed to pause the bridge contracts: %s")

	// Set the factory as registrar in the registry
	run(blueprints.SetRegistrarTransaction(*env, bridgeEnv, serviceAddress, registryAddress),
		"failed to set the factory as registrar: %s")

	// Add the registry to the factory
	run(blueprints.SetDeploymentRegistryTransaction(*env, bridgeEnv, serviceAddress, registryAddress),
		"failed to add the registry to the factory: %s")

	// Set the factory as delegated deployer in the ERC20 deployer
	run(blueprints.SetDelegatedDeployerTransaction(*env, bridgeEnv, serviceAddress, erc20DeployerAddress),
		"failed to set the erc20 deployer as delegated deployer: %s")

	// Set the factory as delegated deployer in the ERC721 deployer
	run(blueprints.SetDelegatedDeployerTransaction(*env, bridgeEnv, serviceAddress, erc721DeployerAddress),
		"failed to set the erc721 deployer as delegated deployer: %s")

	// Add the ERC20 Deployer as a deployer in the factory
	run(blueprints.AddDeployerTransaction(*env, bridgeEnv, serviceAddress, "ERC20", erc20DeployerAddress),
		"failed to add the erc20 deployer in the factory: %s")

	// Add the ERC721 Deployer as a deployer in the factory
	run(blueprints.AddDeployerTransaction(*env, bridgeEnv, serviceAddress, "ERC721", erc721DeployerAddress),
		"failed to add the erc721 deployer in the factory: %s")

	// 	/* --- EVM Contract Integration --- */

	// Deploy FlowEVMBridgeAccessor, providing EVM contract host (network service account) as argument
	run(
		blueprints.DeployFlowEVMBridgeAccessorContractTransaction(*env, bridgeEnv, serviceAddress),
		"failed to deploy FlowEVMBridgeAccessor contract: %s")

	// Integrate the EVM contract with the BridgeAccessor
	run(blueprints.IntegrateEVMWithBridgeAccessorTransaction(*env, bridgeEnv, serviceAddress),
		"failed to integrate the EVM contract with the BridgeAccessor: %s")

	// Set the bridge onboarding fees
	run(blueprints.UpdateOnboardFeeTransaction(*env, bridgeEnv, serviceAddress, 1.0),
		"failed to update the bridge onboarding fees: %s")

	// Set the bridge base fee
	run(blueprints.UpdateBaseFeeTransaction(*env, bridgeEnv, serviceAddress, 0.001),
		"failed to update the bridge base fees: %s")

	tokenChunks := bridge.GetCadenceTokenChunkedJSONArguments(false)
	nftChunks := bridge.GetCadenceTokenChunkedJSONArguments(true)

	// Add the FT Template Cadence Code Chunks
	run(blueprints.UpsertContractCodeChunksTransaction(*env, bridgeEnv, serviceAddress, "bridgedToken", tokenChunks),
		"failed to add the FT template code chunks: %s")

	// Add the NFT Template Cadence Code Chunks
	run(blueprints.UpsertContractCodeChunksTransaction(*env, bridgeEnv, serviceAddress, "bridgedNFT", nftChunks),
		"failed to add the NFT template code chunks: %s")

	// Retrieve the WFLOW bytecode from the JSON args
	wflowBytecode, err := bridge.GetSolidityContractCode("WFLOW")
	if err != nil {
		panic(fmt.Sprintf("failed to get WFLOW bytecode: %s", err))
	}

	// deploy the WFLOW contract to the service account's COA
	txOutput = runAndReturn(blueprints.DeployEVMContractTransaction(*env, bridgeEnv, serviceAddress, wflowBytecode, gasLimit, deploymentValue),
		"failed to deploy the WFLOW contract in the Service Account COA: %s")

	wflowAddress, err := getContractAddressFromEVMEvent(txOutput)
	if err != nil {
		panic(fmt.Sprintf("failed to deploy WFLOW contract: %s", err))
	}

	// Create WFLOW Token Handler, supplying the WFLOW EVM address
	run(blueprints.CreateWFLOWTokenHandlerTransaction(*env, bridgeEnv, serviceAddress, wflowAddress),
		"failed to create the WFLOW token handler: %s")

	// Enable WFLOW Token Handler, supplying the Cadence FlowToken.Vault type
	flowVaultType := "A." + env.FlowTokenAddress + ".FlowToken.Vault"

	run(blueprints.EnableWFLOWTokenHandlerTransaction(*env, bridgeEnv, serviceAddress, flowVaultType),
		"failed to enable the WFLOW token handler: %s")

	// Unpause the bridge
	run(blueprints.PauseBridgeTransaction(*env, bridgeEnv, serviceAddress, false),
		"failed to un-pause the bridge contracts: %s")
}

// getContractAddressFromEVMEvent gets the deployment address from a evm deployment transaction
func getContractAddressFromEVMEvent(output ProcedureOutput) (string, error) {
	for _, event := range output.Events {
		if strings.Contains(string(event.Type), "TransactionExecuted") {
			// decode the event payload
			data, _ := ccf.Decode(nil, event.Payload)
			// get the contractAddress field from the event
			contractAddr := cadence.SearchFieldByName(
				data.(cadence.Event),
				"contractAddress",
			).(cadence.String)

			if contractAddr.String() == "" {
				return "", fmt.Errorf(
					"Contract address not found in event")
			}
			address := strings.ToLower(strings.TrimPrefix(contractAddr.String(), "0x"))
			// For some reason, there are extra quotations here in the address
			// so the first and last character needs to be removed for it to only be the address
			return address[1 : len(address)-1], nil
		}
	}
	return "", fmt.Errorf(
		"No TransactionExecuted event found in the output of the transaction.")
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
			Transaction(blueprints.FundAccountTransaction(
				service,
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
			Transaction(blueprints.RegisterNodeTransaction(
				service,
				flowToken,
				fungibleToken,
				nodeAddress,
				id),
				0),
		)
		panicOnMetaInvokeErrf("failed to register node: %s", txError, err)
	}
}

func (b *bootstrapExecutor) deployStakingProxyContract(deployTo flow.Address, env *templates.Environment) {
	contract := contracts.FlowStakingProxy()
	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployContractTransaction(deployTo, contract, "StakingProxy"),
			0,
		),
	)
	env.StakingProxyAddress = deployTo.String()
	panicOnMetaInvokeErrf("failed to deploy StakingProxy contract: %s", txError, err)
}

func (b *bootstrapExecutor) deployVersionBeacon(
	deployTo flow.Address,
	versionFreezePeriod cadence.UInt64,
	env *templates.Environment,
) {
	tx := blueprints.DeployNodeVersionBeaconTransaction(deployTo, versionFreezePeriod)
	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			tx,
			0,
		),
	)
	env.NodeVersionBeaconAddress = deployTo.String()
	panicOnMetaInvokeErrf("failed to deploy NodeVersionBeacon contract: %s", txError, err)
}

func (b *bootstrapExecutor) deployRandomBeaconHistory(
	deployTo flow.Address,
	env *templates.Environment,
) {
	tx := blueprints.DeployRandomBeaconHistoryTransaction(deployTo)
	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			tx,
			0,
		),
	)
	env.RandomBeaconHistoryAddress = deployTo.String()
	panicOnMetaInvokeErrf("failed to deploy RandomBeaconHistory history contract: %s", txError, err)
}

func (b *bootstrapExecutor) deployLockedTokensContract(
	deployTo flow.Address,
	env *templates.Environment,
) {

	publicKeys, err := flow.EncodeRuntimeAccountPublicKeys(b.accountKeys.ServiceAccountPublicKeys)
	if err != nil {
		panic(err)
	}

	contract := contracts.FlowLockedTokens(*env)

	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployLockedTokensTransaction(deployTo, contract, publicKeys),
			0,
		),
	)
	env.LockedTokensAddress = deployTo.String()
	panicOnMetaInvokeErrf("failed to deploy LockedTokens contract: %s", txError, err)
}

func (b *bootstrapExecutor) deployStakingCollection(
	deployTo flow.Address,
	env *templates.Environment,
) {
	contract := contracts.FlowStakingCollection(*env)
	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployContractTransaction(deployTo, contract, "FlowStakingCollection"),
			0,
		),
	)
	env.StakingCollectionAddress = deployTo.String()
	panicOnMetaInvokeErrf("failed to deploy FlowStakingCollection contract: %s", txError, err)
}

func (b *bootstrapExecutor) deployMigrationContract(deployTo flow.Address) {
	tx := blueprints.DeployContractTransaction(
		deployTo,
		migration.ContractCode(),
		migration.ContractName,
	)
	txError, err := b.invokeMetaTransaction(
		b.ctx,
		Transaction(tx, 0),
	)
	panicOnMetaInvokeErrf("failed to deploy Migration contract: %s", txError, err)
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
	output, err := b.runMetaTransaction(parentCtx, tx)
	if err != nil {
		return nil, err
	}

	return output.Err, err
}

// runMetaTransaction invokes a meta transaction inside the context of an
// outer transaction.
func (b *bootstrapExecutor) runMetaTransaction(
	parentCtx Context,
	tx *TransactionProcedure,
) (
	ProcedureOutput,
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

	return executor.Output(), err
}
