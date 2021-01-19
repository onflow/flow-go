package integration_test

import (
	"encoding/hex"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	emulator "github.com/onflow/flow-emulator"
	sdk "github.com/onflow/flow-go-sdk"
	sdktemplates "github.com/onflow/flow-go-sdk/templates"

	"github.com/onflow/flow-core-contracts/lib/go/contracts"
	"github.com/onflow/flow-core-contracts/lib/go/templates"
	"github.com/onflow/flow-go-sdk/test"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/utils/unittest"

	hotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	hotstuffmodel "github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/module/epochs"
	modulemock "github.com/onflow/flow-go/module/mock"
	clusterstate "github.com/onflow/flow-go/state/cluster"
	protomock "github.com/onflow/flow-go/state/protocol/mock"
)

type ClusterNode struct {
	NodeID  flow.Identifier
	Key     *sdk.AccountKey
	Address sdk.Address
	Voter   module.ClusterRootQCVoter
}

// ClusterEpochTestSuite ...
type ClusterEpochTestSuite struct {
	suite.Suite

	env          templates.Environment
	blockchain   *emulator.Blockchain
	qcAddress    sdk.Address
	qcAccountKey *sdk.AccountKey
}

// SetupTest creates an instance of the emulated chain and deploys the EpochQC contract
func (s *ClusterEpochTestSuite) SetupTest() {

	// create a new instance of the emulated blockchain
	blockchain, err := emulator.NewBlockchain()
	require.NoError(s.T(), err)
	s.blockchain = blockchain

	qcAddress, accountKey := s.DeployEpochQCContract()
	s.qcAddress = qcAddress
	s.qcAccountKey = accountKey
}

// TestQuroumCertificate tests one Epoch of the EpochClusterQC contract
func (s *ClusterEpochTestSuite) TestQuroumCertificate() {

	// initial cluster and total node count
	clusterCount := 3
	nodeCount := 30

	s.SetupTest()

	// create clustering with x clusters with x*y nodes
	clustering, nodes := s.CreateClusterList(clusterCount, nodeCount)

	// create initial cluster map
	_ = make([][]ClusterNode, clusterCount)

	// mock the epoch object to return counter 0 and clustering as our clusterList
	epoch := &protomock.Epoch{}
	epoch.On("Counter").Return(1, nil)
	epoch.On("Clustering").Return(clustering, nil)

	for _, node := range nodes {
		cluster, _, _ := clustering.ByNodeID(node.NodeID)
		_ = s.CreateNode(1, cluster, node)
	}

	// submit admin transaction to start voting
}

// DeployEpochQCContract deploys the `EpochQC` contract to the emulated chain and returns the
// Account key used along with the signer and the environment with the QC address set
func (s *ClusterEpochTestSuite) DeployEpochQCContract() (sdk.Address, *sdk.AccountKey) {
	// TODO: this method needs to return the QCSigner as it will be used to start the voting

	// create new account keys for the Quorum Certificate account
	QCAccountKey, _ := test.AccountKeyGenerator().NewWithSigner()
	QCCode := contracts.FlowQC()

	// deploy the contract to the emulator
	QCAddress, err := s.blockchain.CreateAccount([]*sdk.AccountKey{QCAccountKey}, []sdktemplates.Contract{
		{
			Name:   "FlowEpochClusterQC",
			Source: string(QCCode),
		},
	})
	require.NoError(s.T(), err)

	env := templates.Environment{
		QuorumCertificateAddress: QCAddress.Hex(),
	}
	s.env = env

	return QCAddress, QCAccountKey
}

// CreateClusterList creates a clustering with the nodes split evenly and returns the resulting `ClusterList`
func (s *ClusterEpochTestSuite) CreateClusterList(clusterCount, nodeCount int) (flow.ClusterList, flow.IdentityList) {

	// create list of nodes to be used for the clustering
	nodes := unittest.IdentityListFixture(nodeCount, unittest.WithRole(flow.RoleCollection))
	// create cluster assignment
	clusterAssignment := unittest.ClusterAssignment(uint(clusterCount), nodes)

	// create `ClusterList` object from nodes and assignment
	clusterList, err := flow.NewClusterList(clusterAssignment, nodes)
	require.NoError(s.T(), err)

	return clusterList, nodes
}

