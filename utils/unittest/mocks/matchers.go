package mocks

import (
	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/mock"
)

// MatchLock returns an argument matcher that checks if the argument is a `lockctx.Proof` that holds
// the provided lock.
//
// Example:
//
//	events.
//		On("BatchStore", mocks.MatchLock(storage.LockInsertEvent), blockID, expectedEvents, mock.Anything).
//		Return(func(lctx lockctx.Proof, blockID flow.Identifier, events []flow.EventsList, batch storage.ReaderBatchWriter) error {
//			require.NotNil(t, batch)
//			return nil
//		}).
//		Once()
func MatchLock(lock string) any {
	return mock.MatchedBy(func(lctx lockctx.Proof) bool { return lctx.HoldsLock(lock) })
}
