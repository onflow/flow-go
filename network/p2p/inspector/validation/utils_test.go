package validation

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDuplicateStringTracker tests the duplicateStrTracker.track function.
func TestDuplicateStringTracker(t *testing.T) {
	tracker := make(duplicateStrTracker)
	require.Equal(t, 1, tracker.track("test1"))
	require.Equal(t, 2, tracker.track("test1"))

	// tracking a new string, 3 times
	require.Equal(t, 1, tracker.track("test2"))
	require.Equal(t, 2, tracker.track("test2"))
	require.Equal(t, 3, tracker.track("test2"))

	// tracking an empty string
	require.Equal(t, 1, tracker.track(""))
}
