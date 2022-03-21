package basic_test

import (
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/meter/basic"
)

func TestComputationMetering(t *testing.T) {

	t.Run("get limits", func(t *testing.T) {
		m := basic.NewMeter(1, 2)
		require.Equal(t, uint(1), m.TotalComputationLimit())
		require.Equal(t, uint(2), m.TotalMemoryLimit())
	})

	t.Run("meter computation and memory", func(t *testing.T) {
		m := basic.NewMeter(10, 10)

		err := m.MeterComputation(uint(common.ComputationKindStatement), 1)
		require.NoError(t, err)
		require.Equal(t, uint(1), m.TotalComputationUsed())

		err = m.MeterComputation(uint(common.ComputationKindStatement), 2)
		require.NoError(t, err)
		require.Equal(t, uint(1+2), m.TotalComputationUsed())

		err = m.MeterComputation(uint(common.ComputationKindFunctionInvocation), 8)
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

	t.Run("merge meters", func(t *testing.T) {
		compKind := uint(common.ComputationKindStatement)
		m := basic.NewMeter(9, 0)

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

		err = m.MergeMeter(child2)
		require.NoError(t, err)
		require.Equal(t, uint(1+2+3), m.TotalComputationUsed())

		// error on merge (hitting limit)
		err = m.MergeMeter(child3)
		require.Error(t, err)
		require.True(t, errors.IsComputationLimitExceededError(err))
	})
}
