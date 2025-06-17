package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

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
