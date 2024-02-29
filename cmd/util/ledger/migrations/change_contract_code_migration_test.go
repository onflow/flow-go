package migrations_test

import (
	"context"
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

func newContractPayload(address common.Address, contractName string, contract []byte) *ledger.Payload {
	return ledger.NewPayload(
		convert.RegisterIDToLedgerKey(
			flow.ContractRegisterID(flow.ConvertAddress(address), contractName),
		),
		contract,
	)
}

const contractA = `
access(all) contract A { 
    access(all) fun foo() {} 
}`

const updatedContractA = `
access(all) contract A {
    access(all) fun bar() {}
}`

const contractB = `
access(all) contract B {
    access(all) fun foo() {}
}`

const updatedContractB = `
access(all) contract B {
    access(all) fun bar() {}
}`

func TestChangeContractCodeMigration(t *testing.T) {
	t.Parallel()

	address1, err := common.HexToAddress("0x1")
	require.NoError(t, err)

	address2, err := common.HexToAddress("0x2")
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("no contracts", func(t *testing.T) {
		t.Parallel()

		log := zerolog.New(zerolog.NewTestWriter(t))

		migration := migrations.NewChangeContractCodeMigration(flow.Emulator, log)

		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		_, err = migration.MigrateAccount(ctx, address1,
			[]*ledger.Payload{},
		)

		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)
	})

	t.Run("1 contract - dont migrate", func(t *testing.T) {
		t.Parallel()

		log := zerolog.New(zerolog.NewTestWriter(t))

		migration := migrations.NewChangeContractCodeMigration(flow.Emulator, log)

		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		payloads, err := migration.MigrateAccount(ctx, address1,
			[]*ledger.Payload{
				newContractPayload(address1, "A", []byte(contractA)),
			},
		)

		require.NoError(t, err)
		require.Len(t, payloads, 1)
		require.Equal(t, []byte(contractA), []byte(payloads[0].Value()))

		err = migration.Close()
		require.NoError(t, err)
	})

	t.Run("1 contract - migrate", func(t *testing.T) {
		t.Parallel()

		log := zerolog.New(zerolog.NewTestWriter(t))

		migration := migrations.NewChangeContractCodeMigration(flow.Emulator, log)

		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		migration.RegisterContractChange(
			migrations.StagedContract{
				Address: address1,
				Contract: migrations.Contract{
					Name: "A",
					Code: []byte(updatedContractA),
				},
			},
		)

		payloads, err := migration.MigrateAccount(ctx, address1,
			[]*ledger.Payload{
				newContractPayload(address1, "A", []byte(contractA)),
			},
		)

		require.NoError(t, err)
		require.Len(t, payloads, 1)
		require.Equal(t, updatedContractA, string(payloads[0].Value()))

		err = migration.Close()
		require.NoError(t, err)
	})

	t.Run("2 contracts - migrate 1", func(t *testing.T) {
		t.Parallel()

		log := zerolog.New(zerolog.NewTestWriter(t))

		migration := migrations.NewChangeContractCodeMigration(flow.Emulator, log)

		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		migration.RegisterContractChange(
			migrations.StagedContract{
				Address: address1,
				Contract: migrations.Contract{
					Name: "A",
					Code: []byte(updatedContractA),
				},
			},
		)

		payloads, err := migration.MigrateAccount(ctx, address1,
			[]*ledger.Payload{
				newContractPayload(address1, "A", []byte(contractA)),
				newContractPayload(address1, "B", []byte(contractB)),
			},
		)

		require.NoError(t, err)
		require.Len(t, payloads, 2)
		require.Equal(t, []byte(updatedContractA), []byte(payloads[0].Value()))
		require.Equal(t, []byte(contractB), []byte(payloads[1].Value()))

		err = migration.Close()
		require.NoError(t, err)
	})

	t.Run("2 contracts - migrate 2", func(t *testing.T) {
		t.Parallel()

		log := zerolog.New(zerolog.NewTestWriter(t))

		migration := migrations.NewChangeContractCodeMigration(flow.Emulator, log)

		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		migration.RegisterContractChange(
			migrations.StagedContract{
				Address: address1,
				Contract: migrations.Contract{
					Name: "A",
					Code: []byte(updatedContractA),
				},
			},
		)
		migration.RegisterContractChange(
			migrations.StagedContract{
				Address: address1,
				Contract: migrations.Contract{
					Name: "B",
					Code: []byte(updatedContractB),
				},
			},
		)

		payloads, err := migration.MigrateAccount(ctx, address1,
			[]*ledger.Payload{
				newContractPayload(address1, "A", []byte(contractA)),
				newContractPayload(address1, "B", []byte(contractB)),
			},
		)

		require.NoError(t, err)
		require.Len(t, payloads, 2)
		require.Equal(t, []byte(updatedContractA), []byte(payloads[0].Value()))
		require.Equal(t, []byte(updatedContractB), []byte(payloads[1].Value()))

		err = migration.Close()
		require.NoError(t, err)
	})

	t.Run("2 contracts on different accounts - migrate 1", func(t *testing.T) {
		t.Parallel()

		log := zerolog.New(zerolog.NewTestWriter(t))

		migration := migrations.NewChangeContractCodeMigration(flow.Emulator, log)

		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		migration.RegisterContractChange(
			migrations.StagedContract{
				Address: address1,
				Contract: migrations.Contract{
					Name: "A",
					Code: []byte(updatedContractA),
				},
			},
		)

		payloads, err := migration.MigrateAccount(ctx, address1,
			[]*ledger.Payload{
				newContractPayload(address1, "A", []byte(contractA)),
				newContractPayload(address2, "A", []byte(contractA)),
			},
		)

		require.NoError(t, err)
		require.Len(t, payloads, 2)
		require.Equal(t, []byte(updatedContractA), []byte(payloads[0].Value()))
		require.Equal(t, []byte(contractA), []byte(payloads[1].Value()))

		err = migration.Close()
		require.NoError(t, err)
	})

	t.Run("not all contracts on one account migrated", func(t *testing.T) {
		t.Parallel()

		log := zerolog.New(zerolog.NewTestWriter(t))

		migration := migrations.NewChangeContractCodeMigration(flow.Emulator, log)

		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		migration.RegisterContractChange(
			migrations.StagedContract{
				Address: address1,
				Contract: migrations.Contract{
					Name: "A",
					Code: []byte(updatedContractA),
				},
			},
		)
		migration.RegisterContractChange(
			migrations.StagedContract{
				Address: address1,
				Contract: migrations.Contract{
					Name: "B",
					Code: []byte(updatedContractB),
				},
			},
		)

		_, err = migration.MigrateAccount(ctx, address1,
			[]*ledger.Payload{
				newContractPayload(address1, "A", []byte(contractA)),
			},
		)

		require.Error(t, err)
	})

	t.Run("not all accounts migrated", func(t *testing.T) {
		t.Parallel()

		log := zerolog.New(zerolog.NewTestWriter(t))

		migration := migrations.NewChangeContractCodeMigration(flow.Emulator, log)

		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		migration.RegisterContractChange(
			migrations.StagedContract{
				Address: address2,
				Contract: migrations.Contract{
					Name: "A",
					Code: []byte(updatedContractA),
				},
			},
		)

		_, err = migration.MigrateAccount(ctx, address1,
			[]*ledger.Payload{
				newContractPayload(address1, "A", []byte(contractA)),
			},
		)

		require.NoError(t, err)

		err = migration.Close()
		require.Error(t, err)
	})
}
