package state

import (
	"github.com/onflow/flow-go/model/flow"
)

const keyAddressState = "account_address_state"

// LedgerBoundAddressGenerator is a decorator for an address generator.
// It uses the underlying generator it gets from the chain.
// The only change is that when next address is called the state is updated as well.
type LedgerBoundAddressGenerator struct {
	generator flow.AddressGenerator
	state     *State
}

func NewLedgerBoundAddressGenerator(state *State, chain flow.Chain) (*LedgerBoundAddressGenerator, error) {
	stateBytes, err := state.Get("", "", keyAddressState)
	if err != nil {
		return nil, err
	}

	addressGenerator := chain.BytesToAddressGenerator(stateBytes)
	return &LedgerBoundAddressGenerator{
		state:     state,
		generator: addressGenerator,
	}, nil
}

func (g *LedgerBoundAddressGenerator) NextAddress() (flow.Address, error) {
	address, err := g.generator.NextAddress()
	if err != nil {
		return address, err
	}

	// update the ledger state
	stateBytes := g.generator.Bytes()
	g.state.Set("", "", keyAddressState, stateBytes)

	return address, nil
}

func (g *LedgerBoundAddressGenerator) CurrentAddress() flow.Address {
	return g.generator.CurrentAddress()
}

func (g *LedgerBoundAddressGenerator) Bytes() []byte {
	return g.generator.Bytes()
}
