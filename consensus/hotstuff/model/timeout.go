package model

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// TimerInfo represents a local timeout for a view, and indicates the Pacemaker has not yet
// observed evidence to transition to the next view (QC or TC). When a timeout occurs for
// the first time in a view, we will broadcast a TimeoutObject and continue waiting for evidence
// to enter the next view, but we will no longer submit a vote for this view. A timeout may occur
// multiple times for the same round (indicated by Tick > 0), which is an indication
// to re-broadcast our TimeoutObject for the view, to ensure liveness.
type TimerInfo struct {
	// View is round at which timer was created.
	View uint64
	// Tick is the number of times a timeout event has been emitted for this view (beginning with 0).
	// It is used to de-duplicate TimeoutObject re-broadcasts.
	Tick uint64
	// StartTime represents time of entering the view
	StartTime time.Time
	// Duration is how long we waited before timing out the view.
	// It does not include subsequent timeouts (ie. all timeout events emitted for the same
	// view will have the same Duration).
	Duration time.Duration
}

// NewViewEvent indicates that a new view has started. While it has the same
// data model as `TimerInfo`, their semantics are different (hence we use
// different types): `TimerInfo` represents a continuous time interval. In
// contrast, NewViewEvent marks the specific point in time, when the timer
// is started.
type NewViewEvent TimerInfo

// TimeoutObject represents intent of replica to leave its current view with a timeout. This concept is very similar to
// HotStuff vote. Valid TimeoutObject is signed by staking key.
type TimeoutObject struct {
	// View is the view number which is replica is timing out
	View uint64
	// NewestQC is the newest QC (by view) known to the creator of this TimeoutObject
	NewestQC *flow.QuorumCertificate
	// LastViewTC is the timeout certificate for previous view if NewestQC.View != View - 1, else nil
	LastViewTC *flow.TimeoutCertificate
	// SignerID is the identifier of replica that has signed this TimeoutObject
	// SignerID must be the origin ID from networking layer, which cryptographically
	//  authenticates the message's sender.
	SignerID flow.Identifier
	// SigData is a BLS signature created by staking key signing View + NewestQC.View
	// This signature is further aggregated in TimeoutCertificate.
	SigData crypto.Signature
}

// ID returns the TimeoutObject's identifier
func (t *TimeoutObject) ID() flow.Identifier {
	body := struct {
		View         uint64
		NewestQCID   flow.Identifier
		LastViewTCID flow.Identifier
		SignerID     flow.Identifier
		SigData      crypto.Signature
	}{
		View:         t.View,
		NewestQCID:   t.NewestQC.ID(),
		LastViewTCID: t.LastViewTC.ID(),
		SignerID:     t.SignerID,
		SigData:      t.SigData,
	}
	return flow.MakeID(body)
}

func (t *TimeoutObject) String() string {
	return fmt.Sprintf("View: %d, HighestQC.View: %d, LastViewTC: %v", t.View, t.NewestQC.View, t.LastViewTC)
}

// LogContext returns a `zerolog.Contex` including the most important properties of the TC:
//   - view number that this TC is for
//   - view and ID of the block that the included QC points to
//   - [optional] if the TC also includes a TC for the prior view, i.e. `LastViewTC` ≠ nil:
//     the new of `LastViewTC` and the view that `LastViewTC.NewestQC` is for
func (t *TimeoutObject) LogContext(logger zerolog.Logger) zerolog.Context {
	logContext := logger.With().
		Uint64("timeout_newest_qc_view", t.NewestQC.View).
		Hex("timeout_newest_qc_block_id", t.NewestQC.BlockID[:]).
		Uint64("timeout_view", t.View)

	if t.LastViewTC != nil {
		logContext.
			Uint64("last_view_tc_view", t.LastViewTC.View).
			Uint64("last_view_tc_newest_qc_view", t.LastViewTC.NewestQC.View)
	}
	return logContext
}
