package flow_test

import (
	"math/rand"
	"testing"

	clone "github.com/huandu/go-clone/generic"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestTimeoutCertificateID_Malleability confirms that the TimeoutCertificate struct, which implements
// the [flow.IDEntity] interface, is resistant to tampering.
func TestTimeoutCertificateID_Malleability(t *testing.T) {
	unittest.RequireEntityNonMalleable(t, helper.MakeTC())
}

// TestTimeoutCertificate_Equals verifies the correctness of the Equals method on TimeoutCertificates.
// It checks that TimeoutCertificates are considered equal if and only if all fields match.
func TestTimeoutCertificate_Equals(t *testing.T) {
	// Create two TimeoutCertificates with random but different values. Note: random selection for `SignerIndices` has limited variability
	// and yields sometimes the same value for both tc1 and tc2. Therefore, we explicitly set different values for `SignerIndices`.
	tc1, tc2 := helper.MakeTC(helper.WithTCSigners([]byte{74, 0})), helper.MakeTC(helper.WithTCSigners([]byte{37, 0}))
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

// TestNewTimeoutCertificate verifies the behavior of the NewTimeoutCertificate constructor.
// It ensures proper handling of both valid and invalid untrusted input fields.
//
// Test Cases:
//
// 1. Valid input:
//   - Verifies that a properly populated UntrustedTimeoutCertificate results in a valid TimeoutCertificate.
//
// 2. Invalid input with nil NewestQC:
//   - Ensures an error is returned when the NewestQC field is nil.
//
// 3. Invalid input with nil SignerIndices:
//   - Ensures an error is returned when the SignerIndices field is nil.
//
// 4. Invalid input with empty SignerIndices:
//   - Ensures an error is returned when the SignerIndices field is an empty slice.
//
// 5. Invalid input with nil SigData:
//   - Ensures an error is returned when the SigData field is nil.
//
// 6. Invalid input with empty SigData:
//   - Ensures an error is returned when the SigData field is an empty byte slice.
//
// 7. Invalid input with nil NewestQCViews:
//   - Ensures an error is returned when the NewestQCViews field is nil.
//
// 8. Invalid input with empty NewestQCViews:
//   - Ensures an error is returned when the NewestQCViews field is an empty slice.
//
// 9. Invalid input when the View is lower than NewestQC's View:
//   - Ensures an error is returned when the TimeoutCertificate's View is less than the included QuorumCertificate's View.
//
// 10. Invalid input when NewestQCViews contains view higher than NewestQC.View:
//   - Ensures an error is returned if NewestQCViews includes a view that exceeds the view of the NewestQC.
func TestNewTimeoutCertificate(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		validTC := helper.MakeTC()
		tc, err := flow.NewTimeoutCertificate(
			flow.UntrustedTimeoutCertificate(*validTC),
		)
		require.NoError(t, err)
		require.NotNil(t, tc)
	})

	t.Run("invalid input with nil NewestQC", func(t *testing.T) {
		tc := helper.MakeTC()
		tc.NewestQC = nil

		res, err := flow.NewTimeoutCertificate(flow.UntrustedTimeoutCertificate(*tc))
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "newest QC must not be nil")
	})

	t.Run("invalid input with nil SignerIndices", func(t *testing.T) {
		tc := helper.MakeTC()
		tc.SignerIndices = nil

		res, err := flow.NewTimeoutCertificate(flow.UntrustedTimeoutCertificate(*tc))
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "signer indices must not be empty")
	})

	t.Run("invalid input with empty SignerIndices", func(t *testing.T) {
		tc := helper.MakeTC()
		tc.SignerIndices = []byte{}

		res, err := flow.NewTimeoutCertificate(flow.UntrustedTimeoutCertificate(*tc))
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "signer indices must not be empty")
	})

	t.Run("invalid input with nil SigData", func(t *testing.T) {
		tc := helper.MakeTC()
		tc.SigData = nil

		res, err := flow.NewTimeoutCertificate(flow.UntrustedTimeoutCertificate(*tc))
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "signature must not be empty")
	})

	t.Run("invalid input with empty SigData", func(t *testing.T) {
		tc := helper.MakeTC()
		tc.SigData = []byte{}

		res, err := flow.NewTimeoutCertificate(flow.UntrustedTimeoutCertificate(*tc))
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "signature must not be empty")
	})

	t.Run("invalid input with nil NewestQCViews", func(t *testing.T) {
		tc := helper.MakeTC()
		tc.NewestQCViews = nil

		res, err := flow.NewTimeoutCertificate(flow.UntrustedTimeoutCertificate(*tc))
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "newest QC views must not be empty")
	})

	t.Run("invalid input with empty NewestQCViews", func(t *testing.T) {
		tc := helper.MakeTC()
		tc.NewestQCViews = []uint64{}

		res, err := flow.NewTimeoutCertificate(flow.UntrustedTimeoutCertificate(*tc))
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "newest QC views must not be empty")
	})

	t.Run("invalid input with TC.View < QC.View", func(t *testing.T) {
		qc := helper.MakeQC(helper.WithQCView(100))
		tc, err := flow.NewTimeoutCertificate(
			flow.UntrustedTimeoutCertificate(
				*helper.MakeTC(
					helper.WithTCView(99),
					helper.WithTCNewestQC(qc),
				)),
		)
		require.Error(t, err)
		require.Nil(t, tc)
		assert.Contains(t, err.Error(), "TC's QC view (100) cannot be newer than the TC's view (99)")
	})

	t.Run("invalid input when NewestQCViews has view higher than NewestQC.View", func(t *testing.T) {
		qc := helper.MakeQC(helper.WithQCView(50))
		tc, err := flow.NewTimeoutCertificate(
			flow.UntrustedTimeoutCertificate(
				*helper.MakeTC(
					helper.WithTCView(51),
					helper.WithTCNewestQC(qc),
					helper.WithTCHighQCViews([]uint64{40, 50, 60}), // highest = 60 > QC.View = 50
				),
			),
		)
		require.Error(t, err)
		require.Nil(t, tc)
		assert.Contains(t, err.Error(), "included QC (view=50) should be equal or higher to highest contributed view: 60")
	})
}
