package epochs

import (
	"context"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	sdktemplates "github.com/onflow/flow-go-sdk/templates"
	"github.com/onflow/flow-go-sdk/test"

	hotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	hotstuffmodel "github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/epochs"
	modulemock "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/utils/unittest"

	clusterstate "github.com/onflow/flow-go/state/cluster"
	protomock "github.com/onflow/flow-go/state/protocol/mock"
)

type ClusterNode struct {
	NodeID  flow.Identifier
	Key     *sdk.AccountKey
	Address sdk.Address
	Signer  sdkcrypto.Signer
	Voter   module.ClusterRootQCVoter
}

// TestQuroumCertificate tests one Epoch of the EpochClusterQC contract
func (s *ClusterEpochTestSuite) TestQuorumCertificate() {

	// initial cluster and total node count
	clusterCount := 3
	nodeCount := 30

	s.SetupTest()

	// create clustering with x clusters with x*y nodes
	clustering, nodes := s.CreateClusterList(clusterCount, nodeCount)

	// create initial ClusterNodes list
	clusterNodes := make([]*ClusterNode, clusterCount)

	// mock the epoch object to return counter 0 and clustering as our clusterList
	epoch := &protomock.Epoch{}
	epoch.On("Counter").Return(1, nil)
	epoch.On("Clustering").Return(clustering, nil)

	for _, node := range nodes {

		// find cluster and create root block
		cluster, _, _ := clustering.ByNodeID(node.NodeID)
		rootBlock := clusterstate.CanonicalRootBlock(1, cluster)

		clusterNode := s.CreateClusterNode(rootBlock, node)
		clusterNodes = append(clusterNodes, clusterNode)
	}

	err := s.PublishVoter()
	require.NoError(s.T(), err)

	err = s.StartVoting(clustering, clusterCount, nodeCount)
	require.NoError(s.T(), err)

	err = s.CreateVoterResource(clusterNodes)
	require.NoError(s.T(), err)

	// cast vote to qc contract for each node
	for _, node := range clusterNodes {
		err = node.Voter.Vote(context.Background(), epoch)
		require.NoError(s.T(), err)
	}

	err = s.StopVoting()
	require.NoError(s.T(), err)

	// check if each node has voted
	for _, node := range clusterNodes {
		hasVoted, err := s.NodeHasVoted(node.NodeID)
		require.NoError(s.T(), err)

		assert.True(s.T(), hasVoted)
	}
}

// CreateClusterNode ...
func (s *ClusterEpochTestSuite) CreateClusterNode(rootBlock *cluster.Block, me *flow.Identity) *ClusterNode {

	key, signer := test.AccountKeyGenerator().NewWithSigner()

	// create account on emualted chain
	address, err := s.blockchain.CreateAccount([]*sdk.AccountKey{key}, []sdktemplates.Contract{})
	require.NoError(s.T(), err)

	client, err := epochs.NewQCContractClient(s.emulatorClient, me.NodeID, address.String(), 0, s.qcAddress.String(), signer)
	require.NoError(s.T(), err)

	local := &modulemock.Local{}
	local.On("NodeID").Return(me.NodeID)

	vote := hotstuffmodel.VoteFromFlow(me.NodeID, unittest.IdentifierFixture(), 0, unittest.SignatureFixture())

	hotSigner := &hotstuff.Signer{}
	hotSigner.On("CreateVote", mock.Anything).Return(vote, nil)

	snapshot := &protomock.Snapshot{}
	snapshot.On("Phase").Return(flow.EpochPhaseSetup, nil)

	state := &protomock.State{}
	state.On("CanonicalRootBlock").Return(rootBlock)
	state.On("Final").Return(snapshot)

	// create QC voter object to be used for voting for the root QC contract
	voter := epochs.NewRootQCVoter(zerolog.Logger{}, local, hotSigner, state, client)

	// create node and set node
	node := &ClusterNode{
		NodeID:  me.NodeID,
		Key:     key,
		Address: address,
		Voter:   voter,
		Signer:  signer,
	}

	return node
}
