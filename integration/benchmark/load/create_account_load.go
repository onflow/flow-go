package load

import (
	"github.com/rs/zerolog"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/integration/benchmark/account"
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
			tx := flowsdk.NewTransaction().
				SetScript(
					[]byte(`
						transaction() {
							prepare(signer: AuthAccount) {
								AuthAccount(payer: signer)
							}
						}`,
					),
				)

			return tx, nil

		})
}

var _ Load = (*CreateAccountLoad)(nil)
