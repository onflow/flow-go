package fvm

import (
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"

	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/handler"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/utils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

var _ runtime.Interface = &TransactionEnv{}

// TransactionEnv is a read-write environment used for executing flow transactions.
type TransactionEnv struct {
	commonEnv

	addressGenerator flow.AddressGenerator
	tx               *flow.TransactionBody
	txIndex          uint32
	txID             flow.Identifier
}

func NewTransactionEnvironment(
	ctx Context,
	vm *VirtualMachine,
	sth *state.StateHolder,
	programs *programs.Programs,
	tx *flow.TransactionBody,
	txIndex uint32,
	traceSpan otelTrace.Span,
) *TransactionEnv {

	txID := tx.ID()
	accounts := environment.NewAccounts(sth)
	generator := environment.NewAccountCreator(sth, ctx.Chain)
	programsHandler := handler.NewProgramsHandler(programs, sth)
	// TODO set the flags on context
	accountKeys := handler.NewAccountKeyHandler(accounts)
	tracer := environment.NewTracer(ctx.Tracer, traceSpan, ctx.ExtensiveTracing)
	meter := environment.NewMeter(sth)

	env := &TransactionEnv{
		commonEnv: commonEnv{
			Tracer: tracer,
			Meter:  meter,
			ProgramLogger: environment.NewProgramLogger(
				tracer,
				ctx.Logger,
				ctx.Metrics,
				ctx.CadenceLoggingEnabled,
			),
			UUIDGenerator: environment.NewUUIDGenerator(tracer, meter, sth),
			UnsafeRandomGenerator: environment.NewUnsafeRandomGenerator(
				tracer,
				ctx.BlockHeader,
			),
			CryptoLibrary: environment.NewCryptoLibrary(tracer, meter),
			BlockInfo: environment.NewBlockInfo(
				tracer,
				meter,
				ctx.BlockHeader,
				ctx.Blocks,
			),
			TransactionInfo: environment.NewTransactionInfo(
				tracer,
				tx.Authorizers,
				ctx.Chain.ServiceAddress(),
			),
			EventEmitter: environment.NewEventEmitter(
				tracer,
				meter,
				ctx.Chain,
				txID,
				txIndex,
				tx.Payer,
				ctx.ServiceEventCollectionEnabled,
				ctx.EventCollectionByteSizeLimit,
			),
			ValueStore: environment.NewValueStore(
				tracer,
				meter,
				accounts),
			ContractReader: environment.NewContractReader(
				tracer,
				meter,
				accounts),
			SystemContracts: NewSystemContracts(),
			ctx:             ctx,
			sth:             sth,
			vm:              vm,
			programs:        programsHandler,
			accounts:        accounts,
			accountKeys:     accountKeys,
			frozenAccounts:  nil,
		},

		addressGenerator: generator,
		tx:               tx,
		txIndex:          txIndex,
		txID:             txID,
	}

	env.SystemContracts.SetEnvironment(env)

	// TODO(patrick): rm this hack
	env.AccountInterface = env
	env.fullEnv = env

	env.contracts = handler.NewContractHandler(
		accounts,
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

	runtimeEnv := e.BorrowCadenceRuntime()
	defer e.ReturnCadenceRuntime(runtimeEnv)

	value, err := e.vm.Runtime.ReadStored(
		service,
		path,
		runtime.Context{
			Interface:   e,
			Environment: runtimeEnv,
		},
	)

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

	runtimeEnv := e.BorrowCadenceRuntime()
	defer e.ReturnCadenceRuntime(runtimeEnv)

	value, err := e.vm.Runtime.ReadStored(
		service,
		blueprints.IsContractDeploymentRestrictedPath,
		runtime.Context{
			Interface:   e,
			Environment: runtimeEnv,
		},
	)
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

func (e *TransactionEnv) SetAccountFrozen(address common.Address, frozen bool) error {

	flowAddress := flow.Address(address)

	if flowAddress == e.ctx.Chain.ServiceAddress() {
		err := errors.NewValueErrorf(flowAddress.String(), "cannot freeze service account")
		return fmt.Errorf("setting account frozen failed: %w", err)
	}

	if !e.IsServiceAccountAuthorizer() {
		err := errors.NewOperationAuthorizationErrorf("SetAccountFrozen", "accounts can be frozen only by transactions authorized by the service account")
		return fmt.Errorf("setting account frozen failed: %w", err)
	}

	err := e.accounts.SetAccountFrozen(flowAddress, frozen)
	if err != nil {
		return fmt.Errorf("setting account frozen failed: %w", err)
	}

	if frozen {
		e.frozenAccounts = append(e.frozenAccounts, address)
	}

	return nil
}

func (e *TransactionEnv) CreateAccount(payer runtime.Address) (address runtime.Address, err error) {
	defer e.StartSpanFromRoot(trace.FVMEnvCreateAccount).End()

	err = e.MeterComputation(environment.ComputationKindCreateAccount, 1)
	if err != nil {
		return address, err
	}

	e.sth.DisableAllLimitEnforcements() // don't enforce limit during account creation
	defer e.sth.EnableAllLimitEnforcements()

	flowAddress, err := e.addressGenerator.NextAddress()
	if err != nil {
		return address, err
	}

	err = e.accounts.Create(nil, flowAddress)
	if err != nil {
		return address, fmt.Errorf("create account failed: %w", err)
	}

	if e.ctx.ServiceAccountEnabled {
		_, invokeErr := e.SetupNewAccount(flowAddress, payer)
		if invokeErr != nil {
			return address, invokeErr
		}
	}

	e.ctx.Metrics.RuntimeSetNumberOfAccounts(e.addressGenerator.AddressCount())
	return runtime.Address(flowAddress), nil
}

// AddEncodedAccountKey adds an encoded public key to an existing account.
//
// This function returns an error if the specified account does not exist or
// if the key insertion fails.
func (e *TransactionEnv) AddEncodedAccountKey(address runtime.Address, publicKey []byte) error {
	defer e.StartSpanFromRoot(trace.FVMEnvAddAccountKey).End()

	err := e.MeterComputation(environment.ComputationKindAddEncodedAccountKey, 1)
	if err != nil {
		return fmt.Errorf("add encoded account key failed: %w", err)
	}

	// TODO do a call to track the computation usage and memory usage
	e.sth.DisableAllLimitEnforcements() // don't enforce limit during adding a key
	defer e.sth.EnableAllLimitEnforcements()

	err = e.accounts.CheckAccountNotFrozen(flow.Address(address))
	if err != nil {
		return fmt.Errorf("add encoded account key failed: %w", err)
	}

	err = e.accountKeys.AddEncodedAccountKey(address, publicKey)

	if err != nil {
		return fmt.Errorf("add encoded account key failed: %w", err)
	}
	return nil
}

// RevokeEncodedAccountKey revokes a public key by index from an existing account.
//
// This function returns an error if the specified account does not exist, the
// provided key is invalid, or if key revoking fails.
func (e *TransactionEnv) RevokeEncodedAccountKey(address runtime.Address, index int) (publicKey []byte, err error) {
	defer e.StartSpanFromRoot(trace.FVMEnvRemoveAccountKey).End()

	err = e.MeterComputation(environment.ComputationKindRevokeEncodedAccountKey, 1)
	if err != nil {
		return publicKey, fmt.Errorf("revoke encoded account key failed: %w", err)
	}

	err = e.accounts.CheckAccountNotFrozen(flow.Address(address))
	if err != nil {
		return nil, fmt.Errorf("revoke encoded account key failed: %w", err)
	}

	encodedKey, err := e.accountKeys.RemoveAccountKey(address, index)
	if err != nil {
		return nil, fmt.Errorf("revoke encoded account key failed: %w", err)
	}

	return encodedKey, nil
}

// AddAccountKey adds a public key to an existing account.
//
// This function returns an error if the specified account does not exist or
// if the key insertion fails.
func (e *TransactionEnv) AddAccountKey(
	address runtime.Address,
	publicKey *runtime.PublicKey,
	hashAlgo runtime.HashAlgorithm,
	weight int,
) (
	*runtime.AccountKey,
	error,
) {
	defer e.StartSpanFromRoot(trace.FVMEnvAddAccountKey).End()

	err := e.MeterComputation(environment.ComputationKindAddAccountKey, 1)
	if err != nil {
		return nil, fmt.Errorf("add account key failed: %w", err)
	}

	accKey, err := e.accountKeys.AddAccountKey(address, publicKey, hashAlgo, weight)
	if err != nil {
		return nil, fmt.Errorf("add account key failed: %w", err)
	}

	return accKey, nil
}

// RevokeAccountKey revokes a public key by index from an existing account,
// and returns the revoked key.
//
// This function returns a nil key with no errors, if a key doesn't exist at the given index.
// An error is returned if the specified account does not exist, the provided index is not valid,
// or if the key revoking fails.
func (e *TransactionEnv) RevokeAccountKey(address runtime.Address, keyIndex int) (*runtime.AccountKey, error) {
	defer e.StartSpanFromRoot(trace.FVMEnvRemoveAccountKey).End()

	err := e.MeterComputation(environment.ComputationKindRevokeAccountKey, 1)
	if err != nil {
		return nil, fmt.Errorf("revoke account key failed: %w", err)
	}

	return e.accountKeys.RevokeAccountKey(address, keyIndex)
}

func (e *TransactionEnv) UpdateAccountContractCode(address runtime.Address, name string, code []byte) (err error) {
	defer e.StartSpanFromRoot(trace.FVMEnvUpdateAccountContractCode).End()

	err = e.MeterComputation(environment.ComputationKindUpdateAccountContractCode, 1)
	if err != nil {
		return fmt.Errorf("update account contract code failed: %w", err)
	}

	err = e.accounts.CheckAccountNotFrozen(flow.Address(address))
	if err != nil {
		return fmt.Errorf("update account contract code failed: %w", err)
	}

	err = e.contracts.SetContract(address, name, code, e.SigningAccounts())
	if err != nil {
		return fmt.Errorf("updating account contract code failed: %w", err)
	}

	return nil
}

func (e *TransactionEnv) RemoveAccountContractCode(address runtime.Address, name string) (err error) {
	defer e.StartSpanFromRoot(trace.FVMEnvRemoveAccountContractCode).End()

	err = e.MeterComputation(environment.ComputationKindRemoveAccountContractCode, 1)
	if err != nil {
		return fmt.Errorf("remove account contract code failed: %w", err)
	}

	err = e.accounts.CheckAccountNotFrozen(flow.Address(address))
	if err != nil {
		return fmt.Errorf("remove account contract code failed: %w", err)
	}

	err = e.contracts.RemoveContract(address, name, e.SigningAccounts())
	if err != nil {
		return fmt.Errorf("remove account contract code failed: %w", err)
	}

	return nil
}
