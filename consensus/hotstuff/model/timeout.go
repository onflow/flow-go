package model

import (
	"time"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// TimeoutMode enum type
type TimeoutMode int

const (
	// ReplicaTimeout represents the time period that the replica is waiting for the block for the current view.
	ReplicaTimeout TimeoutMode = iota
	// VoteCollectionTimeout represents the time period that the leader is waiting for votes in order to build
	// the next block.
	VoteCollectionTimeout
)

// TimerInfo represents a time period that pacemaker is waiting for a specific event.
// The end of the time period is the timeout that will trigger pacemaker's view change.
type TimerInfo struct {
	Mode      TimeoutMode
	View      uint64
	StartTime time.Time
	Duration  time.Duration
}

func (m TimeoutMode) String() string {
	return [...]string{"ReplicaTimeout", "VoteCollectionTimeout"}[m]
}

// TimeoutObject represents intent of replica to leave its current view with a timeout. This concept is very similar to
// HotStuff vote. Valid TimeoutObject is signed by staking key.
type TimeoutObject struct {
	// View is the view number which is replica is timing out
	View uint64
	// HighestQC is the highest QC known to the creator of this TimeoutObject
	HighestQC *flow.QuorumCertificate
	// LastViewTC is the timeout certificate for previous view if HighestQC.View != View - 1, else nil
	LastViewTC *flow.TimeoutCertificate
	// SignerID is the identifier of replica that has signed this TimeoutObject
	// SignerID must be the origin ID from networking layer, which cryptographically
	//  authenticates the message's sender.
	SignerID flow.Identifier
	// SigData is a BLS signature created by staking key signing View + HighestQC.View
	// This signature is further aggregated in TimeoutCertificate.
	SigData crypto.Signature
}
