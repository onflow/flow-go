package epochs

import (
	"context"
	"encoding/hex"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-core-contracts/lib/go/templates"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
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

	// create initial cluster map
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

	err = s.CreateVoterResource(clustering)
	require.NoError(s.T(), err)

	for _, node := range clusterNodes {
		err = node.Voter.Vote(context.Background(), epoch)
		require.NoError(s.T(), err)
	}

	// TODO: submit endvoting transaction from admin resource

	// TODO: check contract and see if required results are there
}

// CreateClusterNode ...
func (s *ClusterEpochTestSuite) CreateClusterNode(rootBlock *cluster.Block, me *flow.Identity) *ClusterNode {

	key, signer := test.AccountKeyGenerator().NewWithSigner()

	// create account on emualted chain
	address, err := s.blockchain.CreateAccount([]*sdk.AccountKey{key}, []sdktemplates.Contract{})
	require.NoError(s.T(), err)

	// create a mock of QCContractClient to submit vote and check if voted on the emulated chain
	// TODO: we need to mock the underlying flowClient instead of the QCContract client
	rpcClient := MockRPCClient{}
	rpcClient.On("GetAccount", mock.Anything, mock.Anything).Return(func(_ context.Context, address sdk.Address) {
	})

	client := client.NewFromRPCClient(rpcClient)
	client.On("Voted", mock.Anything).Return(func(_ context.Context) bool {

		// execute a script to read if the node has voted and return true or false
		argument := [][]byte{jsoncdc.MustEncode(cadence.String(me.NodeID.String()))}
		result, err := s.blockchain.ExecuteScript(templates.GenerateGetVotingCompletedScript(s.env), argument)
		require.NoError(s.T(), err)
		assert.True(s.T(), result.Succeeded())

		// convert from cadence type to go and return result as bool
		hasVoted := result.Value.ToGoValue().(bool)
		return hasVoted
	})
	client.On("SubmitVote", mock.Anything).Return(func(_ context.Context, vote hotstuffmodel.Vote) error {
		// address := sdk.HexToAddress(me.Address)

		tx := sdk.NewTransaction().
			SetScript(templates.GenerateSubmitVoteScript(s.env)).
			SetGasLimit(1000).
			SetProposalKey(s.blockchain.ServiceKey().Address,
				s.blockchain.ServiceKey().Index,
				s.blockchain.ServiceKey().SequenceNumber).
			SetPayer(s.blockchain.ServiceKey().Address).
			AddAuthorizer(address)

		err := tx.AddArgument(cadence.NewString(hex.EncodeToString(vote.SigData)))
		require.NoError(s.T(), err)

		s.SignAndSubmit(tx,
			[]sdk.Address{s.blockchain.ServiceKey().Address, address},
			[]sdkcrypto.Signer{s.blockchain.ServiceKey().Signer(), signer})

		return nil
	})

	local := &modulemock.Local{}
	local.On("NodeID").Return(me.NodeID)

	vote := hotstuffmodel.VoteFromFlow(me.NodeID, unittest.IdentifierFixture(), 0, unittest.SignatureFixture())

	hotSigner := &hotstuff.Signer{}
	hotSigner.On("CreateVote", mock.Anything).Return(vote, nil)

	snapshot := &protomock.Snapshot{}
	snapshot.On("Phase").Return(flow.EpochPhaseSetup, nil)

	// TODO: create a canonical root block
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
	}

	return node
}
