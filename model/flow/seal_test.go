package flow_test

import (
	"testing"

	"github.com/onflow/flow-go/utils/unittest"
)

// TestSealMalleability checks that Seal is not malleable: any change in its data
// should result in a different ID. This is REQUIRED FOR the STORAGE layer
// to correctly retrieve the block payload!
func TestSealMalleability(t *testing.T) {
	seal := unittest.Seal.Fixture()
	unittest.RequireEntityNotMalleable(t, seal)
}
