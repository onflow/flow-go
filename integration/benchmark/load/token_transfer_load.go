package load

import (
	"github.com/onflow/cadence"
	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/integration/benchmark/account"
	"github.com/onflow/flow-go/integration/benchmark/scripts"
	"github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
)

type TokenTransferLoad struct {
	tokensPerTransfer cadence.UFix64
}

func NewTokenTransferLoad() *TokenTransferLoad {
	return &TokenTransferLoad{
		tokensPerTransfer: cadence.UFix64(100),
	}
}

var _ Load = (*TokenTransferLoad)(nil)

func (l *TokenTransferLoad) Type() LoadType {
	return TokenTransferLoadType
}

func (l *TokenTransferLoad) Setup(_ zerolog.Logger, _ LoadContext) error {
	return nil
}

func (l *TokenTransferLoad) Load(log zerolog.Logger, lc LoadContext) error {
	return sendSimpleTransaction(
		log,
		lc,
		func(
			log zerolog.Logger,
			lc LoadContext,
			acc *account.FlowAccount,
		) (*flowsdk.Transaction, error) {
			sc := systemcontracts.SystemContractsForChain(lc.ChainID)

			// get another account to send tokens to
			var destinationAddress flow.Address
			acc2, err := lc.BorrowAvailableAccount()
			if err != nil {
				if !errors.Is(err, account.ErrNoAccountsAvailable) {
					return nil, err
				}
				// if no accounts are available, just send to the service account
				destinationAddress = sc.FlowServiceAccount.Address
			} else {
				destinationAddress = flow.ConvertAddress(acc2.Address)
				lc.ReturnAvailableAccount(acc2)
			}

			transferTx, err := scripts.TokenTransferTransaction(
				sc.FungibleToken.Address,
				sc.FlowToken.Address,
				destinationAddress,
				l.tokensPerTransfer)
			if err != nil {
				return nil, err
			}

			return transferTx, nil
		},
	)
}
