package fvm

import (
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"

	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/handler"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/meter/weighted"
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

	eventHandler     *handler.EventHandler
	addressGenerator flow.AddressGenerator
	tx               *flow.TransactionBody
	txIndex          uint32
	txID             flow.Identifier
	authorizers      []runtime.Address
}

func NewTransactionEnvironment(
	ctx Context,
	vm *VirtualMachine,
	sth *state.StateHolder,
	programs *programs.Programs,
	tx *flow.TransactionBody,
	txIndex uint32,
	traceSpan otelTrace.Span,
) (*TransactionEnv, error) {

	accounts := state.NewAccounts(sth)
	generator := state.NewStateBoundAddressGenerator(sth, ctx.Chain)
	uuidGenerator := state.NewUUIDGenerator(sth)
	programsHandler := handler.NewProgramsHandler(programs, sth)
	// TODO set the flags on context
	eventHandler := handler.NewEventHandler(ctx.Chain,
		ctx.EventCollectionEnabled,
		ctx.ServiceEventCollectionEnabled,
		ctx.EventCollectionByteSizeLimit,
	)
	accountKeys := handler.NewAccountKeyHandler(accounts)
	metrics := handler.NewMetricsHandler(ctx.Metrics)

	env := &TransactionEnv{
		commonEnv: commonEnv{
			ctx:           ctx,
			sth:           sth,
			vm:            vm,
			programs:      programsHandler,
			accounts:      accounts,
			accountKeys:   accountKeys,
			uuidGenerator: uuidGenerator,
			metrics:       metrics,
			logs:          nil,
			rng:           nil,
			traceSpan:     traceSpan,
		},

		addressGenerator: generator,
		eventHandler:     eventHandler,
		tx:               tx,
		txIndex:          txIndex,
		txID:             tx.ID(),
	}

	// TODO(patrick): rm this hack
	env.MeterInterface = env
	env.AccountInterface = env

	env.contracts = handler.NewContractHandler(accounts,
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

	if ctx.BlockHeader != nil {
		env.seedRNG(ctx.BlockHeader)
	}

	var err error
	// set the execution parameters from the state
	if ctx.AllowContextOverrideByExecutionState {
		err = env.setExecutionParameters()
	}

	return env, err
}

func (e *TransactionEnv) setExecutionParameters() error {
	// Check that the service account exists because all the settings are stored in it
	serviceAddress := e.Context().Chain.ServiceAddress()
	service := runtime.Address(serviceAddress)

	// set the property if no error, but if the error is a fatal error then return it
	setIfOk := func(prop string, err error, setter func()) (fatal error) {
		err, fatal = errors.SplitErrorTypes(err)
		if fatal != nil {
			// this is a fatal error. return it
			e.ctx.Logger.
				Error().
				Err(fatal).
				Msgf("error getting %s", prop)
			return fatal
		}
		if err != nil {
			// this is a general error.
			// could be that no setting was present in the state,
			// or that the setting was not parseable,
			// or some other deterministic thing.
			e.ctx.Logger.
				Debug().
				Err(err).
				Msgf("could not set %s. Using defaults", prop)
			return nil
		}
		// everything is ok. do the setting
		setter()
		return nil
	}

	var ok bool
	var m *weighted.Meter
	// only set the weights if the meter is a weighted.Meter
	if m, ok = e.sth.State().Meter().(*weighted.Meter); !ok {
		return nil
	}

	computationWeights, err := GetExecutionEffortWeights(e, service)
	err = setIfOk(
		"execution effort weights",
		err,
		func() { m.SetComputationWeights(computationWeights) })
	if err != nil {
		return err
	}

	memoryWeights, err := GetExecutionMemoryWeights(e, service)
	err = setIfOk(
		"execution memory weights",
		err,
		func() { m.SetMemoryWeights(memoryWeights) })
	if err != nil {
		return err
	}

	memoryLimit, err := GetExecutionMemoryLimit(e, service)
	err = setIfOk(
		"execution memory limit",
		err,
		func() { m.SetTotalMemoryLimit(memoryLimit) })
	if err != nil {
		return err
	}

	return nil
}

func (e *TransactionEnv) TxIndex() uint32 {
	return e.txIndex
}

func (e *TransactionEnv) TxID() flow.Identifier {
	return e.txID
}

// GetAccountsAuthorizedForContractUpdate returns a list of addresses authorized to update/deploy contracts
func (e *TransactionEnv) GetAccountsAuthorizedForContractUpdate() []common.Address {
	return e.GetAuthorizedAccounts(
		cadence.Path{
			Domain:     blueprints.ContractDeploymentAuthorizedAddressesPathDomain,
			Identifier: blueprints.ContractDeploymentAuthorizedAddressesPathIdentifier,
		})
}

// GetAccountsAuthorizedForContractRemoval returns a list of addresses authorized to remove contracts
func (e *TransactionEnv) GetAccountsAuthorizedForContractRemoval() []common.Address {
	return e.GetAuthorizedAccounts(
		cadence.Path{
			Domain:     blueprints.ContractRemovalAuthorizedAddressesPathDomain,
			Identifier: blueprints.ContractRemovalAuthorizedAddressesPathIdentifier,
		})
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

	value, err := e.vm.Runtime.ReadStored(
		service,
		path,
		runtime.Context{Interface: e},
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

	value, err := e.vm.Runtime.ReadStored(
		service,
		cadence.Path{
			Domain:     blueprints.IsContractDeploymentRestrictedPathDomain,
			Identifier: blueprints.IsContractDeploymentRestrictedPathIdentifier,
		},
		runtime.Context{Interface: e},
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
	return InvokeUseContractAuditVoucherContract(
		e,
		e.traceSpan,
		address,
		string(code[:]))
}

func (e *TransactionEnv) isAuthorizerServiceAccount() bool {
	return e.isAuthorizer(runtime.Address(e.ctx.Chain.ServiceAddress()))
}

func (e *TransactionEnv) isAuthorizer(address runtime.Address) bool {
	for _, accountAddress := range e.getSigningAccounts() {
		if accountAddress == address {
			return true
		}
	}
	return false
}

func (e *TransactionEnv) GetStorageCapacity(address common.Address) (value uint64, err error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvGetStorageCapacity)
		defer sp.End()
	}

	err = e.Meter(meter.ComputationKindGetStorageCapacity, 1)
	if err != nil {
		return value, fmt.Errorf("get storage capacity failed: %w", err)
	}

	result, invokeErr := InvokeAccountStorageCapacityContract(
		e,
		e.traceSpan,
		address)
	if invokeErr != nil {
		return 0, errors.HandleRuntimeError(invokeErr)
	}

	return storageMBUFixToBytesUInt(result), nil
}

func (e *TransactionEnv) GetAccountBalance(address common.Address) (value uint64, err error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvGetAccountBalance)
		defer sp.End()
	}

	err = e.Meter(meter.ComputationKindGetAccountBalance, 1)
	if err != nil {
		return value, fmt.Errorf("get account balance failed: %w", err)
	}

	result, invokeErr := InvokeAccountBalanceContract(e, e.traceSpan, address)
	if invokeErr != nil {
		return 0, errors.HandleRuntimeError(invokeErr)
	}
	return result.ToGoValue().(uint64), nil
}

