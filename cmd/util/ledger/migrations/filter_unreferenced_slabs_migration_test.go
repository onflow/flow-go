package migrations

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"
	"testing"

	"github.com/onflow/atree"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

func TestFilterUnreferencedSlabs(t *testing.T) {
	t.Parallel()

	// Arrange

	const chainID = flow.Emulator
	chain := chainID.Chain()

	testFlowAddress, err := chain.AddressAtIndex(1_000_000)
	require.NoError(t, err)

	testAddress := common.Address(testFlowAddress)

	payloads := map[flow.RegisterID]*ledger.Payload{}

	payloadsLedger := util.NewPayloadsLedger(payloads)

	storageIndices := map[string]uint64{}
	payloadsLedger.AllocateSlabIndexFunc = func(owner []byte) (atree.SlabIndex, error) {
		var index atree.SlabIndex

		storageIndices[string(owner)]++

		binary.BigEndian.PutUint64(
			index[:],
			storageIndices[string(owner)],
		)

		return index, nil
	}

	storage := runtime.NewStorage(payloadsLedger, nil)

	// {Int: Int}
	dict1StaticType := interpreter.NewDictionaryStaticType(
		nil,
		interpreter.PrimitiveStaticTypeInt,
		interpreter.PrimitiveStaticTypeInt,
	)

	inter, err := interpreter.NewInterpreter(
		nil,
		nil,
		&interpreter.Config{
			Storage: storage,
		},
	)
	require.NoError(t, err)

	dict1 := interpreter.NewDictionaryValueWithAddress(
		inter,
		interpreter.EmptyLocationRange,
		dict1StaticType,
		testAddress,
	)

	// Store another dictionary, with a nested array, in the account.
	// It is not referenced through a storage map though.

	arrayStaticType := interpreter.NewVariableSizedStaticType(nil, interpreter.PrimitiveStaticTypeInt)

	dict2StaticType := interpreter.NewDictionaryStaticType(
		nil,
		interpreter.PrimitiveStaticTypeInt,
		arrayStaticType,
	)

	dict2 := interpreter.NewDictionaryValueWithAddress(
		inter,
		interpreter.EmptyLocationRange,
		dict2StaticType,
		testAddress,
	)

	// Ensure the array is large enough to be stored in a separate slab
	arrayCount := 100
	arrayValues := make([]interpreter.Value, arrayCount)
	for i := 0; i < arrayCount; i++ {
		arrayValues[i] = interpreter.NewUnmeteredIntValueFromInt64(int64(i))
	}

	array := interpreter.NewArrayValue(
		inter,
		interpreter.EmptyLocationRange,
		arrayStaticType,
		common.ZeroAddress,
		arrayValues...,
	)

	dict2.Insert(
		inter, interpreter.EmptyLocationRange,
		interpreter.NewUnmeteredIntValueFromInt64(2),
		array,
	)

	storageMap := storage.GetStorageMap(
		testAddress,
		common.PathDomainStorage.Identifier(),
		true,
	)

	// Only insert first dictionary.
	// Second dictionary is unreferenced.

	storageMap.SetValue(
		inter,
		interpreter.StringStorageMapKey("test"),
		dict1,
	)

	err = storage.Commit(inter, false)
	require.NoError(t, err)

	oldPayloads := make([]*ledger.Payload, 0, len(payloads))

	for _, payload := range payloadsLedger.Payloads {
		if len(payload.Value()) == 0 {
			// Don't count empty slabs as result of inlining.
			continue
		}
		oldPayloads = append(oldPayloads, payload)
	}

	// Storage has 4 non-empty payloads:
	// - storage map
	// - dict1
	// - dict2
	// - nested array in dict2
	const totalSlabCount = 4

	require.Len(t, oldPayloads, totalSlabCount)

	// Act

	rwf := &testReportWriterFactory{}
	migration := NewFilterUnreferencedSlabsMigration(t.TempDir(), rwf)

	log := zerolog.New(zerolog.NewTestWriter(t))

	registersByAccount, err := registers.NewByAccountFromPayloads(oldPayloads)
	require.NoError(t, err)

	err = migration.InitMigration(log, registersByAccount, 1)
	require.NoError(t, err)

	accountRegisters := registersByAccount.AccountRegisters(string(testAddress[:]))

	err = migration.MigrateAccount(
		context.Background(),
		testAddress,
		accountRegisters,
	)
	require.NoError(t, err)

	err = migration.Close()
	require.NoError(t, err)

	// Assert

	writer := rwf.reportWriters[filterUnreferencedSlabsName]

	expectedAddress := string(testAddress[:])
	expectedKeys := map[string]struct{}{
		string([]byte{flow.SlabIndexPrefix, 0, 0, 0, 0, 0, 0, 0, 2}): {},
		string([]byte{flow.SlabIndexPrefix, 0, 0, 0, 0, 0, 0, 0, 3}): {},
	}

	newPayloads := registersByAccount.DestructIntoPayloads()
	assert.Len(t, newPayloads, totalSlabCount-len(expectedKeys))

	expectedFilteredPayloads := make([]*ledger.Payload, 0, len(expectedKeys))

	for _, payload := range oldPayloads {
		registerID, _, err := convert.PayloadToRegister(payload)
		require.NoError(t, err)

		if registerID.Owner != expectedAddress {
			continue
		}

		if _, ok := expectedKeys[registerID.Key]; !ok {
			continue
		}

		expectedFilteredPayloads = append(expectedFilteredPayloads, payload)
	}

	assert.Equal(t,
		[]any{
			unreferencedSlabs{
				Account:      testAddress.Hex(),
				PayloadCount: len(expectedFilteredPayloads),
			},
		},
		writer.entries,
	)
	assert.ElementsMatch(t,
		expectedFilteredPayloads,
		migration.filteredPayloads,
	)

	readIsPartial, readFilteredPayloads, err := util.ReadPayloadFile(log, migration.payloadsFile)
	require.NoError(t, err)
	assert.True(t, readIsPartial)
	assert.ElementsMatch(t,
		expectedFilteredPayloads,
		readFilteredPayloads,
	)
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
