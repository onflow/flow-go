package environment

import (
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
)

func TestUUIDPartition(t *testing.T) {
	blockHeader := &flow.Header{}

	usedPartitions := map[byte]struct{}{}

	// With enough samples, all partitions should be used.  (The first 1500 blocks
	// only uses 254 partitions)
	for numBlocks := 0; numBlocks < 2000; numBlocks++ {
		blockId := blockHeader.ID()

		partition0 := uuidPartition(blockId, 0)
		usedPartitions[partition0] = struct{}{}

		for txnIndex := 0; txnIndex < 256; txnIndex++ {
			partition := uuidPartition(blockId, uint32(txnIndex))

			// Ensure neighboring transactions uses neighoring partitions.
			require.Equal(t, partition, partition0+byte(txnIndex))

			// Ensure wrap around.
			for i := 0; i < 5; i++ {
				require.Equal(
					t,
					partition,
					uuidPartition(blockId, uint32(txnIndex+i*256)))
			}
		}

		blockHeader.ParentID = blockId
	}

	require.Len(t, usedPartitions, 256)
}

func TestUUIDGeneratorInitializePartitionNoHeader(t *testing.T) {
	for txnIndex := uint32(0); txnIndex < 256; txnIndex++ {
		uuids := NewUUIDGenerator(
			tracing.NewTracerSpan(),
			zerolog.Nop(),
			nil,
			nil,
			nil,
			txnIndex)
		require.False(t, uuids.initialized)

		uuids.maybeInitializePartition()

		require.True(t, uuids.initialized)
		require.Equal(t, uuids.partition, byte(0))
		require.Equal(t, uuids.registerId, flow.UUIDRegisterID(byte(0)))
	}
}

func TestUUIDGeneratorInitializePartition(t *testing.T) {
	blockHeader := &flow.Header{}

	for numBlocks := 0; numBlocks < 10; numBlocks++ {
		blockId := blockHeader.ID()

		for txnIndex := uint32(0); txnIndex < 256; txnIndex++ {
			uuids := NewUUIDGenerator(
				tracing.NewTracerSpan(),
				zerolog.Nop(),
				nil,
				nil,
				blockHeader,
				txnIndex)
			require.False(t, uuids.initialized)

			uuids.maybeInitializePartition()

			require.True(t, uuids.initialized)

			expectedPartition := uuidPartition(blockId, txnIndex)

			require.Equal(t, uuids.partition, expectedPartition)
			require.Equal(
				t,
				uuids.registerId,
				flow.UUIDRegisterID(expectedPartition))
		}

		blockHeader.ParentID = blockId
	}
}

func TestUUIDGeneratorIdGeneration(t *testing.T) {
	for txnIndex := uint32(0); txnIndex < 256; txnIndex++ {
		testUUIDGenerator(t, &flow.Header{}, txnIndex)
	}
}

