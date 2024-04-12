package migrations

import (
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
	payloadsLedger.AllocateStorageIndexFunc = func(owner []byte) (atree.SlabIndex, error) {
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
	dictionaryStaticType := interpreter.NewDictionaryStaticType(
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
		dictionaryStaticType,
		testAddress,
	)

	// Storage another dictionary in the account.
	// It is not referenced through a storage map though.

	interpreter.NewDictionaryValueWithAddress(
		inter,
		interpreter.EmptyLocationRange,
		dictionaryStaticType,
		testAddress,
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

	const totalSlabCount = 4

	require.Len(t, oldPayloads, totalSlabCount)

	// Act

	rwf := &testReportWriterFactory{}
	migration := NewFilterUnreferencedSlabsMigration(rwf)

	log := zerolog.New(zerolog.NewTestWriter(t))

	err = migration.InitMigration(log, nil, 0)
	require.NoError(t, err)

	newPayloads, err := migration.MigrateAccount(nil, testAddress, oldPayloads)
	require.NoError(t, err)

	// Assert

	assert.Len(t, newPayloads, totalSlabCount-1)

	writer := rwf.reportWriters[filterUnreferencedSlabsName]

	expectedFilteredPayloads := make([]*ledger.Payload, 0, 1)

	expectedAddress := string(testAddress[:])
	expectedKey := string([]byte{flow.SlabIndexPrefix, 0, 0, 0, 0, 0, 0, 0, 2})

	for _, payload := range oldPayloads {
		registerID, _, err := convert.PayloadToRegister(payload)
		require.NoError(t, err)

		if registerID.Owner != expectedAddress ||
			registerID.Key != expectedKey {

			continue
		}

		expectedFilteredPayloads = append(expectedFilteredPayloads, payload)
		break
	}

	assert.Equal(t,
		[]any{
			unreferencedSlabs{
				Account:  testAddress,
				Payloads: expectedFilteredPayloads,
			},
		},
		writer.entries,
	)
}
