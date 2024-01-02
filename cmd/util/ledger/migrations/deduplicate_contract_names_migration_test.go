package migrations_test

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
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

	address, err := common.HexToAddress("0x1")
	require.NoError(t, err)

	ctx := context.Background()

	accountStatus := environment.NewAccountStatus()
	accountStatus.SetStorageUsed(1000)
	accountStatusPayload := ledger.NewPayload(
		convert.RegisterIDToLedgerKey(
			flow.AccountStatusRegisterID(flow.ConvertAddress(address)),
		),
		accountStatus.ToBytes(),
	)

	contractNamesPayload := func(contractNames []byte) *ledger.Payload {
		return ledger.NewPayload(
			convert.RegisterIDToLedgerKey(
				flow.RegisterID{
					Owner: string(address.Bytes()),
					Key:   flow.ContractNamesKey,
				},
			),
			contractNames,
		)
	}

	requireContractNames := func(payloads []*ledger.Payload, f func([]string)) {
		for _, payload := range payloads {
			key, err := payload.Key()
			require.NoError(t, err)
			id, err := convert.LedgerKeyToRegisterID(key)
			require.NoError(t, err)

			if id.Key != flow.ContractNamesKey {
				continue
			}

			contracts := make([]string, 0)
			err = cbor.Unmarshal(payload.Value(), &contracts)
			require.NoError(t, err)

			f(contracts)

		}
	}

	t.Run("no contract names", func(t *testing.T) {
		payloads, err := migration.MigrateAccount(ctx, address,
			[]*ledger.Payload{
				accountStatusPayload,
			},
		)

		require.NoError(t, err)
		require.Equal(t, 1, len(payloads))
	})

	t.Run("one contract", func(t *testing.T) {
		contractNames := []string{"test"}
		newContractNames, err := cbor.Marshal(contractNames)
		require.NoError(t, err)

		payloads, err := migration.MigrateAccount(ctx, address,
			[]*ledger.Payload{
				accountStatusPayload,
				contractNamesPayload(newContractNames),
			},
		)

		require.NoError(t, err)
		require.Equal(t, 2, len(payloads))

		requireContractNames(payloads, func(contracts []string) {
			require.Equal(t, 1, len(contracts))
			require.Equal(t, "test", contracts[0])
		})
	})

	t.Run("two unique contracts", func(t *testing.T) {
		contractNames := []string{"test", "test2"}
		newContractNames, err := cbor.Marshal(contractNames)
		require.NoError(t, err)

		payloads, err := migration.MigrateAccount(ctx, address,
			[]*ledger.Payload{
				accountStatusPayload,
				contractNamesPayload(newContractNames),
			},
		)

		require.NoError(t, err)
		require.Equal(t, 2, len(payloads))

		requireContractNames(payloads, func(contracts []string) {
			require.Equal(t, 2, len(contracts))
			require.Equal(t, "test", contracts[0])
			require.Equal(t, "test2", contracts[1])
		})
	})

	t.Run("two contracts", func(t *testing.T) {
		contractNames := []string{"test", "test"}
		newContractNames, err := cbor.Marshal(contractNames)
		require.NoError(t, err)

		payloads, err := migration.MigrateAccount(ctx, address,
			[]*ledger.Payload{
				accountStatusPayload,
				contractNamesPayload(newContractNames),
			},
		)

		require.NoError(t, err)
		require.Equal(t, 2, len(payloads))

		requireContractNames(payloads, func(contracts []string) {
			require.Equal(t, 1, len(contracts))
			require.Equal(t, "test", contracts[0])
		})
	})

	t.Run("not sorted contracts", func(t *testing.T) {
		contractNames := []string{"test2", "test"}
		newContractNames, err := cbor.Marshal(contractNames)
		require.NoError(t, err)

		_, err = migration.MigrateAccount(ctx, address,
			[]*ledger.Payload{
				accountStatusPayload,
				contractNamesPayload(newContractNames),
			},
		)

		require.Error(t, err)
	})

	t.Run("duplicate contracts", func(t *testing.T) {
		contractNames := []string{"test", "test", "test2", "test3", "test3"}
		newContractNames, err := cbor.Marshal(contractNames)
		require.NoError(t, err)

		payloads, err := migration.MigrateAccount(ctx, address,
			[]*ledger.Payload{
				accountStatusPayload,
				contractNamesPayload(newContractNames),
			},
		)

		require.NoError(t, err)
		require.Equal(t, 2, len(payloads))

		requireContractNames(payloads, func(contracts []string) {
			require.Equal(t, 3, len(contracts))
			require.Equal(t, "test", contracts[0])
			require.Equal(t, "test2", contracts[1])
			require.Equal(t, "test3", contracts[2])
		})
	})

	t.Run("random contracts", func(t *testing.T) {
		contractNames := make([]string, 1000)
		uniqueContracts := 1
		for i := 0; i < 1000; i++ {
			// i > 0 so it's easier to know how many unique contracts there are
			if i > 0 && rand.Float32() < 0.5 {
				uniqueContracts++
			}
			contractNames[i] = fmt.Sprintf("test%d", uniqueContracts)
		}

		// sort contractNames alphabetically, because they are not sorted
		sort.Slice(contractNames, func(i, j int) bool {
			return contractNames[i] < contractNames[j]
		})

		newContractNames, err := cbor.Marshal(contractNames)
		require.NoError(t, err)

		payloads, err := migration.MigrateAccount(ctx, address,
			[]*ledger.Payload{
				accountStatusPayload,
				contractNamesPayload(newContractNames),
			},
		)

		require.NoError(t, err)
		require.Equal(t, 2, len(payloads))

		requireContractNames(payloads, func(contracts []string) {
			require.Equal(t, uniqueContracts, len(contracts))
		})
	})
}
