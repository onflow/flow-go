package validation

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDuplicateStrTracker ensures duplicateStrTracker performs simple set and returns expected result for duplicate str.
func TestDuplicateStrTracker(t *testing.T) {
	tracker := make(duplicateStrTracker)
	s := "hello world"
	require.False(t, tracker.isDuplicate(s))
	tracker.set(s)
	require.True(t, tracker.isDuplicate(s))
}
