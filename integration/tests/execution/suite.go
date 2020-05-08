package execution

import (
	"context"
	"fmt"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine/ghost/client"
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/integration/tests/common"
	"github.com/dapperlabs/flow-go/model/flow"
)

type Suite struct {
	suite.Suite
	common.TestnetStateTracker
	cancel  context.CancelFunc
	net     *testnet.FlowNetwork
	nodeIDs []flow.Identifier
	ghostID flow.Identifier
	exe1ID  flow.Identifier
	verID   flow.Identifier
}

func (s *Suite) Ghost() *client.GhostClient {
	ghost := s.net.ContainerByID(s.ghostID)
	client, err := common.GetGhostClient(ghost)
	require.NoError(s.T(), err, "could not get ghost client")
	return client
}

func (s *Suite) AccessClient() *testnet.Client {
	client, err := testnet.NewClient(fmt.Sprintf(":%s", s.net.AccessPorts[testnet.AccessNodeAPIPort]))
	require.NoError(s.T(), err, "could not get access client")
	return client
}

func (s *Suite) TearDownTest() {
	gs.net.Remove()
	gs.cancel()
}
