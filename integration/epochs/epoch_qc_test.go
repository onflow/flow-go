package epochs

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	sdk "github.com/onflow/flow-go-sdk"
	sdktemplates "github.com/onflow/flow-go-sdk/templates"
	"github.com/onflow/flow-go-sdk/test"

	hotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	hotstuffmodel "github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/epochs"
	modulemock "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/utils/unittest"

	clusterstate "github.com/onflow/flow-go/state/cluster"
	protomock "github.com/onflow/flow-go/state/protocol/mock"
)

// TestClusterQuorumCertificate tests one Epoch of the EpochClusterQC contract
func (s *ClusterEpochTestSuite) TestClusterQuorumCertificate() {

	// initial cluster and total node count
	epochCounter := uint64(1)
	clusterCount := 3
	nodesPerCluster := 1

	s.SetupTest()

	// create clustering with x clusters with x*y nodes
	clustering, nodes := s.CreateClusterList(clusterCount, nodesPerCluster)

	// mock the epoch object to return counter 0 and clustering as our clusterList
	epoch := &protomock.Epoch{}
	epoch.On("Counter").Return(epochCounter, nil)
	epoch.On("Clustering").Return(clustering, nil)

	s.PublishVoter()
	s.StartVoting(clustering, clusterCount, nodesPerCluster)

	// create cluster nodes with voter resource
	for _, node := range nodes {
		nodeID := node.NodeID

		// find cluster and create root block
		cluster, _, _ := clustering.ByNodeID(node.NodeID)
		rootBlock := clusterstate.CanonicalRootBlock(uint64(epochCounter), cluster)

		key, signer := test.AccountKeyGenerator().NewWithSigner()

		// create account on emualted chain
		address, err := s.blockchain.CreateAccount([]*sdk.AccountKey{key}, []sdktemplates.Contract{})
		require.NoError(s.T(), err)

		client, err := epochs.NewQCContractClient(s.emulatorClient, nodeID, address.String(), 0, s.qcAddress.String(), signer)
		require.NoError(s.T(), err)

		local := &modulemock.Local{}
		local.On("NodeID").Return(nodeID)

		vote := hotstuffmodel.VoteFromFlow(nodeID, unittest.IdentifierFixture(), 0, unittest.SignatureFixture())
		hotSigner := &hotstuff.Signer{}
		hotSigner.On("CreateVote", mock.Anything).Return(vote, nil)

		snapshot := &protomock.Snapshot{}
		snapshot.On("Phase").Return(flow.EpochPhaseSetup, nil)

		state := &protomock.State{}
		state.On("CanonicalRootBlock").Return(rootBlock)
		state.On("Final").Return(snapshot)

		// create QC voter object to be used for voting for the root QC contract
		voter := epochs.NewRootQCVoter(zerolog.Logger{}, local, hotSigner, state, client)

		fmt.Printf("creating voter resource: %s", nodeID)
		s.CreateVoterResource(address, nodeID, signer)

		// cast vote
		fmt.Printf("cast vote: %s", nodeID)
		err = voter.Vote(context.Background(), epoch)
		require.NoError(s.T(), err)
	}

	// stop voting
	s.StopVoting()

	// check if each node has voted
	for _, node := range nodes {
		hasVoted := s.NodeHasVoted(node.NodeID)
		assert.True(s.T(), hasVoted)
	}
}
