package integration_test

import (
	"testing"

	"github.com/rs/zerolog"

	emulator "github.com/onflow/flow-emulator"
	sdk "github.com/onflow/flow-go-sdk"
	sdktemplates "github.com/onflow/flow-go-sdk/templates"

	"github.com/onflow/flow-go-sdk/test"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-core-contracts/lib/go/contracts"
	"github.com/onflow/flow-core-contracts/lib/go/templates"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/epochs"
	module "github.com/onflow/flow-go/module/mock"
	protomock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

/**
1. Deploy EpochQCAggregator contract emulator
2. Create Flow accounts for each collection node
3. Fund each of these Flow accounts so they can pay transaction fees
4. Set up necessary resources in each Flow account (details TBD in #4529)
5. Instantiate QCVoter for each collection node with:
	a. Keys for their Flow account
	b. Hooked up to emulator
6. Start the QCVoter for each collection node (essentially mocking the EpochSetup service event being emitted)
**/

type ClusterNode struct {
	NodeID  flow.Identifier
	Key     *sdk.AccountKey
	Address sdk.Address
	Voter   module.ClusterRootQCVoter
}

func TestQuroumCertificate(t *testing.T) {

	// create a new instance of the emulated blockchain
	blockchain, err := emulator.NewBlockchain()
	require.NoError(t, err)

	accountKeys := test.AccountKeyGenerator()

	// create new account keys for the Quorum Certificate account
	QCAccountKey, QCSigner := accountKeys.NewWithSigner()
	QCCode := contracts.FlowQC()

	// deploy the contract to the emulator
	QCAddress, err := blockchain.CreateAccount([]*sdk.AccountKey{QCAccountKey}, []sdktemplates.Contract{
		{
			Name:   "FlowEpochClusterQC",
			Source: string(QCCode),
		},
	})
	require.NoError(t, err)

	env := templates.Environment{
		QuorumCertificateAddress: QCAddress.Hex(),
	}

	// create clusters
	numberOfClusters := 3
	numberOfNodesPerCluster := 10

	// create flow keys
	clusters := make([][]ClusterNode, numberOfClusters)

	nodes := unittest.IdentityListFixture(numberOfClusters*numberOfNodesPerCluster, unittest.WithRole(flow.RoleCollection))
	clusterAssignment := unittest.ClusterAssignment(uint(numberOfClusters), nodes)

	clusterList, err := flow.NewClusterList(clusterAssignment, nodes)
	require.NoError(t, err)

	epoch := &protomock.Epoch{}
	epoch.On("Counter").Return(0, nil)
	epoch.On("Clustering").Return(clusterList, nil)

	// create QC voter for each node in the cluster
	for i := 1; i <= numberOfClusters; i++ {
		for j := 1; j <= numberOfNodesPerCluster; i++ {

			nodeID := unittest.IdentifierFixture()
			key, signer := accountKeys.NewWithSigner()

			// create flow account
			address, err := blockchain.CreateAccount([]*sdk.AccountKey{key}, []sdktemplates.Contract{})
			require.NoError(t, err)

			// create QC client for clustesrs node
			client := &module.QCContractClient{}
			client.On("").Return()
			// client, err := epochs.NewQCContractClient(nodeID, address.String(), key.Index,
			// 	"ACCESS_ADDRESS", QCAddress.String(), signer)
			// require.NoError(err)

			// create QCVoter object
			local := &module.Local{}
			local.On("NodeID").Return(nodeID)
			local.On("Sign", mock.Anything, mock.Anything).Return(crypto.Signature(nodeID[:]), nil)

			hotstuffSigner := &hotstuff.Signer{}

			voter := epochs.NewRootQCVoter(zerolog.Logger{}, local)

			node := ClusterNode{
				NodeID:  nodeID,
				Key:     key,
				Address: address,
				Voter:   voter,
			}

			// set node voter object
			clusters[i][j] = node
		}
	}
}
