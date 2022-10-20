package model

import (
	"fmt"
	"time"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// TimerInfo represents a time period that pacemaker is waiting for a specific event.
// The end of the time period is the timeout that will trigger pacemaker's view change.
type TimerInfo struct {
	View      uint64
	Tick      uint64
	StartTime time.Time
	Duration  time.Duration
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
