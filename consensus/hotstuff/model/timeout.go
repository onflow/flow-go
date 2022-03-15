package model

import (
	"github.com/onflow/flow-go/model/flow"
	"time"
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

type TimeoutObject struct {
	View       uint64
	HighestQC  *flow.QuorumCertificate  // highest QC that this node has seen
	LastViewTC *flow.TimeoutCertificate // TC for View - 1 if HighestQC.View != View - 1, else nil
	SignerID   flow.Identifier
	SigData    []byte // each node provides unique signature of View + HighestQC.View
}
