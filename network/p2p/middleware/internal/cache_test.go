package internal_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/p2p/middleware/internal"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestNewDisallowListCache tests the NewDisallowListCache function. It verifies that the returned disallowListCache
// is not nil.
func TestNewDisallowListCache(t *testing.T) {
	disallowListCache := internal.NewDisallowListCache(uint32(100), unittest.Logger(), metrics.NewNoopCollector())

	// Verify that the new disallowListCache is not nil
	assert.NotNil(t, disallowListCache)
}
