package model_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
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
	require.False(t, timeout1.Equals(timeout2), "Initially, all fields are different, so the objects should not be equal.")

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

// TestTimeoutObject_Equals_Nil verifies the behavior of the Equals method when either
// or both the receiver and the function input are nil
func TestTimeoutObject_Equals_Nil(t *testing.T) {
	var nilTO *model.TimeoutObject
	to := helper.TimeoutObjectFixture()
	t.Run("nil receiver", func(t *testing.T) {
		require.False(t, nilTO.Equals(to))
	})
	t.Run("nil input", func(t *testing.T) {
		require.False(t, to.Equals(nilTO))
	})
	t.Run("both nil", func(t *testing.T) {
		require.True(t, nilTO.Equals(nil))
	})
}

// TestNewTimeoutObject verifies the behavior of the NewTimeoutObject constructor.
// It ensures proper handling of both valid and invalid untrusted input fields.
//
// Test Cases:
//
// 1. Valid input:
//   - Verifies that a properly populated UntrustedTimeoutObject results in a valid TimeoutObject.
//
// 2. Invalid input with nil NewestQC:
//   - Ensures an error is returned when the NewestQC field is nil.
//
// 3. Invalid input with zero SignerID:
//   - Ensures an error is returned when the SignerID is flow.ZeroID.
//
// 4. Invalid input with nil SigData:
//   - Ensures an error is returned when the SigData field is nil.
//
// 5. Invalid input with empty SigData:
//   - Ensures an error is returned when the SigData field is an empty byte slice.
//
// 6. Invalid input when View is lower than or equal to NewestQC.View:
//   - Ensures an error is returned when the TimeoutObject's View is less than or equal to the included QC's View.
//
// 7. Invalid input when TC present but for wrong view:
//   - Ensures an error is returned when LastViewTC.View is not one less than the TimeoutObject's View.
//
// 8. Invalid input when TC's QC newer than TimeoutObject's QC:
//   - Ensures an error is returned when TimeoutObject's NewestQC.View is older than LastViewTC.NewestQC.View.
//
// 9. Invalid input when LastViewTC missing when QC does not prove previous round:
//   - Ensures an error is returned when TimeoutObject lacks both a QC for previous round and a LastViewTC.
func TestNewTimeoutObject(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		res, err := model.NewTimeoutObject(model.UntrustedTimeoutObject(*helper.TimeoutObjectFixture()))
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("invalid input with nil NewestQC", func(t *testing.T) {
		to := helper.TimeoutObjectFixture()
		to.NewestQC = nil

		res, err := model.NewTimeoutObject(model.UntrustedTimeoutObject(*to))
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "newest QC must not be nil")
	})

	t.Run("invalid input with zero SignerID", func(t *testing.T) {
		to := helper.TimeoutObjectFixture()
		to.SignerID = flow.ZeroID

		res, err := model.NewTimeoutObject(model.UntrustedTimeoutObject(*to))
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "signer ID must not be zero")
	})

	t.Run("invalid input with nil SigData", func(t *testing.T) {
		to := helper.TimeoutObjectFixture()
		to.SigData = nil

		res, err := model.NewTimeoutObject(model.UntrustedTimeoutObject(*to))
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "signature must not be empty")
	})

	t.Run("invalid input with empty SigData", func(t *testing.T) {
		to := helper.TimeoutObjectFixture()
		to.SigData = []byte{}

		res, err := model.NewTimeoutObject(model.UntrustedTimeoutObject(*to))
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "signature must not be empty")
	})

	t.Run("invalid input when View <= NewestQC.View", func(t *testing.T) {
		qc := helper.MakeQC(helper.WithQCView(100))
		res, err := model.NewTimeoutObject(
			model.UntrustedTimeoutObject(
				*helper.TimeoutObjectFixture(
					helper.WithTimeoutNewestQC(qc),
					helper.WithTimeoutObjectView(100), // Equal to QC view
				),
			))
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "TO's QC 100 cannot be newer than the TO's view 100")
	})

	t.Run("invalid input when LastViewTC.View is not View - 1", func(t *testing.T) {
		tc := helper.MakeTC(helper.WithTCView(50))
		qc := helper.MakeQC(helper.WithQCView(40))

		result, err := model.NewTimeoutObject(
			model.UntrustedTimeoutObject(
				*helper.TimeoutObjectFixture(
					helper.WithTimeoutObjectView(100),
					helper.WithTimeoutNewestQC(qc),
					helper.WithTimeoutLastViewTC(tc),
				),
			),
		)
		require.Error(t, err)
		require.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid TC for non-previous view")
	})

	t.Run("invalid input when TimeoutObject's QC is older than TC's QC", func(t *testing.T) {
		tcQC := helper.MakeQC(helper.WithQCView(150))
		tc := helper.MakeTC(helper.WithTCNewestQC(tcQC), helper.WithTCView(99))

		res, err := model.NewTimeoutObject(
			model.UntrustedTimeoutObject(
				*helper.TimeoutObjectFixture(
					helper.WithTimeoutObjectView(100),
					helper.WithTimeoutLastViewTC(tc),
					helper.WithTimeoutNewestQC(helper.MakeQC(helper.WithQCView(80))), // older than TC.NewestQC
				),
			),
		)
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "timeout.NewestQC is older")
	})

	t.Run("invalid input when no QC for previous round and TC is missing", func(t *testing.T) {
		qc := helper.MakeQC(helper.WithQCView(90))

		res, err := model.NewTimeoutObject(
			model.UntrustedTimeoutObject(
				*helper.TimeoutObjectFixture(
					helper.WithTimeoutObjectView(100),
					helper.WithTimeoutNewestQC(qc),
					helper.WithTimeoutLastViewTC(nil),
				),
			),
		)
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "must include TC")
	})
}
