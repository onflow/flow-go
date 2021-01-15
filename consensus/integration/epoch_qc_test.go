package integration_test

import (
	"crypto"
	"testing"

	emulator "github.com/onflow/flow-emulator"
	"github.com/onflow/flow-go-sdk"
	sdk "github.com/onflow/flow-go-sdk"
	sdktemplates "github.com/onflow/flow-go-sdk/templates"

	"github.com/onflow/flow-go-sdk/test"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-core-contracts/lib/go/contracts"
	"github.com/onflow/flow-core-contracts/lib/go/templates"

	"github.com/onflow/flow-go/module"
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
	Signer  crypto.Signer
	Key     sdk.AccountKey
	Address sdk.Address
	Voter   module.QCVoter
}

func TestQuroumCertificate(t *testing.T) {

	// create a new instance of the emulated blockchain
	blockchain, err := emulator.NewBlockchain()
	require.NoError(err)

	accountKeys := test.AccountKeyGenerator()

	// create new account keys for the Quorum Certificate account
	QCAccountKey, QCSigner := accountKeys.NewWithSigner()
	QCCode := contracts.FlowQC()

	// deploy the contract to the emulator
	QCAddress, err := blockchain.CreateAccount([]*flow.AccountKey{QCAccountKey}, []sdktemplates.Contract{
		{
			Name:   "FlowEpochClusterQC",
			Source: string(QCCode),
		},
	})
	require.NoError(err)

	env := templates.Environment{
		QuorumCertificateAddress: QCAddress.Hex(),
	}

	// create clusters
	numberOfClusters := 3
	numberOfNodesPerCluster := 10

	// create flow keys
	clusters := make([][]ClusterNode, numberOfClusters)

	// create QC voter for each node in the cluster
	for i := 1; i <= numberOfClusters; i++ {
		for j := 1; j <= numberOfNodesPerCluster; i++ {
			key, signer := accountKeys.NewWithSigner()

			// create flow account
			address, err := blockchain.CreateAccount([]*flow.AccountKey{key})
			require.NoError(err)

			// create QC client for clustesrs node
			client, err := module.QCClient(unittest.IdentifierFixture(), address.String(), key.Index,
				"ACCESS_ADDRESS", QCAddress.String(), signer)
			require.NoError(err)

			// create QCVoter object
			voter := module.QCVoter()

			node := ClusterNode{
				Signer:  signer,
				Key:     key,
				Address: address,
				Voter:   voter,
			}

			// set node voter object
			clusters[i][j] = node
		}
	}
}
