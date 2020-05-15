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
