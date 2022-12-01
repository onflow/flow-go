package environment

import (
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/require"
)

func TestDerivedDataProgramInvalidator(t *testing.T) {
	invalidator := DerivedDataInvalidator{}.ProgramInvalidator()

	require.False(t, invalidator.ShouldInvalidateEntries())
	require.False(t, invalidator.ShouldInvalidateEntry(
		common.AddressLocation{},
		nil,
		nil))

	invalidator = DerivedDataInvalidator{
		ContractUpdateKeys: []ContractUpdateKey{
			{}, // For now, the entry's value does not matter.
		},
		FrozenAccounts: nil,
	}.ProgramInvalidator()

	require.True(t, invalidator.ShouldInvalidateEntries())
	require.True(t, invalidator.ShouldInvalidateEntry(
		common.AddressLocation{},
		nil,
		nil))

	invalidator = DerivedDataInvalidator{
		ContractUpdateKeys: nil,
		FrozenAccounts: []common.Address{
			{}, // For now, the entry's value does not matter
		},
	}.ProgramInvalidator()

	require.True(t, invalidator.ShouldInvalidateEntries())
	require.True(t, invalidator.ShouldInvalidateEntry(
		common.AddressLocation{},
		nil,
		nil))
}
