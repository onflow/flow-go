package integration_test

import (
	"testing"

	emulator "github.com/onflow/flow-emulator"
	sdktemplates "github.com/onflow/flow-go-sdk/templates"

	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/test"
	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-core-contracts/lib/go/contracts"
	"github.com/onflow/flow-core-contracts/lib/go/templates"
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

func TestQuroumCertificate(t *testing.T) {

	// create a new instance of the emulated blockchain
	blockchain, err := emulator.NewBlockchain()
	if err != nil {
		panic(err)
	}

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
	if !assert.NoError(t, err) {
		t.Log(err.Error())
	}
	env := te\mplates.Environment{
		QuorumCertificateAddress: QCAddress.String(),
	}
```
	// create clusters
	numberOfClusters := 3
	numberOfNodesPerCluster := 10
}
