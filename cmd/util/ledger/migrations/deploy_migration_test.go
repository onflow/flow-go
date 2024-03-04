package migrations

import (
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func newBootstrapPayloads(
	chainID flow.ChainID,
	bootstrapProcedureOptions ...fvm.BootstrapProcedureOption,
) ([]*ledger.Payload, error) {

	ctx := fvm.NewContext(
		fvm.WithChain(chainID.Chain()),
	)

	vm := fvm.NewVirtualMachine()

	storageSnapshot := snapshot.MapStorageSnapshot{}

	bootstrapProcedure := fvm.Bootstrap(
		unittest.ServiceAccountPublicKey,
		bootstrapProcedureOptions...,
	)

	executionSnapshot, _, err := vm.Run(
		ctx,
		bootstrapProcedure,
		storageSnapshot,
	)
	if err != nil {
		return nil, err
	}

	payloads := make([]*ledger.Payload, 0, len(executionSnapshot.WriteSet))

	for registerID, registerValue := range executionSnapshot.WriteSet {
		payloadKey := convert.RegisterIDToLedgerKey(registerID)
		payload := ledger.NewPayload(payloadKey, registerValue)
		payloads = append(payloads, payload)
	}

	return payloads, nil
}

func TestDeploy(t *testing.T) {
	t.Parallel()

	const chainID = flow.Emulator

	chain := chainID.Chain()

	systemContracts := systemcontracts.SystemContractsForChain(chainID)
	serviceAccountAddress := systemContracts.FlowServiceAccount.Address
	fungibleTokenAddress := systemContracts.FungibleToken.Address

	targetAddress := serviceAccountAddress

	migration := NewDeploymentMigration(
		chainID,
		Contract{
			Name: "NewContract",
			Code: []byte(fmt.Sprintf(
				`
                  import FungibleToken from %s

                  access(all)
                  contract NewContract {

                      access(all)
                      fun answer(): Int {
                          return 42
                      }
                  }
                `,
				fungibleTokenAddress.HexWithPrefix(),
			)),
		},
		targetAddress,
		map[flow.Address]struct{}{
			targetAddress: {},
		},
		zerolog.New(zerolog.NewTestWriter(t)),
	)

	bootstrapPayloads, err := newBootstrapPayloads(chainID)
	require.NoError(t, err)

	filteredPayloads := make([]*ledger.Payload, 0, len(bootstrapPayloads))

	// TODO: move to NewTransactionBasedMigration

	// Filter the bootstrapped payloads to only include the target account (service account)
	// and the account where the fungible token is deployed

	for _, payload := range bootstrapPayloads {
		registerID, _, err := convert.PayloadToRegister(payload)
		require.NoError(t, err)

		if len(registerID.Owner) > 0 {
			registerAddress := flow.Address([]byte(registerID.Owner))
			switch registerAddress {
			case targetAddress, fungibleTokenAddress:
				filteredPayloads = append(filteredPayloads, payload)
			}
		} else {
			filteredPayloads = append(filteredPayloads, payload)
		}
	}

	newPayloads, err := migration(filteredPayloads)
	require.NoError(t, err)

	txBody := flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(
			`
              import NewContract from %s

              transaction {
                  execute {
                      log(NewContract.answer())
                  }
              }
            `,
			targetAddress.HexWithPrefix(),
		)))

	vm := fvm.NewVirtualMachine()

	storageSnapshot := snapshot.MapStorageSnapshot{}

	for _, newPayload := range newPayloads {
		registerID, registerValue, err := convert.PayloadToRegister(newPayload)
		require.NoError(t, err)

		storageSnapshot[registerID] = registerValue
	}

	ctx := fvm.NewContext(
		fvm.WithChain(chain),
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
		fvm.WithCadenceLogging(true),
	)

	_, output, err := vm.Run(
		ctx,
		fvm.Transaction(txBody, 0),
		storageSnapshot,
	)

	require.NoError(t, err)
	require.NoError(t, output.Err)
	require.Len(t, output.Logs, 1)
	require.Equal(t, "42", output.Logs[0])
}
