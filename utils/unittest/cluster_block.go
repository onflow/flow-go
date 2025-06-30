package unittest

import (
	"time"

	"github.com/onflow/flow-go/model/cluster"
)

var ClusterBlock clusterBlockFactory

type clusterBlockFactory struct{}

func ClusterBlockFixture(opts ...func(*cluster.Block)) *cluster.Block {
	block := &cluster.Block{
		Header:  HeaderBodyFixture(),
		Payload: *ClusterPayloadFixture(3),
	}
	for _, opt := range opts {
		opt(block)
	}
	return block
}

func (f *clusterBlockFactory) WithParent(parent *cluster.Block) func(*cluster.Block) {
	return func(block *cluster.Block) {
		block.Header.Height = parent.Header.Height + 1
		block.Header.View = parent.Header.View + 1
		block.Header.ChainID = parent.Header.ChainID
		block.Header.Timestamp = time.Now().UTC()
		block.Header.ParentID = parent.ID()
		block.Header.ParentView = parent.Header.View
	}
}

func (f *clusterBlockFactory) WithHeight(height uint64) func(*cluster.Block) {
	return func(block *cluster.Block) {
		block.Header.Height = height
	}
}

func (f *clusterBlockFactory) WithPayload(payload cluster.Payload) func(*cluster.Block) {
	return func(b *cluster.Block) {
		b.Payload = payload
	}
}
