package unittest

import (
	"fmt"
	"time"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

var ClusterBlock clusterBlockFactory

type clusterBlockFactory struct{}

func ClusterBlockFixture(opts ...func(*cluster.Block)) *cluster.Block {
	block := &cluster.Block{
		HeaderBody: HeaderBodyFixture(),
		Payload:    *ClusterPayloadFixture(3),
	}
	block.ChainID = "cluster"
	for _, opt := range opts {
		opt(block)
	}
	return block
}

func (f *clusterBlockFactory) WithParent(parent *cluster.Block) func(*cluster.Block) {
	return func(block *cluster.Block) {
		block.Height = parent.Height + 1
		block.View = parent.View + 1
		block.ChainID = parent.ChainID
		block.Timestamp = uint64(time.Now().UnixMilli())
		block.ParentID = parent.ID()
		block.ParentView = parent.View
	}
}

func (f *clusterBlockFactory) WithHeight(height uint64) func(*cluster.Block) {
	return func(block *cluster.Block) {
		block.Height = height
	}
}

func (f *clusterBlockFactory) WithChainID(chainID flow.ChainID) func(*cluster.Block) {
	return func(block *cluster.Block) {
		block.ChainID = chainID
	}
}

func (f *clusterBlockFactory) WithProposerID(proposerID flow.Identifier) func(*cluster.Block) {
	return func(block *cluster.Block) {
		block.ProposerID = proposerID
	}
}

func (f *clusterBlockFactory) WithPayload(payload cluster.Payload) func(*cluster.Block) {
	return func(b *cluster.Block) {
		b.Payload = payload
	}
}

func (f *clusterBlockFactory) Genesis() (*cluster.Block, error) {
	headerBody, err := flow.NewRootHeaderBody(flow.UntrustedHeaderBody{
		View:      0,
		ChainID:   "cluster",
		Timestamp: uint64(flow.GenesisTime.UnixMilli()),
		ParentID:  flow.ZeroID,
	})
	if err != nil {
		return nil, err
	}

	payload, err := cluster.NewRootPayload(
		cluster.UntrustedPayload(*cluster.NewEmptyPayload(flow.ZeroID)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create root cluster payload: %w", err)
	}

	block, err := cluster.NewRootBlock(
		cluster.UntrustedBlock{
			HeaderBody: *headerBody,
			Payload:    *payload,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create root cluster block: %w", err)
	}
	return block, nil
}