func (e *TransactionEnv) GetAccountAvailableBalance(address common.Address) (value uint64, err error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvGetAccountBalance)
		defer sp.End()
	}

	err = e.Meter(meter.ComputationKindGetAccountAvailableBalance, 1)
	if err != nil {
		return value, fmt.Errorf("get account available balance failed: %w", err)
	}

	result, invokeErr := InvokeAccountAvailableBalanceContract(
		e,
		e.traceSpan,
		address)

	if invokeErr != nil {
		return 0, errors.HandleRuntimeError(invokeErr)
	}
	return result.ToGoValue().(uint64), nil
}

func (e *TransactionEnv) ResolveLocation(
	identifiers []runtime.Identifier,
	location runtime.Location,
) ([]runtime.ResolvedLocation, error) {
	if e.isTraceable() && e.ctx.ExtensiveTracing {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvResolveLocation)
		defer sp.End()
	}

	err := e.Meter(meter.ComputationKindResolveLocation, 1)
	if err != nil {
		return nil, fmt.Errorf("resolve location failed: %w", err)
	}

	addressLocation, isAddress := location.(common.AddressLocation)

	// if the location is not an address location, e.g. an identifier location (`import Crypto`),
	// then return a single resolved location which declares all identifiers.
	if !isAddress {
		return []runtime.ResolvedLocation{
			{
				Location:    location,
				Identifiers: identifiers,
			},
		}, nil
	}

	// if the location is an address,
	// and no specific identifiers where requested in the import statement,
	// then fetch all identifiers at this address
	if len(identifiers) == 0 {
		address := flow.Address(addressLocation.Address)

		err := e.accounts.CheckAccountNotFrozen(address)
		if err != nil {
			return nil, fmt.Errorf("resolving location failed: %w", err)
		}

		contractNames, err := e.contracts.GetContractNames(addressLocation.Address)
		if err != nil {
			return nil, fmt.Errorf("resolving location failed: %w", err)
		}

		// if there are no contractNames deployed,
		// then return no resolved locations
		if len(contractNames) == 0 {
			return nil, nil
		}

		identifiers = make([]ast.Identifier, len(contractNames))

		for i := range identifiers {
			identifiers[i] = runtime.Identifier{
				Identifier: contractNames[i],
			}
		}
	}

	// return one resolved location per identifier.
	// each resolved location is an address contract location
	resolvedLocations := make([]runtime.ResolvedLocation, len(identifiers))
	for i := range resolvedLocations {
		identifier := identifiers[i]
		resolvedLocations[i] = runtime.ResolvedLocation{
			Location: common.AddressLocation{
				Address: addressLocation.Address,
				Name:    identifier.Identifier,
			},
			Identifiers: []runtime.Identifier{identifier},
		}
	}

	return resolvedLocations, nil
}