func testUUIDGenerator(t *testing.T, blockHeader *flow.Header, txnIndex uint32) {
	generator := NewUUIDGenerator(
		tracing.NewTracerSpan(),
		zerolog.Nop(),
		nil,
		nil,
		blockHeader,
		txnIndex)
	generator.maybeInitializePartition()

	partition := generator.partition
	partitionMinValue := uint64(partition) << 40
	maxUint56 := uint64(0xFFFFFFFFFFFFFF)
	maxUint56Split := uint64(0xFFFF00FFFFFFFFFF)

	t.Run(
		fmt.Sprintf("basic get and set uint (partition: %d)", partition),
		func(t *testing.T) {
			txnState := state.NewTransactionState(nil, state.DefaultParameters())
			uuidsA := NewUUIDGenerator(
				tracing.NewTracerSpan(),
				zerolog.Nop(),
				NewMeter(txnState),
				txnState,
				blockHeader,
				txnIndex)
			uuidsA.maybeInitializePartition()

			uuid, err := uuidsA.getUint64() // start from zero
			require.NoError(t, err)
			require.Equal(t, uint64(0), uuid)

			err = uuidsA.setUint56(5)
			require.NoError(t, err)

			// create new UUIDs instance
			uuidsB := NewUUIDGenerator(
				tracing.NewTracerSpan(),
				zerolog.Nop(),
				NewMeter(txnState),
				txnState,
				blockHeader,
				txnIndex)
			uuidsB.maybeInitializePartition()

			uuid, err = uuidsB.getUint64() // should read saved value
			require.NoError(t, err)

			require.Equal(t, uint64(5), uuid)
		})

	t.Run(
		fmt.Sprintf("basic id generation (partition: %d)", partition),
		func(t *testing.T) {
			txnState := state.NewTransactionState(nil, state.DefaultParameters())
			genA := NewUUIDGenerator(
				tracing.NewTracerSpan(),
				zerolog.Nop(),
				NewMeter(txnState),
				txnState,
				blockHeader,
				txnIndex)

			uuidA, err := genA.GenerateUUID()
			require.NoError(t, err)
			uuidB, err := genA.GenerateUUID()
			require.NoError(t, err)
			uuidC, err := genA.GenerateUUID()
			require.NoError(t, err)

			require.Equal(t, partitionMinValue, uuidA)
			require.Equal(t, partitionMinValue+1, uuidB)
			require.Equal(t, partitionMinValue|1, uuidB)
			require.Equal(t, partitionMinValue+2, uuidC)
			require.Equal(t, partitionMinValue|2, uuidC)

			// Create new generator instance from same ledger
			genB := NewUUIDGenerator(
				tracing.NewTracerSpan(),
				zerolog.Nop(),
				NewMeter(txnState),
				txnState,
				blockHeader,
				txnIndex)

			uuidD, err := genB.GenerateUUID()
			require.NoError(t, err)
			uuidE, err := genB.GenerateUUID()
			require.NoError(t, err)
			uuidF, err := genB.GenerateUUID()
			require.NoError(t, err)

			require.Equal(t, partitionMinValue+3, uuidD)
			require.Equal(t, partitionMinValue|3, uuidD)
			require.Equal(t, partitionMinValue+4, uuidE)
			require.Equal(t, partitionMinValue|4, uuidE)
			require.Equal(t, partitionMinValue+5, uuidF)
			require.Equal(t, partitionMinValue|5, uuidF)
		})

	t.Run(
		fmt.Sprintf("setUint56 overflows (partition: %d)", partition),
		func(t *testing.T) {
			txnState := state.NewTransactionState(nil, state.DefaultParameters())
			uuids := NewUUIDGenerator(
				tracing.NewTracerSpan(),
				zerolog.Nop(),
				NewMeter(txnState),
				txnState,
				blockHeader,
				txnIndex)
			uuids.maybeInitializePartition()

			err := uuids.setUint56(maxUint56)
			require.NoError(t, err)

			value, err := uuids.getUint64()
			require.NoError(t, err)
			require.Equal(t, value, maxUint56)

			err = uuids.setUint56(maxUint56 + 1)
			require.ErrorContains(t, err, "overflowed")

			value, err = uuids.getUint64()
			require.NoError(t, err)
			require.Equal(t, value, maxUint56)
		})

	t.Run(
		fmt.Sprintf("id generation overflows (partition: %d)", partition),
		func(t *testing.T) {
			txnState := state.NewTransactionState(nil, state.DefaultParameters())
			uuids := NewUUIDGenerator(
				tracing.NewTracerSpan(),
				zerolog.Nop(),
				NewMeter(txnState),
				txnState,
				blockHeader,
				txnIndex)
			uuids.maybeInitializePartition()

			err := uuids.setUint56(maxUint56 - 1)
			require.NoError(t, err)

			value, err := uuids.GenerateUUID()
			require.NoError(t, err)
			require.Equal(t, value, partitionMinValue+maxUint56Split-1)
			require.Equal(t, value, partitionMinValue|(maxUint56Split-1))

			value, err = uuids.getUint64()
			require.NoError(t, err)
			require.Equal(t, value, maxUint56)

			_, err = uuids.GenerateUUID()
			require.ErrorContains(t, err, "overflowed")

			value, err = uuids.getUint64()
			require.NoError(t, err)
			require.Equal(t, value, maxUint56)
		})
}

