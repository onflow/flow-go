package weighted_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/meter/weighted"
)

func TestWeightedComputationMetering(t *testing.T) {

	t.Run("get limits", func(t *testing.T) {
		m := weighted.NewMeter(
			1,
			2,
			weighted.WithComputationWeights(map[common.ComputationKind]uint64{}),
			weighted.WithMemoryWeights(map[common.MemoryKind]uint64{}))
		require.Equal(t, uint(1), m.TotalComputationLimit())
		require.Equal(t, uint(2), m.TotalMemoryLimit())
	})

	t.Run("get limits max", func(t *testing.T) {
		m := weighted.NewMeter(
			math.MaxUint32,
			math.MaxUint32,
			weighted.WithComputationWeights(map[common.ComputationKind]uint64{}),
			weighted.WithMemoryWeights(map[common.MemoryKind]uint64{}))
		require.Equal(t, uint(math.MaxUint32), m.TotalComputationLimit())
		require.Equal(t, uint(math.MaxUint32), m.TotalMemoryLimit())
	})

	t.Run("meter computation and memory", func(t *testing.T) {
		m := weighted.NewMeter(
			10,
			10,
			weighted.WithComputationWeights(map[common.ComputationKind]uint64{0: 1 << weighted.MeterInternalPrecisionBytes}),
			weighted.WithMemoryWeights(map[common.MemoryKind]uint64{0: 1 << weighted.MeterInternalPrecisionBytes}),
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

		err = m.MeterMemory(0, 2)
		require.NoError(t, err)
		require.Equal(t, uint(2), m.TotalMemoryUsed())

		err = m.MeterMemory(0, 3)
		require.NoError(t, err)
		require.Equal(t, uint(2+3), m.TotalMemoryUsed())

		err = m.MeterMemory(0, 8)
		require.Error(t, err)
		require.True(t, errors.IsMemoryLimitExceededError(err))
	})

	t.Run("meter computation and memory with weights", func(t *testing.T) {
		m := weighted.NewMeter(
			100,
			100,
			weighted.WithComputationWeights(map[common.ComputationKind]uint64{0: 13 << weighted.MeterInternalPrecisionBytes}),
			weighted.WithMemoryWeights(map[common.MemoryKind]uint64{0: 17 << weighted.MeterInternalPrecisionBytes}),
		)

		err := m.MeterComputation(0, 1)
		require.NoError(t, err)
		require.Equal(t, uint(13), m.TotalComputationUsed())
		require.Equal(t, uint(1), m.ComputationIntensities()[0])

		err = m.MeterMemory(0, 2)
		require.NoError(t, err)
		require.Equal(t, uint(34), m.TotalMemoryUsed())
		require.Equal(t, uint(2), m.MemoryIntensities()[0])
	})

	t.Run("meter computation and memory with weights lower than MeterInternalPrecisionBytes", func(t *testing.T) {
		m := weighted.NewMeter(
			100,
			100,
			weighted.WithComputationWeights(map[common.ComputationKind]uint64{0: 1}),
			weighted.WithMemoryWeights(map[common.MemoryKind]uint64{0: 1}),
		)

		internalPrecisionMinusOne := uint((1 << weighted.MeterInternalPrecisionBytes) - 1)

		err := m.MeterComputation(0, internalPrecisionMinusOne)
		require.NoError(t, err)
		require.Equal(t, uint(0), m.TotalComputationUsed())
		require.Equal(t, internalPrecisionMinusOne, m.ComputationIntensities()[0])

		err = m.MeterComputation(0, 1)
		require.NoError(t, err)
		require.Equal(t, uint(1), m.TotalComputationUsed())
		require.Equal(t, uint(1<<weighted.MeterInternalPrecisionBytes), m.ComputationIntensities()[0])

		err = m.MeterMemory(0, internalPrecisionMinusOne)
		require.NoError(t, err)
		require.Equal(t, uint(0), m.TotalMemoryUsed())
		require.Equal(t, internalPrecisionMinusOne, m.MemoryIntensities()[0])

		err = m.MeterMemory(0, 1)
		require.NoError(t, err)
		require.Equal(t, uint(1), m.TotalMemoryUsed())
		require.Equal(t, uint(1<<weighted.MeterInternalPrecisionBytes), m.MemoryIntensities()[0])
	})

	t.Run("merge meters", func(t *testing.T) {
		compKind := common.ComputationKind(0)
		m := weighted.NewMeter(
			9,
			0,
			weighted.WithComputationWeights(map[common.ComputationKind]uint64{0: 1 << weighted.MeterInternalPrecisionBytes}),
			weighted.WithMemoryWeights(map[common.MemoryKind]uint64{0: 1 << weighted.MeterInternalPrecisionBytes}),
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

		err = m.MergeMeter(child1)
		require.NoError(t, err)
		require.Equal(t, uint(1+2), m.TotalComputationUsed())
		require.Equal(t, uint(1+2), m.ComputationIntensities()[compKind])

		err = m.MergeMeter(child2)
		require.NoError(t, err)
		require.Equal(t, uint(1+2+3), m.TotalComputationUsed())
		require.Equal(t, uint(1+2+3), m.ComputationIntensities()[compKind])

		// error on merge (hitting limit)
		err = m.MergeMeter(child3)
		require.Error(t, err)
		require.True(t, errors.IsComputationLimitExceededError(err))
	})

	t.Run("merge meters - large values - computation", func(t *testing.T) {
		m := weighted.NewMeter(
			math.MaxUint32,
			math.MaxUint32,
			weighted.WithComputationWeights(map[common.ComputationKind]uint64{
				0: math.MaxUint32 << weighted.MeterInternalPrecisionBytes,
			}),
		)

		err := m.MeterComputation(0, 1)
		require.NoError(t, err)

		child1 := m.NewChild()
		err = child1.MeterComputation(0, 1)
		require.NoError(t, err)

		err = m.MergeMeter(child1)
		require.True(t, errors.IsComputationLimitExceededError(err))
	})

	t.Run("merge meters - large values - memory", func(t *testing.T) {
		m := weighted.NewMeter(
			math.MaxUint32,
			math.MaxUint32,
			weighted.WithMemoryWeights(map[common.MemoryKind]uint64{
				0: math.MaxUint32 << weighted.MeterInternalPrecisionBytes,
			}),
		)

		err := m.MeterMemory(0, 1)
		require.NoError(t, err)

		child1 := m.NewChild()
		err = child1.MeterMemory(0, 1)
		require.NoError(t, err)

		err = m.MergeMeter(child1)
		require.True(t, errors.IsMemoryLimitExceededError(err))
	})

	t.Run("add intensity - test limits - computation", func(t *testing.T) {
		var m *weighted.Meter
		reset := func() {
			m = weighted.NewMeter(
				math.MaxUint32,
				math.MaxUint32,
				weighted.WithComputationWeights(map[common.ComputationKind]uint64{
					0: 0,
					1: 1,
					2: 1 << weighted.MeterInternalPrecisionBytes,
					3: math.MaxUint64,
				}),
			)
		}

		reset()
		err := m.MeterComputation(0, 1)
		require.NoError(t, err)
		require.Equal(t, uint(0), m.TotalComputationUsed())
		reset()
		err = m.MeterComputation(0, 1<<weighted.MeterInternalPrecisionBytes)
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
		err = m.MeterComputation(1, 1<<weighted.MeterInternalPrecisionBytes)
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
		err = m.MeterComputation(2, 1<<weighted.MeterInternalPrecisionBytes)
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
		err = m.MeterComputation(3, 1<<weighted.MeterInternalPrecisionBytes)
		require.True(t, errors.IsComputationLimitExceededError(err))
		reset()
		err = m.MeterComputation(3, math.MaxUint32)
		require.True(t, errors.IsComputationLimitExceededError(err))
	})

	t.Run("add intensity - test limits - memory", func(t *testing.T) {
		var m *weighted.Meter
		reset := func() {
			m = weighted.NewMeter(
				math.MaxUint32,
				math.MaxUint32,
				weighted.WithMemoryWeights(map[common.MemoryKind]uint64{
					0: 0,
					1: 1,
					2: 1 << weighted.MeterInternalPrecisionBytes,
					3: math.MaxUint64,
				}),
			)
		}

		reset()
		err := m.MeterMemory(0, 1)
		require.NoError(t, err)
		require.Equal(t, uint(0), m.TotalMemoryUsed())
		reset()
		err = m.MeterMemory(0, 1<<weighted.MeterInternalPrecisionBytes)
		require.NoError(t, err)
		require.Equal(t, uint(0), m.TotalMemoryUsed())
		reset()
		err = m.MeterMemory(0, math.MaxUint32)
		require.NoError(t, err)
		require.Equal(t, uint(0), m.TotalMemoryUsed())

		reset()
		err = m.MeterMemory(1, 1)
		require.NoError(t, err)
		require.Equal(t, uint(0), m.TotalMemoryUsed())
		reset()
		err = m.MeterMemory(1, 1<<weighted.MeterInternalPrecisionBytes)
		require.NoError(t, err)
		require.Equal(t, uint(1), m.TotalMemoryUsed())
		reset()
		err = m.MeterMemory(1, math.MaxUint32)
		require.NoError(t, err)
		require.Equal(t, uint(1<<16-1), m.TotalMemoryUsed())

		reset()
		err = m.MeterMemory(2, 1)
		require.NoError(t, err)
		require.Equal(t, uint(1), m.TotalMemoryUsed())
		reset()
		err = m.MeterMemory(2, 1<<weighted.MeterInternalPrecisionBytes)
		require.NoError(t, err)
		require.Equal(t, uint(1<<16), m.TotalMemoryUsed())
		reset()
		err = m.MeterMemory(2, math.MaxUint32)
		require.NoError(t, err)
		require.Equal(t, uint(math.MaxUint32), m.TotalMemoryUsed())

		reset()
		err = m.MeterMemory(3, 1)
		require.True(t, errors.IsMemoryLimitExceededError(err))
		reset()
		err = m.MeterMemory(3, 1<<weighted.MeterInternalPrecisionBytes)
		require.True(t, errors.IsMemoryLimitExceededError(err))
		reset()
		err = m.MeterMemory(3, math.MaxUint32)
		require.True(t, errors.IsMemoryLimitExceededError(err))
	})
}
