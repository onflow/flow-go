package fvm

import "github.com/dapperlabs/flow-go/model/flow"

const keyAddressState = "account_address_state"

type Addresses struct {
	ledger Ledger
	chain  flow.Chain
}

func NewAddresses(ledger Ledger, chain flow.Chain) *Addresses {
	return &Addresses{
		ledger: ledger,
		chain:  chain,
	}
}

func (a *Addresses) GetGeneratorState() (flow.AddressGenerator, error) {
	stateBytes, err := a.ledger.Get(fullKeyHash("", "", keyAddressState))
	if err != nil {
		return nil, err
	}

	return a.chain.BytesToAddressState(stateBytes), nil
}

func (a *Addresses) SetGeneratorState(state flow.AddressGenerator) {
	stateBytes := state.Bytes()
	a.ledger.Set(fullKeyHash("", "", keyAddressState), stateBytes)
}
