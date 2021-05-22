package epochs

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

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

func TestClusterEpoch(t *testing.T) {
	suite.Run(t, new(Suite))
}

// TestQuroumCertificate tests one Epoch of the EpochClusterQC contract
func (s *Suite) TestEpochQuorumCertificate() {

	// initial cluster and total node count
	epochCounter := uint64(1)
	clusterCount := 3
	nodesPerCluster := 10

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

		client := epochs.NewQCContractClient(zerolog.Nop(), s.emulatorClient, nodeID, address.String(), 0, s.qcAddress.String(), signer)
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

		// create voter resource
		s.CreateVoterResource(address, nodeID, signer)

		// cast vote
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

// CreateClusterNode ...
func (s *Suite) CreateClusterNode(rootBlock *cluster.Block, me *flow.Identity) *ClusterNode {

	key, signer := test.AccountKeyGenerator().NewWithSigner()

	// create account on emualted chain
	address, err := s.blockchain.CreateAccount([]*sdk.AccountKey{key}, []sdktemplates.Contract{})
	require.NoError(s.T(), err)

	client := epochs.NewQCContractClient(zerolog.Nop(), s.emulatorClient, me.NodeID, address.String(), 0, s.qcAddress.String(), signer)
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
