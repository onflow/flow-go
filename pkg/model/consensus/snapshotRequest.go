// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

// SnapshotRequest is a request for a snapshot hash of the collection memory pool.
type SnapshotRequest struct {
	Nonce       uint64
	MempoolHash []byte
}
