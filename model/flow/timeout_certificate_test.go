package flow_test

import (
	"math/rand"
	"testing"

	clone "github.com/huandu/go-clone/generic" //nolint:goimports

	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

// TestTimeoutCertificateID_Malleability confirms that the TimeoutCertificate struct, which implements
// the [flow.IDEntity] interface, is resistant to tampering.
func TestTimeoutCertificateID_Malleability(t *testing.T) {
	unittest.RequireEntityNonMalleable(t, helper.MakeTC())
}

// TestTimeoutCertificate_Equals verifies the correctness of the Equals method on TimeoutCertificates.
// It checks that TimeoutCertificates are considered equal if and only if all fields match.
func TestTimeoutCertificate_Equals(t *testing.T) {
	// Create two TimeoutCertificates with random but different values.
	tc1, tc2 := helper.MakeTC(), helper.MakeTC()
	require.False(t, tc1.Equals(tc2), "Initially, all fields are different, so the objects should not be equal")

	// List of mutations to apply on tc1 to gradually make it equal to tc2
	mutations := []func(){
		func() {
			tc1.View = tc2.View
		}, func() {
			tc1.NewestQCViews = clone.Clone(tc2.NewestQCViews) // deep copy
		}, func() {
			tc1.NewestQC = clone.Clone(tc2.NewestQC) // deep copy
		}, func() {
			tc1.SignerIndices = clone.Clone(tc2.SignerIndices) // deep copy
		}, func() {
			tc1.SigData = clone.Clone(tc2.SigData) // deep copy
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
		require.False(t, tc1.Equals(tc2))
	}

	// Apply the final mutation; now all relevant fields should match, so the objects must be equal.
	mutations[len(mutations)-1]()
	require.True(t, tc1.Equals(tc2))
}

// TestTimeoutCertificate_Equals_EmptyNewestQCViews verifies the behavior of the Equals method when either
// or both `NewestQCViews` are nil in the receiver and/or the function input.
func TestTimeoutCertificate_Equals_EmptyNewestQCViews(t *testing.T) {
	// Create two identical TimeoutCertificates
	tc1 := helper.MakeTC()
	tc2 := clone.Clone(tc1)
	require.True(t, tc1.Equals(tc2), "Initially, all fields are identical, so the objects should be equal")
	require.True(t, len(tc1.NewestQCViews) > 0, "sanity check that NewestQCViews is not empty")

	t.Run("NewestQCViews is nil in tc2 only", func(t *testing.T) {
		tc2.NewestQCViews = nil
		require.False(t, tc1.Equals(tc2))
		require.False(t, tc2.Equals(tc1))
	})
	t.Run("NewestQCViews is empty slice in tc2 only", func(t *testing.T) {
		tc2.NewestQCViews = []uint64{}
		require.False(t, tc1.Equals(tc2))
		require.False(t, tc2.Equals(tc1))
	})
	t.Run("NewestQCViews is nil in tc1 and tc2", func(t *testing.T) {
		tc1.NewestQCViews = nil
		tc2.NewestQCViews = nil
		require.True(t, tc1.Equals(tc2))
		require.True(t, tc2.Equals(tc1))
	})
	t.Run("NewestQCViews is empty slice in tc1 and tc2", func(t *testing.T) {
		tc1.NewestQCViews = []uint64{}
		tc2.NewestQCViews = []uint64{}
		require.True(t, tc1.Equals(tc2))
		require.True(t, tc2.Equals(tc1))
	})
	t.Run("NewestQCViews is empty slice in tc1 and nil tc2", func(t *testing.T) {
		tc1.NewestQCViews = []uint64{}
		tc2.NewestQCViews = nil
		require.True(t, tc1.Equals(tc2))
		require.True(t, tc2.Equals(tc1))
	})
}

// TestTimeoutCertificate_Equals_Nil verifies the behavior of the Equals method when either
// or both the receiver and the function input are nil
func TestTimeoutCertificate_Equals_Nil(t *testing.T) {
	var nilTC *flow.TimeoutCertificate
	tc := helper.MakeTC()
	t.Run("nil receiver", func(t *testing.T) {
		require.False(t, nilTC.Equals(tc))
	})
	t.Run("nil input", func(t *testing.T) {
		require.False(t, tc.Equals(nilTC))
	})
	t.Run("both nil", func(t *testing.T) {
		require.True(t, nilTC.Equals(nil))
	})
}