func TestUUIDGeneratorHardcodedPartitionIdGeneration(t *testing.T) {
	txnState := state.NewTransactionState(nil, state.DefaultParameters())
	uuids := NewUUIDGenerator(
		tracing.NewTracerSpan(),
		zerolog.Nop(),
		NewMeter(txnState),
		txnState,
		nil,
		0)

	// Hardcoded the partition to check for exact bytes
	uuids.initialized = true
	uuids.partition = 0xde
	uuids.registerId = flow.UUIDRegisterID(0xde)

	value, err := uuids.GenerateUUID()
	require.NoError(t, err)
	require.Equal(t, value, uint64(0x0000de0000000000))

	value, err = uuids.getUint64()
	require.NoError(t, err)
	require.Equal(t, value, uint64(1))

	value, err = uuids.GenerateUUID()
	require.NoError(t, err)
	require.Equal(t, value, uint64(0x0000de0000000001))

	value, err = uuids.getUint64()
	require.NoError(t, err)
	require.Equal(t, value, uint64(2))

	value, err = uuids.GenerateUUID()
	require.NoError(t, err)
	require.Equal(t, value, uint64(0x0000de0000000002))

	value, err = uuids.getUint64()
	require.NoError(t, err)
	require.Equal(t, value, uint64(3))

	// pretend we increamented the counter up to cafBad
	cafBad := uint64(0x1c2a3f4b5a6d70)
	decafBad := uint64(0x1c2ade3f4b5a6d70)

	err = uuids.setUint56(cafBad)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		value, err = uuids.GenerateUUID()
		require.NoError(t, err)
		require.Equal(t, value, decafBad+uint64(i))
	}

	value, err = uuids.getUint64()
	require.NoError(t, err)
	require.Equal(t, value, cafBad+uint64(5))

	// pretend we increamented the counter up to overflow - 2
	maxUint56Minus2 := uint64(0xfffffffffffffd)
	err = uuids.setUint56(maxUint56Minus2)
	require.NoError(t, err)

	value, err = uuids.GenerateUUID()
	require.NoError(t, err)
	require.Equal(t, value, uint64(0xffffdefffffffffd))

	value, err = uuids.getUint64()
	require.NoError(t, err)
	require.Equal(t, value, maxUint56Minus2+1)

	value, err = uuids.GenerateUUID()
	require.NoError(t, err)
	require.Equal(t, value, uint64(0xffffdefffffffffe))

	value, err = uuids.getUint64()
	require.NoError(t, err)
	require.Equal(t, value, maxUint56Minus2+2)

	_, err = uuids.GenerateUUID()
	require.ErrorContains(t, err, "overflowed")

	value, err = uuids.getUint64()
	require.NoError(t, err)
	require.Equal(t, value, maxUint56Minus2+2)
}

func TestContinuati(t *testing.T) {
	txnState := state.NewTransactionState(nil, state.DefaultParameters())
	uuids := NewUUIDGenerator(
		tracing.NewTracerSpan(),
		zerolog.Nop(),
		NewMeter(txnState),
		txnState,
		nil,
		0)

	// Hardcoded the partition to check for exact bytes
	uuids.initialized = true
	uuids.partition = 0x01
	uuids.registerId = flow.UUIDRegisterID(0x01)

	value, err := uuids.GenerateUUID()
	require.NoError(t, err)
	require.Equal(t, value, uint64(0x0000010000000000))

	err = uuids.setUint56(0xFFFFFFFFFF)
	require.NoError(t, err)

	value, err = uuids.GenerateUUID()
	require.NoError(t, err)
	require.Equal(t, value, uint64(0x000001FFFFFFFFFF))

	value, err = uuids.GenerateUUID()
	require.NoError(t, err)
	require.Equal(t, value, uint64(0x0001010000000000))

}
