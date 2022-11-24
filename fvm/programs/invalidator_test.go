package programs

import (
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/require"
)

type testInvalidator struct {
	invalidateAll  bool
	invalidateName string
}

func (invalidator testInvalidator) ShouldInvalidateEntries() bool {
	return invalidator.invalidateAll ||
		invalidator.invalidateName != ""
}

func (invalidator testInvalidator) ShouldInvalidateEntry(
	entry ProgramEntry,
) bool {
	return invalidator.invalidateAll ||
		invalidator.invalidateName == entry.Location.Name
}

func TestModifiedSetsInvalidator(t *testing.T) {
	invalidator := ModifiedSetsInvalidator{}

	require.False(t, invalidator.ShouldInvalidateEntries())
	require.False(t, invalidator.ShouldInvalidateEntry(ProgramEntry{}))

	invalidator = ModifiedSetsInvalidator{
		ContractUpdateKeys: []ContractUpdateKey{
			{}, // For now, the entry's value does not matter.
		},
		FrozenAccounts: nil,
	}

	require.True(t, invalidator.ShouldInvalidateEntries())
	require.True(t, invalidator.ShouldInvalidateEntry(ProgramEntry{}))

	invalidator = ModifiedSetsInvalidator{
		ContractUpdateKeys: nil,
		FrozenAccounts: []common.Address{
			{}, // For now, the entry's value does not matter
		},
	}

	require.True(t, invalidator.ShouldInvalidateEntries())
	require.True(t, invalidator.ShouldInvalidateEntry(ProgramEntry{}))
}

func TestChainedInvalidator(t *testing.T) {
	var chain chainedDerivedDataInvalidators[ProgramEntry]
	require.False(t, chain.ShouldInvalidateEntries())
	require.False(t, chain.ShouldInvalidateEntry(ProgramEntry{}))

	chain = chainedDerivedDataInvalidators[ProgramEntry]{}
	require.False(t, chain.ShouldInvalidateEntries())
	require.False(t, chain.ShouldInvalidateEntry(ProgramEntry{}))

	chain = chainedDerivedDataInvalidators[ProgramEntry]{
		{
			DerivedDataInvalidator: testInvalidator{},
			executionTime:          1,
		},
		{
			DerivedDataInvalidator: testInvalidator{},
			executionTime:          2,
		},
		{
			DerivedDataInvalidator: testInvalidator{},
			executionTime:          3,
		},
	}
	require.False(t, chain.ShouldInvalidateEntries())

	chain = chainedDerivedDataInvalidators[ProgramEntry]{
		{
			DerivedDataInvalidator: testInvalidator{invalidateName: "1"},
			executionTime:          1,
		},
		{
			DerivedDataInvalidator: testInvalidator{invalidateName: "3a"},
			executionTime:          3,
		},
		{
			DerivedDataInvalidator: testInvalidator{invalidateName: "3b"},
			executionTime:          3,
		},
		{
			DerivedDataInvalidator: testInvalidator{invalidateName: "7"},
			executionTime:          7,
		},
	}
	require.True(t, chain.ShouldInvalidateEntries())

	for _, name := range []string{"1", "3a", "3b", "7"} {
		entry := ProgramEntry{
			Location: common.AddressLocation{
				Name: name,
			},
		}

		require.True(t, chain.ShouldInvalidateEntry(entry))
	}

	for _, name := range []string{"0", "2", "3c", "4", "8"} {
		entry := ProgramEntry{
			Location: common.AddressLocation{
				Name: name,
			},
		}

		require.False(t, chain.ShouldInvalidateEntry(entry))
	}

	require.Equal(t, chain, chain.ApplicableInvalidators(0))
	require.Equal(t, chain, chain.ApplicableInvalidators(1))
	require.Equal(t, chain[1:], chain.ApplicableInvalidators(2))
	require.Equal(t, chain[1:], chain.ApplicableInvalidators(3))
	require.Equal(t, chain[3:], chain.ApplicableInvalidators(4))
	require.Equal(t, chain[3:], chain.ApplicableInvalidators(5))
	require.Equal(t, chain[3:], chain.ApplicableInvalidators(6))
	require.Equal(t, chain[3:], chain.ApplicableInvalidators(7))
	require.Nil(t, chain.ApplicableInvalidators(8))

	entry := ProgramEntry{
		Location: common.AddressLocation{
			Name: "1",
		},
	}
	require.True(t, chain.ShouldInvalidateEntry(entry))
	require.False(t, chain.ApplicableInvalidators(3).ShouldInvalidateEntry(entry))
}
