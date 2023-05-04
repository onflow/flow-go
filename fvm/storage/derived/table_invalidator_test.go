package derived

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/storage/snapshot"
)

type testInvalidator struct {
	callCount      int
	invalidateAll  bool
	invalidateName string
}

func (invalidator testInvalidator) ShouldInvalidateEntries() bool {
	return invalidator.invalidateAll ||
		invalidator.invalidateName != ""
}

func (invalidator *testInvalidator) ShouldInvalidateEntry(
	key string,
	value *string,
	snapshot *snapshot.ExecutionSnapshot,
) bool {
	invalidator.callCount += 1
	return invalidator.invalidateAll ||
		invalidator.invalidateName == key
}

func TestChainedInvalidator(t *testing.T) {
	var chain chainedTableInvalidators[string, *string]
	require.False(t, chain.ShouldInvalidateEntries())
	require.False(t, chain.ShouldInvalidateEntry("", nil, nil))

	chain = chainedTableInvalidators[string, *string]{}
	require.False(t, chain.ShouldInvalidateEntries())
	require.False(t, chain.ShouldInvalidateEntry("", nil, nil))

	chain = chainedTableInvalidators[string, *string]{
		{
			TableInvalidator: &testInvalidator{},
			executionTime:    1,
		},
		{
			TableInvalidator: &testInvalidator{},
			executionTime:    2,
		},
		{
			TableInvalidator: &testInvalidator{},
			executionTime:    3,
		},
	}
	require.False(t, chain.ShouldInvalidateEntries())

	chain = chainedTableInvalidators[string, *string]{
		{
			TableInvalidator: &testInvalidator{invalidateName: "1"},
			executionTime:    1,
		},
		{
			TableInvalidator: &testInvalidator{invalidateName: "3a"},
			executionTime:    3,
		},
		{
			TableInvalidator: &testInvalidator{invalidateName: "3b"},
			executionTime:    3,
		},
		{
			TableInvalidator: &testInvalidator{invalidateName: "7"},
			executionTime:    7,
		},
	}
	require.True(t, chain.ShouldInvalidateEntries())

	for _, name := range []string{"1", "3a", "3b", "7"} {
		require.True(t, chain.ShouldInvalidateEntry(name, nil, nil))
	}

	for _, name := range []string{"0", "2", "3c", "4", "8"} {
		require.False(t, chain.ShouldInvalidateEntry(name, nil, nil))
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

	key := "1"
	require.True(t, chain.ShouldInvalidateEntry(key, nil, nil))
	require.False(
		t,
		chain.ApplicableInvalidators(3).ShouldInvalidateEntry(key, nil, nil))
}
