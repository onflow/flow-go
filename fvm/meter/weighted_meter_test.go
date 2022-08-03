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
			1,
			2,
			meter.WithComputationWeights(map[common.ComputationKind]uint64{}),
			meter.WithMemoryWeights(map[common.MemoryKind]uint64{}))
		require.Equal(t, uint(1), m.TotalEnforcedComputationLimit())
		require.Equal(t, uint(2), m.TotalEnforcedMemoryLimit())
	})

	t.Run("get limits max", func(t *testing.T) {
		m := meter.NewMeter(
			math.MaxUint32,
			math.MaxUint32,
			meter.WithComputationWeights(map[common.ComputationKind]uint64{}),
			meter.WithMemoryWeights(map[common.MemoryKind]uint64{}))
		require.Equal(t, uint(math.MaxUint32), m.TotalEnforcedComputationLimit())
		require.Equal(t, uint(math.MaxUint32), m.TotalEnforcedMemoryLimit())
	})

	t.Run("meter computation and memory", func(t *testing.T) {
		m := meter.NewMeter(
			10,
			10,
			meter.WithComputationWeights(map[common.ComputationKind]uint64{0: 1 << meter.MeterExecutionInternalPrecisionBytes}),
			meter.WithMemoryWeights(map[common.MemoryKind]uint64{0: 1}),
		)

		err := m.MeterComputation(0, 1)
		require.NoError(t, err)
		require.Equal(t, uint(1), m.TotalObservedComputationUsed())
		require.Equal(t, uint(1), m.TotalEnforcedComputationUsed())

		err = m.MeterComputation(0, 2)
		require.NoError(t, err)
		require.Equal(t, uint(1+2), m.TotalObservedComputationUsed())
		require.Equal(t, uint(1+2), m.TotalEnforcedComputationUsed())

		err = m.MeterComputation(0, 8)
		require.Error(t, err)
		require.True(t, errors.IsComputationLimitExceededError(err))
		require.Equal(t, err.(*errors.ComputationLimitExceededError).Error(), errors.NewComputationLimitExceededError(10).Error())

		err = m.MeterMemory(0, 2)
		require.NoError(t, err)
		require.Equal(t, uint(2), m.TotalObservedMemoryEstimate())
		require.Equal(t, uint(2), m.TotalEnforcedMemoryEstimate())

		err = m.MeterMemory(0, 3)
		require.NoError(t, err)
		require.Equal(t, uint(2+3), m.TotalObservedMemoryEstimate())
		require.Equal(t, uint(2+3), m.TotalEnforcedMemoryEstimate())

		err = m.MeterMemory(0, 8)
		require.Error(t, err)
		require.True(t, errors.IsMemoryLimitExceededError(err))
		require.Equal(t, err.(*errors.MemoryLimitExceededError).Error(), errors.NewMemoryLimitExceededError(10).Error())
	})

	t.Run("meter computation and memory with weights", func(t *testing.T) {
		m := meter.NewMeter(
			100,
			100,
			meter.WithComputationWeights(map[common.ComputationKind]uint64{0: 13 << meter.MeterExecutionInternalPrecisionBytes}),
			meter.WithMemoryWeights(map[common.MemoryKind]uint64{0: 17}),
		)

		err := m.MeterComputation(0, 1)
		require.NoError(t, err)
		require.Equal(t, uint(13), m.TotalObservedComputationUsed())
		require.Equal(t, uint(13), m.TotalEnforcedComputationUsed())
		require.Equal(t, uint(1), m.ObservedComputationIntensities()[0])
		require.Equal(t, uint(1), m.EnforcedComputationIntensities()[0])

		err = m.MeterMemory(0, 2)
		require.NoError(t, err)
		require.Equal(t, uint(34), m.TotalObservedMemoryEstimate())
		require.Equal(t, uint(34), m.TotalEnforcedMemoryEstimate())
		require.Equal(t, uint(2), m.ObservedMemoryIntensities()[0])
		require.Equal(t, uint(2), m.EnforcedMemoryIntensities()[0])
	})

	t.Run("meter computation with weights lower than MeterInternalPrecisionBytes", func(t *testing.T) {
		m := meter.NewMeter(
			100,
			100,
			meter.WithComputationWeights(map[common.ComputationKind]uint64{0: 1}),
			meter.WithMemoryWeights(map[common.MemoryKind]uint64{0: 1}),
		)

		internalPrecisionMinusOne := uint((1 << meter.MeterExecutionInternalPrecisionBytes) - 1)

		err := m.MeterComputation(0, internalPrecisionMinusOne)
		require.NoError(t, err)
		require.Equal(t, uint(0), m.TotalObservedComputationUsed())
		require.Equal(t, uint(0), m.TotalEnforcedComputationUsed())
		require.Equal(t, internalPrecisionMinusOne, m.ObservedComputationIntensities()[0])
		require.Equal(t, internalPrecisionMinusOne, m.EnforcedComputationIntensities()[0])

		err = m.MeterComputation(0, 1)
		require.NoError(t, err)
		require.Equal(t, uint(1), m.TotalObservedComputationUsed())
		require.Equal(t, uint(1), m.TotalEnforcedComputationUsed())
		require.Equal(t, uint(1<<meter.MeterExecutionInternalPrecisionBytes), m.ObservedComputationIntensities()[0])
		require.Equal(t, uint(1<<meter.MeterExecutionInternalPrecisionBytes), m.EnforcedComputationIntensities()[0])
	})

	t.Run("merge meters", func(t *testing.T) {
		compKind := common.ComputationKind(0)
		m := meter.NewMeter(
			9,
			0,
			meter.WithComputationWeights(map[common.ComputationKind]uint64{0: 1 << meter.MeterExecutionInternalPrecisionBytes}),
			meter.WithMemoryWeights(map[common.MemoryKind]uint64{0: 1}),
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

		err = m.MergeMeter(child1, true)
		require.NoError(t, err)
		require.Equal(t, uint(1+2), m.TotalObservedComputationUsed())
		require.Equal(t, uint(1+2), m.TotalEnforcedComputationUsed())
		require.Equal(t, uint(1+2), m.ObservedComputationIntensities()[compKind])
		require.Equal(t, uint(1+2), m.EnforcedComputationIntensities()[compKind])

		err = m.MergeMeter(child2, true)
		require.NoError(t, err)
		require.Equal(t, uint(1+2+3), m.TotalObservedComputationUsed())
		require.Equal(t, uint(1+2+3), m.TotalEnforcedComputationUsed())
		require.Equal(t, uint(1+2+3), m.ObservedComputationIntensities()[compKind])
		require.Equal(t, uint(1+2+3), m.EnforcedComputationIntensities()[compKind])

		// error on merge (hitting limit)
		err = m.MergeMeter(child3, true)
		require.Error(t, err)
		require.True(t, errors.IsComputationLimitExceededError(err))
		require.Equal(t, err.(*errors.ComputationLimitExceededError).Error(), errors.NewComputationLimitExceededError(9).Error())
	})

	t.Run("merge meters - ignore limits", func(t *testing.T) {
		compKind := common.ComputationKind(0)
		m := meter.NewMeter(
			9,
			0,
			meter.WithComputationWeights(map[common.ComputationKind]uint64{0: 1 << meter.MeterExecutionInternalPrecisionBytes}),
		)

		err := m.MeterComputation(compKind, 1)
		require.NoError(t, err)

		child := m.NewChild()
		err = child.MeterComputation(compKind, 1)
		require.NoError(t, err)

		// hitting limit and ignoring it
		err = m.MergeMeter(child, false)
		require.NoError(t, err)
		require.Equal(t, uint(1+1), m.TotalObservedComputationUsed())
		require.Equal(t, uint(1+1), m.TotalEnforcedComputationUsed())
		require.Equal(t, uint(1+1), m.ObservedComputationIntensities()[compKind])
		require.Equal(t, uint(1+1), m.EnforcedComputationIntensities()[compKind])
	})

	t.Run("merge meters - large values - computation", func(t *testing.T) {
		m := meter.NewMeter(
			math.MaxUint32,
			math.MaxUint32,
			meter.WithComputationWeights(map[common.ComputationKind]uint64{
				0: math.MaxUint32 << meter.MeterExecutionInternalPrecisionBytes,
			}),
		)

		err := m.MeterComputation(0, 1)
		require.NoError(t, err)

		child1 := m.NewChild()
		err = child1.MeterComputation(0, 1)
		require.NoError(t, err)

		err = m.MergeMeter(child1, true)
		require.True(t, errors.IsComputationLimitExceededError(err))
	})

	t.Run("merge meters - large values - memory", func(t *testing.T) {
		m := meter.NewMeter(
			math.MaxUint32,
			math.MaxUint32,
			meter.WithMemoryWeights(map[common.MemoryKind]uint64{
				0: math.MaxUint32,
			}),
		)

		err := m.MeterMemory(0, 1)
		require.NoError(t, err)

		child1 := m.NewChild()
		err = child1.MeterMemory(0, 1)
		require.NoError(t, err)

		err = m.MergeMeter(child1, true)

		require.Error(t, err)
		require.True(t, errors.IsMemoryLimitExceededError(err))
		require.Equal(t, err.(*errors.MemoryLimitExceededError).Error(), errors.NewMemoryLimitExceededError(math.MaxUint32).Error())
	})

	t.Run("add intensity - test limits - computation", func(t *testing.T) {
		var m meter.Meter
		reset := func() {
			m = meter.NewMeter(
				math.MaxUint32,
				math.MaxUint32,
				meter.WithComputationWeights(map[common.ComputationKind]uint64{
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
		require.Equal(t, uint(0), m.TotalObservedComputationUsed())
		require.Equal(t, uint(0), m.TotalEnforcedComputationUsed())
		reset()
		err = m.MeterComputation(0, 1<<meter.MeterExecutionInternalPrecisionBytes)
		require.NoError(t, err)
		require.Equal(t, uint(0), m.TotalObservedComputationUsed())
		require.Equal(t, uint(0), m.TotalEnforcedComputationUsed())
		reset()
		err = m.MeterComputation(0, math.MaxUint32)
		require.NoError(t, err)
		require.Equal(t, uint(0), m.TotalObservedComputationUsed())
		require.Equal(t, uint(0), m.TotalEnforcedComputationUsed())

		reset()
		err = m.MeterComputation(1, 1)
		require.NoError(t, err)
		require.Equal(t, uint(0), m.TotalObservedComputationUsed())
		require.Equal(t, uint(0), m.TotalEnforcedComputationUsed())
		reset()
		err = m.MeterComputation(1, 1<<meter.MeterExecutionInternalPrecisionBytes)
		require.NoError(t, err)
		require.Equal(t, uint(1), m.TotalObservedComputationUsed())
		require.Equal(t, uint(1), m.TotalEnforcedComputationUsed())
		reset()
		err = m.MeterComputation(1, math.MaxUint32)
		require.NoError(t, err)
		require.Equal(t, uint(1<<16-1), m.TotalObservedComputationUsed())
		require.Equal(t, uint(1<<16-1), m.TotalEnforcedComputationUsed())

		reset()
		err = m.MeterComputation(2, 1)
		require.NoError(t, err)
		require.Equal(t, uint(1), m.TotalObservedComputationUsed())
		require.Equal(t, uint(1), m.TotalEnforcedComputationUsed())
		reset()
		err = m.MeterComputation(2, 1<<meter.MeterExecutionInternalPrecisionBytes)
		require.NoError(t, err)
		require.Equal(t, uint(1<<16), m.TotalObservedComputationUsed())
		require.Equal(t, uint(1<<16), m.TotalEnforcedComputationUsed())
		reset()
		err = m.MeterComputation(2, math.MaxUint32)
		require.NoError(t, err)
		require.Equal(t, uint(math.MaxUint32), m.TotalObservedComputationUsed())
		require.Equal(t, uint(math.MaxUint32), m.TotalEnforcedComputationUsed())

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
				math.MaxUint32,
				math.MaxUint32,
				meter.WithMemoryWeights(map[common.MemoryKind]uint64{
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
		require.Equal(t, uint(0), m.TotalObservedMemoryEstimate())
		require.Equal(t, uint(0), m.TotalEnforcedMemoryEstimate())
		reset()
		err = m.MeterMemory(0, 1)
		require.NoError(t, err)
		require.Equal(t, uint(0), m.TotalObservedMemoryEstimate())
		require.Equal(t, uint(0), m.TotalEnforcedMemoryEstimate())
		reset()
		err = m.MeterMemory(0, math.MaxUint32)
		require.NoError(t, err)
		require.Equal(t, uint(0), m.TotalObservedMemoryEstimate())
		require.Equal(t, uint(0), m.TotalEnforcedMemoryEstimate())

		reset()
		err = m.MeterMemory(1, 1)
		require.NoError(t, err)
		require.Equal(t, uint(1), m.TotalObservedMemoryEstimate())
		require.Equal(t, uint(1), m.TotalEnforcedMemoryEstimate())
		reset()
		err = m.MeterMemory(1, 1)
		require.NoError(t, err)
		require.Equal(t, uint(1), m.TotalObservedMemoryEstimate())
		require.Equal(t, uint(1), m.TotalEnforcedMemoryEstimate())
		reset()
		err = m.MeterMemory(1, math.MaxUint32)
		require.NoError(t, err)
		require.Equal(t, uint(math.MaxUint32), m.TotalObservedMemoryEstimate())
		require.Equal(t, uint(math.MaxUint32), m.TotalEnforcedMemoryEstimate())

		reset()
		err = m.MeterMemory(2, 1)
		require.NoError(t, err)
		require.Equal(t, uint(2), m.TotalObservedMemoryEstimate())
		require.Equal(t, uint(2), m.TotalEnforcedMemoryEstimate())
		reset()
		err = m.MeterMemory(2, 1)
		require.NoError(t, err)
		require.Equal(t, uint(2), m.TotalObservedMemoryEstimate())
		require.Equal(t, uint(2), m.TotalEnforcedMemoryEstimate())
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
