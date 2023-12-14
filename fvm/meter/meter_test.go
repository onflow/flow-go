package meter_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/model/flow"
)

func TestWeightedComputationMetering(t *testing.T) {

	t.Run("get limits", func(t *testing.T) {
		m := meter.NewMeter(
			meter.DefaultParameters().
				WithComputationLimit(1).
				WithMemoryLimit(2),
		)
		require.Equal(t, uint(1), m.TotalComputationLimit())
		require.Equal(t, uint64(2), m.TotalMemoryLimit())
	})

	t.Run("get limits max", func(t *testing.T) {
		m := meter.NewMeter(
			meter.DefaultParameters().
				WithComputationLimit(math.MaxUint32).
				WithMemoryLimit(math.MaxUint32),
		)
		require.Equal(t, uint(math.MaxUint32), m.TotalComputationLimit())
		require.Equal(t, uint64(math.MaxUint32), m.TotalMemoryLimit())
	})

	t.Run("meter computation and memory", func(t *testing.T) {
		m := meter.NewMeter(
			meter.DefaultParameters().
				WithComputationLimit(10).
				WithComputationWeights(
					map[common.ComputationKind]uint64{0: 1 << meter.MeterExecutionInternalPrecisionBytes}).
				WithMemoryLimit(10).
				WithMemoryWeights(map[common.MemoryKind]uint64{0: 1}),
		)

		err := m.MeterComputation(0, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), m.TotalComputationUsed())

		err = m.MeterComputation(0, 2)
		require.NoError(t, err)
		require.Equal(t, uint64(1+2), m.TotalComputationUsed())

		err = m.MeterComputation(0, 8)
		require.Error(t, err)
		require.True(t, errors.IsComputationLimitExceededError(err))
		require.Equal(t, err.Error(), errors.NewComputationLimitExceededError(10).Error())

		err = m.MeterMemory(0, 2)
		require.NoError(t, err)
		require.Equal(t, uint64(2), m.TotalMemoryEstimate())

		err = m.MeterMemory(0, 3)
		require.NoError(t, err)
		require.Equal(t, uint64(2+3), m.TotalMemoryEstimate())

		err = m.MeterMemory(0, 8)
		require.Error(t, err)
		require.True(t, errors.IsMemoryLimitExceededError(err))
		require.Equal(t, err.Error(), errors.NewMemoryLimitExceededError(10).Error())
	})

	t.Run("meter computation and memory with weights", func(t *testing.T) {
		m := meter.NewMeter(
			meter.DefaultParameters().
				WithComputationLimit(100).
				WithComputationWeights(
					map[common.ComputationKind]uint64{0: 13 << meter.MeterExecutionInternalPrecisionBytes}).
				WithMemoryLimit(100).
				WithMemoryWeights(map[common.MemoryKind]uint64{0: 17}),
		)

		err := m.MeterComputation(0, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(13), m.TotalComputationUsed())
		require.Equal(t, uint(1), m.ComputationIntensities()[0])

		err = m.MeterMemory(0, 2)
		require.NoError(t, err)
		require.Equal(t, uint64(34), m.TotalMemoryEstimate())
		require.Equal(t, uint(2), m.MemoryIntensities()[0])
	})

	t.Run("meter computation with weights lower than MeterInternalPrecisionBytes", func(t *testing.T) {
		m := meter.NewMeter(
			meter.DefaultParameters().
				WithComputationLimit(100).
				WithComputationWeights(map[common.ComputationKind]uint64{0: 1}).
				WithMemoryLimit(100).
				WithMemoryWeights(map[common.MemoryKind]uint64{0: 1}),
		)

		internalPrecisionMinusOne := uint((1 << meter.MeterExecutionInternalPrecisionBytes) - 1)

		err := m.MeterComputation(0, internalPrecisionMinusOne)
		require.NoError(t, err)
		require.Equal(t, uint64(0), m.TotalComputationUsed())
		require.Equal(t, internalPrecisionMinusOne, m.ComputationIntensities()[0])

		err = m.MeterComputation(0, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), m.TotalComputationUsed())
		require.Equal(t, uint(1<<meter.MeterExecutionInternalPrecisionBytes), m.ComputationIntensities()[0])
	})

	t.Run("merge meters", func(t *testing.T) {
		compKind := common.ComputationKind(0)
		m := meter.NewMeter(
			meter.DefaultParameters().
				WithComputationLimit(9).
				WithComputationWeights(
					map[common.ComputationKind]uint64{0: 1 << meter.MeterExecutionInternalPrecisionBytes}).
				WithMemoryLimit(0).
				WithMemoryWeights(map[common.MemoryKind]uint64{0: 1}),
		)

		err := m.MeterComputation(compKind, 1)
		require.NoError(t, err)

		child1 := meter.NewMeter(m.MeterParameters)
		err = child1.MeterComputation(compKind, 2)
		require.NoError(t, err)

		child2 := meter.NewMeter(m.MeterParameters)
		err = child2.MeterComputation(compKind, 3)
		require.NoError(t, err)

		child3 := meter.NewMeter(m.MeterParameters)
		err = child3.MeterComputation(compKind, 4)
		require.NoError(t, err)

		m.MergeMeter(child1)
		require.Equal(t, uint64(1+2), m.TotalComputationUsed())
		require.Equal(t, uint(1+2), m.ComputationIntensities()[compKind])

		m.MergeMeter(child2)
		require.Equal(t, uint64(1+2+3), m.TotalComputationUsed())
		require.Equal(t, uint(1+2+3), m.ComputationIntensities()[compKind])

		// merge hits limit, but is accepted.
		m.MergeMeter(child3)
		require.Equal(t, uint64(1+2+3+4), m.TotalComputationUsed())
		require.Equal(t, uint(1+2+3+4), m.ComputationIntensities()[compKind])

		// error after merge (hitting limit)
		err = m.MeterComputation(compKind, 0)
		require.Error(t, err)
		require.True(t, errors.IsComputationLimitExceededError(err))
		require.Equal(t, err.Error(), errors.NewComputationLimitExceededError(9).Error())
	})

	t.Run("merge meters - ignore limits", func(t *testing.T) {
		compKind := common.ComputationKind(0)
		m := meter.NewMeter(
			meter.DefaultParameters().
				WithComputationLimit(9).
				WithMemoryLimit(0).
				WithComputationWeights(map[common.ComputationKind]uint64{0: 1 << meter.MeterExecutionInternalPrecisionBytes}),
		)

		err := m.MeterComputation(compKind, 1)
		require.NoError(t, err)

		child := meter.NewMeter(m.MeterParameters)
		err = child.MeterComputation(compKind, 1)
		require.NoError(t, err)

		// hitting limit and ignoring it
		m.MergeMeter(child)
		require.Equal(t, uint64(1+1), m.TotalComputationUsed())
		require.Equal(t, uint(1+1), m.ComputationIntensities()[compKind])
	})

	t.Run("merge meters - large values - computation", func(t *testing.T) {
		m := meter.NewMeter(
			meter.DefaultParameters().
				WithComputationLimit(math.MaxUint32).
				WithComputationWeights(map[common.ComputationKind]uint64{
					0: math.MaxUint32 << meter.MeterExecutionInternalPrecisionBytes,
				}),
		)

		err := m.MeterComputation(0, 1)
		require.NoError(t, err)

		child1 := meter.NewMeter(m.MeterParameters)
		err = child1.MeterComputation(0, 1)
		require.NoError(t, err)

		m.MergeMeter(child1)

		err = m.MeterComputation(0, 0)
		require.True(t, errors.IsComputationLimitExceededError(err))
	})

	t.Run("merge meters - large values - memory", func(t *testing.T) {
		m := meter.NewMeter(
			meter.DefaultParameters().
				WithMemoryLimit(math.MaxUint32).
				WithMemoryWeights(map[common.MemoryKind]uint64{
					0: math.MaxUint32,
				}),
		)

		err := m.MeterMemory(0, 1)
		require.NoError(t, err)

		child1 := meter.NewMeter(m.MeterParameters)
		err = child1.MeterMemory(0, 1)
		require.NoError(t, err)

		m.MergeMeter(child1)

		err = m.MeterMemory(0, 0)
		require.Error(t, err)
		require.True(t, errors.IsMemoryLimitExceededError(err))
		require.Equal(t, err.Error(), errors.NewMemoryLimitExceededError(math.MaxUint32).Error())
	})

	t.Run("add intensity - test limits - computation", func(t *testing.T) {
		var m *meter.Meter
		reset := func() {
			m = meter.NewMeter(
				meter.DefaultParameters().
					WithComputationLimit(math.MaxUint32).
					WithComputationWeights(map[common.ComputationKind]uint64{
						0: 0,
						1: 1,
						2: 1 << meter.MeterExecutionInternalPrecisionBytes,
						3: math.MaxUint64,
					}),
			)
		}

		reset()
		err := m.MeterComputation(0, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(0), m.TotalComputationUsed())
		reset()
		err = m.MeterComputation(0, 1<<meter.MeterExecutionInternalPrecisionBytes)
		require.NoError(t, err)
		require.Equal(t, uint64(0), m.TotalComputationUsed())
		reset()
		err = m.MeterComputation(0, math.MaxUint32)
		require.NoError(t, err)
		require.Equal(t, uint64(0), m.TotalComputationUsed())

		reset()
		err = m.MeterComputation(1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(0), m.TotalComputationUsed())
		reset()
		err = m.MeterComputation(1, 1<<meter.MeterExecutionInternalPrecisionBytes)
		require.NoError(t, err)
		require.Equal(t, uint64(1), m.TotalComputationUsed())
		reset()
		err = m.MeterComputation(1, math.MaxUint32)
		require.NoError(t, err)
		require.Equal(t, uint64(1<<16-1), m.TotalComputationUsed())

		reset()
		err = m.MeterComputation(2, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), m.TotalComputationUsed())
		reset()
		err = m.MeterComputation(2, 1<<meter.MeterExecutionInternalPrecisionBytes)
		require.NoError(t, err)
		require.Equal(t, uint64(1<<16), m.TotalComputationUsed())
		reset()
		err = m.MeterComputation(2, math.MaxUint32)
		require.NoError(t, err)
		require.Equal(t, uint64(math.MaxUint32), m.TotalComputationUsed())

		reset()
		err = m.MeterComputation(3, 1)
		require.True(t, errors.IsComputationLimitExceededError(err))
		reset()
		err = m.MeterComputation(3, 1<<meter.MeterExecutionInternalPrecisionBytes)
		require.True(t, errors.IsComputationLimitExceededError(err))
		reset()
		err = m.MeterComputation(3, math.MaxUint32)
		require.True(t, errors.IsComputationLimitExceededError(err))
	})

	t.Run("add intensity - test limits - memory", func(t *testing.T) {
		var m *meter.Meter
		reset := func() {
			m = meter.NewMeter(
				meter.DefaultParameters().
					WithMemoryLimit(math.MaxUint32).
					WithMemoryWeights(map[common.MemoryKind]uint64{
						0: 0,
						1: 1,
						2: 2,
						3: math.MaxUint64,
					}),
			)
		}

		reset()
		err := m.MeterMemory(0, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(0), m.TotalMemoryEstimate())
		reset()
		err = m.MeterMemory(0, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(0), m.TotalMemoryEstimate())
		reset()
		err = m.MeterMemory(0, math.MaxUint32)
		require.NoError(t, err)
		require.Equal(t, uint64(0), m.TotalMemoryEstimate())

		reset()
		err = m.MeterMemory(1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), m.TotalMemoryEstimate())
		reset()
		err = m.MeterMemory(1, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(1), m.TotalMemoryEstimate())
		reset()
		err = m.MeterMemory(1, math.MaxUint32)
		require.NoError(t, err)
		require.Equal(t, uint64(math.MaxUint32), m.TotalMemoryEstimate())

		reset()
		err = m.MeterMemory(2, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(2), m.TotalMemoryEstimate())
		reset()
		err = m.MeterMemory(2, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(2), m.TotalMemoryEstimate())
		reset()
		err = m.MeterMemory(2, math.MaxUint32)
		require.True(t, errors.IsMemoryLimitExceededError(err))

		reset()
		err = m.MeterMemory(3, 1)
		require.True(t, errors.IsMemoryLimitExceededError(err))
		reset()
		err = m.MeterMemory(3, 1)
		require.True(t, errors.IsMemoryLimitExceededError(err))
		reset()
		err = m.MeterMemory(3, math.MaxUint32)
		require.True(t, errors.IsMemoryLimitExceededError(err))
	})
}

