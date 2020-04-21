package consensus

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/ghost/client"
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/integration/tests/common"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestCollectionGuaranteeCycle(t *testing.T) {
	suite.Run(t, new(GuaranteeSuite))
}

type GuaranteeSuite struct {
	suite.Suite
	cancel  context.CancelFunc
	net     *testnet.FlowNetwork
	nodeIDs []flow.Identifier
	ghostID flow.Identifier
	collID  flow.Identifier
	reader  *client.FlowMessageStreamReader
}

func (gs *GuaranteeSuite) Consensus(index int) *testnet.Container {
	require.True(gs.T(), index < len(gs.nodeIDs), "invalid node index (%d)", index)
	node, found := gs.net.ContainerByID(gs.nodeIDs[index])
	require.True(gs.T(), found, "could not find node")
	return node
}

func (gs *GuaranteeSuite) Ghost() *client.GhostClient {
	ghost, found := gs.net.ContainerByID(gs.ghostID)
	require.True(gs.T(), found, "could not find ghost containter")
	client, err := common.GetGhostClient(ghost)
	require.NoError(gs.T(), err, "could not get ghost client")
	return client
}

func (gs *GuaranteeSuite) SetupTest() {

	// to collect node configs...
	var nodeConfigs []testnet.NodeConfig

	// generate the three consensus identities
	gs.nodeIDs = unittest.IdentifierListFixture(3)

	// need one execution node
	exeConfig := testnet.NewNodeConfig(flow.RoleExecution)
	nodeConfigs = append(nodeConfigs, exeConfig)

	// need one verification node
	verConfig := testnet.NewNodeConfig(flow.RoleVerification)
	nodeConfigs = append(nodeConfigs, verConfig)

	// need one collection node
	gs.collID = unittest.IdentifierFixture()
	collConfig := testnet.NewNodeConfig(flow.RoleCollection, testnet.WithID(gs.collID))
	nodeConfigs = append(nodeConfigs, collConfig)

	// generate consensus node config for each consensus identity
	for _, nodeID := range gs.nodeIDs {
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithID(nodeID))
		nodeConfigs = append(nodeConfigs, nodeConfig)
	}

	// add the ghost node config
	gs.ghostID = unittest.IdentifierFixture()
	ghostConfig := testnet.NewNodeConfig(flow.RoleCollection, testnet.WithID(gs.ghostID), testnet.AsGhost(true))
	nodeConfigs = append(nodeConfigs, ghostConfig)

	// generate the network config
	netConfig := testnet.NewNetworkConfig("consensus_collection_guarantee_cycle", nodeConfigs)

	// initialize the network
	gs.net = testnet.PrepareFlowNetwork(gs.T(), netConfig)

	// start the network
	ctx, cancel := context.WithCancel(context.Background())
	gs.cancel = cancel
	gs.net.Start(ctx)

	// subscribe to the ghost
	for attempts := 0; ; attempts++ {
		var err error
		gs.reader, err = gs.Ghost().Subscribe(context.Background())
		if err == nil {
			break
		}
		if attempts >= 10 {
			require.NoError(gs.T(), err, "could not subscribe to ghost (%d attempts)", attempts)
		}
	}
}

func (gs *GuaranteeSuite) TearDownTest() {
	err := gs.net.Stop()
	gs.cancel()
	require.NoError(gs.T(), err, "should stop without error")
}

func (gs *GuaranteeSuite) TestCollectionGuaranteeIncluded() {

	// generate a sentinel collection guarantee
	sentinel := unittest.CollectionGuaranteeFixture()
	sentinel.SignerIDs = []flow.Identifier{gs.collID}

	// try sending it to the first consensus node as soon as possible
	for attempts := 0; ; attempts++ {
		err := gs.Ghost().Send(context.Background(), engine.CollectionProvider, gs.nodeIDs[:1], sentinel)
		if err == nil {
			break
		}
		if attempts >= 10 {
			require.NoError(gs.T(), err, "could not send sentinel guarantee (%d attempts)", attempts)
		}
	}

	gs.T().Logf("sentinel collection guarantee: %x", sentinel.CollectionID)

	// we try to find a block with the guarantee included and three confirmations
	deadline := time.Now().Add(time.Minute)
	confirmations := make(map[flow.Identifier]uint)
InclusionLoop:
	for time.Now().Before(deadline) {

		// we read the next message until we reach deadline
		_, msg, err := gs.reader.Next()
		require.NoError(gs.T(), err, "could not read next message")

		// we only care about block proposals at the moment
		proposal, ok := msg.(*messages.BlockProposal)
		if !ok {
			continue
		}

		// log the proposal details
		proposalID := proposal.Header.ID()
		guarantees := proposal.Payload.Guarantees
		gs.T().Logf("block proposal received: %x", proposalID)

		// if the collection guarantee is included, we add the block to those we
		// monitor for confirmations
		for _, guarantee := range guarantees {
			if guarantee.CollectionID == sentinel.CollectionID {
				confirmations[proposalID] = 0
				gs.T().Log("sentinel guarantee included!")
				continue InclusionLoop
			}
		}

		// if the parent is in the map, it is on a chain that included the
		// guarantee; take parent confirmatians plus one as the confirmations
		// for the follow-up block
		n, ok := confirmations[proposal.Header.ParentID]
		if ok {
			confirmations[proposalID] = n + 1
			gs.T().Logf("sentinel guarantee confirmed! (count: %d)", confirmations[proposalID])
		}

		// if we reached three or more confirmations, we are done!
		if confirmations[proposalID] >= 3 {
			break
		}
	}

	// make sure we found the guarantee in at least one block proposal
	require.NotEmpty(gs.T(), confirmations, "collection guarantee should have been included in at least one block")

	// check if we have a block with 3 confirmations that contained it
	max := uint(0)
	for _, n := range confirmations {
		if n > max {
			max = n
		}
	}
	require.GreaterOrEqual(gs.T(), max, uint(3), "should have confirmed one inclusion block at least three times")
}
