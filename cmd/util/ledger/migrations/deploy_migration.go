package migrations

import (
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	coreContracts "github.com/onflow/flow-core-contracts/lib/go/contracts"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

func NewTransactionBasedMigration(
	tx *flow.TransactionBody,
	chainID flow.ChainID,
	logger zerolog.Logger,
) ledger.Migration {
	return func(payloads []*ledger.Payload) ([]*ledger.Payload, error) {

		options := computation.DefaultFVMOptions(chainID, false, false)
		options = append(options,
			fvm.WithContractDeploymentRestricted(false),
			fvm.WithContractRemovalRestricted(false),
			fvm.WithAuthorizationChecksEnabled(false),
			fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
			fvm.WithTransactionFeesEnabled(false))
		ctx := fvm.NewContext(options...)

		snapshot, err := util.NewPayloadSnapshot(payloads)
		if err != nil {
			return nil, err
		}

		vm := fvm.NewVirtualMachine()

		executionSnapshot, res, err := vm.Run(
			ctx,
			fvm.Transaction(tx, 0),
			snapshot,
		)

		if err != nil {
			return nil, err
		}

		if res.Err != nil {
			return nil, res.Err
		}

		return MergeRegisterChanges(
			snapshot.Payloads,
			executionSnapshot.WriteSet,
			logger,
		)
	}
}

func NewDeploymentMigration(
	chainID flow.ChainID,
	contract Contract,
	authorizer flow.Address,
	logger zerolog.Logger,
) ledger.Migration {

	script := []byte(`
      transaction(name: String, code: String) {
          prepare(signer: auth(AddContract) &Account) {
              signer.contracts.add(name: name, code: code.utf8)
          }
      }
    `)

	tx := flow.NewTransactionBody().
		SetScript(script).
		AddArgument(jsoncdc.MustEncode(cadence.String(contract.Name))).
		AddArgument(jsoncdc.MustEncode(cadence.String(contract.Code))).
		AddAuthorizer(authorizer)

	return NewTransactionBasedMigration(tx, chainID, logger)
}

func NewBurnerDeploymentMigration(
	chainID flow.ChainID,
	logger zerolog.Logger,
) ledger.Migration {

	return NewDeploymentMigration(
		chainID,
		Contract{
			Name: "Burner",
			Code: coreContracts.Burner(),
		},
		chainID.Chain().ServiceAddress(),
		logger,
	)
}
