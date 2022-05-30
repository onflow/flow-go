package singleton

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequiredApprovalsForSealingContruction(t *testing.T) {
	instance := AcquireRequiredApprovalsForSealConstruction()
	// a second copy of the same instance
	instance2 := AcquireRequiredApprovalsForSealConstruction()
	require.Equal(t, instance, instance2)

	// should get the default value
	val := instance.GetValue()
	require.Equal(t, DefaultRequiredApprovalsForSealConstruction, val)

	// SetValue should return the old value
	old := instance.SetValue(0)
	require.Equal(t, val, old)

	// value should be updated by SetValue
	newVal := instance.GetValue()
	require.Equal(t, 0, newVal)

	// the second copy should get the updated value
	require.Equal(t, 0, instance2.GetValue())

	// a newly created instance should get the same value
	require.Equal(t, 0, AcquireRequiredApprovalsForSealConstruction().GetValue())

	// test updating 10 times
	for i := 1; i <= 10; i++ {
		old := instance.SetValue(uint(i))
		require.Equal(t, i-1, old)
		require.Equal(t, i, instance.GetValue())
	}
}

func TestRequiredApprovalsForSealingContructionConcurrent(t *testing.T) {
	// 1. Concurrently create 10 instances with 10 go routines
	// 2. Each go routine will set different values for 10 times
	// 3. Wait until all 10 go routines to finish
	// 4. Verify that all 10 instances should get the same value
}
