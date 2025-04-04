package model_test

import (
	"testing"

	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestMalleability verifies that the entities which implements the ID are not malleable.
func TestMalleability(t *testing.T) {
	t.Run("RepeatableTimeoutObject", func(t *testing.T) {
		unittest.RequireEntityNonMalleable(t, helper.RepeatableTimeoutObjectFixture())
	})

	t.Run("TimeoutObject", func(t *testing.T) {
		unittest.RequireEntityNonMalleable(t, helper.TimeoutObjectFixture())
	})
}
