package epochs_test

import (
	"context"
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
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	sdktemplates "github.com/onflow/flow-go-sdk/templates"

	"github.com/onflow/flow-core-contracts/lib/go/contracts"
	"github.com/onflow/flow-core-contracts/lib/go/templates"
	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go-sdk/test"

	"github.com/onflow/flow-go/model/cluster"
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

	env        templates.Environment
	blockchain *emulator.Blockchain

	// Quorum Certificate deployed account and address
	qcAddress    sdk.Address
	qcAccountKey *sdk.AccountKey
	qcSigner     sdkcrypto.Signer
}

// SetupTest creates an instance of the emulated chain and deploys the EpochQC contract
func (s *ClusterEpochTestSuite) SetupTest() {

	// create a new instance of the emulated blockchain
	blockchain, err := emulator.NewBlockchain()
	require.NoError(s.T(), err)
	s.blockchain = blockchain

	qcAddress, accountKey, signer := s.DeployEpochQCContract()
	s.qcAddress = qcAddress
	s.qcAccountKey = accountKey
	s.qcSigner = signer
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
	_ = make([][]ClusterNode, clusterCount)

	// mock the epoch object to return counter 0 and clustering as our clusterList
	epoch := &protomock.Epoch{}
	epoch.On("Counter").Return(1, nil)
	epoch.On("Clustering").Return(clustering, nil)

	for _, node := range nodes {
		cluster, _, _ := clustering.ByNodeID(node.NodeID)
		rootBlock := clusterstate.CanonicalRootBlock(1, cluster)
		_ = s.CreateNode(rootBlock, node)
	}

	// sign and publish voter transaction
	publishVoterTx := sdk.NewTransaction().
		SetScript(templates.GeneratePublishVoterScript(s.env)).
		SetGasLimit(100).
		SetProposalKey(s.blockchain.ServiceKey().Address,
			s.blockchain.ServiceKey().Index, s.blockchain.ServiceKey().SequenceNumber).
		SetPayer(s.blockchain.ServiceKey().Address).
		AddAuthorizer(s.qcAddress)
	s.SignAndSubmit(publishVoterTx,
		[]sdk.Address{s.blockchain.ServiceKey().Address, s.qcAddress},
		[]sdkcrypto.Signer{s.blockchain.ServiceKey().Signer(), s.qcSigner})

	// submit admin transaction to start voting
	startVotingTx := sdk.NewTransaction().
		SetScript(templates.GenerateStartVotingScript(s.env)).
		SetGasLimit(100).
		SetProposalKey(s.blockchain.ServiceKey().Address,
			s.blockchain.ServiceKey().Index, s.blockchain.ServiceKey().SequenceNumber).
		SetPayer(s.blockchain.ServiceKey().Address).
		AddAuthorizer(s.qcAddress)

	clusterIndicies := make([]cadence.Value, clusterCount)
	clusterNodeWeights := make([]cadence.Value, clusterCount)
	clusterNodeIDs := make([]cadence.Value, clusterCount)

	// for each cluster add node ids to transaction arguments
	for index, cluster := range clustering {

		// create cadence value
		clusterIndicies = append(clusterIndicies, cadence.NewUInt16(uint16(index)))

		// create list of string node ids
		nodeIDs := make([]cadence.Value, nodeCount/clusterCount)
		nodeWeights := make([]cadence.Value, nodeCount/clusterCount)

		for _, node := range cluster {
			nodeIDs = append(nodeIDs, cadence.NewString(node.NodeID.String()))
			nodeWeights = append(nodeWeights, cadence.NewUInt64(node.Stake))
		}

		clusterNodeIDs[index] = cadence.NewArray(nodeIDs)
		clusterNodeWeights[index] = cadence.NewArray(nodeWeights)
	}

	// add cluster indicies to tx argument
	err := startVotingTx.AddArgument(cadence.NewArray(clusterIndicies))
	require.NoError(s.T(), err)

	// add cluster node ids to tx argument
	err = startVotingTx.AddArgument(cadence.NewArray(clusterNodeIDs))
	require.NoError(s.T(), err)

	// add cluster weight to tx argument
	err = startVotingTx.AddArgument(cadence.NewArray(clusterNodeWeights))
	require.NoError(s.T(), err)

	s.SignAndSubmit(startVotingTx,
		[]sdk.Address{s.blockchain.ServiceKey().Address, s.qcAddress},
		[]sdkcrypto.Signer{s.blockchain.ServiceKey().Signer(), s.qcSigner})

	// TODO: register each node in each cluster to vote
	// https://github.com/onflow/flow-core-contracts/blob/josh/qc/lib/go/test/flow_qc_test.go#L195-L221

	for _, cluster := range clustering {
		for _, node := range cluster {
			registerVoterTx := sdk.NewTransaction().
				SetScript(templates.GenerateCreateVoterScript(s.env)).
				SetGasLimit(100).
				SetProposalKey(s.blockchain.ServiceKey().Address, s.blockchain.ServiceKey().Index, s.blockchain.ServiceKey().SequenceNumber).
				SetPayer(s.blockchain.ServiceKey().Address).
				AddAuthorizer(sdk.HexToAddress(node.Address))

			err = registerVoterTx.AddArgument(cadence.NewAddress(s.qcAddress))
			require.NoError(s.T(), err)

			err = registerVoterTx.AddArgument(cadence.NewString(node.NodeID.String()))
			require.NoError(s.T(), err)

			s.SignAndSubmit(registerVoterTx,
				[]sdk.Address{s.blockchain.ServiceKey().Address, s.qcAddress},
				[]sdkcrypto.Signer{s.blockchain.ServiceKey().Signer(), s.qcSigner},
			)
		}
	}
}

