package migrations

import (
	"context"
	"encoding/json"
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
		require.Contains(t, logWriter.logs[0], `error: expected token '{'`)

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
		require.Contains(t, logWriter.logs[0], `error: expected token '}'`)

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
		require.Contains(t, logWriter.logs[0], `error: expected token '{'`)

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
		require.NoError(t, err)

		require.Len(t, logWriter.logs, 1)
		assert.Contains(t,
			logWriter.logs[0],
			`"failed to find all contract registers that need to be changed for address"`,
		)
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

	addressA, err := addressGenerator.NextAddress()
	require.NoError(t, err)

	addressB, err := addressGenerator.NextAddress()
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
				Address: common.Address(addressA),
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

		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		payloads1 := []*ledger.Payload{
			newContractPayload(common.Address(addressA), "A", []byte(oldCodeA)),
		}
		payloads2 := []*ledger.Payload{
			newContractPayload(ftAddress, "FungibleToken", []byte(ftContract)),
		}

		payloads1, err = migration.MigrateAccount(
			context.Background(),
			common.Address(addressA),
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
				Address: common.Address(addressA),
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
			newContractPayload(common.Address(addressA), "A", []byte(oldCodeA)),
		}

		payloads2 := []*ledger.Payload{
			newContractPayload(otherAddress, "FungibleToken", []byte(ftContract)),
		}

		payloads1, err = migration.MigrateAccount(
			context.Background(),
			common.Address(addressA),
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

		accountPayloads := []*ledger.Payload{contractACode}
		allPayloads := []*ledger.Payload{contractACode, contractBCode}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		migration := NewStagedContractsMigration(chainID, log).
			WithContractUpdateValidation()

		migration.RegisterContractUpdates(stagedContracts)

		err := migration.InitMigration(log, allPayloads, 0)
		require.NoError(t, err)

		accountPayloads, err = migration.MigrateAccount(
			context.Background(),
			common.Address(addressA),
			accountPayloads,
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Empty(t, logWriter.logs)

		require.Len(t, accountPayloads, 1)
		assert.Equal(t, newCodeA, string(accountPayloads[0].Value()))
	})
}