func TestMemoryWeights(t *testing.T) {
	for kind := common.MemoryKindUnknown + 1; kind < common.MemoryKindLast; kind++ {
		weight, ok := meter.DefaultMemoryWeights[kind]
		if !assert.True(t, ok, fmt.Sprintf("missing weight for memory kind '%s'", kind.String())) {
			continue
		}
		assert.Greater(
			t,
			weight,
			uint64(0),
			fmt.Sprintf(
				"weight for memory kind '%s' is not a positive integer: %d",
				kind.String(),
				weight,
			),
		)
	}
}

func TestStorageLimits(t *testing.T) {
	t.Run("metering storage read - within limit", func(t *testing.T) {
		meter1 := meter.NewMeter(
			meter.DefaultParameters(),
		)

		key1 := flow.NewRegisterID(flow.EmptyAddress, "1")
		val1 := []byte{0x1, 0x2, 0x3}
		size1 := meter.GetStorageKeyValueSizeForTesting(key1, val1)

		// first read of key1
		err := meter1.MeterStorageRead(key1, val1, false)
		require.NoError(t, err)
		require.Equal(t, meter1.TotalBytesReadFromStorage(), size1)

		// second read of key1
		err = meter1.MeterStorageRead(key1, val1, false)
		require.NoError(t, err)
		require.Equal(t, meter1.TotalBytesReadFromStorage(), size1)

		// first read of key2
		key2 := flow.NewRegisterID(flow.EmptyAddress, "2")
		val2 := []byte{0x3, 0x2, 0x1}
		size2 := meter.GetStorageKeyValueSizeForTesting(key2, val2)

		err = meter1.MeterStorageRead(key2, val2, false)
		require.NoError(t, err)
		require.Equal(t, meter1.TotalBytesReadFromStorage(), size1+size2)
	})

	t.Run("metering storage written - within limit", func(t *testing.T) {
		meter1 := meter.NewMeter(
			meter.DefaultParameters(),
		)

		key1 := flow.NewRegisterID(flow.EmptyAddress, "1")
		val1 := []byte{0x1, 0x2, 0x3}
		val2 := []byte{0x1, 0x2, 0x3, 0x4}

		// first write of key1
		err := meter1.MeterStorageWrite(key1, val1, false)
		require.NoError(t, err)
		require.Equal(t, meter1.TotalBytesWrittenToStorage(), meter.GetStorageKeyValueSizeForTesting(key1, val1))

		// second write of key1 with val2
		err = meter1.MeterStorageWrite(key1, val2, false)
		require.NoError(t, err)
		require.Equal(t, meter1.TotalBytesWrittenToStorage(), meter.GetStorageKeyValueSizeForTesting(key1, val2))

		// first write of key2
		key2 := flow.NewRegisterID(flow.EmptyAddress, "2")
		err = meter1.MeterStorageWrite(key2, val2, false)
		require.NoError(t, err)
		require.Equal(t, meter1.TotalBytesWrittenToStorage(),
			meter.GetStorageKeyValueSizeForTesting(key1, val2)+meter.GetStorageKeyValueSizeForTesting(key2, val2))
	})

	t.Run("metering storage read - exceeding limit - not enforced", func(t *testing.T) {
		meter1 := meter.NewMeter(
			meter.DefaultParameters().WithStorageInteractionLimit(1),
		)

		key1 := flow.NewRegisterID(flow.EmptyAddress, "1")
		val1 := []byte{0x1, 0x2, 0x3}

		err := meter1.MeterStorageRead(key1, val1, false /* not enforced */)
		require.NoError(t, err)
		require.Equal(t, meter1.TotalBytesReadFromStorage(), meter.GetStorageKeyValueSizeForTesting(key1, val1))
	})

	t.Run("metering storage read - exceeding limit - enforced", func(t *testing.T) {
		testLimit := uint64(1)
		meter1 := meter.NewMeter(
			meter.DefaultParameters().WithStorageInteractionLimit(testLimit),
		)

		key1 := flow.NewRegisterID(flow.EmptyAddress, "1")
		val1 := []byte{0x1, 0x2, 0x3}

		err := meter1.MeterStorageRead(key1, val1, true /* enforced */)

		ledgerInteractionLimitExceedError := errors.NewLedgerInteractionLimitExceededError(
			meter.GetStorageKeyValueSizeForTesting(key1, val1),
			testLimit,
		)
		require.ErrorAs(t, err, &ledgerInteractionLimitExceedError)
	})

	t.Run("metering storage written - exceeding limit - not enforced", func(t *testing.T) {
		testLimit := uint64(1)
		meter1 := meter.NewMeter(
			meter.DefaultParameters().WithStorageInteractionLimit(testLimit),
		)

		key1 := flow.NewRegisterID(flow.EmptyAddress, "1")
		val1 := []byte{0x1, 0x2, 0x3}

		err := meter1.MeterStorageWrite(key1, val1, false /* not enforced */)
		require.NoError(t, err)
	})

	t.Run("metering storage written - exceeding limit - enforced", func(t *testing.T) {
		testLimit := uint64(1)
		meter1 := meter.NewMeter(
			meter.DefaultParameters().WithStorageInteractionLimit(testLimit),
		)

		key1 := flow.NewRegisterID(flow.EmptyAddress, "1")
		val1 := []byte{0x1, 0x2, 0x3}

		err := meter1.MeterStorageWrite(key1, val1, true /* enforced */)

		ledgerInteractionLimitExceedError := errors.NewLedgerInteractionLimitExceededError(
			meter.GetStorageKeyValueSizeForTesting(key1, val1),
			testLimit,
		)
		require.ErrorAs(t, err, &ledgerInteractionLimitExceedError)
	})

	t.Run("metering storage read and written - within limit", func(t *testing.T) {
		meter1 := meter.NewMeter(
			meter.DefaultParameters(),
		)

		key1 := flow.NewRegisterID(flow.EmptyAddress, "1")
		key2 := flow.NewRegisterID(flow.EmptyAddress, "2")
		val1 := []byte{0x1, 0x2, 0x3}
		val2 := []byte{0x1, 0x2, 0x3, 0x4}
		size1 := meter.GetStorageKeyValueSizeForTesting(key1, val1)
		size2 := meter.GetStorageKeyValueSizeForTesting(key2, val2)

		// read of key1
		err := meter1.MeterStorageRead(key1, val1, false)
		require.NoError(t, err)
		require.Equal(t, meter1.TotalBytesReadFromStorage(), size1)
		require.Equal(t, meter1.TotalBytesOfStorageInteractions(), size1)

		// write of key2
		err = meter1.MeterStorageWrite(key2, val2, false)
		require.NoError(t, err)
		require.Equal(t, meter1.TotalBytesWrittenToStorage(), size2)
		require.Equal(t, meter1.TotalBytesOfStorageInteractions(), size1+size2)
	})

	t.Run("metering storage read and written - exceeding limit - not enforced", func(t *testing.T) {
		key1 := flow.NewRegisterID(flow.EmptyAddress, "1")
		key2 := flow.NewRegisterID(flow.EmptyAddress, "2")
		val1 := []byte{0x1, 0x2, 0x3}
		val2 := []byte{0x1, 0x2, 0x3, 0x4}
		size1 := meter.GetStorageKeyValueSizeForTesting(key1, val1)
		size2 := meter.GetStorageKeyValueSizeForTesting(key2, val2)

		meter1 := meter.NewMeter(
			meter.DefaultParameters().WithStorageInteractionLimit(size1 + size2 - 1),
		)

		// read of key1
		err := meter1.MeterStorageRead(key1, val1, false)
		require.NoError(t, err)
		require.Equal(t, meter1.TotalBytesReadFromStorage(), size1)
		require.Equal(t, meter1.TotalBytesOfStorageInteractions(), size1)

		// write of key2
		err = meter1.MeterStorageWrite(key2, val2, false)
		require.NoError(t, err)
		require.Equal(t, meter1.TotalBytesWrittenToStorage(), size2)
		require.Equal(t, meter1.TotalBytesOfStorageInteractions(), size1+size2)
	})

	t.Run("metering storage read and written - exceeding limit - enforced", func(t *testing.T) {
		key1 := flow.NewRegisterID(flow.EmptyAddress, "1")
		key2 := flow.NewRegisterID(flow.EmptyAddress, "2")
		val1 := []byte{0x1, 0x2, 0x3}
		val2 := []byte{0x1, 0x2, 0x3, 0x4}
		size1 := meter.GetStorageKeyValueSizeForTesting(key1, val1)
		size2 := meter.GetStorageKeyValueSizeForTesting(key2, val2)
		testLimit := size1 + size2 - 1
		meter1 := meter.NewMeter(
			meter.DefaultParameters().WithStorageInteractionLimit(testLimit),
		)

		// read of key1
		err := meter1.MeterStorageRead(key1, val1, true)
		require.NoError(t, err)
		require.Equal(t, meter1.TotalBytesReadFromStorage(), size1)
		require.Equal(t, meter1.TotalBytesOfStorageInteractions(), size1)

		// write of key2
		err = meter1.MeterStorageWrite(key2, val2, true)
		ledgerInteractionLimitExceedError := errors.NewLedgerInteractionLimitExceededError(
			size1+size2,
			testLimit,
		)
		require.ErrorAs(t, err, &ledgerInteractionLimitExceedError)
	})

	t.Run("merge storage metering", func(t *testing.T) {
		// meter 1
		meter1 := meter.NewMeter(
			meter.DefaultParameters(),
		)
		readKey1 := flow.NewRegisterID(flow.EmptyAddress, "r1")
		readVal1 := []byte{0x1, 0x2, 0x3}
		readSize1 := meter.GetStorageKeyValueSizeForTesting(readKey1, readVal1)
		err := meter1.MeterStorageRead(readKey1, readVal1, false)
		require.NoError(t, err)

		writeKey1 := flow.NewRegisterID(flow.EmptyAddress, "w1")
		writeVal1 := []byte{0x1, 0x2, 0x3, 0x4}
		writeSize1 := meter.GetStorageKeyValueSizeForTesting(writeKey1, writeVal1)
		err = meter1.MeterStorageWrite(writeKey1, writeVal1, false)
		require.NoError(t, err)

		// meter 2
		meter2 := meter.NewMeter(
			meter.DefaultParameters(),
		)

		writeKey2 := flow.NewRegisterID(flow.EmptyAddress, "w2")
		writeVal2 := []byte{0x1, 0x2, 0x3, 0x4, 0x5}
		writeSize2 := meter.GetStorageKeyValueSizeForTesting(writeKey2, writeVal2)

		err = meter1.MeterStorageRead(readKey1, readVal1, false)
		require.NoError(t, err)

		err = meter1.MeterStorageWrite(writeKey1, writeVal1, false)
		require.NoError(t, err)

		// read the same key value as meter1
		err = meter2.MeterStorageRead(readKey1, readVal1, false)
		require.NoError(t, err)

		err = meter2.MeterStorageWrite(writeKey2, writeVal2, false)
		require.NoError(t, err)

		// merge
		meter1.MergeMeter(meter2)

		require.Equal(t, meter1.TotalBytesOfStorageInteractions(), readSize1+writeSize1+writeSize2)
		require.Equal(t, meter1.TotalBytesReadFromStorage(), readSize1)
		require.Equal(t, meter1.TotalBytesWrittenToStorage(), writeSize1+writeSize2)

		reads, writes := meter1.GetStorageRWSizeMapForTesting()
		readKey1Val, ok := reads[readKey1]
		require.True(t, ok)
		require.Equal(t, readKey1Val, readSize1) // meter merge only takes child values for rw bookkeeping

		writeKey1Val, ok := writes[writeKey1]
		require.True(t, ok)
		require.Equal(t, writeKey1Val, writeSize1)

		writeKey2Val, ok := writes[writeKey2]
		require.True(t, ok)
		require.Equal(t, writeKey2Val, writeSize2)
	})
}

