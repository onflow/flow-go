package updatable_configs_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/updatable_configs"
)

func TestRequiredApprovalsForSealingContruction(t *testing.T) {

	instance, err := updatable_configs.NewSealingConfigs(
		flow.DefaultRequiredApprovalsForSealConstruction,
		flow.DefaultRequiredApprovalsForSealValidation,
		flow.DefaultChunkAssignmentAlpha,
		flow.DefaultEmergencySealingActive,
	)
	require.NoError(t, err)

	// should get the default value
	val := instance.RequireApprovalsForSealConstructionDynamicValue()
	require.Equal(t, uint(1), val)

	// SetValue should return the old value
	old, err := instance.SetRequiredApprovalsForSealingConstruction(0)
	require.NoError(t, err)
	require.Equal(t, val, old)

	// value should be updated by SetRequiredApprovalsForSealingConstruction
	newVal := instance.RequireApprovalsForSealConstructionDynamicValue()
	require.Equal(t, uint(0), newVal)

	// test updating multiple times
	for i := 1; i <= flow.DefaultChunkAssignmentAlpha; i++ {
		old, err := instance.SetRequiredApprovalsForSealingConstruction(uint(i))
		require.NoError(t, err, err)
		require.Equal(t, uint(i-1), old)
		require.Equal(t, uint(i), instance.RequireApprovalsForSealConstructionDynamicValue())
	}

	_, err = instance.SetRequiredApprovalsForSealingConstruction(flow.DefaultChunkAssignmentAlpha + 1)
	require.Error(t, err)
}
