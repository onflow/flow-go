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
	stateTransaction *state.StateHolder
	chain            flow.Chain
	accounts         Accounts

	isServiceAccountEnabled bool

	tracer  *Tracer
	meter   Meter
	metrics MetricsReporter

	systemContracts *SystemContracts
}

func NewAddressGenerator(
	stateTransaction *state.StateHolder,
	chain flow.Chain,
) AddressGenerator {
	return &accountCreator{
		stateTransaction: stateTransaction,
		chain:            chain,
	}
}

func NewBootstrapAccountCreator(
	stateTransaction *state.StateHolder,
	chain flow.Chain,
	accounts Accounts,
) BootstrapAccountCreator {
	return &accountCreator{
		stateTransaction: stateTransaction,
		chain:            chain,
		accounts:         accounts,
	}
}

func NewAccountCreator(
	stateTransaction *state.StateHolder,
	chain flow.Chain,
	accounts Accounts,
	isServiceAccountEnabled bool,
	tracer *Tracer,
	meter Meter,
	metrics MetricsReporter,
	systemContracts *SystemContracts,
) AccountCreator {
	return &accountCreator{
		stateTransaction:        stateTransaction,
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
	stateBytes, err := creator.stateTransaction.Get(
		"",
		keyAddressState,
		creator.stateTransaction.EnforceLimits())
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
	err = creator.stateTransaction.Set(
		"",
		keyAddressState,
		addressGenerator.Bytes(),
		creator.stateTransaction.EnforceLimits())
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

	creator.stateTransaction.DisableAllLimitEnforcements() // don't enforce limit during account creation
	defer creator.stateTransaction.EnableAllLimitEnforcements()

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
