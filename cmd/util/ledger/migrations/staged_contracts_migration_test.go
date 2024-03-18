package migrations

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"

	"github.com/onflow/cadence/runtime/common"
)

func newContractPayload(address common.Address, contractName string, contract []byte) *ledger.Payload {
	return ledger.NewPayload(
		convert.RegisterIDToLedgerKey(
			flow.ContractRegisterID(flow.ConvertAddress(address), contractName),
		),
		contract,
	)
}

type logWriter struct {
	logs []string
}

var _ io.Writer = &logWriter{}

func (l *logWriter) Write(bytes []byte) (int, error) {
	l.logs = append(l.logs, string(bytes))
	return len(bytes), nil
}

func TestStagedContractsMigration(t *testing.T) {
	t.Parallel()

	chainID := flow.Emulator
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
				Contract: Contract{
					Name: "A",
					Code: []byte(newCode),
				},
				Address: common.Address(address1),
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		migration := NewStagedContractsMigration(chainID, log)
		migration.RegisterContractUpdates(stagedContracts)

		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		payloads, err := migration.MigrateAccount(
			context.Background(),
			common.Address(address1),
			[]*ledger.Payload{
				newContractPayload(common.Address(address1), "A", []byte(oldCode)),
			},
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Empty(t, logWriter.logs)

		require.Len(t, payloads, 1)
		require.Equal(t, newCode, string(payloads[0].Value()))
	})

	t.Run("syntax error in new code", func(t *testing.T) {
		t.Parallel()

		oldCode := "access(all) contract A {}"
		newCode := "access(all) contract A { access(all) struct B () }"

		stagedContracts := []StagedContract{
			{
				Contract: Contract{
					Name: "A",
					Code: []byte(newCode),
				},
				Address: common.Address(address1),
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		migration := NewStagedContractsMigration(chainID, log).
			WithContractUpdateValidation()
		migration.RegisterContractUpdates(stagedContracts)

		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		payloads, err := migration.MigrateAccount(
			context.Background(),
			common.Address(address1),
			[]*ledger.Payload{
				newContractPayload(common.Address(address1), "A", []byte(oldCode)),
			},
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Len(t, logWriter.logs, 1)
		require.Contains(t, logWriter.logs[0], `"migration":"StagedContractsMigration","error":"Parsing failed`)

		// Payloads should still have the old code
		require.Len(t, payloads, 1)
		require.Equal(t, oldCode, string(payloads[0].Value()))
	})

	t.Run("syntax error in old code", func(t *testing.T) {
		t.Parallel()

		oldCode := "access(all) contract A {"
		newCode := "access(all) contract A { access(all) struct B {} }"

		stagedContracts := []StagedContract{
			{
				Contract: Contract{
					Name: "A",
					Code: []byte(newCode),
				},
				Address: common.Address(address1),
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		migration := NewStagedContractsMigration(chainID, log).
			WithContractUpdateValidation()
		migration.RegisterContractUpdates(stagedContracts)

		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		payloads, err := migration.MigrateAccount(
			context.Background(),
			common.Address(address1),
			[]*ledger.Payload{
				newContractPayload(common.Address(address1), "A", []byte(oldCode)),
			},
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Len(t, logWriter.logs, 1)
		require.Contains(t, logWriter.logs[0], `"migration":"StagedContractsMigration","error":"Parsing failed`)

		// Payloads should still have the old code
		require.Len(t, payloads, 1)
		require.Equal(t, oldCode, string(payloads[0].Value()))
	})

	t.Run("one fail, one success", func(t *testing.T) {
		t.Parallel()

		oldCode1 := "access(all) contract A {}"
		oldCode2 := "access(all) contract B {}"

		newCode1 := "access(all) contract A { access(all) struct C () }" // broken
		newCode2 := "access(all) contract B { access(all) struct C {} }" // all good

		stagedContracts := []StagedContract{
			{
				Contract: Contract{
					Name: "A",
					Code: []byte(newCode1),
				},
				Address: common.Address(address1),
			},
			{
				Contract: Contract{
					Name: "B",
					Code: []byte(newCode2),
				},
				Address: common.Address(address1),
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		migration := NewStagedContractsMigration(chainID, log).
			WithContractUpdateValidation()
		migration.RegisterContractUpdates(stagedContracts)

		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		payloads, err := migration.MigrateAccount(
			context.Background(),
			common.Address(address1),
			[]*ledger.Payload{
				newContractPayload(common.Address(address1), "A", []byte(oldCode1)),
				newContractPayload(common.Address(address1), "B", []byte(oldCode2)),
			},
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Len(t, logWriter.logs, 1)
		require.Contains(t, logWriter.logs[0], `"migration":"StagedContractsMigration","error":"Parsing failed`)

		require.Len(t, payloads, 2)
		// First payload should still have the old code
		require.Equal(t, oldCode1, string(payloads[0].Value()))
		// Second payload should have the updated code
		require.Equal(t, newCode2, string(payloads[1].Value()))
	})

	t.Run("different accounts", func(t *testing.T) {
		t.Parallel()

		oldCode := "access(all) contract A {}"
		newCode := "access(all) contract A { access(all) struct B {} }"

		stagedContracts := []StagedContract{
			{
				Contract: Contract{
					Name: "A",
					Code: []byte(newCode),
				},
				Address: common.Address(address2),
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		migration := NewStagedContractsMigration(chainID, log)
		migration.RegisterContractUpdates(stagedContracts)

		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		payloads1 := []*ledger.Payload{
			newContractPayload(common.Address(address1), "A", []byte(oldCode)),
		}
		payloads2 := []*ledger.Payload{
			newContractPayload(common.Address(address2), "A", []byte(oldCode)),
		}

		// Run migration for account 1,
		// There are no staged updates for contracts in account 1.
		// So codes should not have been updated.
		payloads1, err = migration.MigrateAccount(
			context.Background(),
			common.Address(address1),
			payloads1,
		)
		require.NoError(t, err)
		require.Len(t, payloads1, 1)
		require.Equal(t, oldCode, string(payloads1[0].Value()))

		// Run migration for account 2
		// There is one staged update for contracts in account 2.
		// So one payload/contract-code should be updated, and the other should remain the same.
		payloads2, err = migration.MigrateAccount(
			context.Background(),
			common.Address(address2),
			payloads2,
		)
		require.NoError(t, err)
		require.Len(t, payloads2, 1)
		require.Equal(t, newCode, string(payloads2[0].Value()))

		err = migration.Close()
		require.NoError(t, err)

		// No errors.
		require.Empty(t, logWriter.logs)
	})

	t.Run("multiple updates for same contract", func(t *testing.T) {
		t.Parallel()

		oldCode := "access(all) contract A {}"
		update1 := "access(all) contract A { access(all) struct B {} }"
		update2 := "access(all) contract A { access(all) struct B {} access(all) struct C {} }"

		stagedContracts := []StagedContract{
			{
				Contract: Contract{
					Name: "A",
					Code: []byte(update1),
				},
				Address: common.Address(address1),
			},
			{
				Contract: Contract{
					Name: "A",
					Code: []byte(update2),
				},
				Address: common.Address(address1),
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		migration := NewStagedContractsMigration(chainID, log)

		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		migration.RegisterContractUpdates(stagedContracts)

		payloads, err := migration.MigrateAccount(
			context.Background(),
			common.Address(address1),
			[]*ledger.Payload{
				newContractPayload(common.Address(address1), "A", []byte(oldCode)),
			},
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

		require.Len(t, payloads, 1)
		require.Equal(t, update2, string(payloads[0].Value()))
	})

	t.Run("missing old contract", func(t *testing.T) {
		t.Parallel()

		newCode := "access(all) contract A { access(all) struct B {} }"

		stagedContracts := []StagedContract{
			{
				Contract: Contract{
					Name: "A",
					Code: []byte(newCode),
				},
				Address: common.Address(address1),
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		migration := NewStagedContractsMigration(chainID, log)

		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		migration.RegisterContractUpdates(stagedContracts)

		// NOTE: no payloads
		_, err = migration.MigrateAccount(
			context.Background(),
			common.Address(address1),
			nil,
		)
		require.ErrorContains(t, err, "failed to find all contract registers that need to be changed")
	})
}

func TestStagedContractsWithImports(t *testing.T) {
	t.Parallel()

	chainID := flow.Emulator

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
				Contract: Contract{
					Name: "A",
					Code: []byte(newCodeA),
				},
				Address: common.Address(address1),
			},
			{
				Contract: Contract{
					Name: "B",
					Code: []byte(newCodeB),
				},
				Address: common.Address(address2),
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		migration := NewStagedContractsMigration(chainID, log)
		migration.RegisterContractUpdates(stagedContracts)

		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		payloads1 := []*ledger.Payload{
			newContractPayload(common.Address(address1), "A", []byte(oldCodeA)),
		}
		payloads2 := []*ledger.Payload{
			newContractPayload(common.Address(address2), "B", []byte(oldCodeB)),
		}

		payloads1, err = migration.MigrateAccount(
			context.Background(),
			common.Address(address1),
			payloads1,
		)
		require.NoError(t, err)

		payloads2, err = migration.MigrateAccount(
			context.Background(),
			common.Address(address2),
			payloads2,
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Empty(t, logWriter.logs)

		require.Len(t, payloads1, 1)
		assert.Equal(t, newCodeA, string(payloads1[0].Value()))

		require.Len(t, payloads2, 1)
		assert.Equal(t, newCodeB, string(payloads2[0].Value()))
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
				Contract: Contract{
					Name: "A",
					Code: []byte(newCodeA),
				},
				Address: common.Address(address1),
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		migration := NewStagedContractsMigration(chainID, log).
			WithContractUpdateValidation()
		migration.RegisterContractUpdates(stagedContracts)

		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		payloads1 := []*ledger.Payload{
			newContractPayload(common.Address(address1), "A", []byte(oldCodeA)),
		}
		payloads2 := []*ledger.Payload{
			newContractPayload(common.Address(address2), "B", []byte(oldCodeB)),
		}

		payloads1, err = migration.MigrateAccount(
			context.Background(),
			common.Address(address1),
			payloads1,
		)
		require.NoError(t, err)

		payloads2, err = migration.MigrateAccount(
			context.Background(),
			common.Address(address2),
			payloads2,
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Len(t, logWriter.logs, 1)
		require.Contains(
			t,
			logWriter.logs[0],
			"cannot find declaration `B` in `ee82856bf20e2aa6.B`",
		)

		// Payloads should be the old ones
		require.Len(t, payloads1, 1)
		assert.Equal(t, oldCodeA, string(payloads1[0].Value()))

		require.Len(t, payloads2, 1)
		assert.Equal(t, oldCodeB, string(payloads2[0].Value()))
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
				Contract: Contract{
					Name: "A",
					Code: []byte(newCodeA),
				},
				Address: common.Address(address1),
			},
			{
				Contract: Contract{
					Name: "B",
					Code: []byte(newCodeB),
				},
				Address: common.Address(address2),
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		migration := NewStagedContractsMigration(chainID, log).
			WithContractUpdateValidation()
		migration.RegisterContractUpdates(stagedContracts)

		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		payloads1 := []*ledger.Payload{
			newContractPayload(common.Address(address1), "A", []byte(oldCodeA)),
		}
		payloads2 := []*ledger.Payload{
			newContractPayload(common.Address(address2), "B", []byte(oldCodeB)),
		}

		payloads1, err = migration.MigrateAccount(
			context.Background(),
			common.Address(address1),
			payloads1,
		)
		require.NoError(t, err)

		payloads2, err = migration.MigrateAccount(
			context.Background(),
			common.Address(address2),
			payloads2,
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

		// Payloads should be the old ones
		require.Len(t, payloads1, 1)
		assert.Equal(t, oldCodeA, string(payloads1[0].Value()))

		require.Len(t, payloads2, 1)
		assert.Equal(t, oldCodeB, string(payloads2[0].Value()))
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
				Contract: Contract{
					Name: "A",
					Code: []byte(newCodeA),
				},
				Address: common.Address(address1),
			},
			{
				Contract: Contract{
					Name: "C",
					Code: []byte(newCodeC),
				},
				Address: common.Address(address1),
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		migration := NewStagedContractsMigration(chainID, log).
			WithContractUpdateValidation()
		migration.RegisterContractUpdates(stagedContracts)

		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		payloads1 := []*ledger.Payload{
			newContractPayload(common.Address(address1), "A", []byte(oldCodeA)),
			newContractPayload(common.Address(address1), "C", []byte(oldCodeC)),
		}

		payloads2 := []*ledger.Payload{
			newContractPayload(common.Address(address2), "B", []byte(oldCodeB)),
		}

		payloads1, err = migration.MigrateAccount(
			context.Background(),
			common.Address(address1),
			payloads1,
		)
		require.NoError(t, err)

		payloads2, err = migration.MigrateAccount(
			context.Background(),
			common.Address(address2),
			payloads2,
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Len(t, logWriter.logs, 1)
		require.Contains(
			t,
			logWriter.logs[0],
			"cannot find declaration `B` in `ee82856bf20e2aa6.B`",
		)

		// A and B should be the old ones.
		// C should be updated.
		// Type checking failures in unrelated contracts should not
		// stop other contracts from being migrated.
		require.Len(t, payloads1, 2)
		require.Equal(t, oldCodeA, string(payloads1[0].Value()))
		require.Equal(t, newCodeC, string(payloads1[1].Value()))

		require.Len(t, payloads2, 1)
		require.Equal(t, oldCodeB, string(payloads2[0].Value()))
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

	chainID := flow.Emulator
	systemContracts := systemcontracts.SystemContractsForChain(chainID)

	addressGenerator := chainID.Chain().NewAddressGenerator()

	address, err := addressGenerator.NextAddress()
	require.NoError(t, err)

	t.Run("FungibleToken.Vault", func(t *testing.T) {
		t.Parallel()

		ftAddress := common.Address(systemContracts.FungibleToken.Address)

		oldCodeA := fmt.Sprintf(`
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

		newCodeA := fmt.Sprintf(`
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
				Contract: Contract{
					Name: "A",
					Code: []byte(newCodeA),
				},
				Address: common.Address(address),
			},
			{
				Contract: Contract{
					Name: "FungibleToken",
					Code: []byte(ftContract),
				},
				Address: ftAddress,
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		migration := NewStagedContractsMigration(chainID, log).
			WithContractUpdateValidation()

		migration.RegisterContractUpdates(stagedContracts)

		err = migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		payloads1 := []*ledger.Payload{
			newContractPayload(common.Address(address), "A", []byte(oldCodeA)),
		}
		payloads2 := []*ledger.Payload{
			newContractPayload(ftAddress, "FungibleToken", []byte(ftContract)),
		}

		payloads1, err = migration.MigrateAccount(
			context.Background(),
			common.Address(address),
			payloads1,
		)
		require.NoError(t, err)

		payloads2, err = migration.MigrateAccount(
			context.Background(),
			ftAddress,
			payloads2,
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Empty(t, logWriter.logs)

		require.Len(t, payloads1, 1)
		assert.Equal(t, newCodeA, string(payloads1[0].Value()))

		require.Len(t, payloads2, 1)
		assert.Equal(t, ftContract, string(payloads2[0].Value()))
	})

	t.Run("other type", func(t *testing.T) {
		t.Parallel()

		otherAddress, err := common.HexToAddress("0x2")
		require.NoError(t, err)

		oldCodeA := fmt.Sprintf(`
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

		newCodeA := fmt.Sprintf(`
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
				Contract: Contract{
					Name: "A",
					Code: []byte(newCodeA),
				},
				Address: common.Address(address),
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		migration := NewStagedContractsMigration(chainID, log).
			WithContractUpdateValidation()

		migration.RegisterContractUpdates(stagedContracts)

		err = migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		payloads1 := []*ledger.Payload{
			newContractPayload(common.Address(address), "A", []byte(oldCodeA)),
		}

		payloads2 := []*ledger.Payload{
			newContractPayload(otherAddress, "FungibleToken", []byte(ftContract)),
		}

		payloads1, err = migration.MigrateAccount(
			context.Background(),
			common.Address(address),
			payloads1,
		)
		require.NoError(t, err)

		payloads2, err = migration.MigrateAccount(
			context.Background(),
			otherAddress,
			payloads2,
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Len(t, logWriter.logs, 1)
		assert.Contains(t,
			logWriter.logs[0],
			"cannot find declaration `FungibleToken` in `0000000000000002.FungibleToken`",
		)

		require.Len(t, payloads1, 1)
		assert.Equal(t, oldCodeA, string(payloads1[0].Value()))

		require.Len(t, payloads2, 1)
		assert.Equal(t, ftContract, string(payloads2[0].Value()))
	})

	t.Run("import from other account", func(t *testing.T) {
		t.Parallel()

		addressA := address

		addressB, err := addressGenerator.NextAddress()
		require.NoError(t, err)

		oldCodeA := fmt.Sprintf(`
            import B from %s

            pub contract A {}
        `,
			addressB.HexWithPrefix(),
		)

		newCodeA := fmt.Sprintf(`
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
				Contract: Contract{
					Name: "A",
					Code: []byte(newCodeA),
				},
				Address: common.Address(addressA),
			},
		}

		contractACode := newContractPayload(common.Address(addressA), "A", []byte(oldCodeA))
		contractBCode := newContractPayload(common.Address(addressB), "B", []byte(codeB))

		payloads1 := []*ledger.Payload{contractACode}
		allPayloads := []*ledger.Payload{contractACode, contractBCode}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		migration := NewStagedContractsMigration(chainID, log).
			WithContractUpdateValidation()

		migration.RegisterContractUpdates(stagedContracts)

		err = migration.InitMigration(log, allPayloads, 0)
		require.NoError(t, err)

		payloads1, err = migration.MigrateAccount(
			context.Background(),
			common.Address(addressA),
			payloads1,
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Empty(t, logWriter.logs)

		require.Len(t, payloads1, 1)
		assert.Equal(t, newCodeA, string(payloads1[0].Value()))
	})

}
