package load

import (
	"github.com/rs/zerolog"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/integration/benchmark/account"
	"github.com/onflow/flow-go/integration/benchmark/scripts"
)

type CreateAccountLoad struct {
}

func NewCreateAccountLoad() *CreateAccountLoad {
	return &CreateAccountLoad{}
}

func (c CreateAccountLoad) Type() LoadType {
	return CreateAccount
}

func (c CreateAccountLoad) Setup(zerolog.Logger, LoadContext) error {
	// no setup needed
	return nil
}

func (c CreateAccountLoad) Load(log zerolog.Logger, lc LoadContext) error {

	return sendSimpleTransaction(
		log,
		lc,
		func(
			log zerolog.Logger,
			lc LoadContext,
			acc *account.FlowAccount,
		) (*flowsdk.Transaction, error) {
			sc := systemcontracts.SystemContractsForChain(lc.ChainID)

			tx := flowsdk.NewTransaction().
				SetScript(scripts.CreateAccountsTransaction(
					flowsdk.Address(sc.FungibleToken.Address),
					flowsdk.Address(sc.FlowToken.Address)))

			return tx, nil

		})
}

var _ Load = (*CreateAccountLoad)(nil)
