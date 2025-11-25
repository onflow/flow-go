package model

import (
	"bytes"
	"fmt"
	"time"

	"github.com/onflow/crypto"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
)

// TimerInfo represents a local timeout for a view, and indicates the Pacemaker has not yet
// observed evidence to transition to the next view (QC or TC). When a timeout occurs for
// the first time in a view, we will broadcast a TimeoutObject and continue waiting for evidence
// to enter the next view, but we will no longer submit a vote for this view. A timeout may occur
// multiple times for the same round, which is an indication
// to re-broadcast our TimeoutObject for the view, to ensure liveness.
type TimerInfo struct {
	// View is round at which timer was created.
	View uint64
	// StartTime represents time of entering the view.
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
//
//structwrite:immutable - mutations allowed only within the constructor
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
	// TimeoutTick is the number of times the `timeout.Controller` has (re-)emitted the
	// timeout for this view. When the timer for the view's original duration expires, a `TimeoutObject`
	// with `TimeoutTick = 0` is broadcast. Subsequently, `timeout.Controller` re-broadcasts the
	// `TimeoutObject` periodically  based on some internal heuristic. Each time we attempt a re-broadcast,
	// the `TimeoutTick` is incremented. Incrementing the field prevents de-duplicated within the network layer,
	// which in turn guarantees quick delivery of the `TimeoutObject` after GST and facilitates recovery.
	TimeoutTick uint64
}

// UntrustedTimeoutObject is an untrusted input-only representation of a TimeoutObject,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedTimeoutObject should be validated and converted into
// a trusted TimeoutObject using NewTimeoutObject constructor.
type UntrustedTimeoutObject TimeoutObject

// NewTimeoutObject creates a new instance of TimeoutObject.
// Construction TimeoutObject allowed only within the constructor.
//
// All errors indicate a valid TimeoutObject cannot be constructed from the input.
func NewTimeoutObject(untrusted UntrustedTimeoutObject) (*TimeoutObject, error) {
	if untrusted.NewestQC == nil {
		return nil, fmt.Errorf("newest QC must not be nil")
	}
	if untrusted.SignerID == flow.ZeroID {
		return nil, fmt.Errorf("signer ID must not be zero")
	}
	if len(untrusted.SigData) == 0 {
		return nil, fmt.Errorf("signature must not be empty")
	}
	if untrusted.View <= untrusted.NewestQC.View {
		return nil, fmt.Errorf("TO's QC %d cannot be newer than the TO's view %d", untrusted.NewestQC.View, untrusted.View)
	}

	// If a TC is included, the TC must be for the past round, no matter whether a QC
	// for the last round is also included. In some edge cases, a node might observe
	// _both_ QC and TC for the previous round, in which case it can include both.
	if untrusted.LastViewTC != nil {
		if untrusted.View != untrusted.LastViewTC.View+1 {
			return nil, fmt.Errorf("invalid TC for non-previous view, expected view %d, got view %d", untrusted.View-1, untrusted.LastViewTC.View)
		}
		if untrusted.NewestQC.View < untrusted.LastViewTC.NewestQC.View {
			return nil, fmt.Errorf("timeout.NewestQC is older (view=%d) than the QC in timeout.LastViewTC (view=%d)", untrusted.NewestQC.View, untrusted.LastViewTC.NewestQC.View)
		}
	}
	// The TO must contain a proof that sender legitimately entered View. Transitioning
	// to round timeout.View is possible either by observing a QC or a TC for the previous round.
	// If no QC is included, we require a TC to be present, which by check must be for
	// the previous round.
	lastViewSuccessful := untrusted.View == untrusted.NewestQC.View+1
	if !lastViewSuccessful {
		// The TO's sender did _not_ observe a QC for round timeout.View-1. Hence, it should
		// include a TC for the previous round. Otherwise, the TO is invalid.
		if untrusted.LastViewTC == nil {
			return nil, fmt.Errorf("must include TC")
		}
	}

	return &TimeoutObject{
		View:        untrusted.View,
		NewestQC:    untrusted.NewestQC,
		LastViewTC:  untrusted.LastViewTC,
		SignerID:    untrusted.SignerID,
		SigData:     untrusted.SigData,
		TimeoutTick: untrusted.TimeoutTick,
	}, nil
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

// Equals returns true if and only if the receiver TimeoutObject is equal to the `other`. Nil values are supported.
// It compares View, NewestQC, LastViewTC, SignerID and SigData and is used for de-duplicate TimeoutObjects in the cache.
// It excludes TimeoutTick: two TimeoutObjects with different TimeoutTick values are considered equivalent.
func (t *TimeoutObject) Equals(other *TimeoutObject) bool {
	// Shortcut if `t` and `other` point to the same object; covers case where both are nil.
	if t == other {
		return true
	}
	if t == nil || other == nil { // only one is nil, the other not (otherwise we would have returned above)
		return false
	}
	// both are not nil, so we can compare the fields
	return t.View == other.View &&
		t.NewestQC.Equals(other.NewestQC) &&
		t.LastViewTC.Equals(other.LastViewTC) &&
		t.SignerID == other.SignerID &&
		bytes.Equal(t.SigData, other.SigData)
}

// String returns a partial string representation of the TimeoutObject,
// including the signer ID, view, and the newest QC view.
func (t *TimeoutObject) String() string {
	return fmt.Sprintf(
		"Signer ID: %s, View: %d, NewestQC.View: %d",
		t.SignerID.String(),
		t.View,
		t.NewestQC.View,
	)
}

// LogContext returns a `zerolog.Context` including the most important properties of the TO:
//   - view and ID of the block that the included QC points to
//   - number of times a re-broadcast of this timeout was attempted
//   - view number that this TO is for
//   - [optional] if the TC also includes a TC for the prior view, i.e. `LastViewTC` ≠ nil:
//     the new of `LastViewTC` and the view that `LastViewTC.NewestQC` is for
func (t *TimeoutObject) LogContext(logger zerolog.Logger) zerolog.Context {
	logContext := logger.With().
		Uint64("timeout_newest_qc_view", t.NewestQC.View).
		Hex("timeout_newest_qc_block_id", t.NewestQC.BlockID[:]).
		Uint64("timeout_view", t.View).
		Uint64("timeout_tick", t.TimeoutTick)

	if t.LastViewTC != nil {
		logContext.
			Uint64("last_view_tc_view", t.LastViewTC.View).
			Uint64("last_view_tc_newest_qc_view", t.LastViewTC.NewestQC.View)
	}
	return logContext
}
