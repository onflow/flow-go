package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/model/flow"
)

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
// 7. Invalid input when the View is lower than NewestQC's View:
//   - Ensures an error is returned when the TimeoutCertificate's View is less than the included QuorumCertificate's View.
//
// 8. Invalid input when NewestQCViews contains view higher than NewestQC.View:
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
