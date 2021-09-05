package follower

import (
	"fmt"

	"github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/cmd/access/node_builder"
	access "github.com/onflow/flow-go/cmd/access/node_builder"
	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/module"
	finalizer "github.com/onflow/flow-go/module/finalizer/consensus"
)

type consensusFollowerWithoutVerifer struct {
	*ConsensusFollowerImpl
}


func newConsensusFollowerWithoutVerifer(
	networkPrivKey crypto.PrivateKey,
	bindAddr string,
	bootstapIdentities []BootstrapNodeInfo,
	opts ...Option,
) (*consensusFollowerWithoutVerifer, error) {
	config := &Config{
		networkPrivKey: networkPrivKey,
		bootstrapNodes: bootstapIdentities,
		bindAddr:       bindAddr,
	}

	for _, opt := range opts {
		opt(config)
	}

	accessNodeOptions := getAccessNodeOptions(config)

	anb := build(accessNodeOptions)
	consensusFollower := &ConsensusFollowerImpl{NodeBuilder: anb}
	anb.BaseConfig.NodeRole = "consensus_follower"

	anb.FinalizationDistributor.AddOnBlockFinalizedConsumer(consensusFollower.onBlockFinalized)

	consensusFollowerWithoutVerifer := &consensusFollowerWithoutVerifer{consensusFollower}
	return consensusFollowerWithoutVerifer, nil
}

func build(accessNodeOptions []access.Option) *access.UnstakedAccessNodeBuilder {
	anb := access.FlowAccessNode(accessNodeOptions...)
	nodeBuilder := access.NewUnstakedAccessNodeBuilder(anb)

	nodeBuilder.Initialize()
	buildConsensusFollower(nodeBuilder.FlowAccessNodeBuilder)

	return nodeBuilder
}

func buildConsensusFollower(anb *node_builder.FlowAccessNodeBuilder) {
	anb.
		BuildFollowerState().
		BuildSyncCore().
		BuildCommittee().
		BuildLatestHeader()

		buildFollowerCore(anb)

	anb.BuildFollowerEngine().
		BuildFinalizedHeader().
		BuildSyncEngine()
}

func buildFollowerCore(anb *node_builder.FlowAccessNodeBuilder)  {
	anb.Component("follower core", func(_ cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
		// create a finalizer that will handle updating the protocol
		// state when the follower detects newly finalized blocks

		//final := new(mockfinalizer.Finalizer)
		//final.On("MakeValid", mock.Anything).Return(nil)
		//final.On("OnBlockIncorporated", mock.Anything).Return()
		final := finalizer.NewFinalizer(node.DB, node.Storage.Headers, anb.FollowerState)

		// initialize the verifier for the protocol consensus
		verifier := new(mocks.Verifier)
		verifier.On("VerifyVote", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
		verifier.On("VerifyQC", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

		followerCore, err := consensus.NewFollower(node.Logger, anb.Committee, node.Storage.Headers, final, verifier,
			anb.FinalizationDistributor, node.RootBlock.Header, node.RootQC, anb.Finalized, anb.Pending)
		if err != nil {
			return nil, fmt.Errorf("could not initialize follower core: %w", err)
		}
		anb.FollowerCore = followerCore

		return anb.FollowerCore, nil
	})
}
