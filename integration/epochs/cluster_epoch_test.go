package epochs

import (
	"encoding/hex"
	"fmt"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	emulator "github.com/onflow/flow-emulator"

	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"

	sdktemplates "github.com/onflow/flow-go-sdk/templates"
	emulatormod "github.com/onflow/flow-go/module/emulator"

	"github.com/onflow/flow-core-contracts/lib/go/contracts"
	"github.com/onflow/flow-core-contracts/lib/go/templates"

	"github.com/onflow/flow-go-sdk/test"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// Suite tests the quorum certificate voting process against the
// QCAggregator contract running on the emulator.
type Suite struct {
	suite.Suite

	env            templates.Environment
	blockchain     *emulator.Blockchain
	emulatorClient *emulatormod.EmulatorClient

	// Quorum Certificate deployed account and address
	qcAddress    sdk.Address
	qcAccountKey *sdk.AccountKey
	qcSigner     sdkcrypto.Signer
}

// SetupTest creates an instance of the emulated chain and deploys the EpochQC contract
func (s *Suite) SetupTest() {

	// create a new instance of the emulated blockchain
	var err error
	s.blockchain, err = emulator.NewBlockchain(emulator.WithStorageLimitEnabled(false))
	s.Require().NoError(err)
	s.emulatorClient = emulatormod.NewEmulatorClient(s.blockchain)

	// deploy epoch qc contract
	s.deployEpochQCContract()
}

// deployEpochQCContract deploys the `EpochQC` contract to the emulated chain and sets the
// Account key used along with the signer and the environment with the QC address
func (s *Suite) deployEpochQCContract() {

	// create new account keys for the Quorum Certificate account
	QCAccountKey, QCSigner := test.AccountKeyGenerator().NewWithSigner()
	QCCode := contracts.FlowQC()

	// deploy the contract to the emulator
	QCAddress, err := s.blockchain.CreateAccount([]*sdk.AccountKey{QCAccountKey}, []sdktemplates.Contract{
		{
			Name:   "FlowClusterQC",
			Source: string(QCCode),
		},
	})
	s.Require().NoError(err)

	env := templates.Environment{
		QuorumCertificateAddress: QCAddress.Hex(),
	}
	s.env = env
	s.qcAddress = QCAddress
	s.qcAccountKey = QCAccountKey
	s.qcSigner = QCSigner
}

// CreateClusterList creates a clustering with the nodes split evenly and returns the resulting `ClusterList`
func (s *Suite) CreateClusterList(clusterCount, nodesPerCluster int) (flow.ClusterList, flow.IdentityList) {

	// create list of nodes to be used for the clustering
	nodes := unittest.IdentityListFixture(clusterCount*nodesPerCluster, unittest.WithRole(flow.RoleCollection))
	// create cluster assignment
	clusterAssignment := unittest.ClusterAssignment(uint(clusterCount), nodes)

	// create `ClusterList` object from nodes and assignment
	clusterList, err := flow.NewClusterList(clusterAssignment, nodes)
	s.Require().NoError(err)

	return clusterList, nodes
}

// PublishVoter publishes the Voter resource to a set path in candence
func (s *Suite) PublishVoter() {

	// sign and publish voter transaction
	publishVoterTx := sdk.NewTransaction().
		SetScript(templates.GeneratePublishVoterScript(s.env)).
		SetGasLimit(9999).
		SetProposalKey(s.blockchain.ServiceKey().Address,
			s.blockchain.ServiceKey().Index, s.blockchain.ServiceKey().SequenceNumber).
		SetPayer(s.blockchain.ServiceKey().Address).
		AddAuthorizer(s.qcAddress)

	s.SignAndSubmit(publishVoterTx,
		[]sdk.Address{s.blockchain.ServiceKey().Address, s.qcAddress},
		[]sdkcrypto.Signer{s.blockchain.ServiceKey().Signer(), s.qcSigner})
}

// StartVoting starts the voting in the EpochQCContract with the admin resource
// for a specific clustering
func (s *Suite) StartVoting(clustering flow.ClusterList, clusterCount, nodesPerCluster int) {
	// submit admin transaction to start voting
	startVotingTx := sdk.NewTransaction().
		SetScript(templates.GenerateStartVotingScript(s.env)).
		SetGasLimit(9999).
		SetProposalKey(s.blockchain.ServiceKey().Address,
			s.blockchain.ServiceKey().Index, s.blockchain.ServiceKey().SequenceNumber).
		SetPayer(s.blockchain.ServiceKey().Address).
		AddAuthorizer(s.qcAddress)

	clusterIndices := make([]cadence.Value, 0, clusterCount)
	clusterNodeWeights := make([]cadence.Value, clusterCount)
	clusterNodeIDs := make([]cadence.Value, clusterCount)

	// for each cluster add node ids to transaction arguments
	for index, cluster := range clustering {

		// create cadence value
		clusterIndices = append(clusterIndices, cadence.NewUInt16(uint16(index)))

		// create list of string node ids
		nodeIDs := make([]cadence.Value, 0, nodesPerCluster)
		nodeWeights := make([]cadence.Value, 0, nodesPerCluster)

		for _, node := range cluster {
			cdcNodeID, err := cadence.NewString(node.NodeID.String())
			require.NoError(s.T(), err)
			nodeIDs = append(nodeIDs, cdcNodeID)
			nodeWeights = append(nodeWeights, cadence.NewUInt64(node.Weight))
		}

		clusterNodeIDs[index] = cadence.NewArray(nodeIDs)
		clusterNodeWeights[index] = cadence.NewArray(nodeWeights)
	}

	// add cluster indicies to tx argument
	err := startVotingTx.AddArgument(cadence.NewArray(clusterIndices))
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
}

// CreateVoterResource creates the Voter resource in cadence for a cluster node
func (s *Suite) CreateVoterResource(address sdk.Address, nodeID flow.Identifier, publicStakingKey crypto.PublicKey, nodeSigner sdkcrypto.Signer) {

	registerVoterTx := sdk.NewTransaction().
		SetScript(templates.GenerateCreateVoterScript(s.env)).
		SetGasLimit(9999).
		SetProposalKey(s.blockchain.ServiceKey().Address,
			s.blockchain.ServiceKey().Index, s.blockchain.ServiceKey().SequenceNumber).
		SetPayer(s.blockchain.ServiceKey().Address).
		AddAuthorizer(address)

	err := registerVoterTx.AddArgument(cadence.NewAddress(s.qcAddress))
	require.NoError(s.T(), err)

	cdcNodeID, err := cadence.NewString(nodeID.String())
	require.NoError(s.T(), err)
	err = registerVoterTx.AddArgument(cdcNodeID)
	require.NoError(s.T(), err)

	cdcStakingPubKey, err := cadence.NewString(hex.EncodeToString(publicStakingKey.Encode()))
	require.NoError(s.T(), err)
	err = registerVoterTx.AddArgument(cdcStakingPubKey)
	require.NoError(s.T(), err)

	s.SignAndSubmit(registerVoterTx,
		[]sdk.Address{s.blockchain.ServiceKey().Address, address},
		[]sdkcrypto.Signer{s.blockchain.ServiceKey().Signer(), nodeSigner})
}

func (s *Suite) StopVoting() {
	tx := sdk.NewTransaction().
		SetScript(templates.GenerateStopVotingScript(s.env)).
		SetGasLimit(9999).
		SetProposalKey(s.blockchain.ServiceKey().Address,
			s.blockchain.ServiceKey().Index, s.blockchain.ServiceKey().SequenceNumber).
		SetPayer(s.blockchain.ServiceKey().Address).
		AddAuthorizer(s.qcAddress)

	s.SignAndSubmit(tx,
		[]sdk.Address{s.blockchain.ServiceKey().Address, s.qcAddress},
		[]sdkcrypto.Signer{s.blockchain.ServiceKey().Signer(), s.qcSigner})
}

func (s *Suite) NodeHasVoted(nodeID flow.Identifier) bool {
	result, err := s.blockchain.ExecuteScript(templates.GenerateGetNodeHasVotedScript(s.env),
		[][]byte{jsoncdc.MustEncode(cadence.String(nodeID.String()))})
	require.NoError(s.T(), err)

	if !assert.True(s.T(), result.Succeeded()) {
		return false
	}

	return result.Value.ToGoValue().(bool)
}

/**
Methods below are from the flow-core-contracts repo
**/

func (s *Suite) SignAndSubmit(tx *sdk.Transaction, signerAddresses []sdk.Address, signers []sdkcrypto.Signer) {

	// sign transaction with each signer
	for i := len(signerAddresses) - 1; i >= 0; i-- {
		signerAddress := signerAddresses[i]
		signer := signers[i]

		if i == 0 {
			err := tx.SignEnvelope(signerAddress, 0, signer)
			require.NoError(s.T(), err)
		} else {
			err := tx.SignPayload(signerAddress, 0, signer)
			require.NoError(s.T(), err)

		}
	}

	// submit transaction
	_, err := s.emulatorClient.Submit(tx)
	require.NoError(s.T(), err)
}

// This function initializes Cluster records in order to pass the cluster information
// as an argument to the startVoting transaction
func initClusters(clusterNodeIDStrings [][]string, numberOfClusters, numberOfNodesPerCluster int) [][]cadence.Value {
	clusterIndices := make([]cadence.Value, numberOfClusters)
	clusterNodeIDs := make([]cadence.Value, numberOfClusters)
	clusterNodeWeights := make([]cadence.Value, numberOfClusters)

	for i := 0; i < numberOfClusters; i++ {

		clusterIndices[i] = cadence.NewUInt16(uint16(i))

		nodeIDs := make([]cadence.Value, numberOfNodesPerCluster)
		nodeWeights := make([]cadence.Value, numberOfNodesPerCluster)

		for j := 0; j < numberOfNodesPerCluster; j++ {
			nodeID := fmt.Sprintf("%064d", i*numberOfNodesPerCluster+j)
			cdcNodeID, err := cadence.NewString(nodeID)
			if err != nil {
				panic(err)
			}
			nodeIDs[j] = cdcNodeID

			// default weight per node
			nodeWeights[j] = cadence.NewUInt64(uint64(100))

		}

		clusterNodeIDs[i] = cadence.NewArray(nodeIDs)
		clusterNodeWeights[i] = cadence.NewArray(nodeWeights)

	}

	return [][]cadence.Value{clusterIndices, clusterNodeIDs, clusterNodeWeights}
}
