package migrations_test

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
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

type logWriter struct {
	logs []string
}

var _ io.Writer = &logWriter{}

func (l *logWriter) Write(bytes []byte) (int, error) {
	l.logs = append(l.logs, string(bytes))
	return len(bytes), nil
}

func TestChangeContractCodeMigration(t *testing.T) {
	t.Parallel()

	chainID := flow.Emulator
	addressGenerator := chainID.Chain().NewAddressGenerator()

	address1, err := addressGenerator.NextAddress()
	require.NoError(t, err)

	address2, err := addressGenerator.NextAddress()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("no contracts", func(t *testing.T) {
		t.Parallel()

		writer := &logWriter{}
		log := zerolog.New(writer)

		rwf := &testReportWriterFactory{}

		migration := migrations.NewChangeContractCodeMigration(flow.Emulator, log, rwf)

		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		_, err = migration.MigrateAccount(ctx,
			common.Address(address1),
			[]*ledger.Payload{},
		)

		require.NoError(t, err)

		err = migration.Close()
		require.NoError(t, err)

		require.Empty(t, writer.logs)
	})

	t.Run("1 contract - dont migrate", func(t *testing.T) {
		t.Parallel()

		writer := &logWriter{}
		log := zerolog.New(writer)

		rwf := &testReportWriterFactory{}

		migration := migrations.NewChangeContractCodeMigration(flow.Emulator, log, rwf)

		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		payloads, err := migration.MigrateAccount(
			ctx,
			common.Address(address1),
			[]*ledger.Payload{
				newContractPayload(
					common.Address(address1),
					"A",
					[]byte(contractA),
				),
			},
		)

		require.NoError(t, err)
		require.Len(t, payloads, 1)
		require.Equal(t, []byte(contractA), []byte(payloads[0].Value()))

		err = migration.Close()
		require.NoError(t, err)

		require.Empty(t, writer.logs)
	})

	t.Run("1 contract - migrate", func(t *testing.T) {
		t.Parallel()

		writer := &logWriter{}
		log := zerolog.New(writer)

		rwf := &testReportWriterFactory{}

		migration := migrations.NewChangeContractCodeMigration(flow.Emulator, log, rwf)

		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		migration.RegisterContractChange(
			migrations.StagedContract{
				Address: common.Address(address1),
				Contract: migrations.Contract{
					Name: "A",
					Code: []byte(updatedContractA),
				},
			},
		)

		payloads, err := migration.MigrateAccount(
			ctx,
			common.Address(address1),
			[]*ledger.Payload{
				newContractPayload(
					common.Address(address1),
					"A",
					[]byte(contractA),
				),
			},
		)

		require.NoError(t, err)
		require.Len(t, payloads, 1)
		require.Equal(t, updatedContractA, string(payloads[0].Value()))

		err = migration.Close()
		require.NoError(t, err)

		require.Empty(t, writer.logs)
	})

	t.Run("2 contracts - migrate 1", func(t *testing.T) {
		t.Parallel()

		writer := &logWriter{}
		log := zerolog.New(writer)

		rwf := &testReportWriterFactory{}

		migration := migrations.NewChangeContractCodeMigration(flow.Emulator, log, rwf)

		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		migration.RegisterContractChange(
			migrations.StagedContract{
				Address: common.Address(address1),
				Contract: migrations.Contract{
					Name: "A",
					Code: []byte(updatedContractA),
				},
			},
		)

		payloads, err := migration.MigrateAccount(
			ctx,
			common.Address(address1),
			[]*ledger.Payload{
				newContractPayload(common.Address(address1), "A", []byte(contractA)),
				newContractPayload(common.Address(address1), "B", []byte(contractB)),
			},
		)

		require.NoError(t, err)
		require.Len(t, payloads, 2)
		require.Equal(t, []byte(updatedContractA), []byte(payloads[0].Value()))
		require.Equal(t, []byte(contractB), []byte(payloads[1].Value()))

		err = migration.Close()
		require.NoError(t, err)

		require.Empty(t, writer.logs)
	})

	t.Run("2 contracts - migrate 2", func(t *testing.T) {
		t.Parallel()

		writer := &logWriter{}
		log := zerolog.New(writer)

		rwf := &testReportWriterFactory{}

		migration := migrations.NewChangeContractCodeMigration(flow.Emulator, log, rwf)

		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		migration.RegisterContractChange(
			migrations.StagedContract{
				Address: common.Address(address1),
				Contract: migrations.Contract{
					Name: "A",
					Code: []byte(updatedContractA),
				},
			},
		)
		migration.RegisterContractChange(
			migrations.StagedContract{
				Address: common.Address(address1),
				Contract: migrations.Contract{
					Name: "B",
					Code: []byte(updatedContractB),
				},
			},
		)

		payloads, err := migration.MigrateAccount(
			ctx,
			common.Address(address1),
			[]*ledger.Payload{
				newContractPayload(common.Address(address1), "A", []byte(contractA)),
				newContractPayload(common.Address(address1), "B", []byte(contractB)),
			},
		)

		require.NoError(t, err)
		require.Len(t, payloads, 2)
		require.Equal(t, []byte(updatedContractA), []byte(payloads[0].Value()))
		require.Equal(t, []byte(updatedContractB), []byte(payloads[1].Value()))

		err = migration.Close()
		require.NoError(t, err)

		require.Empty(t, writer.logs)
	})

	t.Run("2 contracts on different accounts - migrate 1", func(t *testing.T) {
		t.Parallel()

		writer := &logWriter{}
		log := zerolog.New(writer)

		rwf := &testReportWriterFactory{}

		migration := migrations.NewChangeContractCodeMigration(flow.Emulator, log, rwf)

		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		migration.RegisterContractChange(
			migrations.StagedContract{
				Address: common.Address(address1),
				Contract: migrations.Contract{
					Name: "A",
					Code: []byte(updatedContractA),
				},
			},
		)

		payloads, err := migration.MigrateAccount(
			ctx,
			common.Address(address1),
			[]*ledger.Payload{
				newContractPayload(common.Address(address1), "A", []byte(contractA)),
				newContractPayload(common.Address(address2), "A", []byte(contractA)),
			},
		)

		require.NoError(t, err)
		require.Len(t, payloads, 2)
		require.Equal(t, []byte(updatedContractA), []byte(payloads[0].Value()))
		require.Equal(t, []byte(contractA), []byte(payloads[1].Value()))

		err = migration.Close()
		require.NoError(t, err)

		require.Len(t, writer.logs, 1)
		assert.Contains(t,
			writer.logs[0],
			"payload address ee82856bf20e2aa6 does not match expected address f8d6e0586b0a20c7",
		)
	})

	t.Run("not all contracts on one account migrated", func(t *testing.T) {
		t.Parallel()

		writer := &logWriter{}
		log := zerolog.New(writer)

		rwf := &testReportWriterFactory{}

		migration := migrations.NewChangeContractCodeMigration(flow.Emulator, log, rwf)

		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		migration.RegisterContractChange(
			migrations.StagedContract{
				Address: common.Address(address1),
				Contract: migrations.Contract{
					Name: "A",
					Code: []byte(updatedContractA),
				},
			},
		)
		migration.RegisterContractChange(
			migrations.StagedContract{
				Address: common.Address(address1),
				Contract: migrations.Contract{
					Name: "B",
					Code: []byte(updatedContractB),
				},
			},
		)

		_, err = migration.MigrateAccount(
			ctx,
			common.Address(address1),
			[]*ledger.Payload{
				newContractPayload(common.Address(address1), "A", []byte(contractA)),
			},
		)

		require.NoError(t, err)

		require.Len(t, writer.logs, 1)
		assert.Contains(t,
			writer.logs[0],
			`"failed to find all contract registers that need to be changed for address"`,
		)
	})

	t.Run("not all accounts migrated", func(t *testing.T) {
		t.Parallel()

		writer := &logWriter{}
		log := zerolog.New(writer)

		rwf := &testReportWriterFactory{}

		migration := migrations.NewChangeContractCodeMigration(flow.Emulator, log, rwf)

		err := migration.InitMigration(log, nil, 0)
		require.NoError(t, err)

		migration.RegisterContractChange(
			migrations.StagedContract{
				Address: common.Address(address2),
				Contract: migrations.Contract{
					Name: "A",
					Code: []byte(updatedContractA),
				},
			},
		)

		_, err = migration.MigrateAccount(
			ctx,
			common.Address(address1),
			[]*ledger.Payload{
				newContractPayload(common.Address(address1), "A", []byte(contractA)),
			},
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

type testReportWriterFactory struct {
	lock          sync.Mutex
	reportWriters map[string]*testReportWriter
}

func (f *testReportWriterFactory) ReportWriter(dataNamespace string) reporters.ReportWriter {
	f.lock.Lock()
	defer f.lock.Unlock()

	if f.reportWriters == nil {
		f.reportWriters = make(map[string]*testReportWriter)
	}
	reportWriter := &testReportWriter{}
	if _, ok := f.reportWriters[dataNamespace]; ok {
		panic(fmt.Sprintf("report writer already exists for namespace %s", dataNamespace))
	}
	f.reportWriters[dataNamespace] = reportWriter
	return reportWriter
}

type testReportWriter struct {
	lock    sync.Mutex
	entries []any
}

var _ reporters.ReportWriter = &testReportWriter{}

func (r *testReportWriter) Write(entry any) {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.entries = append(r.entries, entry)
}

func (r *testReportWriter) Close() {}
