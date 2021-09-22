package access

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

type AccessSuite struct {
	suite.Suite

	// root context for the current test
	ctx    context.Context
	cancel context.CancelFunc

	net *testnet.FlowNetwork
}

func TestAccessSuite(t *testing.T) {
	suite.Run(t, new(AccessSuite))
}

func (suite *AccessSuite) TearDownTest() {
	// avoid nil pointer errors for skipped tests
	if suite.cancel != nil {
		defer suite.cancel()
	}
	if suite.net != nil {
		suite.net.Remove()
	}
}

func (suite *AccessSuite) SetupTest() {
	acsConfig := testnet.NewNodeConfig(flow.RoleAccess)
	nodeConfigs := []testnet.NodeConfig{acsConfig}

	// need one dummy execution node (unused ghost)
	exeConfig := testnet.NewNodeConfig(flow.RoleExecution)
	nodeConfigs = append(nodeConfigs, exeConfig)

	// need one dummy verification node (unused ghost)
	verConfig := testnet.NewNodeConfig(flow.RoleVerification, testnet.AsGhost())
	nodeConfigs = append(nodeConfigs, verConfig)

	// need three consensus nodes (unused ghost)
	for n := 0; n < 3; n++ {
		conID := unittest.IdentifierFixture()
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus,
			testnet.WithID(conID),
			testnet.AsGhost())
		nodeConfigs = append(nodeConfigs, nodeConfig)
	}

	// need one controllable collection node (used ghost)
	collID := unittest.IdentifierFixture()
	collConfig := testnet.NewNodeConfig(flow.RoleCollection, testnet.WithID(collID), testnet.WithAdditionalFlag(fmt.Sprintf("--secure-access-node-id=%s", acsConfig.Identifier.String())))
	nodeConfigs = append(nodeConfigs, collConfig)

	conf := testnet.NewNetworkConfig("access_api_test", nodeConfigs)
	suite.net = testnet.PrepareFlowNetwork(suite.T(), conf)

	// start the network
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
	suite.net.Start(suite.ctx)
}

func (suite *AccessSuite) TestHTTPProxyPortOpen() {
	httpProxyAddress := fmt.Sprintf(":%s", suite.net.AccessPorts[testnet.AccessNodeAPIProxyPort])
	conn, err := net.DialTimeout("tcp", httpProxyAddress, 1*time.Second)
	require.NoError(suite.T(), err, "http proxy port not open on the access node")
	conn.Close()
}
