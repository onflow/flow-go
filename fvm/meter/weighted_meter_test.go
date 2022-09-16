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
		require.Equal(t, uint(1), m.TotalComputationUsed())

		err = m.MeterComputation(0, 2)
		require.NoError(t, err)
		require.Equal(t, uint(1+2), m.TotalComputationUsed())

		err = m.MeterComputation(0, 8)
		require.Error(t, err)
		require.True(t, errors.IsComputationLimitExceededError(err))
		require.Equal(t, err.(errors.ComputationLimitExceededError).Error(), errors.NewComputationLimitExceededError(10).Error())

		err = m.MeterMemory(0, 2)
		require.NoError(t, err)
		require.Equal(t, uint64(2), m.TotalMemoryEstimate())

		err = m.MeterMemory(0, 3)
		require.NoError(t, err)
		require.Equal(t, uint64(2+3), m.TotalMemoryEstimate())

		err = m.MeterMemory(0, 8)
		require.Error(t, err)
		require.True(t, errors.IsMemoryLimitExceededError(err))
		require.Equal(t, err.(errors.MemoryLimitExceededError).Error(), errors.NewMemoryLimitExceededError(10).Error())
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
		require.Equal(t, uint(13), m.TotalComputationUsed())
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
		require.Equal(t, uint(0), m.TotalComputationUsed())
		require.Equal(t, internalPrecisionMinusOne, m.ComputationIntensities()[0])

		err = m.MeterComputation(0, 1)
		require.NoError(t, err)
		require.Equal(t, uint(1), m.TotalComputationUsed())
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

		child1 := m.NewChild()
		err = child1.MeterComputation(compKind, 2)
		require.NoError(t, err)

		child2 := m.NewChild()
		err = child2.MeterComputation(compKind, 3)
		require.NoError(t, err)

		child3 := m.NewChild()
		err = child3.MeterComputation(compKind, 4)
		require.NoError(t, err)

		m.MergeMeter(child1)
		require.Equal(t, uint(1+2), m.TotalComputationUsed())
		require.Equal(t, uint(1+2), m.ComputationIntensities()[compKind])

		m.MergeMeter(child2)
		require.Equal(t, uint(1+2+3), m.TotalComputationUsed())
		require.Equal(t, uint(1+2+3), m.ComputationIntensities()[compKind])

		// merge hits limit, but is accepted.
		m.MergeMeter(child3)
		require.Equal(t, uint(1+2+3+4), m.TotalComputationUsed())
		require.Equal(t, uint(1+2+3+4), m.ComputationIntensities()[compKind])

		// error after merge (hitting limit)
		err = m.MeterComputation(compKind, 0)
		require.Error(t, err)
		require.True(t, errors.IsComputationLimitExceededError(err))
		require.Equal(t, err.(errors.ComputationLimitExceededError).Error(), errors.NewComputationLimitExceededError(9).Error())
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

		child := m.NewChild()
		err = child.MeterComputation(compKind, 1)
		require.NoError(t, err)

		// hitting limit and ignoring it
		m.MergeMeter(child)
		require.Equal(t, uint(1+1), m.TotalComputationUsed())
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

		child1 := m.NewChild()
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

		child1 := m.NewChild()
		err = child1.MeterMemory(0, 1)
		require.NoError(t, err)

		m.MergeMeter(child1)

		err = m.MeterMemory(0, 0)
		require.Error(t, err)
		require.True(t, errors.IsMemoryLimitExceededError(err))
		require.Equal(t, err.(errors.MemoryLimitExceededError).Error(), errors.NewMemoryLimitExceededError(math.MaxUint32).Error())
	})

	t.Run("add intensity - test limits - computation", func(t *testing.T) {
		var m meter.Meter
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
		require.Equal(t, uint(0), m.TotalComputationUsed())
		reset()
		err = m.MeterComputation(0, 1<<meter.MeterExecutionInternalPrecisionBytes)
		require.NoError(t, err)
		require.Equal(t, uint(0), m.TotalComputationUsed())
		reset()
		err = m.MeterComputation(0, math.MaxUint32)
		require.NoError(t, err)
		require.Equal(t, uint(0), m.TotalComputationUsed())

		reset()
		err = m.MeterComputation(1, 1)
		require.NoError(t, err)
		require.Equal(t, uint(0), m.TotalComputationUsed())
		reset()
		err = m.MeterComputation(1, 1<<meter.MeterExecutionInternalPrecisionBytes)
		require.NoError(t, err)
		require.Equal(t, uint(1), m.TotalComputationUsed())
		reset()
		err = m.MeterComputation(1, math.MaxUint32)
		require.NoError(t, err)
		require.Equal(t, uint(1<<16-1), m.TotalComputationUsed())

		reset()
		err = m.MeterComputation(2, 1)
		require.NoError(t, err)
		require.Equal(t, uint(1), m.TotalComputationUsed())
		reset()
		err = m.MeterComputation(2, 1<<meter.MeterExecutionInternalPrecisionBytes)
		require.NoError(t, err)
		require.Equal(t, uint(1<<16), m.TotalComputationUsed())
		reset()
		err = m.MeterComputation(2, math.MaxUint32)
		require.NoError(t, err)
		require.Equal(t, uint(math.MaxUint32), m.TotalComputationUsed())

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
		var m meter.Meter
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
		assert.True(t, ok, fmt.Sprintf("missing weight for memory kind '%s'", kind.String()))
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

func TestMeterParameters(t *testing.T) {
	t.Run("IsMemoryLimitSet return false - without WithMemoryLimit() called", func(t *testing.T) {
		meterParams := meter.DefaultParameters()
		require.False(t, meterParams.IsMemoryLimitSet())
		require.Equal(t, uint64(math.MaxUint64), meterParams.TotalMemoryLimit())
	})

	t.Run("IsMemoryLimitSet return true - with WithMemoryLimit() called", func(t *testing.T) {
		testMemoryLimit := uint64(123)
		meterParams := meter.DefaultParameters().WithMemoryLimit(testMemoryLimit)
		require.True(t, meterParams.IsMemoryLimitSet())
		require.Equal(t, testMemoryLimit, meterParams.TotalMemoryLimit())
	})
}
