// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package synchronization

// Range is a range for which we want to request blocks.
type Range struct {
	From uint64
	To   uint64
}
