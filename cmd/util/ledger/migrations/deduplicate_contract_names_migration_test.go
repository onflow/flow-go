package migrations_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

func TestDeduplicateContractNamesMigration(t *testing.T) {
	migration := migrations.DeduplicateContractNamesMigration{}

	log := zerolog.New(zerolog.NewTestWriter(t))

	err := migration.InitMigration(log, nil, 0)
	require.NoError(t, err)

	ctx := context.Background()

	address, err := common.HexToAddress("0x1")
	require.NoError(t, err)

	contractNames := []string{"test", "test", "test2", "test3", "test3"}

	var buf bytes.Buffer
	cborEncoder := cbor.NewEncoder(&buf)
	err = cborEncoder.Encode(contractNames)
	require.NoError(t, err)
	newContractNames := buf.Bytes()

	accountStatus := environment.NewAccountStatus()

	accountStatus.SetStorageUsed(1000)

	payloads, err := migration.MigrateAccount(ctx, address,
		[]*ledger.Payload{

			ledger.NewPayload(
				convert.RegisterIDToLedgerKey(
					flow.RegisterID{
						Owner: string(address.Bytes()),
						Key:   flow.ContractNamesKey,
					},
				),
				newContractNames,
			),
			ledger.NewPayload(
				convert.RegisterIDToLedgerKey(
					flow.AccountStatusRegisterID(flow.ConvertAddress(address)),
				),
				accountStatus.ToBytes(),
			),
		},
	)

	require.NoError(t, err)
	require.Equal(t, 2, len(payloads))

	for _, payload := range payloads {
		key, err := payload.Key()
		require.NoError(t, err)
		id, err := convert.LedgerKeyToRegisterID(key)
		require.NoError(t, err)

		if id.Key != flow.ContractNamesKey {
			continue
		}

		value := payload.Value()

		contracts := make([]string, 0)
		buf := bytes.NewReader(value)
		cborDecoder := cbor.NewDecoder(buf)
		err = cborDecoder.Decode(&contracts)
		require.NoError(t, err)

		require.Equal(t, 3, len(contracts))
	}
}
