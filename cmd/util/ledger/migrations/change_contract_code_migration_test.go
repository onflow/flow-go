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
		migration := NewStagedContractsMigration("test", "test", log, rwf, options)

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
		migration := NewStagedContractsMigration("test", "test", log, rwf, options)

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
		migration := NewStagedContractsMigration("test", "test", log, rwf, options).
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
		migration := NewStagedContractsMigration("test", "test", log, rwf, options).
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
		migration := NewStagedContractsMigration("test", "test", log, rwf, options).
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
		migration := NewStagedContractsMigration("test", "test", log, rwf, options).
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
		migration := NewStagedContractsMigration("test", "test", log, rwf, options).
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
		migration := NewStagedContractsMigration("test", "test", log, rwf, options).
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
