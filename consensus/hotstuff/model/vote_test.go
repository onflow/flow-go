package model_test

import (
	"testing"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestVoteNonMalleable(t *testing.T) {
	unittest.RequireEntityNonMalleable(t, unittest.VoteFixture())
}
