package reporters

import (
	"os"

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
	logger                zerolog.Logger
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
		logger:                zerolog.New(os.Stderr).With().Timestamp().Logger(),
	}
}

func (r *BaseReporter) Report(payloads []ledger.Payload) error {
	for _, p := range payloads {
		// owner
		owner := p.Key.KeyParts[0].Value
		r.accounts[string(owner)] = true
		r.regCountByAccounts[string(owner)]++
		r.storageUsedByAccounts[string(owner)] += len(p.Value)
		// TODO add flow token balance tracker
		// if string(p.Key.KeyParts[2].Value) == "publicflowTokenBalance" {
		// ownerAddress := common.BytesToAddress(owner)
		// decoded, err := interpreter.DecodeValue(p.Value, &ownerAddress, nil)
		// fmt.Println(">>>", string(p.Key.KeyParts[2].Value), p.Key.String(), decoded, err)
		// }
	}

	r.logger.Info().Msgf("Chain: %s", r.chain)

	// number of accounts
	r.logger.Info().Msgf("Number of unique accounts %d", len(r.accounts))

	// TODO accounts with most storage
	// TODO median storage used
	return nil
}
