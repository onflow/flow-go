package environment

import (
	"fmt"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

const (
	keyAddressState = "account_address_state"

	FungibleTokenAccountIndex = 2
	FlowTokenAccountIndex     = 3
	FlowFeesAccountIndex      = 4
)

// AccountCreator make use of the storage state and the chain's address
// generator to create accounts.
//
// It also serves as a decorator for the chain's address generator which
// updates the state when next address is called (This secondary functionality
// is only used in utility command line).
type AccountCreator struct {
	stateTransaction *state.StateHolder
	chain            flow.Chain
}

func NewAccountCreator(
	stateTransaction *state.StateHolder,
	chain flow.Chain,
) *AccountCreator {
	return &AccountCreator{
		stateTransaction: stateTransaction,
		chain:            chain,
	}
}

func (creator *AccountCreator) bytes() ([]byte, error) {
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
func (creator *AccountCreator) Bytes() []byte {
	stateBytes, err := creator.bytes()
	if err != nil {
		panic(err)
	}
	return stateBytes
}

func (creator *AccountCreator) constructAddressGen() (
	flow.AddressGenerator,
	error,
) {
	stateBytes, err := creator.bytes()
	if err != nil {
		return nil, err
	}
	return creator.chain.BytesToAddressGenerator(stateBytes), nil
}

func (creator *AccountCreator) NextAddress() (flow.Address, error) {
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

func (creator *AccountCreator) CurrentAddress() flow.Address {
	var address flow.Address
	addressGenerator, err := creator.constructAddressGen()
	if err != nil {
		// TODO update CurrentAddress to return an error if needed
		panic(err)
	}

	address = addressGenerator.CurrentAddress()
	return address
}

func (creator *AccountCreator) AddressCount() uint64 {
	addressGenerator, err := creator.constructAddressGen()
	if err != nil {
		// TODO update CurrentAddress to return an error if needed
		panic(err)
	}

	return addressGenerator.AddressCount()
}
