package environment

import (
	"fmt"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

const (
	keyAddressState = "account_address_state"

	FungibleTokenAccountIndex = 2
	FlowTokenAccountIndex     = 3
	FlowFeesAccountIndex      = 4
)

type AddressGenerator interface {
	Bytes() []byte
	NextAddress() (flow.Address, error)
	CurrentAddress() flow.Address
	AddressCount() uint64
}

type BootstrapAccountCreator interface {
	CreateBootstrapAccount(
		publicKeys []flow.AccountPublicKey,
	) (
		flow.Address,
		error,
	)
}

// This ensures cadence can't access unexpected operations while parsing
// programs.
type ParseRestrictedAccountCreator struct {
	txnState *state.TransactionState
	impl     AccountCreator
}

func NewParseRestrictedAccountCreator(
	txnState *state.TransactionState,
	creator AccountCreator,
) AccountCreator {
	return ParseRestrictedAccountCreator{
		txnState: txnState,
		impl:     creator,
	}
}

func (creator ParseRestrictedAccountCreator) CreateAccount(
	payer runtime.Address,
) (
	runtime.Address,
	error,
) {
	return parseRestrict1Arg1Ret(
		creator.txnState,
		trace.FVMEnvCreateAccount,
		creator.impl.CreateAccount,
		payer)
}

type AccountCreator interface {
	CreateAccount(payer runtime.Address) (runtime.Address, error)
}

type NoAccountCreator struct {
}

func (NoAccountCreator) CreateAccount(
	payer runtime.Address,
) (
	runtime.Address,
	error,
) {
	return runtime.Address{}, errors.NewOperationNotSupportedError(
		"CreateAccount")
}

// accountCreator make use of the storage state and the chain's address
// generator to create accounts.
//
// It also serves as a decorator for the chain's address generator which
// updates the state when next address is called (This secondary functionality
// is only used in utility command line).
type accountCreator struct {
	txnState *state.TransactionState
	chain    flow.Chain
	accounts Accounts

	isServiceAccountEnabled bool

	tracer  *Tracer
	meter   Meter
	metrics MetricsReporter

	systemContracts *SystemContracts
}

func NewAddressGenerator(
	txnState *state.TransactionState,
	chain flow.Chain,
) AddressGenerator {
	return &accountCreator{
		txnState: txnState,
		chain:    chain,
	}
}

func NewBootstrapAccountCreator(
	txnState *state.TransactionState,
	chain flow.Chain,
	accounts Accounts,
) BootstrapAccountCreator {
	return &accountCreator{
		txnState: txnState,
		chain:    chain,
		accounts: accounts,
	}
}

func NewAccountCreator(
	txnState *state.TransactionState,
	chain flow.Chain,
	accounts Accounts,
	isServiceAccountEnabled bool,
	tracer *Tracer,
	meter Meter,
	metrics MetricsReporter,
	systemContracts *SystemContracts,
) AccountCreator {
	return &accountCreator{
		txnState:                txnState,
		chain:                   chain,
		accounts:                accounts,
		isServiceAccountEnabled: isServiceAccountEnabled,
		tracer:                  tracer,
		meter:                   meter,
		metrics:                 metrics,
		systemContracts:         systemContracts,
	}
}

func (creator *accountCreator) bytes() ([]byte, error) {
	stateBytes, err := creator.txnState.Get(
		"",
		keyAddressState,
		creator.txnState.EnforceLimits())
	if err != nil {
		return nil, fmt.Errorf(
			"failed to read address generator state from the state: %w",
			err)
	}
	return stateBytes, nil
}

// TODO return error instead of a panic
// this requires changes outside of fvm since the type is defined on flow model
// and emulator and others might be dependent on that
func (creator *accountCreator) Bytes() []byte {
	stateBytes, err := creator.bytes()
	if err != nil {
		panic(err)
	}
	return stateBytes
}

func (creator *accountCreator) constructAddressGen() (
	flow.AddressGenerator,
	error,
) {
	stateBytes, err := creator.bytes()
	if err != nil {
		return nil, err
	}
	return creator.chain.BytesToAddressGenerator(stateBytes), nil
}

func (creator *accountCreator) NextAddress() (flow.Address, error) {
	var address flow.Address
	addressGenerator, err := creator.constructAddressGen()
	if err != nil {
		return address, err
	}

	address, err = addressGenerator.NextAddress()
	if err != nil {
		return address, err
	}

	// update the ledger state
	err = creator.txnState.Set(
		"",
		keyAddressState,
		addressGenerator.Bytes(),
		creator.txnState.EnforceLimits())
	if err != nil {
		return address, fmt.Errorf(
			"failed to update the state with address generator state: %w",
			err)
	}
	return address, nil
}

func (creator *accountCreator) CurrentAddress() flow.Address {
	var address flow.Address
	addressGenerator, err := creator.constructAddressGen()
	if err != nil {
		// TODO update CurrentAddress to return an error if needed
		panic(err)
	}

	address = addressGenerator.CurrentAddress()
	return address
}

func (creator *accountCreator) AddressCount() uint64 {
	addressGenerator, err := creator.constructAddressGen()
	if err != nil {
		// TODO update CurrentAddress to return an error if needed
		panic(err)
	}

	return addressGenerator.AddressCount()
}

func (creator *accountCreator) createBasicAccount(
	publicKeys []flow.AccountPublicKey,
) (
	flow.Address,
	error,
) {
	flowAddress, err := creator.NextAddress()
	if err != nil {
		return flow.Address{}, err
	}

	err = creator.accounts.Create(publicKeys, flowAddress)
	if err != nil {
		return flow.Address{}, fmt.Errorf("create account failed: %w", err)
	}

	return flowAddress, nil
}

func (creator *accountCreator) CreateBootstrapAccount(
	publicKeys []flow.AccountPublicKey,
) (
	flow.Address,
	error,
) {
	return creator.createBasicAccount(publicKeys)
}

func (creator *accountCreator) CreateAccount(
	payer runtime.Address,
) (
	runtime.Address,
	error,
) {
	defer creator.tracer.StartSpanFromRoot(trace.FVMEnvCreateAccount).End()

	err := creator.meter.MeterComputation(ComputationKindCreateAccount, 1)
	if err != nil {
		return common.Address{}, err
	}

	// don't enforce limit during account creation
	var addr runtime.Address
	creator.txnState.RunWithAllLimitsDisabled(func() {
		addr, err = creator.createAccount(payer)
	})

	return addr, err
}

func (creator *accountCreator) createAccount(
	payer runtime.Address,
) (
	runtime.Address,
	error,
) {
	flowAddress, err := creator.createBasicAccount(nil)
	if err != nil {
		return common.Address{}, err
	}

	if creator.isServiceAccountEnabled {
		_, invokeErr := creator.systemContracts.SetupNewAccount(
			flowAddress,
			payer)
		if invokeErr != nil {
			return common.Address{}, invokeErr
		}
	}

	creator.metrics.RuntimeSetNumberOfAccounts(creator.AddressCount())
	return runtime.Address(flowAddress), nil
}
