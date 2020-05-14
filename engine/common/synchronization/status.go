// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package synchronization

import (
	"time"
)

// Status keeps track of a block download status.
type Status struct {
	Queued    time.Time // when we originally this block request
	Requested time.Time // the last time we requested this block
	Attempts  uint      // how many times we've requested this block
}

// ShouldRetry returns true if we this request is ready to be retried.
// Uses an exponential backoff.
func (s Status) ShouldRetry(retryInterval time.Duration) bool {
	return time.Now().After(s.Requested.Add(retryInterval * 2))
}
