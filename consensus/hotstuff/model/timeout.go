package model

import (
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