func (s *ClusterEpochTestSuite) CreateNode(epoch uint64, cluster flow.IdentityList, me *flow.Identity) *ClusterNode {

	key, signer := test.AccountKeyGenerator().NewWithSigner()

	// create account on emualted chain
	// address, err := s.blockchain.CreateAccount([]*sdk.AccountKey{key}, []sdktemplates.Contract{})
	// require.NoError(s.T(), err)

	// create a mock of QCContractClient to submit vote and check if voted on the emulated chain
	client := &modulemock.QCContractClient{}

	// mock `Voted`
	mockVoted := client.On("Voted", mock.Anything)
	mockVoted.RunFn = func(args mock.Arguments) {

		// execute a script to read if the node has voted and return true or false
		argument := [][]byte{jsoncdc.MustEncode(cadence.String(me.NodeID.String()))}
		result, err := s.blockchain.ExecuteScript(templates.GenerateGetVotingCompletedScript(s.env), argument)
		require.NoError(s.T(), err)
		assert.True(s.T(), result.Succeeded())

		// convert from cadence type to go and return result as bool
		hasVoted := result.Value.ToGoValue().(bool)
		mockVoted.ReturnArguments = mock.Arguments{hasVoted}
	}

	// mock `SubmitVote`
	mockSubmitVote := client.On("SubmitVote", mock.Anything)
	mockSubmitVote.RunFn = func(args mock.Arguments) {

		vote := args[0].(hotstuffmodel.Vote)
		address := sdk.HexToAddress(me.Address)

		tx := sdk.NewTransaction().
			SetScript(templates.GenerateSubmitVoteScript(s.env)).
			SetGasLimit(1000).
			SetProposalKey(s.blockchain.ServiceKey().Address,
				s.blockchain.ServiceKey().Index,
				s.blockchain.ServiceKey().SequenceNumber).
			SetPayer(s.blockchain.ServiceKey().Address).
			AddAuthorizer(s.qcAddress)

		err := tx.AddArgument(cadence.NewString(hex.EncodeToString(vote.SigData)))
		require.NoError(s.T(), err)

		// TODO: needs to be signed by service account and qc address
		err = tx.SignPayload(address, 0, signer)
		require.NoError(s.T(), err)

		// submit transaction
		s.Submit(tx)

		// return error as nil
		mockVoted.ReturnArguments = mock.Arguments{nil}
	}

	local := &modulemock.Local{}
	local.On("NodeID").Return(me.NodeID)

	// TODO: return hotstuff.Vote object
	vote := hotstuffmodel.VoteFromFlow(me.NodeID, unittest.IdentifierFixture(), 0, unittest.SignatureFixture())

	hotSigner := &hotstuff.Signer{}
	hotSigner.On("CreateVote", mock.Anything).Return(vote, nil)

	snapshot := &protomock.Snapshot{}
	snapshot.On("Phase").Return(flow.EpochPhaseSetup, nil)

	// TODO: create a canonical root block
	rootBlock := clusterstate.CanonicalRootBlock(epoch, cluster)
	state := &protomock.State{}
	state.On("CanonicalRootBlock").Return(rootBlock)
	state.On("Final").Return(snapshot)

	// create QC voter object to be used for voting for the root QC contract
	voter := epochs.NewRootQCVoter(zerolog.Logger{}, local, hotSigner, state, client)

	// create node and set node
	node := &ClusterNode{
		NodeID:  me.NodeID,
		Key:     key,
		Address: sdk.HexToAddress(me.Address),
		Voter:   voter,
	}

	return node
}

func (s *ClusterEpochTestSuite) Submit(tx *sdk.Transaction) {
	// submit the signed transaction
	err := s.blockchain.AddTransaction(*tx)
	require.NoError(s.T(), err)

	result, err := s.blockchain.ExecuteNextTransaction()
	require.NoError(s.T(), err)

	if !assert.True(s.T(), result.Succeeded()) {
		// TODO: handle error in submitting tx
	}

	_, err = s.blockchain.CommitBlock()
	assert.NoError(s.T(), err)
}
