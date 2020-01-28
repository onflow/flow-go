package types

import "time"

type TimeoutMode int

const (
	ReplicaTimeout        TimeoutMode = iota
	VoteCollectionTimeout TimeoutMode = iota
)

type TimerInfo struct {
	Mode      TimeoutMode
	View      uint64
	StartTime time.Time
	Duration  time.Duration
}

type Timeout struct {
	TimerInfo
	TimeoutFired time.Time
}

func (m TimeoutMode) String() string {
	return [...]string{"ReplicaTimeout", "VoteCollectionTimeout"}[m]
}
