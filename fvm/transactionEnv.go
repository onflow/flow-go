package fvm

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"

	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/handler"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/utils"
	"github.com/onflow/flow-go/model/flow"
)

var _ runtime.Interface = &TransactionEnv{}

// TransactionEnv is a read-write environment used for executing flow transactions.
type TransactionEnv struct {
	commonEnv

	tx      *flow.TransactionBody
	txIndex uint32
	txID    flow.Identifier
}

func NewTransactionEnvironment(
	ctx Context,
	vm *VirtualMachine,
	sth *state.StateHolder,
	programs handler.TransactionPrograms,
	tx *flow.TransactionBody,
	txIndex uint32,
	traceSpan otelTrace.Span,
) *TransactionEnv {

	txID := tx.ID()
	// TODO set the flags on context
	tracer := environment.NewTracer(ctx.Tracer, traceSpan, ctx.ExtensiveTracing)
	meter := environment.NewMeter(sth)

	env := &TransactionEnv{
		commonEnv: newCommonEnv(
			ctx,
			sth,
			programs,
			tracer,
			meter,
		),
		tx:      tx,
		txIndex: txIndex,
		txID:    txID,
	}

	env.TransactionInfo = environment.NewTransactionInfo(
		tracer,
		tx.Authorizers,
		ctx.Chain.ServiceAddress(),
	)
	env.EventEmitter = environment.NewEventEmitter(
		tracer,
		meter,
		ctx.Chain,
		txID,
		txIndex,
		tx.Payer,
		ctx.ServiceEventCollectionEnabled,
		ctx.EventCollectionByteSizeLimit,
	)
	env.AccountCreator = environment.NewAccountCreator(
		sth,
		ctx.Chain,
		env.accounts,
		ctx.ServiceAccountEnabled,
		tracer,
		meter,
		ctx.Metrics,
		env.SystemContracts)
	env.AccountFreezer = environment.NewAccountFreezer(
		ctx.Chain.ServiceAddress(),
		env.accounts,
		env.TransactionInfo)
	env.ContractUpdater = handler.NewContractUpdater(
		tracer,
		meter,
		env.accounts,
		env.TransactionInfo,
		func() bool {
			enabled, defined := env.GetIsContractDeploymentRestricted()
			if !defined {
				// If the contract deployment bool is not set by the state
				// fallback to the default value set by the configuration
				// after the contract deployment bool is set by the state on all chains, this logic can be simplified
				return ctx.RestrictContractDeployment
			}
			return enabled
		},
		func() bool {
			// TODO read this from the chain similar to the contract deployment
			// but for now we would honor the fallback context flag
			return ctx.RestrictContractRemoval
		},
		env.GetAccountsAuthorizedForContractUpdate,
		env.GetAccountsAuthorizedForContractRemoval,
		env.useContractAuditVoucher,
	)
	env.AccountKeyUpdater = handler.NewAccountKeyUpdater(
		tracer,
		meter,
		env.accounts,
		sth,
		env)

	env.Runtime.SetEnvironment(env)

	// TODO(patrick): rm this hack
	env.fullEnv = env

	return env
}

func (e *TransactionEnv) TxIndex() uint32 {
	return e.txIndex
}

func (e *TransactionEnv) TxID() flow.Identifier {
	return e.txID
}

// GetAccountsAuthorizedForContractUpdate returns a list of addresses authorized to update/deploy contracts
func (e *TransactionEnv) GetAccountsAuthorizedForContractUpdate() []common.Address {
	return e.GetAuthorizedAccounts(blueprints.ContractDeploymentAuthorizedAddressesPath)
}

// GetAccountsAuthorizedForContractRemoval returns a list of addresses authorized to remove contracts
func (e *TransactionEnv) GetAccountsAuthorizedForContractRemoval() []common.Address {
	return e.GetAuthorizedAccounts(blueprints.ContractRemovalAuthorizedAddressesPath)
}

// GetAuthorizedAccounts returns a list of addresses authorized by the service account.
// Used to determine which accounts are permitted to deploy, update, or remove contracts.
//
// It reads a storage path from service account and parse the addresses.
// If any issue occurs on the process (missing registers, stored value properly not set),
// it gracefully handles it and falls back to default behaviour (only service account be authorized).
func (e *TransactionEnv) GetAuthorizedAccounts(path cadence.Path) []common.Address {
	// set default to service account only
	service := runtime.Address(e.ctx.Chain.ServiceAddress())
	defaultAccounts := []runtime.Address{service}

	runtime := e.BorrowCadenceRuntime()
	defer e.ReturnCadenceRuntime(runtime)

	value, err := runtime.ReadStored(service, path)

	const warningMsg = "failed to read contract authorized accounts from service account. using default behaviour instead."

	if err != nil {
		e.ctx.Logger.Warn().Msg(warningMsg)
		return defaultAccounts
	}
	addresses, ok := utils.CadenceValueToAddressSlice(value)
	if !ok {
		e.ctx.Logger.Warn().Msg(warningMsg)
		return defaultAccounts
	}
	return addresses
}

// GetIsContractDeploymentRestricted returns if contract deployment restriction is defined in the state and the value of it
func (e *TransactionEnv) GetIsContractDeploymentRestricted() (restricted bool, defined bool) {
	restricted, defined = false, false
	service := runtime.Address(e.ctx.Chain.ServiceAddress())

	runtime := e.BorrowCadenceRuntime()
	defer e.ReturnCadenceRuntime(runtime)

	value, err := runtime.ReadStored(
		service,
		blueprints.IsContractDeploymentRestrictedPath)
	if err != nil {
		e.ctx.Logger.
			Debug().
			Msg("Failed to read IsContractDeploymentRestricted from the service account. Using value from context instead.")
		return restricted, defined
	}
	restrictedCadence, ok := value.(cadence.Bool)
	if !ok {
		e.ctx.Logger.
			Debug().
			Msg("Failed to parse IsContractDeploymentRestricted from the service account. Using value from context instead.")
		return restricted, defined
	}
	defined = true
	restricted = restrictedCadence.ToGoValue().(bool)
	return restricted, defined
}

func (e *TransactionEnv) useContractAuditVoucher(address runtime.Address, code []byte) (bool, error) {
	return e.UseContractAuditVoucher(address, string(code[:]))
}
