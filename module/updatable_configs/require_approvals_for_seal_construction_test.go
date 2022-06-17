package updatable_configs_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/defaults"
	"github.com/onflow/flow-go/module/updatable_configs"
)

func TestRequiredApprovalsForSealingContruction(t *testing.T) {

	instance, err := updatable_configs.NewRequiredApprovalsForSealConstructionInstance(1, 0, 3)
	require.NoError(t, err)

	// should get the default value
	val := instance.GetValue()
	require.Equal(t, uint(1), val)

	// SetValue should return the old value
	old, err := instance.SetValue(0)
	require.NoError(t, err)
	require.Equal(t, val, old)

	// value should be updated by SetValue
	newVal := instance.GetValue()
	require.Equal(t, uint(0), newVal)

	// test updating multiple times
	for i := 1; i <= defaults.DefaultChunkAssignmentAlpha; i++ {
		old, err := instance.SetValue(uint(i))
		require.NoError(t, err, err)
		require.Equal(t, uint(i-1), old)
		require.Equal(t, uint(i), instance.GetValue())
	}

	_, err = instance.SetValue(defaults.DefaultChunkAssignmentAlpha + 1)
	require.Error(t, err)
}
