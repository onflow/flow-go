package migrations

import (
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/model/flow"
)

type Contract struct {
	Name string
	Code []byte
}

func NewDeploymentMigration(
	chainID flow.ChainID,
	contract Contract,
	authorizer flow.Address,
	expectedWriteAddresses map[flow.Address]struct{},
	logger zerolog.Logger,
) RegistersMigration {

	script := []byte(`
      transaction(name: String, code: String) {
          prepare(signer: auth(AddContract) &Account) {
              signer.contracts.add(name: name, code: code.utf8)
          }
      }
    `)

	txBody, err := flow.NewTransactionBodyBuilder().
		SetScript(script).
		AddArgument(jsoncdc.MustEncode(cadence.String(contract.Name))).
		AddArgument(jsoncdc.MustEncode(cadence.String(contract.Code))).
		SetPayer(authorizer).
		AddAuthorizer(authorizer).
		Build()
	if err != nil {
		return func(registersByAccount *registers.ByAccount) error {
			return fmt.Errorf("failed to run transaction: %w", err)
		}
	}

	return NewTransactionBasedMigration(
		txBody,
		chainID,
		logger,
		expectedWriteAddresses,
	)
}
