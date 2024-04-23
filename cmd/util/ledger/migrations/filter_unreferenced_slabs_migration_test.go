package migrations

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/onflow/atree"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
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
	payloadsLedger.AllocateStorageIndexFunc = func(owner []byte) (atree.StorageIndex, error) {
		var index atree.StorageIndex

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

	// Storage another dictionary, with a nested array, in the account.
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

	dict2.InsertWithoutTransfer(
		inter, interpreter.EmptyLocationRange,
		interpreter.NewUnmeteredIntValueFromInt64(2),
		interpreter.NewArrayValue(
			inter,
			interpreter.EmptyLocationRange,
			arrayStaticType,
			testAddress,
			interpreter.NewUnmeteredIntValueFromInt64(3),
		),
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
		oldPayloads = append(oldPayloads, payload)
	}

	const totalSlabCount = 5

	require.Len(t, oldPayloads, totalSlabCount)

	// Act

	rwf := &testReportWriterFactory{}
	migration := NewFilterUnreferencedSlabsMigration(t.TempDir(), rwf)

	log := zerolog.New(zerolog.NewTestWriter(t))

	err = migration.InitMigration(log, nil, 0)
	require.NoError(t, err)

	ctx := context.Background()

	newPayloads, err := migration.MigrateAccount(ctx, testAddress, oldPayloads)
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
				Account:      testAddress,
				PayloadCount: len(expectedFilteredPayloads),
			},
		},
		writer.entries,
	)
	assert.Equal(t,
		expectedFilteredPayloads,
		migration.filteredPayloads,
	)
	assert.Equal(t,
		[]common.Address{testAddress},
		migration.filteredAccounts,
	)

	readIsPartial, readFilteredPayloads, err := util.ReadPayloadFile(log, migration.payloadsFile)
	require.NoError(t, err)
	assert.True(t, readIsPartial)
	assert.Equal(t, expectedFilteredPayloads, readFilteredPayloads)
}
