package unittest

import (
	"fmt"
	"time"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

var ClusterBlock clusterBlockFactory

type clusterBlockFactory struct{}

func ClusterBlockFixture(opts ...func(*cluster.UnsignedBlock)) *cluster.UnsignedBlock {
	block := &cluster.UnsignedBlock{
		HeaderBody: HeaderBodyFixture(),
		Payload:    *ClusterPayloadFixture(3),
	}
	for _, opt := range opts {
		opt(block)
	}
	return block
}

func (f *clusterBlockFactory) WithParent(parent *cluster.UnsignedBlock) func(*cluster.UnsignedBlock) {
	return func(block *cluster.UnsignedBlock) {
		block.Height = parent.Height + 1
		block.View = parent.View + 1
		block.ChainID = parent.ChainID
		block.Timestamp = uint64(time.Now().UnixMilli())
		block.ParentID = parent.ID()
		block.ParentView = parent.View
	}
}

func (f *clusterBlockFactory) WithHeight(height uint64) func(*cluster.UnsignedBlock) {
	return func(block *cluster.UnsignedBlock) {
		block.Height = height
	}
}

func (f *clusterBlockFactory) WithChainID(chainID flow.ChainID) func(*cluster.UnsignedBlock) {
	return func(block *cluster.UnsignedBlock) {
		block.ChainID = chainID
	}
}

func (f *clusterBlockFactory) WithProposerID(proposerID flow.Identifier) func(*cluster.UnsignedBlock) {
	return func(block *cluster.UnsignedBlock) {
		block.ProposerID = proposerID
	}
}

func (f *clusterBlockFactory) WithPayload(payload cluster.Payload) func(*cluster.UnsignedBlock) {
	return func(b *cluster.UnsignedBlock) {
		b.Payload = payload
	}
}

func (f *clusterBlockFactory) Genesis() (*cluster.UnsignedBlock, error) {
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
		cluster.UntrustedUnsignedBlock{
			HeaderBody: *headerBody,
			Payload:    *payload,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create root cluster block: %w", err)
	}
	return block, nil
}
