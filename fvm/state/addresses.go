package state

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

const keyAddressState = "account_address_state"

type addresses struct {
	ledger Ledger
	chain  flow.Chain
}

func newAddresses(ledger Ledger, chain flow.Chain) *addresses {
	return &addresses{
		ledger: ledger,
		chain:  chain,
	}
}

func (a *addresses) InitAddressGeneratorState() {
	a.SetAddressGeneratorState(a.chain.NewAddressGenerator())
}

func (a *addresses) GetAddressGeneratorState() (flow.AddressGenerator, error) {
	stateBytes, err := a.ledger.Get(RegisterID("", "", keyAddressState))
	if err != nil {
		return nil, err
	}

	return a.chain.BytesToAddressGenerator(stateBytes), nil
}

func (a *addresses) SetAddressGeneratorState(state flow.AddressGenerator) {
	stateBytes := state.Bytes()
	a.ledger.Set(RegisterID("", "", keyAddressState), stateBytes)
}
