package environment

import (
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/interpreter"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/sema"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
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
	txnState state.NestedTransactionPreparer
	impl     AccountCreator
}

func NewParseRestrictedAccountCreator(
	txnState state.NestedTransactionPreparer,
	creator AccountCreator,
) AccountCreator {
	return ParseRestrictedAccountCreator{
		txnState: txnState,
		impl:     creator,
	}
}

func (creator ParseRestrictedAccountCreator) CreateAccount(
	runtimePayer common.Address,
	invocationContext interpreter.InvocationContext,
) (
	common.Address,
	error,
) {
	return parseRestrict2Arg1Ret(
		creator.txnState,
		trace.FVMEnvCreateAccount,
		creator.impl.CreateAccount,
		runtimePayer,
		invocationContext,
	)
}

type AccountCreator interface {
	// CreateAccount creates a new account, deducting the minimum storage reservation from `payer`.
	//
	// `context` is the invocation context of the currently-executing Cadence program
	// (the one that invoked the `Account` constructor).
	CreateAccount(
		runtimePayer common.Address,
		invocationContext interpreter.InvocationContext,
	) (common.Address, error)
}

type NoAccountCreator struct {
}

func (NoAccountCreator) CreateAccount(
	_ common.Address,
	_ interpreter.InvocationContext,
) (
	common.Address,
	error,
) {
	return common.Address{}, errors.NewOperationNotSupportedError(
		"CreateAccount")
}

// accountCreator make use of the storage state and the chain's address
// generator to create accounts.
//
// It also serves as a decorator for the chain's address generator which
// updates the state when next address is called (This secondary functionality
// is only used in utility command line).
type accountCreator struct {
	txnState state.NestedTransactionPreparer
	chain    flow.Chain
	accounts Accounts

	isServiceAccountEnabled bool

	tracer  tracing.TracerSpan
	meter   Meter
	metrics MetricsReporter

	systemContracts *SystemContracts
}

func NewAddressGenerator(
	txnState state.NestedTransactionPreparer,
	chain flow.Chain,
) AddressGenerator {
	return &accountCreator{
		txnState: txnState,
		chain:    chain,
	}
}

func NewBootstrapAccountCreator(
	txnState state.NestedTransactionPreparer,
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
	txnState state.NestedTransactionPreparer,
	chain flow.Chain,
	accounts Accounts,
	isServiceAccountEnabled bool,
	tracer tracing.TracerSpan,
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
	stateBytes, err := creator.txnState.Get(flow.AddressStateRegisterID)
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
		flow.AddressStateRegisterID,
		addressGenerator.Bytes())
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
		return flow.EmptyAddress, err
	}

	err = creator.accounts.Create(publicKeys, flowAddress)
	if err != nil {
		return flow.EmptyAddress, fmt.Errorf("create account failed: %w", err)
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
	payer common.Address,
	invocationContext interpreter.InvocationContext,
) (
	common.Address,
	error,
) {
	defer creator.tracer.StartChildSpan(trace.FVMEnvCreateAccount).End()

	err := creator.meter.MeterComputation(
		common.ComputationUsage{
			Kind:      ComputationKindCreateAccount,
			Intensity: 1,
		},
	)
	if err != nil {
		return common.Address{}, err
	}

	// don't enforce limit during account creation
	var address flow.Address
	creator.txnState.RunWithMeteringDisabled(func() {
		address, err = creator.createAccount(
			flow.Address(payer),
			invocationContext,
		)
	})

	return common.Address(address), err
}

func (creator *accountCreator) createAccount(
	payer flow.Address,
	invocationContext interpreter.InvocationContext,
) (
	flow.Address,
	error,
) {
	address, err := creator.createBasicAccount(nil)
	if err != nil {
		return flow.EmptyAddress, err
	}

	if creator.isServiceAccountEnabled {
		// Run `FlowServiceAccount.setupNewAccount` against the SAME Cadence invocation context
		// (and thus the same storage) as the program that triggered account creation.
		// Rather than borrowing a fresh runtime with its own storage,
		// using the shared context ensures the minimum storage reservation deducted from the payer here is visible to,
		// and committed by, that outer program.
		// Otherwise the deduction would be made in a separate, independently-committed storage instance
		// and could be overwritten, creating FLOW without a corresponding `TokensMinted` event.
		contractLocation := common.AddressLocation{
			Address: common.Address(creator.chain.ServiceAddress()),
			Name:    systemcontracts.ContractNameServiceAccount,
		}

		args := []cadence.Value{
			cadence.Address(address),
			cadence.Address(payer),
		}

		_, invokeErr := runtime.InvokeContractFunctionOnContext(
			invocationContext,
			contractLocation,
			systemcontracts.ContractServiceAccountFunction_setupNewAccount,
			args,
			setupNewAccountArgumentTypes,
		)
		if invokeErr != nil {
			return flow.EmptyAddress, invokeErr
		}
	}

	creator.metrics.RuntimeSetNumberOfAccounts(creator.AddressCount())
	return address, nil
}

// `FlowServiceAccount.setupNewAccount`
// from https://github.com/onflow/flow-core-contracts/blob/master/contracts/FlowServiceAccount.cdc
var setupNewAccountArgumentTypes = []sema.Type{
	sema.NewReferenceType(
		nil,
		sema.NewEntitlementSetAccess(
			[]*sema.EntitlementType{
				sema.SaveValueType,
				sema.BorrowValueType,
				sema.CapabilitiesType,
			},
			sema.Conjunction,
		),
		sema.AccountType,
	),
	sema.NewReferenceType(
		nil,
		sema.NewEntitlementSetAccess(
			[]*sema.EntitlementType{
				sema.BorrowValueType,
			},
			sema.Conjunction,
		),
		sema.AccountType,
	),
}
