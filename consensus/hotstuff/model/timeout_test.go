package model_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/helper"
)

// TestTimeoutObject_Equals verifies the correctness of the Equals method on TimeoutObject.
// It checks that TimeoutObjects are considered equal if and only if all fields except
// TimeoutTick match. This test is crucial for ensuring deduplication logic works correctly
// in the cache.
//
// Fields compared for equality:
// - View
// - NewestQC
// - LastViewTC
// - SignerID
// - SigData
//
// TimeoutTick is explicitly excluded from equality checks.
func TestTimeoutObject_Equals(t *testing.T) {
	// Create two TimeoutObjects with random but different values.
	timeout1 := helper.TimeoutObjectFixture()
	timeout2 := helper.TimeoutObjectFixture()
	// Initially, all fields are different, so the objects should not be equal.
	require.False(t, timeout1.Equals(timeout2))

	// Make View equal, still not enough for equality.
	timeout1.View = timeout2.View
	require.False(t, timeout1.Equals(timeout2))

	// Make NewestQC equal, still not equal.
	timeout1.NewestQC = timeout2.NewestQC
	require.False(t, timeout1.Equals(timeout2))

	// Make LastViewTC equal, still not equal.
	timeout1.LastViewTC = timeout2.LastViewTC
	require.False(t, timeout1.Equals(timeout2))

	// Make SignerID equal, still not equal.
	timeout1.SignerID = timeout2.SignerID
	require.False(t, timeout1.Equals(timeout2))

	// Make SigData equal, now all compared fields are equal, so they should be equal.
	timeout1.SigData = timeout2.SigData
	require.True(t, timeout1.Equals(timeout2))

	// Even if TimeoutTick differs, equality should still hold since TimeoutTick is not important for equality.
	timeout1.TimeoutTick = timeout2.TimeoutTick + 1
	require.True(t, timeout1.Equals(timeout2))
}