// DeployEpochQCContract deploys the `EpochQC` contract to the emulated chain and returns the
// Account key used along with the signer and the environment with the QC address set
func (s *ClusterEpochTestSuite) DeployEpochQCContract() (sdk.Address, *sdk.AccountKey, sdkcrypto.Signer) {

	// create new account keys for the Quorum Certificate account
	QCAccountKey, QCSigner := test.AccountKeyGenerator().NewWithSigner()
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

	return QCAddress, QCAccountKey, QCSigner
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

func (s *ClusterEpochTestSuite) CreateNode(rootBlock *cluster.Block, me *flow.Identity) *ClusterNode {

	key, signer := test.AccountKeyGenerator().NewWithSigner()

	// create account on emualted chain
	address, err := s.blockchain.CreateAccount([]*sdk.AccountKey{key}, []sdktemplates.Contract{})
	require.NoError(s.T(), err)
	me.Address = address.String()

	// create a mock of QCContractClient to submit vote and check if voted on the emulated chain
	// TODO: we need to mock the underlying flowClient instead of the QCContract client
	rpcClient := client.MockRPCClient{}

	client := epochs.NewQCContractClient()
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

	// TODO: return hotstuff.Vote object
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
		Address: sdk.HexToAddress(me.Address),
		Voter:   voter,
	}

	return node
}

/**
Methods below are from the flow-core-contracts repo
**/

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

func (s *ClusterEpochTestSuite) SignAndSubmit(tx *sdk.Transaction, signerAddresses []sdk.Address, signers []sdkcrypto.Signer) {

	// sign transaction with each signer
	for i := len(signerAddresses) - 1; i >= 0; i-- {
		signerAddress := signerAddresses[i]
		signer := signers[i]

		if i == 0 {
			err := tx.SignEnvelope(signerAddress, 0, signer)
			assert.NoError(s.T(), err)
		} else {
			err := tx.SignPayload(signerAddress, 0, signer)
			assert.NoError(s.T(), err)
		}
	}

	// submit transaction
	s.Submit(tx)
}