func (e *TransactionEnv) ProgramLog(message string) error {
	if e.isTraceable() && e.ctx.ExtensiveTracing {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvProgramLog)
		defer sp.End()
	}

	if e.ctx.CadenceLoggingEnabled {
		e.logs = append(e.logs, message)
	}
	return nil
}

func (e *TransactionEnv) Logs() []string {
	return e.logs
}

func (e *TransactionEnv) EmitEvent(event cadence.Event) error {
	// only trace when extensive tracing
	if e.isTraceable() && e.ctx.ExtensiveTracing {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvEmitEvent)
		defer sp.End()
	}

	err := e.Meter(meter.ComputationKindEmitEvent, 1)
	if err != nil {
		return fmt.Errorf("emit event failed: %w", err)
	}

	return e.eventHandler.EmitEvent(event, e.txID, e.txIndex, e.tx.Payer)
}

func (e *TransactionEnv) Events() []flow.Event {
	return e.eventHandler.Events()
}

func (e *TransactionEnv) ServiceEvents() []flow.Event {
	return e.eventHandler.ServiceEvents()
}

func (e *TransactionEnv) Meter(kind common.ComputationKind, intensity uint) error {
	if e.sth.EnforceComputationLimits {
		return e.sth.State().MeterComputation(kind, intensity)
	}
	return nil
}

func (e *TransactionEnv) meterMemory(kind common.MemoryKind, intensity uint) error {
	if e.sth.EnforceMemoryLimits() {
		return e.sth.State().MeterMemory(kind, intensity)
	}
	return nil
}

func (e *TransactionEnv) SetAccountFrozen(address common.Address, frozen bool) error {

	if !e.ctx.AccountFreezeEnabled {
		return errors.NewOperationNotSupportedError("SetAccountFrozen")
	}

	flowAddress := flow.Address(address)

	if flowAddress == e.ctx.Chain.ServiceAddress() {
		err := errors.NewValueErrorf(flowAddress.String(), "cannot freeze service account")
		return fmt.Errorf("setting account frozen failed: %w", err)
	}

	if !e.isAuthorizerServiceAccount() {
		err := errors.NewOperationAuthorizationErrorf("SetAccountFrozen", "accounts can be frozen only by transactions authorized by the service account")
		return fmt.Errorf("setting account frozen failed: %w", err)
	}

	err := e.accounts.SetAccountFrozen(flowAddress, frozen)
	if err != nil {
		return fmt.Errorf("setting account frozen failed: %w", err)
	}
	return nil
}

func (e *TransactionEnv) VerifySignature(
	signature []byte,
	tag string,
	signedData []byte,
	publicKey []byte,
	signatureAlgorithm runtime.SignatureAlgorithm,
	hashAlgorithm runtime.HashAlgorithm,
) (bool, error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvVerifySignature)
		defer sp.End()
	}

	err := e.Meter(meter.ComputationKindVerifySignature, 1)
	if err != nil {
		return false, fmt.Errorf("verify signature failed: %w", err)
	}

	valid, err := crypto.VerifySignatureFromRuntime(
		signature,
		tag,
		signedData,
		publicKey,
		signatureAlgorithm,
		hashAlgorithm,
	)

	if err != nil {
		return false, fmt.Errorf("verify signature failed: %w", err)
	}

	return valid, nil
}

func (e *TransactionEnv) ValidatePublicKey(pk *runtime.PublicKey) error {
	err := e.Meter(meter.ComputationKindValidatePublicKey, 1)
	if err != nil {
		return fmt.Errorf("validate public key failed: %w", err)
	}

	return crypto.ValidatePublicKey(pk.SignAlgo, pk.PublicKey)
}

// Block Environment Functions

