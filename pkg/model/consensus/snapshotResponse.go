// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

// SnapshotResponse is the response to a snapshot hash of the collection mempool.
type SnapshotResponse struct {
	Nonce       uint64
	MempoolHash []byte
}
