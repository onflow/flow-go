package load

import (
	"github.com/onflow/cadence"
	"github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
	"sync"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/integration/benchmark/account"
	"github.com/onflow/flow-go/integration/benchmark/scripts"
)

type TokenTransferMultiLoad struct {
	tokensPerTransfer   cadence.UFix64
	accountsPerTransfer uint

	destinationAddresses      []flow.Address
	setupDestinationAddresses sync.Once
}

func NewTokenTransferMultiLoad() *TokenTransferMultiLoad {
	return &TokenTransferMultiLoad{
		tokensPerTransfer:   cadence.UFix64(100),
		accountsPerTransfer: 100,
	}
}

var _ Load = (*TokenTransferMultiLoad)(nil)

func (l *TokenTransferMultiLoad) Type() LoadType {
	return TokenTransferMultiLoadType
}

func (l *TokenTransferMultiLoad) Setup(_ zerolog.Logger, _ LoadContext) error {
	return nil
}

func (l *TokenTransferMultiLoad) Load(log zerolog.Logger, lc LoadContext) error {
	return sendSimpleTransaction(
		log,
		lc,
		func(
			log zerolog.Logger,
			lc LoadContext,
			acc *account.FlowAccount,
		) (*flowsdk.Transaction, error) {
			sc := systemcontracts.SystemContractsForChain(lc.ChainID)

			// we only need to get the destination addresses once
			l.setupDestinationAddresses.Do(func() {
				destinationSDKAddresses, err := lc.GetAddresses(l.accountsPerTransfer)
				if err != nil {
					log.Warn().Err(err).Msg("failed to get destination addresses")
					l.destinationAddresses = make([]flow.Address, l.accountsPerTransfer)
					for i := 0; uint(i) < l.accountsPerTransfer; i++ {
						l.destinationAddresses[i] = sc.FlowServiceAccount.Address
					}
				}

				l.destinationAddresses = apply(
					destinationSDKAddresses,
					func(a flowsdk.Address) flow.Address {
						return flow.ConvertAddress(a)
					})
			})
			// get another account to send tokens to

			transferTx, err := scripts.TokenTransferMultiTransaction(
				sc.FungibleToken.Address,
				sc.FlowToken.Address,
				l.destinationAddresses,
				l.tokensPerTransfer)
			if err != nil {
				return nil, err
			}

			return transferTx, nil
		},
	)
}

func apply[I any, O any](input []I, transform func(I) O) []O {
	output := make([]O, 0, len(input))
	for _, i := range input {
		output = append(output, transform(i))
	}
	return output

}
