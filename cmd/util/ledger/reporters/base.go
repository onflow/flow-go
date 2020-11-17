package reporters

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

type BaseReporter struct {
	chain                 flow.Chain
	serviceAccountAddress flow.Address
	fungibleTokenAddress  flow.Address
	flowTokenAddress      flow.Address
	accounts              map[string]bool
	regCountByAccounts    map[string]int
	storageUsedByAccounts map[string]int
	log                   zerolog.Logger
}

func NewBaseReporter(chainID flow.ChainID) *BaseReporter {

	chain := chainID.Chain()
	addressGen := chain.NewAddressGenerator()

	serviceAccountAddress, err := addressGen.NextAddress()
	if err != nil {
		panic(err)
	}
	fungibleTokenAddress, err := addressGen.NextAddress()
	if err != nil {
		panic(err)
	}
	flowTokenAddress, err := addressGen.NextAddress()
	if err != nil {
		panic(err)
	}

	return &BaseReporter{
		chain:                 chain,
		serviceAccountAddress: serviceAccountAddress,
		fungibleTokenAddress:  fungibleTokenAddress,
		flowTokenAddress:      flowTokenAddress,
		accounts:              make(map[string]bool),
		regCountByAccounts:    make(map[string]int),
		storageUsedByAccounts: make(map[string]int),
	}
}

func (r *BaseReporter) Report(payloads []ledger.Payload) error {
	for _, p := range payloads {
		// owner
		owner := p.Key.KeyParts[0].Value
		r.accounts[string(owner)] = true
		r.regCountByAccounts[string(owner)] += 1
		r.storageUsedByAccounts[string(owner)] += len(p.Value)

		if strings.Contains(string(p.Key.KeyParts[2].Value), r.flowTokenAddress.String()) {
			fmt.Println(">>>", p.Key.String(), p.Value.String())
		}
	}

	r.log.Info().Msgf("Chain: %s", r.chain)

	// number of accounts
	r.log.Info().Msgf("Number of unique accounts %d", len(r.accounts))

	// accounts with most storage
	// median storage used
	return nil
}
