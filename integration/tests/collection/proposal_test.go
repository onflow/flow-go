package collection

import (
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func (suite *CollectorSuite) TestProposal_SingleCluster() {
	t := suite.T()

	suite.SetupTest("col_proposal_singlecluster", 3, 1)

	// we will submit some transactions to each collector
	var (
		col1 = suite.Collector(0, 0)
		col2 = suite.Collector(0, 1)
		col3 = suite.Collector(0, 2)
	)

	client1, err := client.New(col1.Addr(testnet.ColNodeAPIPort), grpc.WithInsecure())
	require.Nil(t, err)
	client2, err := client.New(col2.Addr(testnet.ColNodeAPIPort), grpc.WithInsecure())
	require.Nil(t, err)
	client3, err := client.New(col3.Addr(testnet.ColNodeAPIPort), grpc.WithInsecure())
	require.Nil(t, err)

	acct := suite.acct

	for i := 0; i < 100; i++ {
		tx := sdk.NewTransaction().
			SetScript(unittest.NoopTxScript()).
			SetReferenceBlockID(sdk.BytesToID())
	}

}
