package cluster_test

import (
	"testing"
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestClusterBlockMalleability(t *testing.T) {
	block := unittest.ClusterBlockFixture()

	unittest.RequireEntityNonMalleable(t, &block,
		unittest.WithFieldGenerator("Header.Timestamp", func() time.Time { return time.Now().UTC() }),
		unittest.WithFieldGenerator("Payload.Collection", func() flow.Collection {
			block.Payload.Collection = unittest.CollectionFixture(3)
			block.SetPayload(*block.Payload)
			return block.Payload.Collection
		}),
	)
}
