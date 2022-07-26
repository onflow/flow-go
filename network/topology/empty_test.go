package topology

import (
	"testing"

	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

// TestEmptyTopology checks that EmptyTopology always creates an empty list of fanout.
func TestEmptyTopology(t *testing.T) {
	ids := unittest.IdentityListFixture(10)
	top := NewEmptyTopology()
	require.Empty(t, top.Fanout(ids))
}
