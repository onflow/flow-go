package migrations

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/model/flow"
)

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

type errorLogWriter struct {
	logs []string
}

var _ io.Writer = &errorLogWriter{}

const errorLogPrefix = "{\"level\":\"error\""

func (l *errorLogWriter) Write(bytes []byte) (int, error) {
	logStr := string(bytes)

	// Ignore non-error logs
	if !strings.HasPrefix(logStr, errorLogPrefix) {
		return 0, nil
	}

	l.logs = append(l.logs, logStr)
	return len(bytes), nil
}

func TestChangeContractCodeMigration(t *testing.T) {
	t.Parallel()

	const chainID = flow.Emulator
	addressGenerator := chainID.Chain().NewAddressGenerator()

	address1, err := addressGenerator.NextAddress()
	require.NoError(t, err)

	address2, err := addressGenerator.NextAddress()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("no contracts", func(t *testing.T) {
		t.Parallel()

		writer := &errorLogWriter{}
		log := zerolog.New(writer)

		rwf := &testReportWriterFactory{}

		options := StagedContractsMigrationOptions{
			ChainID:            flow.Emulator,
			VerboseErrorOutput: true,
		}
		migration := NewStagedContractsMigration(
			"test",
			"test",
			log,
			rwf,
			&LegacyTypeRequirements{},
			options,
		)

		registersByAccount := registers.NewByAccount()

		err := migration.InitMigration(log, registersByAccount, 1)
		require.NoError(t, err)

		err = migration.MigrateAccount(
			ctx,
			common.Address(address1),
			registersByAccount.AccountRegisters(string(address1[:])),
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Empty(t, writer.logs)
	})

	t.Run("1 contract - dont migrate", func(t *testing.T) {
		t.Parallel()

		writer := &errorLogWriter{}
		log := zerolog.New(writer)

		rwf := &testReportWriterFactory{}

		options := StagedContractsMigrationOptions{
			ChainID:            flow.Emulator,
			VerboseErrorOutput: true,
		}
		migration := NewStagedContractsMigration(
			"test",
			"test",
			log,
			rwf,
			&LegacyTypeRequirements{},
			options,
		)

		registersByAccount, err := registersForStagedContracts(
			StagedContract{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "A",
					Code: []byte(contractA),
				},
			},
		)
		require.NoError(t, err)

		err = migration.InitMigration(log, registersByAccount, 1)
		require.NoError(t, err)

		owner1 := string(address1[:])

		accountRegisters1 := registersByAccount.AccountRegisters(owner1)

		err = migration.MigrateAccount(
			ctx,
			common.Address(address1),
			accountRegisters1,
		)
		require.NoError(t, err)

		require.Equal(t, 1, registersByAccount.AccountCount())
		require.Equal(t, 1, accountRegisters1.Count())
		require.Equal(t,
			contractA,
			contractCode(t, registersByAccount, owner1, "A"),
		)

		err = migration.Close()
		require.NoError(t, err)

		require.Empty(t, writer.logs)
	})

	t.Run("1 contract - migrate", func(t *testing.T) {
		t.Parallel()

		writer := &errorLogWriter{}
		log := zerolog.New(writer)

		rwf := &testReportWriterFactory{}

		options := StagedContractsMigrationOptions{
			ChainID:            flow.Emulator,
			VerboseErrorOutput: true,
		}
		migration := NewStagedContractsMigration(
			"test",
			"test",
			log,
			rwf,
			&LegacyTypeRequirements{},
			options,
		).
			WithStagedContractUpdates([]StagedContract{
				{
					Address: common.Address(address1),
					Contract: Contract{
						Name: "A",
						Code: []byte(updatedContractA),
					},
				},
			})

		registersByAccount, err := registersForStagedContracts(
			StagedContract{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "A",
					Code: []byte(contractA),
				},
			},
		)
		require.NoError(t, err)

		err = migration.InitMigration(log, registersByAccount, 1)
		require.NoError(t, err)

		owner1 := string(address1[:])

		accountRegisters1 := registersByAccount.AccountRegisters(owner1)

		err = migration.MigrateAccount(
			ctx,
			common.Address(address1),
			accountRegisters1,
		)
		require.NoError(t, err)

		require.Equal(t, 1, registersByAccount.AccountCount())
		require.Equal(t, 1, accountRegisters1.Count())
		require.Equal(t,
			updatedContractA,
			contractCode(t, registersByAccount, owner1, "A"),
		)

		err = migration.Close()
		require.NoError(t, err)

		require.Empty(t, writer.logs)
	})

	t.Run("2 contracts - migrate 1", func(t *testing.T) {
		t.Parallel()

		writer := &errorLogWriter{}
		log := zerolog.New(writer)

		rwf := &testReportWriterFactory{}

		options := StagedContractsMigrationOptions{
			ChainID:            flow.Emulator,
			VerboseErrorOutput: true,
		}
		migration := NewStagedContractsMigration(
			"test",
			"test",
			log,
			rwf,
			&LegacyTypeRequirements{},
			options,
		).
			WithStagedContractUpdates([]StagedContract{
				{
					Address: common.Address(address1),
					Contract: Contract{
						Name: "A",
						Code: []byte(updatedContractA),
					},
				},
			})

		registersByAccount, err := registersForStagedContracts(
			StagedContract{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "A",
					Code: []byte(contractA),
				},
			},
			StagedContract{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "B",
					Code: []byte(contractB),
				},
			},
		)
		require.NoError(t, err)

		err = migration.InitMigration(log, registersByAccount, 1)
		require.NoError(t, err)

		owner1 := string(address1[:])

		accountRegisters1 := registersByAccount.AccountRegisters(owner1)

		err = migration.MigrateAccount(
			ctx,
			common.Address(address1),
			accountRegisters1,
		)
		require.NoError(t, err)

		require.Equal(t, 1, registersByAccount.AccountCount())
		require.Equal(t, 2, accountRegisters1.Count())
		require.Equal(t,
			updatedContractA,
			contractCode(t, registersByAccount, owner1, "A"),
		)
		require.Equal(t,
			contractB,
			contractCode(t, registersByAccount, owner1, "B"),
		)

		err = migration.Close()
		require.NoError(t, err)

		require.Empty(t, writer.logs)
	})

	t.Run("2 contracts - migrate 2", func(t *testing.T) {
		t.Parallel()

		writer := &errorLogWriter{}
		log := zerolog.New(writer)

		rwf := &testReportWriterFactory{}

		options := StagedContractsMigrationOptions{
			ChainID:            flow.Emulator,
			VerboseErrorOutput: true,
		}
		migration := NewStagedContractsMigration(
			"test",
			"test",
			log,
			rwf,
			&LegacyTypeRequirements{},
			options,
		).
			WithStagedContractUpdates([]StagedContract{
				{
					Address: common.Address(address1),
					Contract: Contract{
						Name: "A",
						Code: []byte(updatedContractA),
					},
				},
				{
					Address: common.Address(address1),
					Contract: Contract{
						Name: "B",
						Code: []byte(updatedContractB),
					},
				},
			})

		registersByAccount, err := registersForStagedContracts(
			StagedContract{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "A",
					Code: []byte(contractA),
				},
			},
			StagedContract{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "B",
					Code: []byte(contractB),
				},
			},
		)
		require.NoError(t, err)

		err = migration.InitMigration(log, registersByAccount, 1)
		require.NoError(t, err)

		owner1 := string(address1[:])

		accountRegisters1 := registersByAccount.AccountRegisters(owner1)

		err = migration.MigrateAccount(
			ctx,
			common.Address(address1),
			accountRegisters1,
		)
		require.NoError(t, err)

		require.Equal(t, 1, registersByAccount.AccountCount())
		require.Equal(t, 2, accountRegisters1.Count())
		require.Equal(t,
			updatedContractA,
			contractCode(t, registersByAccount, owner1, "A"),
		)
		require.Equal(t,
			updatedContractB,
			contractCode(t, registersByAccount, owner1, "B"),
		)

		err = migration.Close()
		require.NoError(t, err)

		require.Empty(t, writer.logs)
	})

	t.Run("2 contracts on different accounts - migrate 1", func(t *testing.T) {
		t.Parallel()

		writer := &errorLogWriter{}
		log := zerolog.New(writer)

		rwf := &testReportWriterFactory{}

		options := StagedContractsMigrationOptions{
			ChainID:            flow.Emulator,
			VerboseErrorOutput: true,
		}
		migration := NewStagedContractsMigration(
			"test",
			"test",
			log,
			rwf,
			&LegacyTypeRequirements{},
			options,
		).
			WithStagedContractUpdates([]StagedContract{
				{
					Address: common.Address(address1),
					Contract: Contract{
						Name: "A",
						Code: []byte(updatedContractA),
					},
				},
			})

		registersByAccount, err := registersForStagedContracts(
			StagedContract{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "A",
					Code: []byte(contractA),
				},
			},
			StagedContract{
				Address: common.Address(address2),
				Contract: Contract{
					Name: "A",
					Code: []byte(contractA),
				},
			},
		)
		require.NoError(t, err)

		err = migration.InitMigration(log, registersByAccount, 1)
		require.NoError(t, err)

		owner1 := string(address1[:])
		owner2 := string(address2[:])

		accountRegisters1 := registersByAccount.AccountRegisters(owner1)
		accountRegisters2 := registersByAccount.AccountRegisters(owner2)

		err = migration.MigrateAccount(
			ctx,
			common.Address(address1),
			accountRegisters1,
		)
		require.NoError(t, err)

		require.Equal(t, 2, registersByAccount.AccountCount())
		require.Equal(t, 1, accountRegisters1.Count())
		require.Equal(t, 1, accountRegisters2.Count())
		require.Equal(t,
			updatedContractA,
			contractCode(t, registersByAccount, owner1, "A"),
		)
		require.Equal(t,
			contractA,
			contractCode(t, registersByAccount, owner2, "A"),
		)

		err = migration.Close()
		require.NoError(t, err)

		require.Empty(t, writer.logs)
	})

	t.Run("not all contracts on one account migrated", func(t *testing.T) {
		t.Parallel()

		writer := &errorLogWriter{}
		log := zerolog.New(writer)

		rwf := &testReportWriterFactory{}

		options := StagedContractsMigrationOptions{
			ChainID:            flow.Emulator,
			VerboseErrorOutput: true,
		}
		migration := NewStagedContractsMigration(
			"test",
			"test",
			log,
			rwf,
			&LegacyTypeRequirements{},
			options,
		).
			WithStagedContractUpdates([]StagedContract{
				{
					Address: common.Address(address1),
					Contract: Contract{
						Name: "A",
						Code: []byte(updatedContractA),
					},
				},
				{
					Address: common.Address(address1),
					Contract: Contract{
						Name: "B",
						Code: []byte(updatedContractB),
					},
				},
			})

		registersByAccount, err := registersForStagedContracts(
			StagedContract{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "A",
					Code: []byte(contractA),
				},
			},
		)
		require.NoError(t, err)

		err = migration.InitMigration(log, registersByAccount, 1)
		require.NoError(t, err)

		owner1 := string(address1[:])

		accountRegisters1 := registersByAccount.AccountRegisters(owner1)

		err = migration.MigrateAccount(
			ctx,
			common.Address(address1),
			accountRegisters1,
		)
		require.NoError(t, err)

		require.Len(t, writer.logs, 1)
		assert.Contains(t,
			writer.logs[0],
			`missing old code`,
		)
	})

	t.Run("not all accounts migrated", func(t *testing.T) {
		t.Parallel()

		writer := &errorLogWriter{}
		log := zerolog.New(writer)

		rwf := &testReportWriterFactory{}

		options := StagedContractsMigrationOptions{
			ChainID:            flow.Emulator,
			VerboseErrorOutput: true,
		}
		migration := NewStagedContractsMigration(
			"test",
			"test",
			log,
			rwf,
			&LegacyTypeRequirements{},
			options,
		).
			WithStagedContractUpdates([]StagedContract{
				{
					Address: common.Address(address2),
					Contract: Contract{
						Name: "A",
						Code: []byte(updatedContractA),
					},
				},
			})

		registersByAccount, err := registersForStagedContracts(
			StagedContract{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "A",
					Code: []byte(contractA),
				},
			},
		)
		require.NoError(t, err)

		err = migration.InitMigration(log, registersByAccount, 1)
		require.NoError(t, err)

		owner1 := string(address1[:])

		accountRegisters1 := registersByAccount.AccountRegisters(owner1)

		err = migration.MigrateAccount(
			ctx,
			common.Address(address1),
			accountRegisters1,
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Len(t, writer.logs, 1)
		assert.Contains(t,
			writer.logs[0],
			`"failed to find all contract registers that need to be changed"`,
		)
	})
}

