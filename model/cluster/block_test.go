package cluster_test

import (
	"testing"
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestClusterBlockMalleability checks that cluster.Block is not malleable: any change in its data
// should result in a different ID.
func TestClusterBlockMalleability(t *testing.T) {
	block := unittest.ClusterBlockFixture()
	unittest.RequireEntityNonMalleable(
		t,
		&block,
		unittest.WithFieldGenerator("Header.Timestamp", func() time.Time { return time.Now().UTC() }),
		unittest.WithFieldGenerator("Payload.Collection", func() flow.Collection {
			return unittest.CollectionFixture(3)
		}),
	)
}
