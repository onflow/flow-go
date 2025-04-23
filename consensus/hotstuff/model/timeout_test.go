package model_test

import (
	"math/rand"
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

	// List of mutations to apply on timeout1 to gradually make it equal to timeout2
	// (excluding TimeoutTick).
	mutations := []func(){
		func() {
			timeout1.View = timeout2.View
		}, func() {
			timeout1.NewestQC = timeout2.NewestQC
		}, func() {
			timeout1.LastViewTC = timeout2.LastViewTC
		}, func() {
			timeout1.SignerID = timeout2.SignerID
		}, func() {
			timeout1.SigData = timeout2.SigData
		},
	}

	// Shuffle the order of mutations
	rand.Shuffle(len(mutations), func(i, j int) {
		mutations[i], mutations[j] = mutations[j], mutations[i]
	})

	// Apply each mutation one at a time, except the last.
	// After each step, the objects should still not be equal.
	for _, mutation := range mutations[:len(mutations)-1] {
		mutation()
		require.False(t, timeout1.Equals(timeout2))
	}

	// Apply the final mutation; now all relevant fields should match, so the objects must be equal.
	mutations[len(mutations)-1]()
	require.True(t, timeout1.Equals(timeout2))

	// Even if TimeoutTick differs, equality should still hold since TimeoutTick is not important for equality.
	timeout1.TimeoutTick = timeout2.TimeoutTick + 1
	require.True(t, timeout1.Equals(timeout2))
}