func (e *TransactionEnv) CreateAccount(payer runtime.Address) (address runtime.Address, err error) {

	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvCreateAccount)
		defer sp.End()
	}

	err = e.Meter(meter.ComputationKindCreateAccount, 1)
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
		_, invokeErr := InvokeSetupNewAccountContract(
			e,
			e.traceSpan,
			flowAddress,
			payer)
		if invokeErr != nil {
			return address, errors.HandleRuntimeError(invokeErr)
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
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvAddAccountKey)
		defer sp.End()
	}

	err := e.Meter(meter.ComputationKindAddEncodedAccountKey, 1)
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
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvRemoveAccountKey)
		defer sp.End()
	}

	err = e.Meter(meter.ComputationKindRevokeEncodedAccountKey, 1)
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
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvAddAccountKey)
		defer sp.End()
	}

	err := e.Meter(meter.ComputationKindAddAccountKey, 1)
	if err != nil {
		return nil, fmt.Errorf("add account key failed: %w", err)
	}

	accKey, err := e.accountKeys.AddAccountKey(address, publicKey, hashAlgo, weight)
	if err != nil {
		return nil, fmt.Errorf("add account key failed: %w", err)
	}

	return accKey, nil
}

// GetAccountKey retrieves a public key by index from an existing account.
//
// This function returns a nil key with no errors, if a key doesn't exist at the given index.
// An error is returned if the specified account does not exist, the provided index is not valid,
// or if the key retrieval fails.
func (e *TransactionEnv) GetAccountKey(address runtime.Address, keyIndex int) (*runtime.AccountKey, error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvGetAccountKey)
		defer sp.End()
	}

	err := e.Meter(meter.ComputationKindGetAccountKey, 1)
	if err != nil {
		return nil, fmt.Errorf("get account key failed: %w", err)
	}

	accKey, err := e.accountKeys.GetAccountKey(address, keyIndex)
	if err != nil {
		return nil, fmt.Errorf("get account key failed: %w", err)
	}
	return accKey, err
}

// RevokeAccountKey revokes a public key by index from an existing account,
// and returns the revoked key.
//
// This function returns a nil key with no errors, if a key doesn't exist at the given index.
// An error is returned if the specified account does not exist, the provided index is not valid,
// or if the key revoking fails.
func (e *TransactionEnv) RevokeAccountKey(address runtime.Address, keyIndex int) (*runtime.AccountKey, error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvRemoveAccountKey)
		defer sp.End()
	}

	err := e.Meter(meter.ComputationKindRevokeAccountKey, 1)
	if err != nil {
		return nil, fmt.Errorf("revoke account key failed: %w", err)
	}

	return e.accountKeys.RevokeAccountKey(address, keyIndex)
}

func (e *TransactionEnv) UpdateAccountContractCode(address runtime.Address, name string, code []byte) (err error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvUpdateAccountContractCode)
		defer sp.End()
	}

	err = e.Meter(meter.ComputationKindUpdateAccountContractCode, 1)
	if err != nil {
		return fmt.Errorf("update account contract code failed: %w", err)
	}

	err = e.accounts.CheckAccountNotFrozen(flow.Address(address))
	if err != nil {
		return fmt.Errorf("update account contract code failed: %w", err)
	}

	err = e.contracts.SetContract(address, name, code, e.getSigningAccounts())
	if err != nil {
		return fmt.Errorf("updating account contract code failed: %w", err)
	}

	return nil
}

func (e *TransactionEnv) RemoveAccountContractCode(address runtime.Address, name string) (err error) {
	if e.isTraceable() {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvRemoveAccountContractCode)
		defer sp.End()
	}

	err = e.Meter(meter.ComputationKindRemoveAccountContractCode, 1)
	if err != nil {
		return fmt.Errorf("remove account contract code failed: %w", err)
	}

	err = e.accounts.CheckAccountNotFrozen(flow.Address(address))
	if err != nil {
		return fmt.Errorf("remove account contract code failed: %w", err)
	}

	err = e.contracts.RemoveContract(address, name, e.getSigningAccounts())
	if err != nil {
		return fmt.Errorf("remove account contract code failed: %w", err)
	}

	return nil
}

func (e *TransactionEnv) GetSigningAccounts() ([]runtime.Address, error) {
	if e.isTraceable() && e.ctx.ExtensiveTracing {
		sp := e.ctx.Tracer.StartSpanFromParent(e.traceSpan, trace.FVMEnvGetSigningAccounts)
		defer sp.End()
	}
	return e.getSigningAccounts(), nil
}

func (e *TransactionEnv) getSigningAccounts() []runtime.Address {
	if e.authorizers == nil {
		e.authorizers = make([]runtime.Address, len(e.tx.Authorizers))

		for i, auth := range e.tx.Authorizers {
			e.authorizers[i] = runtime.Address(auth)
		}
	}
	return e.authorizers
}
