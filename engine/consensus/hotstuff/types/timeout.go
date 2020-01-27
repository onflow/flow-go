package types

import "time"

type TimeoutMode int

const (
	ReplicaTimeout TimeoutMode = iota
	VoteCollectionTimeout TimeoutMode = iota
)

type Timeout struct {
	Mode TimeoutMode
	View uint64
	TimeoutFired time.Time
}

func (m TimeoutMode) String() string {
	return [...]string{"ReplicaTimeout", "VoteCollectionTimeout"}[m]
}