func TestStagedContractConformanceChanges(t *testing.T) {
	t.Parallel()

	chainID := flow.Emulator
	systemContracts := systemcontracts.SystemContractsForChain(chainID)

	addressGenerator := chainID.Chain().NewAddressGenerator()

	address, err := addressGenerator.NextAddress()
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

			oldCode := fmt.Sprintf(`
                import %[2]s from %[1]s

                pub contract C {
                    pub resource A: %[2]s.%[3]s {}
                }`,
				metadataViewsAddress.HexWithPrefix(),
				oldContract,
				oldInterface,
			)

			newCode := fmt.Sprintf(`
                import %[2]s from %[1]s

                access(all) contract C {
                    access(all) resource A: %[2]s.%[3]s {}
                }`,
				viewResolverAddress.HexWithPrefix(),
				newContract,
				newInterface,
			)

			newImportedContract := fmt.Sprintf(`
                access(all) contract %s {
                    access(all) resource interface %s {}
		        }`,
				newContract,
				newInterface,
			)

			stagedContracts := []StagedContract{
				{
					Contract: Contract{
						Name: "A",
						Code: []byte(newCode),
					},
					Address: common.Address(address),
				},
			}

			logWriter := &logWriter{}
			log := zerolog.New(logWriter)

			migration := NewStagedContractsMigration(chainID, log).
				WithContractUpdateValidation()

			migration.RegisterContractUpdates(stagedContracts)

			contractCodePayload := newContractPayload(common.Address(address), "A", []byte(oldCode))
			viewResolverCodePayload := newContractPayload(
				viewResolverAddress,
				newContract,
				[]byte(newImportedContract),
			)

			accountPayloads := []*ledger.Payload{contractCodePayload}
			allPayloads := []*ledger.Payload{contractCodePayload, viewResolverCodePayload}

			err := migration.InitMigration(log, allPayloads, 0)
			require.NoError(t, err)

			accountPayloads, err = migration.MigrateAccount(
				context.Background(),
				common.Address(address),
				accountPayloads,
			)
			require.NoError(t, err)

			err = migration.Close()
			require.NoError(t, err)

			require.Empty(t, logWriter.logs)

			require.Len(t, accountPayloads, 1)
			assert.Equal(t, newCode, string(accountPayloads[0].Value()))
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

		oldCode := fmt.Sprintf(`
            import MetadataViews from %s

            pub contract C {
                pub resource A: MetadataViews.Resolver {}
            }
        `,
			metadataViewsAddress.HexWithPrefix(),
		)

		newCode := fmt.Sprintf(`
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
				Contract: Contract{
					Name: "A",
					Code: []byte(newCode),
				},
				Address: common.Address(address),
			},
		}

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)

		migration := NewStagedContractsMigration(chainID, log).
			WithContractUpdateValidation()

		migration.RegisterContractUpdates(stagedContracts)

		contractCodePayload := newContractPayload(common.Address(address), "A", []byte(oldCode))
		arbitraryContractCodePayload := newContractPayload(
			viewResolverAddress,
			"ArbitraryContract",
			[]byte(arbitraryContract),
		)

		accountPayloads := []*ledger.Payload{contractCodePayload}
		allPayloads := []*ledger.Payload{contractCodePayload, arbitraryContractCodePayload}

		err := migration.InitMigration(log, allPayloads, 0)
		require.NoError(t, err)

		accountPayloads, err = migration.MigrateAccount(
			context.Background(),
			common.Address(address),
			accountPayloads,
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Len(t, logWriter.logs, 1)
		require.Contains(t, logWriter.logs[0], "conformances does not match in `A`")

		require.Len(t, accountPayloads, 1)
		assert.Equal(t, oldCode, string(accountPayloads[0].Value()))
	})
}

func TestConcurrentContractUpdate(t *testing.T) {

	t.Parallel()

	chainID := flow.Emulator
	addressGenerator := chainID.Chain().NewAddressGenerator()

	addressA, err := addressGenerator.NextAddress()
	require.NoError(t, err)

	addressB, err := addressGenerator.NextAddress()
	require.NoError(t, err)

	addressImport, err := addressGenerator.NextAddress()
	require.NoError(t, err)

	oldCodeA := fmt.Sprintf(`
        import Foo from %[1]s
        import Bar from %[1]s
        import Baz from %[1]s

        pub contract A {}
        `,
		addressImport.HexWithPrefix(),
	)

	newCodeA := fmt.Sprintf(`
        import Foo from %[1]s
        import Baz from %[1]s
        import Bar from %[1]s

        access(all) contract A {
            access(all) struct AA {}
        }
    `,
		addressImport.HexWithPrefix(),
	)

	oldCodeB := fmt.Sprintf(`
        import Foo from %[1]s
        import Bar from %[1]s
        import Baz from %[1]s

        pub contract B {}
        `,
		addressImport.HexWithPrefix(),
	)

	newCodeB := fmt.Sprintf(`
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
			Contract: Contract{
				Name: "A",
				Code: []byte(newCodeA),
			},
			Address: common.Address(addressA),
		},
		{
			Contract: Contract{
				Name: "B",
				Code: []byte(newCodeB),
			},
			Address: common.Address(addressB),
		},
	}

	allPayloads := []*ledger.Payload{
		newContractPayload(common.Address(addressA), "A", []byte(oldCodeA)),
		newContractPayload(common.Address(addressB), "B", []byte(oldCodeB)),
		newContractPayload(common.Address(addressImport), "Foo", []byte(codeFoo)),
		newContractPayload(common.Address(addressImport), "Bar", []byte(codeBar)),
		newContractPayload(common.Address(addressImport), "Baz", []byte(codeBaz)),
	}

	rwf := &testReportWriterFactory{}

	logWriter := &logWriter{}
	logger := zerolog.New(logWriter).Level(zerolog.ErrorLevel)

	// NOTE: Run with multiple workers (>2)
	const nWorker = 2

	const evmContractChange = EVMContractChangeNone
	const burnerContractChange = BurnerContractChangeNone

	migrations := NewCadence1Migrations(
		logger,
		rwf,
		Options{
			NWorker:              nWorker,
			ChainID:              chainID,
			EVMContractChange:    evmContractChange,
			BurnerContractChange: burnerContractChange,
			StagedContracts:      stagedContracts,
		},
	)

	for _, migration := range migrations {
		// only run the staged contracts migration
		if migration.Name != "staged-contracts-update-migration" {
			continue
		}

		allPayloads, err = migration.Migrate(allPayloads)
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
	require.Len(t, allPayloads, 5)
}

