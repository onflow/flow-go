package types

import "time"

type TimeoutMode int

const (
	ReplicaTimeout        TimeoutMode = iota
	VoteCollectionTimeout
)

type TimerInfo struct {
	Mode      TimeoutMode
	View      uint64
	StartTime time.Time
	Duration  time.Duration
}

type Timeout struct {
	TimerInfo
	CreatedAt time.Time
}

func (m TimeoutMode) String() string {
	return [...]string{"ReplicaTimeout", "VoteCollectionTimeout"}[m]
}
