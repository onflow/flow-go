package migrations

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rs/zerolog"

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

	address1, err := common.HexToAddress("0x1")
	require.NoError(t, err)

	address2, err := common.HexToAddress("0x2")
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("one contract", func(t *testing.T) {
		t.Parallel()

		oldCode := "access(all) contract A {}"
		newCode := "access(all) contract A { access(all) struct B {} }"

		stagedContractsGetter := func() []StagedContract {
			return []StagedContract{
				{
					Contract: Contract{
						name: "A",
						code: []byte(newCode),
					},
					address: address1,
				},
			}
		}

		migration := NewStagedContractsMigration(stagedContractsGetter)

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)
		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		payloads, err := migration.MigrateAccount(ctx, address1,
			[]*ledger.Payload{
				newContractPayload(address1, "A", []byte(oldCode)),
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

		stagedContractsGetter := func() []StagedContract {
			return []StagedContract{
				{
					Contract: Contract{
						name: "A",
						code: []byte(newCode),
					},
					address: address1,
				},
			}
		}

		migration := NewStagedContractsMigration(stagedContractsGetter)

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)
		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		payloads, err := migration.MigrateAccount(ctx, address1,
			[]*ledger.Payload{
				newContractPayload(address1, "A", []byte(oldCode)),
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

		stagedContractsGetter := func() []StagedContract {
			return []StagedContract{
				{
					Contract: Contract{
						name: "A",
						code: []byte(newCode),
					},
					address: address1,
				},
			}
		}

		migration := NewStagedContractsMigration(stagedContractsGetter)

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)
		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		payloads, err := migration.MigrateAccount(ctx, address1,
			[]*ledger.Payload{
				newContractPayload(address1, "A", []byte(oldCode)),
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

		stagedContractsGetter := func() []StagedContract {
			return []StagedContract{
				{
					Contract: Contract{
						name: "A",
						code: []byte(newCode1),
					},
					address: address1,
				},
				{
					Contract: Contract{
						name: "B",
						code: []byte(newCode2),
					},
					address: address1,
				},
			}
		}

		migration := NewStagedContractsMigration(stagedContractsGetter)

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)
		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		payloads, err := migration.MigrateAccount(ctx, address1,
			[]*ledger.Payload{
				newContractPayload(address1, "A", []byte(oldCode1)),
				newContractPayload(address1, "B", []byte(oldCode2)),
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

		stagedContractsGetter := func() []StagedContract {
			return []StagedContract{
				{
					Contract: Contract{
						name: "A",
						code: []byte(newCode),
					},
					address: address2,
				},
			}
		}

		migration := NewStagedContractsMigration(stagedContractsGetter)

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)
		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		payloads := []*ledger.Payload{
			newContractPayload(address1, "A", []byte(oldCode)),
			newContractPayload(address2, "A", []byte(oldCode)),
		}

		// Run migration for account 1,
		// There are no staged updates for contracts in account 1.
		// So codes should not have been updated.
		payloads, err = migration.MigrateAccount(ctx, address1, payloads)
		require.NoError(t, err)
		require.Len(t, payloads, 2)
		require.Equal(t, oldCode, string(payloads[0].Value()))
		require.Equal(t, oldCode, string(payloads[1].Value()))

		// Run migration for account 2
		// There is one staged update for contracts in account 2.
		// So one payload/contract-code should be updated, and the other should remain the same.
		payloads, err = migration.MigrateAccount(ctx, address2, payloads)
		require.NoError(t, err)
		require.Len(t, payloads, 2)
		require.Equal(t, oldCode, string(payloads[0].Value()))
		require.Equal(t, newCode, string(payloads[1].Value()))

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

		stagedContractsGetter := func() []StagedContract {
			return []StagedContract{
				{
					Contract: Contract{
						name: "A",
						code: []byte(update1),
					},
					address: address1,
				},
				{
					Contract: Contract{
						name: "A",
						code: []byte(update2),
					},
					address: address1,
				},
			}
		}

		migration := NewStagedContractsMigration(stagedContractsGetter)

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)
		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		payloads, err := migration.MigrateAccount(ctx, address1,
			[]*ledger.Payload{
				newContractPayload(address1, "A", []byte(oldCode)),
			},
		)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Len(t, logWriter.logs, 1)
		require.Contains(
			t,
			logWriter.logs[0],
			`existing staged update found for contract 0x0000000000000001.A. Previous update will be overwritten.`,
		)

		require.Len(t, payloads, 1)
		require.Equal(t, update2, string(payloads[0].Value()))
	})
}

func TestStagedContractsWithImports(t *testing.T) {
	t.Parallel()

	address1, err := common.HexToAddress("0x1")
	require.NoError(t, err)

	address2, err := common.HexToAddress("0x2")
	require.NoError(t, err)

	ctx := context.Background()

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

		stagedContractsGetter := func() []StagedContract {
			return []StagedContract{
				{
					Contract: Contract{
						name: "A",
						code: []byte(newCodeA),
					},
					address: address1,
				},
				{
					Contract: Contract{
						name: "B",
						code: []byte(newCodeB),
					},
					address: address2,
				},
			}
		}

		migration := NewStagedContractsMigration(stagedContractsGetter)

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)
		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		payloads := []*ledger.Payload{
			newContractPayload(address1, "A", []byte(oldCodeA)),
			newContractPayload(address2, "B", []byte(oldCodeB)),
		}

		payloads, err = migration.MigrateAccount(ctx, address1, payloads)
		require.NoError(t, err)

		payloads, err = migration.MigrateAccount(ctx, address2, payloads)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Empty(t, logWriter.logs)

		require.Len(t, payloads, 2)
		require.Equal(t, newCodeA, string(payloads[0].Value()))
		require.Equal(t, newCodeB, string(payloads[1].Value()))
	})

	t.Run("broken import", func(t *testing.T) {
		t.Parallel()

		oldCodeA := fmt.Sprintf(`
            import B from %s
            access(all) contract A {}
        `,
			address2.HexWithPrefix(),
		)

		oldCodeB := `pub contract B {}  // not compatible`

		newCodeA := fmt.Sprintf(`
            import B from %s
            access(all) contract A {
                access(all) fun foo(a: B.C) {}
            }
        `,
			address2.HexWithPrefix(),
		)

		stagedContractsGetter := func() []StagedContract {
			return []StagedContract{
				{
					Contract: Contract{
						name: "A",
						code: []byte(newCodeA),
					},
					address: address1,
				},
			}
		}

		migration := NewStagedContractsMigration(stagedContractsGetter)

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)
		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		payloads := []*ledger.Payload{
			newContractPayload(address1, "A", []byte(oldCodeA)),
			newContractPayload(address2, "B", []byte(oldCodeB)),
		}

		payloads, err = migration.MigrateAccount(ctx, address1, payloads)
		require.NoError(t, err)

		payloads, err = migration.MigrateAccount(ctx, address2, payloads)
		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Len(t, logWriter.logs, 1)
		require.Contains(
			t,
			logWriter.logs[0],
			"`pub` is no longer a valid access keyword",
		)

		// Payloads should be the old ones
		require.Len(t, payloads, 2)
		require.Equal(t, oldCodeA, string(payloads[0].Value()))
		require.Equal(t, oldCodeB, string(payloads[1].Value()))
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

		stagedContractsGetter := func() []StagedContract {
			return []StagedContract{
				{
					Contract: Contract{
						name: "A",
						code: []byte(newCodeA),
					},
					address: address1,
				},
				{
					Contract: Contract{
						name: "C",
						code: []byte(newCodeC),
					},
					address: address1,
				},
			}
		}

		migration := NewStagedContractsMigration(stagedContractsGetter)

		logWriter := &logWriter{}
		log := zerolog.New(logWriter)
		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		payloads := []*ledger.Payload{
			newContractPayload(address1, "A", []byte(oldCodeA)),
			newContractPayload(address2, "B", []byte(oldCodeB)),
			newContractPayload(address1, "C", []byte(oldCodeC)),
		}

		payloads, err = migration.MigrateAccount(ctx, address1, payloads)
		require.NoError(t, err)

		payloads, err = migration.MigrateAccount(ctx, address2, payloads)
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
		require.Len(t, payloads, 3)
		require.Equal(t, oldCodeA, string(payloads[0].Value()))
		require.Equal(t, oldCodeB, string(payloads[1].Value()))
		require.Equal(t, newCodeC, string(payloads[2].Value()))
	})
}
