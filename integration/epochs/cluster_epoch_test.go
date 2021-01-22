package epochs

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/cadence"
	emulator "github.com/onflow/flow-emulator"
	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	sdktemplates "github.com/onflow/flow-go-sdk/templates"

	"github.com/onflow/flow-core-contracts/lib/go/contracts"
	"github.com/onflow/flow-core-contracts/lib/go/templates"
	"github.com/onflow/flow-go-sdk/test"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

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

	qcAddress, accountKey, signer := s.deployEpochQCContract()
	s.qcAddress = qcAddress
	s.qcAccountKey = accountKey
	s.qcSigner = signer
}

// deployEpochQCContract deploys the `EpochQC` contract to the emulated chain and returns the
// Account key used along with the signer and the environment with the QC address set
func (s *ClusterEpochTestSuite) deployEpochQCContract() (sdk.Address, *sdk.AccountKey, sdkcrypto.Signer) {

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

// PublishVoter publishes the Voter resource to a set path in candence
func (s *ClusterEpochTestSuite) PublishVoter() error {

	// sign and publish voter transaction
	publishVoterTx := sdk.NewTransaction().
		SetScript(templates.GeneratePublishVoterScript(s.env)).
		SetGasLimit(100).
		SetProposalKey(s.blockchain.ServiceKey().Address,
			s.blockchain.ServiceKey().Index, s.blockchain.ServiceKey().SequenceNumber).
		SetPayer(s.blockchain.ServiceKey().Address).
		AddAuthorizer(s.qcAddress)

	return s.SignAndSubmit(publishVoterTx,
		[]sdk.Address{s.blockchain.ServiceKey().Address, s.qcAddress},
		[]sdkcrypto.Signer{s.blockchain.ServiceKey().Signer(), s.qcSigner})
}

// StartVoting starts the voting in the EpochQCContract with the admin resource
// for a specific clustering
func (s *ClusterEpochTestSuite) StartVoting(clustering flow.ClusterList, clusterCount, nodeCount int) error {
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
	if err != nil {
		return nil
	}

	// add cluster node ids to tx argument
	err = startVotingTx.AddArgument(cadence.NewArray(clusterNodeIDs))
	if err != nil {
		return nil
	}

	// add cluster weight to tx argument
	err = startVotingTx.AddArgument(cadence.NewArray(clusterNodeWeights))
	if err != nil {
		return nil
	}

	return s.SignAndSubmit(startVotingTx,
		[]sdk.Address{s.blockchain.ServiceKey().Address, s.qcAddress},
		[]sdkcrypto.Signer{s.blockchain.ServiceKey().Signer(), s.qcSigner})
}

// CreateVoterResource creates the Voter resource in cadence for a cluster node
func (s *ClusterEpochTestSuite) CreateVoterResource(clustering flow.ClusterList) error {

	for _, cluster := range clustering {
		for _, node := range cluster {
			registerVoterTx := sdk.NewTransaction().
				SetScript(templates.GenerateCreateVoterScript(s.env)).
				SetGasLimit(100).
				SetProposalKey(s.blockchain.ServiceKey().Address,
					s.blockchain.ServiceKey().Index, s.blockchain.ServiceKey().SequenceNumber).
				SetPayer(s.blockchain.ServiceKey().Address).
				AddAuthorizer(sdk.HexToAddress(node.Address))

			err := registerVoterTx.AddArgument(cadence.NewAddress(s.qcAddress))
			if err != nil {
				return err
			}

			err = registerVoterTx.AddArgument(cadence.NewString(node.NodeID.String()))
			if err != nil {
				return err
			}

			err = s.SignAndSubmit(registerVoterTx,
				[]sdk.Address{s.blockchain.ServiceKey().Address, s.qcAddress},
				[]sdkcrypto.Signer{s.blockchain.ServiceKey().Signer(), s.qcSigner})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *ClusterEpochTestSuite) StopVoting() error {
	tx := sdk.NewTransaction().
		SetScript(templates.GenerateStopVotingScript(s.env)).
		SetGasLimit(100).
		SetProposalKey(s.blockchain.ServiceKey().Address,
			s.blockchain.ServiceKey().Index, s.blockchain.ServiceKey().SequenceNumber).
		SetPayer(s.blockchain.ServiceKey().Address).
		AddAuthorizer(s.qcAddress)

	return s.SignAndSubmit(tx,
		[]sdk.Address{s.blockchain.ServiceKey().Address, s.qcAddress},
		[]sdkcrypto.Signer{s.blockchain.ServiceKey().Signer(), s.qcSigner})
}

/**
Methods below are from the flow-core-contracts repo
**/

func (s *ClusterEpochTestSuite) SignAndSubmit(tx *sdk.Transaction, signerAddresses []sdk.Address, signers []sdkcrypto.Signer) error {

	// sign transaction with each signer
	for i := len(signerAddresses) - 1; i >= 0; i-- {
		signerAddress := signerAddresses[i]
		signer := signers[i]

		if i == 0 {
			err := tx.SignEnvelope(signerAddress, 0, signer)
			if err != nil {
				return err
			}
		} else {
			err := tx.SignPayload(signerAddress, 0, signer)
			if err != nil {
				return err
			}
		}
	}

	// submit transaction
	return s.Submit(tx)
}

func (s *ClusterEpochTestSuite) Submit(tx *sdk.Transaction) error {
	// submit the signed transaction
	err := s.blockchain.AddTransaction(*tx)
	if err != nil {
		return err
	}

	result, err := s.blockchain.ExecuteNextTransaction()
	if err != nil {
		return err
	}

	if !assert.True(s.T(), result.Succeeded()) {
		// TODO: handle error in submitting tx
	}

	_, err = s.blockchain.CommitBlock()
	if err != nil {
		return err
	}

	return nil
}
