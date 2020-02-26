package hotstuff

import (
	"time"
)

type TimeoutMode int

const (
	ReplicaTimeout TimeoutMode = iota
	VoteCollectionTimeout
)

type TimerInfo struct {
	Mode      TimeoutMode
	View      uint64
	StartTime time.Time
	Duration  time.Duration
}

func (m TimeoutMode) String() string {
	return [...]string{"ReplicaTimeout", "VoteCollectionTimeout"}[m]
}
