package slices_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/slices"
)

// TestSliceContainsElement tests that the StringSliceContainsElement function returns true if the string slice contains the element.
func TestSliceContainsElement(t *testing.T) {
	a := []string{"a", "b", "c"}

	require.True(t, slices.StringSliceContainsElement(a, "a"))
	require.True(t, slices.StringSliceContainsElement(a, "b"))
	require.True(t, slices.StringSliceContainsElement(a, "c"))
	require.False(t, slices.StringSliceContainsElement(a, "d"))
}

// TestAreStringSlicesEqual tests that the AreStringSlicesEqual function returns true if the string slices are equal.
func TestAreStringSlicesEqual(t *testing.T) {
	a := []string{"a", "b", "c"}
	b := []string{"a", "b", "c"}
	c := []string{"a", "b", "c", "d"}
	d := []string{"a", "b", "d"}

	require.True(t, slices.AreStringSlicesEqual(a, b))
	require.False(t, slices.AreStringSlicesEqual(a, c))
	require.False(t, slices.AreStringSlicesEqual(a, d))
}
