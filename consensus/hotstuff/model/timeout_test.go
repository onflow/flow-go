package model_test

import (
	"testing"

	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestMalleability verifies that the TimeoutObject which implements IDEntity is not malleable.
func TestMalleability(t *testing.T) {
	unittest.RequireEntityNonMalleable(t, helper.TimeoutObjectFixture())
}
