package updatable_configs_test

import (
	"testing"

	"github.com/onflow/flow-go/module/chunks"
	"github.com/onflow/flow-go/module/updatable_configs"
	"github.com/stretchr/testify/require"
)

func TestRequiredApprovalsForSealingContruction(t *testing.T) {
	instance := updatable_configs.AcquireRequiredApprovalsForSealConstructionSetter()
	// a second copy of the same instance
	instance2 := updatable_configs.AcquireRequiredApprovalsForSealConstructionSetter()
	require.Equal(t, instance, instance2)

	// should get the default value
	val := instance.GetValue()
	require.Equal(t, updatable_configs.DefaultRequiredApprovalsForSealConstruction, val)

	// SetValue should return the old value
	old, err := instance.SetValue(0)
	require.NoError(t, err)
	require.Equal(t, val, old)

	// value should be updated by SetValue
	newVal := instance.GetValue()
	require.Equal(t, uint(0), newVal)

	// the second copy should get the updated value
	require.Equal(t, uint(0), instance2.GetValue())

	// a newly created instance should get the same value
	require.Equal(t, uint(0), updatable_configs.AcquireRequiredApprovalsForSealConstructionGetter().GetValue())

	// test updating 10 times
	for i := 1; i <= chunks.DefaultChunkAssignmentAlpha; i++ {
		old, err := instance.SetValue(uint(i))
		require.NoError(t, err, err)
		require.Equal(t, uint(i-1), old)
		require.Equal(t, uint(i), instance.GetValue())
		require.Equal(t, uint(i), updatable_configs.AcquireRequiredApprovalsForSealConstructionGetter().GetValue())
	}

	_, err = instance.SetValue(chunks.DefaultChunkAssignmentAlpha + 1)
	require.Error(t, err)
}

func TestRequiredApprovalsForSealingContructionConcurrent(t *testing.T) {
	// 1. Concurrently create 10 instances with 10 go routines
	// 2. Each go routine will set different values for 10 times
	// 3. Wait until all 10 go routines to finish
	// 4. Verify that all 10 instances should get the same value
}
