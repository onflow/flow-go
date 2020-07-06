package state

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

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

func (a *Addresses) InitGeneratorState() {
	a.SetGeneratorState(a.chain.NewAddressGenerator())
}

func (a *Addresses) GetGeneratorState() (flow.AddressGenerator, error) {
	stateBytes, err := a.ledger.Get(RegisterID("", "", keyAddressState))
	if err != nil {
		return nil, err
	}

	return a.chain.BytesToAddressGenerator(stateBytes), nil
}

func (a *Addresses) SetGeneratorState(state flow.AddressGenerator) {
	stateBytes := state.Bytes()
	a.ledger.Set(RegisterID("", "", keyAddressState), stateBytes)
}
