package state

import (
	"github.com/onflow/flow-go/model/flow"
)

const keyAddressState = "account_address_state"

type AddressGeneratorState struct {
	ledger Ledger
	chain  flow.Chain
}

func NewAddressGeneratorState(ledger Ledger, chain flow.Chain) *AddressGeneratorState {
	return &AddressGeneratorState{
		ledger: ledger,
		chain:  chain,
	}
}

func (state *AddressGeneratorState) Init() {
	state.Set(state.chain.NewAddressGenerator())
}

func (state *AddressGeneratorState) GetGenerator() (flow.AddressGenerator, error) {
	stateBytes, err := state.ledger.Get("", "", keyAddressState)
	if err != nil {
		return nil, err
	}

	addressGenerator := state.chain.BytesToAddressGenerator(stateBytes)

	return &stateBoundAddressGenerator{
		addressGenerator:      addressGenerator,
		addressGeneratorState: state,
	}, nil
}

func (state *AddressGeneratorState) Set(generator flow.AddressGenerator) {
	stateBytes := generator.Bytes()
	state.ledger.Set("", "", keyAddressState, stateBytes)
}

// stateBoundAddressGenerator is a decorator for an address generator.
// It uses the underlying generator.
// The only change is that when next address is called the state is updated as well
type stateBoundAddressGenerator struct {
	addressGenerator      flow.AddressGenerator
	addressGeneratorState *AddressGeneratorState
}

func (g *stateBoundAddressGenerator) NextAddress() (flow.Address, error) {
	address, err := g.addressGenerator.NextAddress()
	if err != nil {
		return address, err
	}
	g.addressGeneratorState.Set(g.addressGenerator)
	return address, err
}

func (g *stateBoundAddressGenerator) CurrentAddress() flow.Address {
	return g.addressGenerator.CurrentAddress()
}

func (g *stateBoundAddressGenerator) Bytes() []byte {
	return g.addressGenerator.Bytes()
}
