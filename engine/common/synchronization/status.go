// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package synchronization

import (
	"time"
)

// Status keeps track of a block download status.
type Status struct {
	Queued    time.Time
	Requested time.Time
	Attempts  uint
}
