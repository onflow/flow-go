package programs

import (
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/state"
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
	location common.AddressLocation,
	program *interpreter.Program,
	state *state.State,
) bool {
	return invalidator.invalidateAll ||
		invalidator.invalidateName == location.Name
}

func TestModifiedSetsInvalidator(t *testing.T) {
	invalidator := ModifiedSetsInvalidator{}

	require.False(t, invalidator.ShouldInvalidateEntries())
	require.False(t, invalidator.ShouldInvalidateEntry(
		common.AddressLocation{},
		nil,
		nil))

	invalidator = ModifiedSetsInvalidator{
		ContractUpdateKeys: []ContractUpdateKey{
			{}, // For now, the entry's value does not matter.
		},
		FrozenAccounts: nil,
	}

	require.True(t, invalidator.ShouldInvalidateEntries())
	require.True(t, invalidator.ShouldInvalidateEntry(
		common.AddressLocation{},
		nil,
		nil))

	invalidator = ModifiedSetsInvalidator{
		ContractUpdateKeys: nil,
		FrozenAccounts: []common.Address{
			{}, // For now, the entry's value does not matter
		},
	}

	require.True(t, invalidator.ShouldInvalidateEntries())
	require.True(t, invalidator.ShouldInvalidateEntry(
		common.AddressLocation{},
		nil,
		nil))
}

func TestChainedInvalidator(t *testing.T) {
	var chain chainedDerivedDataInvalidators[
		common.AddressLocation,
		*interpreter.Program,
	]
	require.False(t, chain.ShouldInvalidateEntries())
	require.False(t, chain.ShouldInvalidateEntry(
		common.AddressLocation{},
		nil,
		nil))

	chain = chainedDerivedDataInvalidators[
		common.AddressLocation,
		*interpreter.Program,
	]{}
	require.False(t, chain.ShouldInvalidateEntries())
	require.False(t, chain.ShouldInvalidateEntry(
		common.AddressLocation{},
		nil,
		nil))

	chain = chainedDerivedDataInvalidators[
		common.AddressLocation,
		*interpreter.Program,
	]{
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

	chain = chainedDerivedDataInvalidators[
		common.AddressLocation,
		*interpreter.Program,
	]{
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
		require.True(
			t,
			chain.ShouldInvalidateEntry(
				common.AddressLocation{
					Name: name,
				},
				nil,
				nil))
	}

	for _, name := range []string{"0", "2", "3c", "4", "8"} {
		require.False(
			t,
			chain.ShouldInvalidateEntry(
				common.AddressLocation{
					Name: name,
				},
				nil,
				nil))
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

	location := common.AddressLocation{
		Name: "1",
	}
	require.True(t, chain.ShouldInvalidateEntry(location, nil, nil))
	require.False(
		t,
		chain.ApplicableInvalidators(3).ShouldInvalidateEntry(location, nil, nil))
}
