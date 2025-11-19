package mocks

import (
	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/mock"
)

func MatchLock(lock string) interface{} {
	return mock.MatchedBy(func(lctx lockctx.Proof) bool { return lctx.HoldsLock(lock) })
}