func TestStagedContractsUpdateValidationErrors(t *testing.T) {
	t.Parallel()

	chainID := flow.Emulator
	systemContracts := systemcontracts.SystemContractsForChain(chainID)

	addressGenerator := chainID.Chain().NewAddressGenerator()

	address, err := addressGenerator.NextAddress()
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

		payloads := []*ledger.Payload{
			newContractPayload(common.Address(address), "A", []byte(oldCodeA)),
		}

		err = migration.InitMigration(log, payloads, 0)
		require.NoError(t, err)

		_, err = migration.MigrateAccount(
			context.Background(),
			common.Address(address),
			payloads,
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Len(t, logWriter.logs, 1)

		var jsonObject map[string]any

		err := json.Unmarshal([]byte(logWriter.logs[0]), &jsonObject)
		require.NoError(t, err)

		assert.Equal(
			t,
			"failed to update contract A in account 0xf8d6e0586b0a20c7: error: mismatching field `a` in `Test`\n"+
				" --> f8d6e0586b0a20c7.A:3:35\n"+
				"  |\n"+
				"3 |                 access(all) var a: String\n"+
				"  |                                    ^^^^^^ incompatible type annotations. expected `Int`, found `String`\n",
			jsonObject["message"],
		)
	})

	t.Run("field mismatch with entitlements", func(t *testing.T) {
		t.Parallel()

		nftAddress := common.Address(systemContracts.NonFungibleToken.Address)

		oldCodeA := fmt.Sprintf(`
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

		newCodeA := fmt.Sprintf(`
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

		contractACode := newContractPayload(common.Address(address), "A", []byte(oldCodeA))
		nftCode := newContractPayload(nftAddress, "NonFungibleToken", []byte(nftContract))

		accountPayloads := []*ledger.Payload{contractACode}
		allPayloads := []*ledger.Payload{contractACode, nftCode}

		err = migration.InitMigration(log, allPayloads, 0)
		require.NoError(t, err)

		_, err = migration.MigrateAccount(
			context.Background(),
			common.Address(address),
			accountPayloads,
		)
		require.NoError(t, err)

		require.Len(t, logWriter.logs, 1)

		var jsonObject map[string]any
		err := json.Unmarshal([]byte(logWriter.logs[0]), &jsonObject)
		require.NoError(t, err)

		assert.Equal(
			t,
			"failed to update contract A in account 0xf8d6e0586b0a20c7: error: mismatching field `a` in `Test`\n"+
				" --> f8d6e0586b0a20c7.A:5:35\n"+
				"  |\n"+
				"5 |                 access(all) var a: Capability<auth(NonFungibleToken.E1) &{NonFungibleToken.Provider}>?\n"+
				"  |                                    ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^ mismatching authorization:"+
				" the entitlements migration would only grant this value `NonFungibleToken.E1, NonFungibleToken.E2`, but the annotation present is `NonFungibleToken.E1`\n",
			jsonObject["message"],
		)
	})
}
