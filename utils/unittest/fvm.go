package unittest

import (
	"testing"

	"github.com/stretchr/testify/require"

	fvmerrors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
)

// EnsureEventsIndexSeq checks if values of given event index sequence are monotonically increasing.
func EnsureEventsIndexSeq(t *testing.T, events []flow.Event, chainID flow.ChainID) {
	expectedEventIndex := uint32(0)
	for _, event := range events {
		require.Equal(t, expectedEventIndex, event.EventIndex)
		expectedEventIndex++
	}
}

// RequireLimitExceededError fails the test unless err's chain contains a
// [fvmerrors.LimitExceededError] with the given kind. If expectedLimit is
// provided, the error's limit must match it.
//
// It also checks the [fvmerrors.NewLimitExceededError] invariant that the
// used amount exceeds the limit.
func RequireLimitExceededError(
	t testing.TB,
	err error,
	kind fvmerrors.LimitKind,
	expectedLimit ...uint64,
) {
	require.LessOrEqual(t, len(expectedLimit), 1,
		"at most one expected limit may be provided")

	limitErr, ok := fvmerrors.AsLimitExceededError(err)
	require.True(t, ok, "expected LimitExceededError, got: %v", err)
	require.Equal(t, kind, limitErr.LimitKind())
	require.Greater(t, limitErr.Used(), limitErr.Limit(),
		"LimitExceededError invariant: used must exceed limit")
	if len(expectedLimit) == 1 {
		require.Equal(t, expectedLimit[0], limitErr.Limit())
	}
}