func TestEventLimits(t *testing.T) {
	t.Run("metering event emit - within limit", func(t *testing.T) {
		meter1 := meter.NewMeter(
			meter.DefaultParameters(),
		)

		testSize1, testSize2 := uint64(123), uint64(234)

		err := meter1.MeterEmittedEvent(testSize1)
		require.NoError(t, err)
		require.Equal(t, testSize1, meter1.TotalEmittedEventBytes())

		err = meter1.MeterEmittedEvent(testSize2)
		require.NoError(t, err)
		require.Equal(t, testSize1+testSize2, meter1.TotalEmittedEventBytes())
	})

	t.Run("metering event emit - exceeding limit", func(t *testing.T) {
		testSize1, testSize2 := uint64(123), uint64(234)
		testEventLimit := testSize1 + testSize2 - 1 // make it fail at 2nd meter
		meter1 := meter.NewMeter(
			meter.DefaultParameters().WithEventEmitByteLimit(testEventLimit),
		)

		err := meter1.MeterEmittedEvent(testSize1)
		require.NoError(t, err)
		require.Equal(t, testSize1, meter1.TotalEmittedEventBytes())

		err = meter1.MeterEmittedEvent(testSize2)
		eventLimitExceededError := errors.NewEventLimitExceededError(
			testSize1+testSize2,
			testEventLimit)
		require.ErrorAs(t, err, &eventLimitExceededError)
	})

	t.Run("merge event metering", func(t *testing.T) {
		// meter 1
		meter1 := meter.NewMeter(
			meter.DefaultParameters(),
		)
		testSize1 := uint64(123)
		err := meter1.MeterEmittedEvent(testSize1)
		require.NoError(t, err)

		// meter 2
		meter2 := meter.NewMeter(
			meter.DefaultParameters(),
		)
		testSize2 := uint64(234)
		err = meter2.MeterEmittedEvent(testSize2)
		require.NoError(t, err)

		// merge
		meter1.MergeMeter(meter2)
		require.Equal(t, testSize1+testSize2, meter1.TotalEmittedEventBytes())
	})
}
