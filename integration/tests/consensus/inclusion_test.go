package consensus

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestCollectionGuaranteeInclusion(t *testing.T) {
	suite.Run(t, new(InclusionSuite))
}

type InclusionSuite struct {
	suite.Suite
	cancel context.CancelFunc
	net    *testnet.FlowNetwork
	conIDs []flow.Identifier
	collID flow.Identifier
	reader *client.FlowMessageStreamReader
}

func (is *InclusionSuite) Collection() *client.GhostClient {
	ghost := is.net.ContainerByID(is.collID)
	client, err := common.GetGhostClient(ghost)
	require.NoError(is.T(), err, "could not get ghost client")
	return client
}

func (is *InclusionSuite) SetupTest() {

	// seed random generator
	rand.Seed(time.Now().UnixNano())

	// to collect node confiis...
	var nodeConfigs []testnet.NodeConfig

	// need one dummy execution node (unused ghost)
	exeConfig := testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel), testnet.AsGhost())
	nodeConfigs = append(nodeConfigs, exeConfig)

	// need one dummy verification node (unused ghost)
	verConfig := testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel), testnet.AsGhost())
	nodeConfigs = append(nodeConfigs, verConfig)

	// need three real consensus nodes
	for n := 0; n < 3; n++ {
		conID := unittest.IdentifierFixture()
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.WarnLevel), testnet.WithID(conID))
		nodeConfigs = append(nodeConfigs, nodeConfig)
		is.conIDs = append(is.conIDs, conID)
	}

	// need one controllable collection node (used ghost)
	is.collID = unittest.IdentifierFixture()
	collConfig := testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel), testnet.WithID(is.collID), testnet.AsGhost())
	nodeConfigs = append(nodeConfigs, collConfig)

	nodeConfigs = append(nodeConfigs,
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.FatalLevel)),
	)

	// generate the network config
	netConfig := testnet.NewNetworkConfig("consensus_collection_guarantee_inclusion", nodeConfigs)

	// initialize the network
	is.net = testnet.PrepareFlowNetwork(is.T(), netConfig)

	// start the network
	ctx, cancel := context.WithCancel(context.Background())
	is.cancel = cancel
	is.net.Start(ctx)

	// subscribe to the ghost
	for attempts := 0; ; attempts++ {
		var err error
		is.reader, err = is.Collection().Subscribe(context.Background())
		if err == nil {
			break
		}
		if attempts >= 10 {
			require.NoError(is.T(), err, "could not subscribe to ghost (%d attempts)", attempts)
		}
	}
}

func (is *InclusionSuite) TearDownTest() {
	is.net.Remove()
	is.cancel()
}

func (is *InclusionSuite) TestCollectionGuaranteeIncluded() {

	// fix the deadline for the test as a whole
	deadline := time.Now().Add(20 * time.Second)

	// generate a sentinel collection guarantee
	sentinel := unittest.CollectionGuaranteeFixture()
	sentinel.SignerIDs = []flow.Identifier{is.collID}
	sentinel.ReferenceBlockID = is.net.Root().ID()

	is.T().Logf("collection guarantee generated: %x\n", sentinel.CollectionID)

	// keep trying to send collection guarantee to at least one consensus node
SendingLoop:
	for time.Now().Before(deadline) {
		conID := is.conIDs[rand.Intn(len(is.conIDs))]
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err := is.Collection().Send(ctx, engine.PushGuarantees, sentinel, conID)
		cancel()
		if err != nil {
			is.T().Logf("could not send collection guarantee: %s\n", err)
			continue
		}
		break SendingLoop
	}

	// we try to find a block with the guarantee included and three confirmations
	confirmations := make(map[flow.Identifier]uint)
InclusionLoop:
	for time.Now().Before(deadline) {

		// we read the next message until we reach deadline
		_, msg, err := is.reader.Next()
		if err != nil {
			is.T().Logf("could not read message: %s\n", err)
			continue
		}

		// we only care about block proposals at the moment
		proposal, ok := msg.(*messages.BlockProposal)
		if !ok {
			continue
		}

		// check if the proposal was already processed
		proposalID := proposal.Header.ID()
		_, processed := confirmations[proposalID]
		if processed {
			continue
		}
		guarantees := proposal.Payload.Guarantees

		// if the collection guarantee is included, we add the block to those we
		// monitor for confirmations
		for _, guarantee := range guarantees {
			if guarantee.CollectionID == sentinel.CollectionID {
				confirmations[proposalID] = 0
				is.T().Logf("%x: collection guarantee included!\n", proposalID)
				continue InclusionLoop
			}
		}

		// if the parent is in the map, it is on a chain that included the
		// guarantee; take parent confirmatians plus one as the confirmations
		// for the follow-up block
		n, ok := confirmations[proposal.Header.ParentID]
		if ok {
			confirmations[proposalID] = n + 1
			is.T().Logf("%x: collection guarantee confirmed! (count: %d)\n", proposalID, n+1)
		}

		// if we reached three or more confirmations, we are done!
		if confirmations[proposalID] >= 3 {
			break
		}
	}

	// make sure we found the guarantee in at least one block proposal
	require.NotEmpty(is.T(), confirmations, "collection guarantee should have been included in at least one block")

	// check if we have a block with 3 confirmations that contained it
	max := uint(0)
	for _, n := range confirmations {
		if n > max {
			max = n
		}
	}
	require.GreaterOrEqual(is.T(), max, uint(3), "should have confirmed one collection guarantee inclusion block at least three times")
}
