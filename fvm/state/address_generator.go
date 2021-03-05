package state

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

const keyAddressState = "account_address_state"

// StateBoundAddressGenerator is a decorator for an address generator.
// It uses the underlying generator it gets from the chain.
// The only change is that when next address is called the state is updated as well.
type StateBoundAddressGenerator struct {
	stateManager *StateManager
	chain        flow.Chain
}

func NewStateBoundAddressGenerator(stateManager *StateManager, chain flow.Chain) (*StateBoundAddressGenerator, error) {
	return &StateBoundAddressGenerator{
		stateManager: stateManager,
		chain:        chain,
	}, nil
}

func (g *StateBoundAddressGenerator) constructAddressGen() (flow.AddressGenerator, error) {
	st := g.stateManager.State()
	stateBytes, err := st.Get("", "", keyAddressState)
	if err != nil {
		return nil, fmt.Errorf("failed to read address generator state from the state: %w", err)
	}
	return g.chain.BytesToAddressGenerator(stateBytes), nil
}

func (g *StateBoundAddressGenerator) NextAddress() (flow.Address, error) {

	var address flow.Address
	addressGenerator, err := g.constructAddressGen()
	if err != nil {
		return address, err
	}

	address, err = addressGenerator.NextAddress()
	if err != nil {
		return address, err
	}

	// update the ledger state
	err = g.stateManager.State().Set("", "", keyAddressState, addressGenerator.Bytes())
	if err != nil {
		return address, fmt.Errorf("failed to update the state with address generator state: %w", err)
	}
	return address, nil
}

func (g *StateBoundAddressGenerator) CurrentAddress() flow.Address {

	var address flow.Address
	addressGenerator, err := g.constructAddressGen()
	if err != nil {
		// TODO update CurrentAddress to return an error if needed
		panic(err)
	}

	address = addressGenerator.CurrentAddress()
	return address
}
