package migrations

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"

	"github.com/onflow/cadence/runtime/common"
)

type logWriter struct {
	logs           []string
	enableInfoLogs bool
}

var _ io.Writer = &logWriter{}

const infoLogPrefix = "{\"level\":\"info\""

func (l *logWriter) Write(bytes []byte) (int, error) {
	logStr := string(bytes)

	if !l.enableInfoLogs && strings.HasPrefix(logStr, infoLogPrefix) {
		return 0, nil
	}

	l.logs = append(l.logs, logStr)
	return len(bytes), nil
}

func registersForStagedContracts(stagedContracts ...StagedContract) (*registers.ByAccount, error) {
	registersByAccount := registers.NewByAccount()

	for _, stagedContract := range stagedContracts {
		err := registersByAccount.Set(
			string(stagedContract.Address[:]),
			flow.ContractKey(stagedContract.Name),
			stagedContract.Code,
		)
		if err != nil {
			return nil, err
		}
	}

	return registersByAccount, nil
}

func TestStagedContractsMigration(t *testing.T) {
	t.Parallel()

	const chainID = flow.Emulator
	addressGenerator := chainID.Chain().NewAddressGenerator()

	address1, err := addressGenerator.NextAddress()
	require.NoError(t, err)

	address2, err := addressGenerator.NextAddress()
	require.NoError(t, err)

	t.Run("one contract", func(t *testing.T) {
		t.Parallel()

		oldCode := "access(all) contract A {}"
		newCode := "access(all) contract A { access(all) struct B {} }"

		stagedContracts := []StagedContract{
			{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "A",
					Code: []byte(newCode),
				},
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		rwf := &testReportWriterFactory{}

		options := StagedContractsMigrationOptions{
			ChainID:            chainID,
			VerboseErrorOutput: true,
		}

		migration := NewStagedContractsMigration("test", "test", log, rwf, options).
			WithStagedContractUpdates(stagedContracts)

		registersByAccount, err := registersForStagedContracts(
			StagedContract{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "A",
					Code: []byte(oldCode),
				},
			},
		)
		require.NoError(t, err)

		err = migration.InitMigration(log, registersByAccount, 1)
		require.NoError(t, err)

		owner1 := string(address1[:])

		accountRegisters1 := registersByAccount.AccountRegisters(owner1)

		err = migration.MigrateAccount(
			context.Background(),
			common.Address(address1),
			accountRegisters1,
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Empty(t, logWriter.logs)

		// Registers should have the new code
		require.Equal(t, 1, registersByAccount.AccountCount())
		require.Equal(t, 1, accountRegisters1.Count())
		require.Equal(t, newCode, contractCode(t, registersByAccount, owner1, "A"))
	})

	t.Run("syntax error in new code", func(t *testing.T) {
		t.Parallel()

		oldCode := "access(all) contract A {}"
		newCode := "access(all) contract A { access(all) struct B () }"

		stagedContracts := []StagedContract{
			{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "A",
					Code: []byte(newCode),
				},
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		rwf := &testReportWriterFactory{}

		options := StagedContractsMigrationOptions{
			ChainID:            chainID,
			VerboseErrorOutput: true,
		}

		migration := NewStagedContractsMigration("test", "test", log, rwf, options).
			WithContractUpdateValidation()
		migration.WithStagedContractUpdates(stagedContracts)

		registersByAccount, err := registersForStagedContracts(
			StagedContract{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "A",
					Code: []byte(oldCode),
				},
			},
		)
		require.NoError(t, err)

		err = migration.InitMigration(log, registersByAccount, 1)
		require.NoError(t, err)

		owner1 := string(address1[:])

		accountRegisters1 := registersByAccount.AccountRegisters(owner1)

		err = migration.MigrateAccount(
			context.Background(),
			common.Address(address1),
			accountRegisters1,
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Len(t, logWriter.logs, 1)
		require.Contains(t, logWriter.logs[0], `error: expected token '{'`)

		// Registers should still have the old code
		require.Equal(t, 1, registersByAccount.AccountCount())
		require.Equal(t, 1, accountRegisters1.Count())
		require.Equal(t, oldCode, contractCode(t, registersByAccount, owner1, "A"))
	})

	t.Run("syntax error in old code", func(t *testing.T) {
		t.Parallel()

		oldCode := "access(all) contract A {"
		newCode := "access(all) contract A { access(all) struct B {} }"

		stagedContracts := []StagedContract{
			{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "A",
					Code: []byte(newCode),
				},
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		rwf := &testReportWriterFactory{}

		options := StagedContractsMigrationOptions{
			ChainID:            chainID,
			VerboseErrorOutput: true,
		}

		migration := NewStagedContractsMigration("test", "test", log, rwf, options).
			WithContractUpdateValidation().
			WithStagedContractUpdates(stagedContracts)

		registersByAccount, err := registersForStagedContracts(
			StagedContract{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "A",
					Code: []byte(oldCode),
				},
			},
		)
		require.NoError(t, err)

		err = migration.InitMigration(log, registersByAccount, 1)
		require.NoError(t, err)

		owner1 := string(address1[:])

		accountRegisters1 := registersByAccount.AccountRegisters(owner1)

		err = migration.MigrateAccount(
			context.Background(),
			common.Address(address1),
			accountRegisters1,
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Len(t, logWriter.logs, 1)
		require.Contains(t, logWriter.logs[0], `error: expected token '}'`)

		// Registers should still have the old code
		require.Equal(t, 1, registersByAccount.AccountCount())
		require.Equal(t, 1, accountRegisters1.Count())
		require.Equal(t, oldCode, contractCode(t, registersByAccount, owner1, "A"))
	})

	t.Run("one fail, one success", func(t *testing.T) {
		t.Parallel()

		oldCode1 := "access(all) contract A {}"
		oldCode2 := "access(all) contract B {}"

		newCode1 := "access(all) contract A { access(all) struct C () }" // broken
		newCode2 := "access(all) contract B { access(all) struct C {} }" // all good

		stagedContracts := []StagedContract{
			{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "A",
					Code: []byte(newCode1),
				},
			},
			{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "B",
					Code: []byte(newCode2),
				},
			},
		}

		logWriter := &logWriter{
			enableInfoLogs: true,
		}

		log := zerolog.New(logWriter)

		rwf := &testReportWriterFactory{}

		options := StagedContractsMigrationOptions{
			ChainID:            chainID,
			VerboseErrorOutput: true,
		}

		const reporterName = "test"
		migration := NewStagedContractsMigration("test", reporterName, log, rwf, options).
			WithContractUpdateValidation().
			WithStagedContractUpdates(stagedContracts)

		registersByAccount, err := registersForStagedContracts(
			StagedContract{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "A",
					Code: []byte(oldCode1),
				},
			},
			StagedContract{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "B",
					Code: []byte(oldCode2),
				},
			},
		)
		require.NoError(t, err)

		err = migration.InitMigration(log, registersByAccount, 1)
		require.NoError(t, err)

		owner1 := string(address1[:])

		accountRegisters1 := registersByAccount.AccountRegisters(owner1)

		err = migration.MigrateAccount(
			context.Background(),
			common.Address(address1),
			accountRegisters1,
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Len(t, logWriter.logs, 2)
		require.Contains(t, logWriter.logs[0], `{"level":"info","message":"total of 2 staged contracts are provided externally"}`)
		require.Contains(t, logWriter.logs[1], `error: expected token '{'`)

		require.Equal(t, 1, registersByAccount.AccountCount())
		require.Equal(t, 2, accountRegisters1.Count())
		// First register should still have the old code
		require.Equal(t, oldCode1, contractCode(t, registersByAccount, owner1, "A"))
		// Second register should have the updated code
		require.Equal(t, newCode2, contractCode(t, registersByAccount, owner1, "B"))

		reportWriter := rwf.reportWriters[reporterName]
		require.NotNil(t, reportWriter)
		assert.ElementsMatch(
			t,
			[]any{
				contractUpdateFailureEntry{
					AccountAddress: common.Address(address1),
					ContractName:   "A",
					Error:          "error: expected token '{'\n --> f8d6e0586b0a20c7.A:1:46\n  |\n1 | access(all) contract A { access(all) struct C () }\n  |                                               ^\n",
				},
				contractUpdateEntry{
					AccountAddress: common.Address(address1),
					ContractName:   "B",
				},
			},
			reportWriter.entries,
		)
	})

	t.Run("different accounts", func(t *testing.T) {
		t.Parallel()

		oldCode := "access(all) contract A {}"
		newCode := "access(all) contract A { access(all) struct B {} }"

		stagedContracts := []StagedContract{
			{
				Address: common.Address(address2),
				Contract: Contract{
					Name: "A",
					Code: []byte(newCode),
				},
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		rwf := &testReportWriterFactory{}

		options := StagedContractsMigrationOptions{
			ChainID:            chainID,
			VerboseErrorOutput: true,
		}

		const reporterName = "test"
		migration := NewStagedContractsMigration("test", reporterName, log, rwf, options).
			WithStagedContractUpdates(stagedContracts)

		registersByAccount, err := registersForStagedContracts(
			StagedContract{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "A",
					Code: []byte(oldCode),
				},
			},
			StagedContract{
				Address: common.Address(address2),
				Contract: Contract{
					Name: "A",
					Code: []byte(oldCode),
				},
			},
		)
		require.NoError(t, err)

		err = migration.InitMigration(log, registersByAccount, 1)
		require.NoError(t, err)

		// Run migration for account 1,
		// There are no staged updates for contracts in account 1.
		// So codes should not have been updated.

		owner1 := string(address1[:])
		accountRegisters1 := registersByAccount.AccountRegisters(owner1)

		err = migration.MigrateAccount(
			context.Background(),
			common.Address(address1),
			accountRegisters1,
		)
		require.NoError(t, err)
		require.Equal(t, 1, accountRegisters1.Count())
		require.Equal(t, oldCode, contractCode(t, registersByAccount, owner1, "A"))

		// Run migration for account 2
		// There is one staged update for contracts in account 2.
		// So one register/contract-code should be updated, and the other should remain the same.

		owner2 := string(address2[:])
		accountRegisters2 := registersByAccount.AccountRegisters(owner2)

		err = migration.MigrateAccount(
			context.Background(),
			common.Address(address2),
			accountRegisters2,
		)
		require.NoError(t, err)
		require.Equal(t, 1, accountRegisters2.Count())
		require.Equal(t, newCode, contractCode(t, registersByAccount, owner2, "A"))

		err = migration.Close()
		require.NoError(t, err)

		// No errors.
		require.Empty(t, logWriter.logs)

		reportWriter := rwf.reportWriters[reporterName]
		require.NotNil(t, reportWriter)
		require.Len(t, reportWriter.entries, 1)
		assert.Equal(
			t,
			contractUpdateEntry{
				AccountAddress: common.Address(address2),
				ContractName:   "A",
			},
			reportWriter.entries[0],
		)
	})

	t.Run("multiple updates for same contract", func(t *testing.T) {
		t.Parallel()

		oldCode := "access(all) contract A {}"
		update1 := "access(all) contract A { access(all) struct B {} }"
		update2 := "access(all) contract A { access(all) struct B {} access(all) struct C {} }"

		stagedContracts := []StagedContract{
			{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "A",
					Code: []byte(update1),
				},
			},
			{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "A",
					Code: []byte(update2),
				},
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		rwf := &testReportWriterFactory{}

		options := StagedContractsMigrationOptions{
			ChainID:            chainID,
			VerboseErrorOutput: true,
		}

		migration := NewStagedContractsMigration("test", "test", log, rwf, options).
			WithStagedContractUpdates(stagedContracts)

		registersByAccount, err := registersForStagedContracts(
			StagedContract{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "A",
					Code: []byte(oldCode),
				},
			},
		)
		require.NoError(t, err)

		err = migration.InitMigration(log, registersByAccount, 1)
		require.NoError(t, err)

		owner1 := string(address1[:])
		accountRegisters1 := registersByAccount.AccountRegisters(owner1)

		err = migration.MigrateAccount(
			context.Background(),
			common.Address(address1),
			accountRegisters1,
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Len(t, logWriter.logs, 1)
		require.Contains(
			t,
			logWriter.logs[0],
			`existing staged update found`,
		)

		require.Equal(t, 1, registersByAccount.AccountCount())
		require.Equal(t, 1, accountRegisters1.Count())
		require.Equal(t, update2, contractCode(t, registersByAccount, owner1, "A"))
	})

	t.Run("missing old contract", func(t *testing.T) {
		t.Parallel()

		newCode := "access(all) contract A { access(all) struct B {} }"

		stagedContracts := []StagedContract{
			{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "A",
					Code: []byte(newCode),
				},
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		rwf := &testReportWriterFactory{}

		options := StagedContractsMigrationOptions{
			ChainID:            chainID,
			VerboseErrorOutput: true,
		}

		migration := NewStagedContractsMigration("test", "test", log, rwf, options).
			WithStagedContractUpdates(stagedContracts)

		// NOTE: no payloads
		registersByAccount := registers.NewByAccount()

		err := migration.InitMigration(log, registersByAccount, 1)
		require.NoError(t, err)

		owner1 := string(address1[:])
		accountRegisters1 := registersByAccount.AccountRegisters(owner1)

		err = migration.MigrateAccount(
			context.Background(),
			common.Address(address1),
			accountRegisters1,
		)
		require.NoError(t, err)

		require.Len(t, logWriter.logs, 1)
		assert.Contains(t,
			logWriter.logs[0],
			"missing old code for contract",
		)
	})

	t.Run("staged contract in storage", func(t *testing.T) {
		t.Parallel()

		oldCode := "access(all) contract A {}"
		newCode := "access(all) contract A { access(all) struct B {} }"

		const chainID = flow.Testnet
		addressGenerator := chainID.Chain().NewAddressGenerator()
		accountAddress, err := addressGenerator.NextAddress()
		require.NoError(t, err)

		stagingAccountAddress := common.Address(flow.HexToAddress("0x2ceae959ed1a7e7a"))

		registersByAccount := registers.NewByAccount()

		// Create account status register
		accountStatus := environment.NewAccountStatus()

		err = registersByAccount.Set(
			string(stagingAccountAddress[:]),
			flow.AccountStatusKey,
			accountStatus.ToBytes(),
		)
		require.NoError(t, err)

		mr, err := NewInterpreterMigrationRuntime(
			registersByAccount,
			chainID,
			InterpreterMigrationRuntimeConfig{},
		)
		require.NoError(t, err)

		// Create new storage map
		domain := common.PathDomainStorage.Identifier()
		storageMap := mr.Storage.GetStorageMap(stagingAccountAddress, domain, true)

		contractUpdateValue := interpreter.NewCompositeValue(
			mr.Interpreter,
			interpreter.EmptyLocationRange,
			common.AddressLocation{
				Address: stagingAccountAddress,
				Name:    "MigrationContractStaging",
			},
			"MigrationContractStaging.ContractUpdate",
			common.CompositeKindStructure,
			[]interpreter.CompositeField{
				{
					Name:  "address",
					Value: interpreter.AddressValue(accountAddress),
				},
				{
					Name:  "name",
					Value: interpreter.NewUnmeteredStringValue("A"),
				},
				{
					Name:  "code",
					Value: interpreter.NewUnmeteredStringValue(newCode),
				},
			},
			stagingAccountAddress,
		)

		capsuleValue := interpreter.NewCompositeValue(
			mr.Interpreter,
			interpreter.EmptyLocationRange,
			common.AddressLocation{
				Address: stagingAccountAddress,
				Name:    "MigrationContractStaging",
			},
			"MigrationContractStaging.Capsule",
			common.CompositeKindResource,
			[]interpreter.CompositeField{
				{
					Name:  "update",
					Value: contractUpdateValue,
				},
			},
			stagingAccountAddress,
		)

		// Write the staged contract capsule value.
		storageMap.WriteValue(
			mr.Interpreter,
			interpreter.StringStorageMapKey("MigrationContractStagingCapsule_some_random_suffix"),
			capsuleValue,
		)

		// Write some random values as well.
		storageMap.WriteValue(
			mr.Interpreter,
			interpreter.StringStorageMapKey("some_key"),
			interpreter.NewUnmeteredStringValue("Also in the same account"),
		)

		storageMap.WriteValue(
			mr.Interpreter,
			interpreter.StringStorageMapKey("MigrationContractStagingCapsule_some_garbage_value"),
			interpreter.NewUnmeteredStringValue("Also in the same storage path prefix"),
		)

		err = mr.Storage.Commit(mr.Interpreter, false)
		require.NoError(t, err)

		result, err := mr.TransactionState.FinalizeMainTransaction()
		require.NoError(t, err)

		err = registers.ApplyChanges(
			registersByAccount,
			result.WriteSet,
			nil,
			zerolog.Nop(),
		)
		require.NoError(t, err)

		logWriter := &logWriter{
			enableInfoLogs: true,
		}
		log := zerolog.New(logWriter)

		rwf := &testReportWriterFactory{}

		// Important: Do not stage contracts externally.
		// Should be scanned and collected from the storage.

		options := StagedContractsMigrationOptions{
			ChainID:            chainID,
			VerboseErrorOutput: true,
		}

		migration := NewStagedContractsMigration("test", "test", log, rwf, options)

		accountOwner := string(accountAddress[:])

		err = registersByAccount.Set(
			accountOwner,
			flow.ContractKey("A"),
			[]byte(oldCode),
		)
		require.NoError(t, err)

		err = migration.InitMigration(log, registersByAccount, 1)
		require.NoError(t, err)

		accountRegisters := registersByAccount.AccountRegisters(accountOwner)

		err = migration.MigrateAccount(
			context.Background(),
			common.Address(accountAddress),
			accountRegisters,
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Len(t, logWriter.logs, 4)
		require.Contains(t, logWriter.logs[0], "found 6 registers in account 0x2ceae959ed1a7e7a")
		require.Contains(t, logWriter.logs[1], "found a value with an unexpected type `String`")
		require.Contains(t, logWriter.logs[2], "found 1 staged contracts from payloads")
		require.Contains(t, logWriter.logs[3], "total of 1 unique contracts are staged for all accounts")

		require.Equal(t, 1, accountRegisters.Count())
		require.Equal(t, newCode, contractCode(t, registersByAccount, accountOwner, "A"))
	})
}

func contractCode(t *testing.T, registersByAccount *registers.ByAccount, owner, contractName string) string {
	code, err := registersByAccount.Get(owner, flow.ContractKey(contractName))
	require.NoError(t, err)
	return string(code)
}

func TestStagedContractsWithImports(t *testing.T) {
	t.Parallel()

	const chainID = flow.Emulator

	addressGenerator := chainID.Chain().NewAddressGenerator()

	address1, err := addressGenerator.NextAddress()
	require.NoError(t, err)

	address2, err := addressGenerator.NextAddress()
	require.NoError(t, err)

	t.Run("valid import", func(t *testing.T) {
		t.Parallel()

		oldCodeA := fmt.Sprintf(`
            import B from %s
            access(all) contract A {}
        `,
			address2.HexWithPrefix(),
		)

		oldCodeB := `access(all) contract B {}`

		newCodeA := fmt.Sprintf(`
            import B from %s
            access(all) contract A {
                access(all) fun foo(a: B.C) {}
            }
        `,
			address2.HexWithPrefix(),
		)

		newCodeB := `
            access(all) contract B {
                access(all) struct C {}
            }
        `

		stagedContracts := []StagedContract{
			{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "A",
					Code: []byte(newCodeA),
				},
			},
			{
				Address: common.Address(address2),
				Contract: Contract{
					Name: "B",
					Code: []byte(newCodeB),
				},
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		rwf := &testReportWriterFactory{}

		options := StagedContractsMigrationOptions{
			ChainID:            chainID,
			VerboseErrorOutput: true,
		}

		migration := NewStagedContractsMigration("test", "test", log, rwf, options).
			WithStagedContractUpdates(stagedContracts)

		registersByAccount, err := registersForStagedContracts(
			StagedContract{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "A",
					Code: []byte(oldCodeA),
				},
			},
			StagedContract{
				Address: common.Address(address2),
				Contract: Contract{
					Name: "B",
					Code: []byte(oldCodeB),
				},
			},
		)
		require.NoError(t, err)

		err = migration.InitMigration(log, registersByAccount, 1)
		require.NoError(t, err)

		owner1 := string(address1[:])
		accountRegisters1 := registersByAccount.AccountRegisters(owner1)

		err = migration.MigrateAccount(
			context.Background(),
			common.Address(address1),
			accountRegisters1,
		)
		require.NoError(t, err)

		owner2 := string(address2[:])
		accountRegisters2 := registersByAccount.AccountRegisters(owner2)

		err = migration.MigrateAccount(
			context.Background(),
			common.Address(address2),
			accountRegisters2,
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Empty(t, logWriter.logs)

		require.Equal(t, 1, accountRegisters1.Count())
		assert.Equal(t, newCodeA, contractCode(t, registersByAccount, owner1, "A"))

		require.Equal(t, 1, accountRegisters2.Count())
		assert.Equal(t, newCodeB, contractCode(t, registersByAccount, owner2, "B"))
	})

	t.Run("broken import, no update staged", func(t *testing.T) {
		t.Parallel()

		oldCodeA := fmt.Sprintf(
			`
                import B from %s
                access(all) contract A {}
            `,
			address2.HexWithPrefix(),
		)

		oldCodeB := `pub contract B {}  // not compatible`

		newCodeA := fmt.Sprintf(
			`
                import B from %s
                access(all) contract A {
                    access(all) fun foo(a: B.C) {}
                }
            `,
			address2.HexWithPrefix(),
		)

		stagedContracts := []StagedContract{
			{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "A",
					Code: []byte(newCodeA),
				},
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		rwf := &testReportWriterFactory{}

		options := StagedContractsMigrationOptions{
			ChainID:            chainID,
			VerboseErrorOutput: true,
		}

		migration := NewStagedContractsMigration("test", "test", log, rwf, options).
			WithContractUpdateValidation().
			WithStagedContractUpdates(stagedContracts)

		registersByAccount, err := registersForStagedContracts(
			StagedContract{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "A",
					Code: []byte(oldCodeA),
				},
			},
			StagedContract{
				Address: common.Address(address2),
				Contract: Contract{
					Name: "B",
					Code: []byte(oldCodeB),
				},
			},
		)
		require.NoError(t, err)

		err = migration.InitMigration(log, registersByAccount, 1)
		require.NoError(t, err)

		owner1 := string(address1[:])
		accountRegisters1 := registersByAccount.AccountRegisters(owner1)

		err = migration.MigrateAccount(
			context.Background(),
			common.Address(address1),
			accountRegisters1,
		)
		require.NoError(t, err)

		owner2 := string(address2[:])
		accountRegisters2 := registersByAccount.AccountRegisters(owner2)

		err = migration.MigrateAccount(
			context.Background(),
			common.Address(address2),
			accountRegisters2,
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Len(t, logWriter.logs, 1)
		require.Contains(
			t,
			logWriter.logs[0],
			"`pub` is no longer a valid access keyword",
		)

		// Registers should be the old ones
		require.Equal(t, 1, accountRegisters1.Count())
		assert.Equal(t, oldCodeA, contractCode(t, registersByAccount, owner1, "A"))

		require.Equal(t, 1, accountRegisters2.Count())
		assert.Equal(t, oldCodeB, contractCode(t, registersByAccount, owner2, "B"))
	})

	t.Run("broken import ", func(t *testing.T) {
		t.Parallel()

		oldCodeA := fmt.Sprintf(
			`
                import B from %s
                access(all) contract A {}
            `,
			address2.HexWithPrefix(),
		)

		oldCodeB := `pub contract B {}  // not compatible`

		newCodeA := fmt.Sprintf(
			`
                import B from %s
                access(all) contract A {
                    access(all) fun foo(a: B.C) {}
                }
            `,
			address2.HexWithPrefix(),
		)

		newCodeB := `pub contract B {}  // not compatible`

		stagedContracts := []StagedContract{
			{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "A",
					Code: []byte(newCodeA),
				},
			},
			{
				Address: common.Address(address2),
				Contract: Contract{
					Name: "B",
					Code: []byte(newCodeB),
				},
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		rwf := &testReportWriterFactory{}

		options := StagedContractsMigrationOptions{
			ChainID:            chainID,
			VerboseErrorOutput: true,
		}

		migration := NewStagedContractsMigration("test", "test", log, rwf, options).
			WithContractUpdateValidation().
			WithStagedContractUpdates(stagedContracts)

		registersByAccount, err := registersForStagedContracts(
			StagedContract{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "A",
					Code: []byte(oldCodeA),
				},
			},
			StagedContract{
				Address: common.Address(address2),
				Contract: Contract{
					Name: "B",
					Code: []byte(oldCodeB),
				},
			},
		)
		require.NoError(t, err)

		err = migration.InitMigration(log, registersByAccount, 1)
		require.NoError(t, err)

		owner1 := string(address1[:])
		accountRegisters1 := registersByAccount.AccountRegisters(owner1)

		err = migration.MigrateAccount(
			context.Background(),
			common.Address(address1),
			accountRegisters1,
		)
		require.NoError(t, err)

		owner2 := string(address2[:])
		accountRegisters2 := registersByAccount.AccountRegisters(owner2)

		err = migration.MigrateAccount(
			context.Background(),
			common.Address(address2),
			accountRegisters2,
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Len(t, logWriter.logs, 2)
		assert.Contains(
			t,
			logWriter.logs[0],
			"cannot find type in this scope: `B`",
		)
		assert.Contains(
			t,
			logWriter.logs[1],
			"`pub` is no longer a valid access keyword",
		)

		// Registers should be the old ones
		require.Equal(t, 1, accountRegisters1.Count())
		assert.Equal(t, oldCodeA, contractCode(t, registersByAccount, owner1, "A"))

		require.Equal(t, 1, accountRegisters2.Count())
		assert.Equal(t, oldCodeB, contractCode(t, registersByAccount, owner2, "B"))
	})

	t.Run("broken import in one, valid third contract", func(t *testing.T) {
		t.Parallel()

		oldCodeA := fmt.Sprintf(`
            import B from %s
            access(all) contract A {}
        `,
			address2.HexWithPrefix(),
		)

		oldCodeB := `pub contract B {}  // not compatible`

		oldCodeC := `pub contract C {}`

		newCodeA := fmt.Sprintf(`
            import B from %s
            access(all) contract A {
                access(all) fun foo(a: B.X) {}
            }
        `,
			address2.HexWithPrefix(),
		)

		newCodeC := `access(all) contract C {}`

		stagedContracts := []StagedContract{
			{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "A",
					Code: []byte(newCodeA),
				},
			},
			{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "C",
					Code: []byte(newCodeC),
				},
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		rwf := &testReportWriterFactory{}

		options := StagedContractsMigrationOptions{
			ChainID:            chainID,
			VerboseErrorOutput: true,
		}

		migration := NewStagedContractsMigration("test", "test", log, rwf, options).
			WithContractUpdateValidation().
			WithStagedContractUpdates(stagedContracts)

		registersByAccount, err := registersForStagedContracts(
			StagedContract{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "A",
					Code: []byte(oldCodeA),
				},
			},
			StagedContract{
				Address: common.Address(address1),
				Contract: Contract{
					Name: "C",
					Code: []byte(oldCodeC),
				},
			},
			StagedContract{
				Address: common.Address(address2),
				Contract: Contract{
					Name: "B",
					Code: []byte(oldCodeB),
				},
			},
		)
		require.NoError(t, err)

		err = migration.InitMigration(log, registersByAccount, 1)
		require.NoError(t, err)

		owner1 := string(address1[:])
		accountRegisters1 := registersByAccount.AccountRegisters(owner1)

		err = migration.MigrateAccount(
			context.Background(),
			common.Address(address1),
			accountRegisters1,
		)
		require.NoError(t, err)

		owner2 := string(address2[:])
		accountRegisters2 := registersByAccount.AccountRegisters(owner2)

		err = migration.MigrateAccount(
			context.Background(),
			common.Address(address2),
			accountRegisters2,
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Len(t, logWriter.logs, 1)
		require.Contains(
			t,
			logWriter.logs[0],
			"`pub` is no longer a valid access keyword",
		)

		// A and B should be the old ones.
		// C should be updated.
		// Type checking failures in unrelated contracts should not
		// stop other contracts from being migrated.
		require.Equal(t, 2, accountRegisters1.Count())
		require.Equal(t, oldCodeA, contractCode(t, registersByAccount, owner1, "A"))
		require.Equal(t, newCodeC, contractCode(t, registersByAccount, owner1, "C"))

		require.Equal(t, 1, accountRegisters2.Count())
		require.Equal(t, oldCodeB, contractCode(t, registersByAccount, owner2, "B"))
	})
}

func TestStagedContractsFromCSV(t *testing.T) {

	t.Parallel()

	t.Run("valid csv", func(t *testing.T) {

		t.Parallel()

		const path = "test-data/staged_contracts_migration/staged_contracts.csv"

		contracts, err := StagedContractsFromCSV(path)
		require.NoError(t, err)

		require.Len(t, contracts, 4)
		assert.Equal(
			t,
			contracts,
			[]StagedContract{
				{
					Address: common.MustBytesToAddress([]byte{0x1}),
					Contract: Contract{
						Name: "Foo",
						Code: []byte("access(all) contract Foo{}"),
					},
				},
				{
					Address: common.MustBytesToAddress([]byte{0x1}),
					Contract: Contract{
						Name: "Bar",
						Code: []byte("access(all) contract Bar{}"),
					},
				},
				{
					Address: common.MustBytesToAddress([]byte{0x2}),
					Contract: Contract{
						Name: "MultilineContract",
						Code: []byte(`
import Foo from 0x01

access(all)
contract MultilineContract{
  init() {
      var a = "hello"
  }
}
`),
					},
				},
				{
					Address: common.MustBytesToAddress([]byte{0x2}),
					Contract: Contract{
						Name: "Baz",
						Code: []byte("import Foo from 0x01 access(all) contract Baz{}"),
					},
				},
			},
		)
	})

	t.Run("malformed csv", func(t *testing.T) {

		t.Parallel()

		const path = "test-data/staged_contracts_migration/staged_contracts_malformed.csv"

		contracts, err := StagedContractsFromCSV(path)
		require.Error(t, err)
		assert.Equal(t, "record on line 2: wrong number of fields", err.Error())
		require.Empty(t, contracts)
	})

	t.Run("too few fields", func(t *testing.T) {

		t.Parallel()

		const path = "test-data/staged_contracts_migration/too_few_fields.csv"

		contracts, err := StagedContractsFromCSV(path)
		require.Error(t, err)
		assert.Equal(t, "record on line 1: wrong number of fields", err.Error())
		require.Empty(t, contracts)
	})

	t.Run("empty path", func(t *testing.T) {

		t.Parallel()

		const emptyPath = ""

		contracts, err := StagedContractsFromCSV(emptyPath)
		require.NoError(t, err)
		require.Empty(t, contracts)
	})
}

func TestStagedContractsWithUpdateValidator(t *testing.T) {
	t.Parallel()

	const chainID = flow.Emulator
	systemContracts := systemcontracts.SystemContractsForChain(chainID)

	chain := chainID.Chain()

	// Prevent conflicts with system contracts
	addressA, err := chain.AddressAtIndex(1_000_000)
	require.NoError(t, err)

	addressB, err := chain.AddressAtIndex(2_000_000)
	require.NoError(t, err)

	t.Run("FungibleToken.Vault", func(t *testing.T) {
		t.Parallel()

		ftAddress := common.Address(systemContracts.FungibleToken.Address)

		oldCodeA := fmt.Sprintf(
			`
              import FungibleToken from %s

              pub contract A {
                  pub var vault: @FungibleToken.Vault?
                  init() {
                      self.vault <- nil
                  }
              }
            `,
			ftAddress.HexWithPrefix(),
		)

		newCodeA := fmt.Sprintf(
			`
                import FungibleToken from %s

                access(all) contract A {
                    access(all) var vault: @{FungibleToken.Vault}?
                    init() {
                        self.vault <- nil
                    }
                }
            `,
			ftAddress.HexWithPrefix(),
		)

		ftContract := `
           access(all) contract FungibleToken {
               access(all) resource interface Vault {}
           }
        `

		stagedContracts := []StagedContract{
			{
				Address: common.Address(addressA),
				Contract: Contract{
					Name: "A",
					Code: []byte(newCodeA),
				},
			},
			{
				Address: ftAddress,
				Contract: Contract{
					Name: "FungibleToken",
					Code: []byte(ftContract),
				},
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		rwf := &testReportWriterFactory{}

		options := StagedContractsMigrationOptions{
			ChainID:            chainID,
			VerboseErrorOutput: true,
		}

		migration := NewStagedContractsMigration("test", "test", log, rwf, options).
			WithContractUpdateValidation().
			WithStagedContractUpdates(stagedContracts)

		registersByAccount, err := registersForStagedContracts(
			StagedContract{
				Address: common.Address(addressA),
				Contract: Contract{
					Name: "A",
					Code: []byte(oldCodeA),
				},
			},
			StagedContract{
				Address: ftAddress,
				Contract: Contract{
					Name: "FungibleToken",
					Code: []byte(ftContract),
				},
			},
		)
		require.NoError(t, err)

		err = migration.InitMigration(log, registersByAccount, 1)
		require.NoError(t, err)

		ownerA := string(addressA[:])
		accountRegistersA := registersByAccount.AccountRegisters(ownerA)

		err = migration.MigrateAccount(
			context.Background(),
			common.Address(addressA),
			accountRegistersA,
		)
		require.NoError(t, err)

		ftOwner := string(ftAddress[:])
		ftAccountRegisters := registersByAccount.AccountRegisters(ftOwner)

		err = migration.MigrateAccount(
			context.Background(),
			ftAddress,
			ftAccountRegisters,
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Empty(t, logWriter.logs)

		require.Equal(t, 1, accountRegistersA.Count())
		assert.Equal(t, newCodeA, contractCode(t, registersByAccount, ownerA, "A"))

		require.Equal(t, 1, ftAccountRegisters.Count())
		assert.Equal(t, ftContract, contractCode(t, registersByAccount, ftOwner, "FungibleToken"))
	})

	t.Run("other type", func(t *testing.T) {
		t.Parallel()

		otherAddress, err := common.HexToAddress("0x2")
		require.NoError(t, err)

		oldCodeA := fmt.Sprintf(
			`
              import FungibleToken from %s

              pub contract A {
                  pub var vault: @FungibleToken.Vault?
                  init() {
                      self.vault <- nil
                  }
              }
            `,
			otherAddress.HexWithPrefix(), // Importing from some other address
		)

		newCodeA := fmt.Sprintf(
			`
              import FungibleToken from %s

              access(all) contract A {
                  access(all) var vault: @{FungibleToken.Vault}?
                  init() {
                      self.vault <- nil
                  }
              }
            `,
			otherAddress.HexWithPrefix(), // Importing from some other address
		)

		ftContract := `
          access(all) contract FungibleToken {
              access(all) resource interface Vault {}
          }
        `

		stagedContracts := []StagedContract{
			{
				Address: common.Address(addressA),
				Contract: Contract{
					Name: "A",
					Code: []byte(newCodeA),
				},
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		rwf := &testReportWriterFactory{}

		options := StagedContractsMigrationOptions{
			ChainID:            chainID,
			VerboseErrorOutput: true,
		}

		migration := NewStagedContractsMigration("test", "test", log, rwf, options).
			WithContractUpdateValidation().
			WithStagedContractUpdates(stagedContracts)

		registersByAccount, err := registersForStagedContracts(
			StagedContract{
				Address: common.Address(addressA),
				Contract: Contract{
					Name: "A",
					Code: []byte(oldCodeA),
				},
			},
			StagedContract{
				Address: otherAddress,
				Contract: Contract{
					Name: "FungibleToken",
					Code: []byte(ftContract),
				},
			},
		)
		require.NoError(t, err)

		err = migration.InitMigration(log, registersByAccount, 1)
		require.NoError(t, err)

		ownerA := string(addressA[:])
		accountRegistersA := registersByAccount.AccountRegisters(ownerA)

		err = migration.MigrateAccount(
			context.Background(),
			common.Address(addressA),
			accountRegistersA,
		)
		require.NoError(t, err)

		otherOwner := string(otherAddress[:])
		otherAccountRegisters := registersByAccount.AccountRegisters(otherOwner)

		err = migration.MigrateAccount(
			context.Background(),
			otherAddress,
			otherAccountRegisters,
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Len(t, logWriter.logs, 1)
		assert.Contains(t,
			logWriter.logs[0],
			"incompatible type annotations. expected `FungibleToken.Vault`, found `{FungibleToken.Vault}`",
		)

		require.Equal(t, 1, accountRegistersA.Count())
		assert.Equal(t, oldCodeA, contractCode(t, registersByAccount, ownerA, "A"))

		require.Equal(t, 1, otherAccountRegisters.Count())
		assert.Equal(t, ftContract, contractCode(t, registersByAccount, otherOwner, "FungibleToken"))
	})

	t.Run("import from other account", func(t *testing.T) {
		t.Parallel()

		oldCodeA := fmt.Sprintf(
			`
              import B from %s

              pub contract A {}
            `,
			addressB.HexWithPrefix(),
		)

		newCodeA := fmt.Sprintf(
			`
              import B from %s

              access(all) contract A {}
            `,
			addressB.HexWithPrefix(),
		)

		codeB := `
          access(all) contract B {}
        `

		stagedContracts := []StagedContract{
			{
				Address: common.Address(addressA),
				Contract: Contract{
					Name: "A",
					Code: []byte(newCodeA),
				},
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		rwf := &testReportWriterFactory{}

		options := StagedContractsMigrationOptions{
			ChainID:            chainID,
			VerboseErrorOutput: true,
		}

		migration := NewStagedContractsMigration("test", "test", log, rwf, options).
			WithContractUpdateValidation().
			WithStagedContractUpdates(stagedContracts)

		registersByAccount, err := registersForStagedContracts(
			StagedContract{
				Address: common.Address(addressA),
				Contract: Contract{
					Name: "A",
					Code: []byte(oldCodeA),
				},
			},
			StagedContract{
				Address: common.Address(addressB),
				Contract: Contract{
					Name: "B",
					Code: []byte(codeB),
				},
			},
		)
		require.NoError(t, err)

		err = migration.InitMigration(log, registersByAccount, 1)
		require.NoError(t, err)

		ownerA := string(addressA[:])
		accountRegistersA := registersByAccount.AccountRegisters(ownerA)

		err = migration.MigrateAccount(
			context.Background(),
			common.Address(addressA),
			accountRegistersA,
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Empty(t, logWriter.logs)

		require.Equal(t, 1, accountRegistersA.Count())
		assert.Equal(t, newCodeA, contractCode(t, registersByAccount, ownerA, "A"))
	})
}

func TestStagedContractConformanceChanges(t *testing.T) {
	t.Parallel()

	const chainID = flow.Emulator
	systemContracts := systemcontracts.SystemContractsForChain(chainID)

	// Prevent conflict with system contracts
	address, err := chainID.Chain().AddressAtIndex(1_000_000)
	require.NoError(t, err)

	type testCase struct {
		oldContract, oldInterface, newContract, newInterface string
	}

	test := func(oldContract, oldInterface, newContract, newInterface string) {

		name := fmt.Sprintf(
			"%s.%s to %s.%s",
			oldContract,
			oldInterface,
			newContract,
			newInterface,
		)

		t.Run(name, func(t *testing.T) {

			t.Parallel()

			metadataViewsAddress := common.Address(systemContracts.MetadataViews.Address)
			viewResolverAddress := common.Address(systemContracts.ViewResolver.Address)

			oldCode := fmt.Sprintf(
				`
                  import %[2]s from %[1]s

                  pub contract C {
                      pub resource A: %[2]s.%[3]s {}
                  }
                `,
				metadataViewsAddress.HexWithPrefix(),
				oldContract,
				oldInterface,
			)

			newCode := fmt.Sprintf(
				`
                  import %[2]s from %[1]s

                  access(all) contract C {
                      access(all) resource A: %[2]s.%[3]s {}
                  }
                `,
				viewResolverAddress.HexWithPrefix(),
				newContract,
				newInterface,
			)

			newImportedContract := fmt.Sprintf(
				`
                  access(all) contract %s {
                      access(all) resource interface %s {}
                  }
                `,
				newContract,
				newInterface,
			)

			stagedContracts := []StagedContract{
				{
					Address: common.Address(address),
					Contract: Contract{
						Name: "A",
						Code: []byte(newCode),
					},
				},
			}

			logWriter := &logWriter{}
			log := zerolog.New(logWriter)

			rwf := &testReportWriterFactory{}

			options := StagedContractsMigrationOptions{
				ChainID:            chainID,
				VerboseErrorOutput: true,
			}

			migration := NewStagedContractsMigration("test", "test", log, rwf, options).
				WithContractUpdateValidation().
				WithStagedContractUpdates(stagedContracts)

			registersByAccount, err := registersForStagedContracts(
				StagedContract{
					Address: common.Address(address),
					Contract: Contract{
						Name: "A",
						Code: []byte(oldCode),
					},
				},
				StagedContract{
					Address: viewResolverAddress,
					Contract: Contract{
						Name: newContract,
						Code: []byte(newImportedContract),
					},
				},
			)
			require.NoError(t, err)

			err = migration.InitMigration(log, registersByAccount, 1)
			require.NoError(t, err)

			owner := string(address[:])
			accountRegisters := registersByAccount.AccountRegisters(owner)
			require.Equal(t, 1, accountRegisters.Count())

			err = migration.MigrateAccount(
				context.Background(),
				common.Address(address),
				accountRegisters,
			)
			require.NoError(t, err)

			err = migration.Close()
			require.NoError(t, err)

			require.Empty(t, logWriter.logs)

			require.Equal(t, 1, accountRegisters.Count())
			assert.Equal(t, newCode, contractCode(t, registersByAccount, owner, "A"))
		})
	}

	testCases := []testCase{
		{
			oldContract:  "MetadataViews",
			oldInterface: "Resolver",
			newContract:  "ViewResolver",
			newInterface: "Resolver",
		},
		{
			oldContract:  "MetadataViews",
			oldInterface: "ResolverCollection",
			newContract:  "ViewResolver",
			newInterface: "ResolverCollection",
		},
		{
			oldContract:  "NonFungibleToken",
			oldInterface: "INFT",
			newContract:  "NonFungibleToken",
			newInterface: "NFT",
		},
	}

	for _, testCase := range testCases {
		test(
			testCase.oldContract,
			testCase.oldInterface,
			testCase.newContract,
			testCase.newInterface,
		)
	}

	t.Run("MetadataViews.Resolver to ArbitraryContract.Resolver unsupported", func(t *testing.T) {

		// `MetadataViews.Resolver` shouldn't be able to replace with any arbitrary `Resolver` interface!

		t.Parallel()

		metadataViewsAddress := common.Address(systemContracts.MetadataViews.Address)
		viewResolverAddress := common.Address(systemContracts.ViewResolver.Address)

		oldCode := fmt.Sprintf(
			`
	          import MetadataViews from %s

	          pub contract C {
	              pub resource A: MetadataViews.Resolver {}
	          }
	       `,
			metadataViewsAddress.HexWithPrefix(),
		)

		newCode := fmt.Sprintf(
			`
	          import ArbitraryContract from %s

	          access(all) contract C {
	              access(all) resource A: ArbitraryContract.Resolver {}
	          }
	       `,
			viewResolverAddress.HexWithPrefix(),
		)

		arbitraryContract := `
	      access(all) contract ArbitraryContract {
	          access(all) resource interface Resolver {}
	      }
	    `

		stagedContracts := []StagedContract{
			{
				Address: common.Address(address),
				Contract: Contract{
					Name: "A",
					Code: []byte(newCode),
				},
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		rwf := &testReportWriterFactory{}

		options := StagedContractsMigrationOptions{
			ChainID:            chainID,
			VerboseErrorOutput: true,
		}

		migration := NewStagedContractsMigration("test", "test", log, rwf, options).
			WithContractUpdateValidation().
			WithStagedContractUpdates(stagedContracts)

		registersByAccount, err := registersForStagedContracts(
			StagedContract{
				Address: common.Address(address),
				Contract: Contract{
					Name: "A",
					Code: []byte(oldCode),
				},
			},
			StagedContract{
				Address: viewResolverAddress,
				Contract: Contract{
					Name: "ArbitraryContract",
					Code: []byte(arbitraryContract),
				},
			},
		)
		require.NoError(t, err)

		err = migration.InitMigration(log, registersByAccount, 1)
		require.NoError(t, err)

		owner := string(address[:])
		accountRegisters := registersByAccount.AccountRegisters(owner)
		require.Equal(t, 1, accountRegisters.Count())

		err = migration.MigrateAccount(
			context.Background(),
			common.Address(address),
			accountRegisters,
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Len(t, logWriter.logs, 1)
		require.Contains(t, logWriter.logs[0], "conformances do not match in `A`")

		require.Equal(t, 1, accountRegisters.Count())
		assert.Equal(t, oldCode, contractCode(t, registersByAccount, owner, "A"))
	})
}

func TestConcurrentContractUpdate(t *testing.T) {

	t.Parallel()

	const chainID = flow.Emulator
	addressGenerator := chainID.Chain().NewAddressGenerator()

	addressA, err := addressGenerator.NextAddress()
	require.NoError(t, err)

	addressB, err := addressGenerator.NextAddress()
	require.NoError(t, err)

	addressImport, err := addressGenerator.NextAddress()
	require.NoError(t, err)

	oldCodeA := fmt.Sprintf(
		`
          import Foo from %[1]s
          import Bar from %[1]s
          import Baz from %[1]s

          pub contract A {}
        `,
		addressImport.HexWithPrefix(),
	)

	newCodeA := fmt.Sprintf(
		`
          import Foo from %[1]s
          import Baz from %[1]s
          import Bar from %[1]s

          access(all) contract A {
              access(all) struct AA {}
          }
        `,
		addressImport.HexWithPrefix(),
	)

	oldCodeB := fmt.Sprintf(
		`
          import Foo from %[1]s
          import Bar from %[1]s
          import Baz from %[1]s

          pub contract B {}
        `,
		addressImport.HexWithPrefix(),
	)

	newCodeB := fmt.Sprintf(
		`
         import Foo from %[1]s
         import Baz from %[1]s
         import Bar from %[1]s

         access(all) contract B {
             access(all) struct BB {}
         }
        `,
		addressImport.HexWithPrefix(),
	)

	codeFoo := `access(all) contract Foo {}`
	codeBar := `access(all) contract Bar {}`
	codeBaz := `access(all) contract Baz {}`

	stagedContracts := []StagedContract{
		{
			Address: common.Address(addressA),
			Contract: Contract{
				Name: "A",
				Code: []byte(newCodeA),
			},
		},
		{
			Address: common.Address(addressB),
			Contract: Contract{
				Name: "B",
				Code: []byte(newCodeB),
			},
		},
	}

	registersByAccount, err := registersForStagedContracts(
		StagedContract{
			Address: common.Address(addressA),
			Contract: Contract{
				Name: "A",
				Code: []byte(oldCodeA),
			},
		},
		StagedContract{
			Address: common.Address(addressB),
			Contract: Contract{
				Name: "B",
				Code: []byte(oldCodeB),
			},
		},
		StagedContract{
			Address: common.Address(addressImport),
			Contract: Contract{
				Name: "Foo",
				Code: []byte(codeFoo),
			},
		},
		StagedContract{
			Address: common.Address(addressImport),
			Contract: Contract{
				Name: "Bar",
				Code: []byte(codeBar),
			},
		},
		StagedContract{
			Address: common.Address(addressImport),
			Contract: Contract{
				Name: "Baz",
				Code: []byte(codeBaz),
			},
		},
	)
	require.NoError(t, err)

	rwf := &testReportWriterFactory{}

	logWriter := &logWriter{}
	logger := zerolog.New(logWriter).Level(zerolog.ErrorLevel)

	// NOTE: Run with multiple workers (>2)
	const nWorker = 2

	const evmContractChange = EVMContractChangeNone
	const burnerContractChange = BurnerContractChangeNone

	migrations := NewCadence1Migrations(
		logger,
		t.TempDir(),
		rwf,
		Options{
			NWorker:              nWorker,
			ChainID:              chainID,
			EVMContractChange:    evmContractChange,
			BurnerContractChange: burnerContractChange,
			StagedContracts:      stagedContracts,
			VerboseErrorOutput:   true,
		},
	)

	for _, migration := range migrations {
		// only run the staged contracts migration
		if migration.Name != stagedContractUpdateMigrationName {
			continue
		}

		err = migration.Migrate(registersByAccount)
		require.NoError(
			t,
			err,
			"migration `%s` failed, logs: %v",
			migration.Name,
			logWriter.logs,
		)
	}

	// No errors.
	require.Empty(t, logWriter.logs)

	require.NoError(t, err)
	require.Equal(t, 5, registersByAccount.Count())
}

func TestStagedContractsUpdateValidationErrors(t *testing.T) {
	t.Parallel()

	const chainID = flow.Emulator
	systemContracts := systemcontracts.SystemContractsForChain(chainID)

	// Prevent conflict with system contracts
	address, err := chainID.Chain().AddressAtIndex(1_000_000)
	require.NoError(t, err)

	t.Run("field mismatch", func(t *testing.T) {
		t.Parallel()

		oldCodeA := `
          access(all) contract Test {
              access(all) var a: Int
              init() {
                  self.a = 0
              }
          }
       `

		newCodeA := `
          access(all) contract Test {
              access(all) var a: String
              init() {
                  self.a = "hello"
              }
          }
       `

		stagedContracts := []StagedContract{
			{
				Address: common.Address(address),
				Contract: Contract{
					Name: "A",
					Code: []byte(newCodeA),
				},
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		rwf := &testReportWriterFactory{}

		options := StagedContractsMigrationOptions{
			ChainID:            chainID,
			VerboseErrorOutput: true,
		}

		migration := NewStagedContractsMigration("test", "test", log, rwf, options).
			WithContractUpdateValidation().
			WithStagedContractUpdates(stagedContracts)

		registersByAccount, err := registersForStagedContracts(
			StagedContract{
				Address: common.Address(address),
				Contract: Contract{
					Name: "A",
					Code: []byte(oldCodeA),
				},
			},
		)
		require.NoError(t, err)

		err = migration.InitMigration(log, registersByAccount, 1)
		require.NoError(t, err)

		owner := string(address[:])
		accountRegisters := registersByAccount.AccountRegisters(owner)
		require.Equal(t, 1, accountRegisters.Count())

		err = migration.MigrateAccount(
			context.Background(),
			common.Address(address),
			accountRegisters,
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Len(t, logWriter.logs, 1)

		var jsonObject map[string]any

		err = json.Unmarshal([]byte(logWriter.logs[0]), &jsonObject)
		require.NoError(t, err)

		assert.Equal(
			t,
			"failed to update contract A in account 0x9bc576e17b370b16: error: mismatching field `a` in `Test`\n"+
				" --> 9bc576e17b370b16.A:3:33\n"+
				"  |\n"+
				"3 |               access(all) var a: String\n"+
				"  |                                  ^^^^^^ incompatible type annotations. expected `Int`, found `String`\n",
			jsonObject["message"],
		)
	})

	t.Run("field mismatch with entitlements", func(t *testing.T) {
		t.Parallel()

		nftAddress := common.Address(systemContracts.NonFungibleToken.Address)

		oldCodeA := fmt.Sprintf(
			`
              import NonFungibleToken from %s

              access(all) contract Test {
                  access(all) var a: Capability<&{NonFungibleToken.Provider}>?
                  init() {
                      self.a = nil
                  }
              }
            `,
			nftAddress.HexWithPrefix(),
		)

		newCodeA := fmt.Sprintf(
			`
              import NonFungibleToken from %s

              access(all) contract Test {
                  access(all) var a: Capability<auth(NonFungibleToken.E1) &{NonFungibleToken.Provider}>?
                  init() {
                      self.a = nil
                  }
              }
            `,
			nftAddress.HexWithPrefix(),
		)

		nftContract := `
          access(all) contract NonFungibleToken {

              access(all) entitlement E1
              access(all) entitlement E2

              access(all) resource interface Provider {
                  access(E1) fun foo()
                  access(E2) fun bar()
              }
          }
        `

		stagedContracts := []StagedContract{
			{
				Address: common.Address(address),
				Contract: Contract{
					Name: "A",
					Code: []byte(newCodeA),
				},
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		rwf := &testReportWriterFactory{}

		options := StagedContractsMigrationOptions{
			ChainID:            chainID,
			VerboseErrorOutput: true,
		}

		migration := NewStagedContractsMigration("test", "test", log, rwf, options).
			WithContractUpdateValidation().
			WithStagedContractUpdates(stagedContracts)

		registersByAccount, err := registersForStagedContracts(
			StagedContract{
				Address: common.Address(address),
				Contract: Contract{
					Name: "A",
					Code: []byte(oldCodeA),
				},
			},
			StagedContract{
				Address: nftAddress,
				Contract: Contract{
					Name: "NonFungibleToken",
					Code: []byte(nftContract),
				},
			},
		)
		require.NoError(t, err)

		err = migration.InitMigration(log, registersByAccount, 1)
		require.NoError(t, err)

		owner := string(address[:])
		accountRegisters := registersByAccount.AccountRegisters(owner)
		require.Equal(t, 1, accountRegisters.Count())

		err = migration.MigrateAccount(
			context.Background(),
			common.Address(address),
			accountRegisters,
		)
		require.NoError(t, err)

		require.Len(t, logWriter.logs, 1)

		var jsonObject map[string]any
		err = json.Unmarshal([]byte(logWriter.logs[0]), &jsonObject)
		require.NoError(t, err)

		assert.Equal(
			t,
			"failed to update contract A in account 0x9bc576e17b370b16: error: mismatching field `a` in `Test`\n"+
				" --> 9bc576e17b370b16.A:5:37\n"+
				"  |\n"+
				"5 |                   access(all) var a: Capability<auth(NonFungibleToken.E1) &{NonFungibleToken.Provider}>?\n"+
				"  |                                      ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^ mismatching authorization:"+
				" the entitlements migration would only grant this value `NonFungibleToken.E1, NonFungibleToken.E2`, but the annotation present is `NonFungibleToken.E1`\n",
			jsonObject["message"],
		)
	})
}

func TestContractUpdateEntry_MarshalJSON(t *testing.T) {

	t.Parallel()

	e := contractUpdateEntry{
		AccountAddress: common.MustBytesToAddress([]byte{0x1}),
		ContractName:   "Test",
	}

	actual, err := e.MarshalJSON()
	require.NoError(t, err)

	require.JSONEq(t,
		//language=JSON
		`{
          "kind": "contract-update-success",
          "account_address": "0x0000000000000001",
          "contract_name": "Test"
        }`,
		string(actual),
	)
}

func TestContractUpdateFailureEntry_MarshalJSON(t *testing.T) {

	t.Parallel()

	e := contractUpdateFailureEntry{
		AccountAddress: common.MustBytesToAddress([]byte{0x1}),
		ContractName:   "Test",
		Error:          "unknown",
	}

	actual, err := e.MarshalJSON()
	require.NoError(t, err)

	require.JSONEq(t,
		//language=JSON
		`{
          "kind": "contract-update-failure",
          "account_address": "0x0000000000000001",
          "contract_name": "Test",
          "error": "unknown"
        }`,
		string(actual),
	)
}
