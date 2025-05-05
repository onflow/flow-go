package storage

import "github.com/jordanschalm/lockctx"

// This file enumerates all named locks used by the storage layer.

const (
	LockInsertBlock   = "lock_insert_block"
	LockFinalizeBlock = "lock_finalize_block"
)

func Locks() []string {
	return []string{LockInsertBlock, LockFinalizeBlock}
}

func Policy() lockctx.Policy {
	return lockctx.NewDAGPolicyBuilder().Build()
}
