// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package synchronization

import (
	"time"

	"github.com/onflow/flow-go/model/flow"
)

// Status keeps track of a block download status.
type Status struct {
	Queued    time.Time    // when we originally queued this block request
	Requested time.Time    // the last time we requested this block
	Attempts  uint         // how many times we've requested this block
	Header    *flow.Header // the requested header, if we've received it
	Received  time.Time    // when we received a response
}

func (s *Status) WasQueued() bool {
	return s != nil
}

func (s *Status) WasRequested() bool {
	if s == nil {
		return false
	}
	return !s.Requested.IsZero()
}

func (s *Status) WasReceived() bool {
	if s == nil {
		return false
	}
	return !s.Received.IsZero()
}

func (s *Status) StatusString() string {
	if s.WasReceived() {
		return "Received"
	} else if s.WasRequested() {
		return "Requested"
	} else if s.WasQueued() {
		return "Queued"
	} else {
		return "Unknown"
	}
}

func NewQueuedStatus() *Status {
	return &Status{
		Queued: time.Now(),
	}
}
