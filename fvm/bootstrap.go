package fvm

import (
	"fmt"
	"math"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-core-contracts/lib/go/contracts"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
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
	accounts  state.Accounts
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

// deployContractAuditVouchers deploys audit vouchers contract to the service account
func (b *BootstrapProcedure) deployContractAuditVouchers(service flow.Address) {
	contract := contracts.FlowContractAudits()

	txError, err := b.vm.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployContractTransaction(
				service,
				contract,
				"FlowContractAudits"),
			0),
		b.sth,
		b.programs,
	)
	panicOnMetaInvokeErrf("failed to deploy contract audit vouchers contract: %s", txError, err)
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
		Transaction(
			blueprints.DeployContractTransaction(service, contract, "FlowDKG"),
			0,
		),
		b.sth,
		b.programs,
	)
	panicOnMetaInvokeErrf("failed to deploy DKG contract: %s", txError, err)
}

func (b *BootstrapProcedure) deployQC(service flow.Address) {
	contract := contracts.FlowQC()
	txError, err := b.vm.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployContractTransaction(service, contract, "FlowClusterQC"),
			0,
		),
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
		Transaction(
			blueprints.DeployIDTableStakingTransaction(service,
				contract,
				b.epochConfig.EpochTokenPayout,
				b.epochConfig.RewardCut),
			0,
		),
		b.sth,
		b.programs,
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
		WithBlocks(&NoopBlockFinder{}),
	)

	txError, err := b.vm.invokeMetaTransaction(
		context,
		Transaction(
			blueprints.DeployEpochTransaction(service, contract, b.epochConfig),
			0,
		),
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
			Transaction(
				blueprints.SetupAccountTransaction(fungibleToken,
					flowToken,
					nodeAddress),
				0,
			),
			b.sth,
			b.programs,
		)
		panicOnMetaInvokeErrf("failed to setup machine account: %s", txError, err)

		// fund the staking account
		txError, err = b.vm.invokeMetaTransaction(
			b.ctx,
			Transaction(blueprints.FundAccountTransaction(service,
				fungibleToken,
				flowToken,
				nodeAddress),
				0),
			b.sth,
			b.programs,
		)
		panicOnMetaInvokeErrf("failed to fund node staking account: %s", txError, err)

		// register the node
		// for collection/consensus nodes this will also create the machine account
		// and set it up with the QC/DKG participant resource
		txError, err = b.vm.invokeMetaTransaction(
			b.ctx,
			Transaction(blueprints.RegisterNodeTransaction(service,
				flowToken,
				nodeAddress,
				id),
				0),
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
		Transaction(
			blueprints.DeployContractTransaction(service, contract, "StakingProxy"),
			0,
		),
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
	publicKeys[0] = blueprints.BytesToCadenceArray(encodedPublicKey)

	contract := contracts.FlowLockedTokens(
		fungibleTokenAddress.Hex(),
		flowTokenAddress.Hex(),
		service.Hex(),
		service.Hex(),
		service.Hex())

	txError, err := b.vm.invokeMetaTransaction(
		b.ctx,
		Transaction(
			blueprints.DeployLockedTokensTransaction(service, contract, publicKeys),
			0,
		),
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
		Transaction(
			blueprints.DeployContractTransaction(service, contract, "FlowStakingCollection"),
			0,
		),
		b.sth,
		b.programs,
	)
	panicOnMetaInvokeErrf("failed to deploy FlowStakingCollection contract: %s", txError, err)
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