func TestSystemContractChanges(t *testing.T) {
	t.Parallel()

	const chainID = flow.Mainnet

	changes := SystemContractChanges(
		chainID,
		SystemContractsMigrationOptions{
			StagedContractsMigrationOptions: StagedContractsMigrationOptions{
				ChainID: chainID,
			},
			EVM:    EVMContractChangeUpdateFull,
			Burner: BurnerContractChangeDeploy,
		},
	)

	var changeLocations []common.AddressLocation

	for _, change := range changes {
		location := change.AddressLocation()
		changeLocations = append(changeLocations, location)
	}

	address1 := common.Address{0x86, 0x24, 0xb5, 0x2f, 0x9d, 0xdc, 0xd0, 0x4a}
	address2 := common.Address{0xe4, 0x67, 0xb9, 0xdd, 0x11, 0xfa, 0x00, 0xdf}
	address3 := common.Address{0x8d, 0x0e, 0x87, 0xb6, 0x51, 0x59, 0xae, 0x63}
	address4 := common.Address{0x62, 0x43, 0x0c, 0xf2, 0x8c, 0x26, 0xd0, 0x95}
	address5 := common.Address{0xf9, 0x19, 0xee, 0x77, 0x44, 0x7b, 0x74, 0x97}
	address6 := common.Address{0x16, 0x54, 0x65, 0x33, 0x99, 0x04, 0x0a, 0x61}
	address7 := common.Address{0xf2, 0x33, 0xdc, 0xee, 0x88, 0xfe, 0x0a, 0xbe}
	address8 := common.Address{0x1d, 0x7e, 0x57, 0xaa, 0x55, 0x81, 0x74, 0x48}

	assert.Equal(t,
		[]common.AddressLocation{
			{
				Name:    "FlowEpoch",
				Address: address1,
			},
			{
				Name:    "FlowIDTableStaking",
				Address: address1,
			},
			{
				Name:    "FlowClusterQC",
				Address: address1,
			},
			{
				Name:    "FlowDKG",
				Address: address1,
			},
			{
				Name:    "FlowServiceAccount",
				Address: address2,
			},
			{
				Name:    "NodeVersionBeacon",
				Address: address2,
			},
			{
				Name:    "RandomBeaconHistory",
				Address: address2,
			},
			{
				Name:    "FlowStorageFees",
				Address: address2,
			},
			{
				Name:    "FlowStakingCollection",
				Address: address3,
			},
			{
				Name:    "StakingProxy",
				Address: address4,
			},
			{
				Name:    "LockedTokens",
				Address: address3,
			},
			{
				Name:    "FlowFees",
				Address: address5,
			},
			{
				Name:    "FlowToken",
				Address: address6,
			},
			{
				Name:    "FungibleToken",
				Address: address7,
			},
			{
				Name:    "FungibleTokenMetadataViews",
				Address: address7,
			},
			{
				Name:    "NonFungibleToken",
				Address: address8,
			},
			{
				Name:    "MetadataViews",
				Address: address8,
			},
			{
				Name:    "ViewResolver",
				Address: address8,
			},
			{
				Name:    "FungibleTokenSwitchboard",
				Address: address7,
			},
			{
				Name:    "EVM",
				Address: address2,
			},
		},
		changeLocations,
	)
}
