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
	// Create two TimeoutCertificate with random but different values.
	tc1 := helper.MakeTC()
	tc2 := helper.MakeTC()
	// Initially, all fields are different, so the objects should not be equal.
	require.False(t, tc1.Equals(tc2))

	// List of mutations to apply on timeout1 to gradually make it equal to tc2
	// (excluding TimeoutTick).
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
