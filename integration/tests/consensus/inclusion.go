package consensus

import (
	"context"
	"math/rand"
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

type InclusionSuite struct {
	suite.Suite

	log    zerolog.Logger
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
	logger := unittest.LoggerWithLevel(zerolog.InfoLevel).With().
		Str("testfile", "inclusion.go").
		Str("testcase", is.T().Name()).
		Logger()
	is.log = logger
	is.log.Info().Msgf("================> SetupTest")

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
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.InfoLevel), testnet.WithID(conID))
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

func (s *InclusionSuite) TearDownTest() {
	s.log.Info().Msgf("================> Start TearDownTest")
	s.net.Remove()
	s.cancel()
	s.log.Info().Msgf("================> Finish TearDownTest")
}

func (is *InclusionSuite) TestCollectionGuaranteeIncluded() {
	t := is.T()
	is.log.Info().Msgf("================> RUNNING TESTING %v", t.Name())
	// fix the deadline for the test as a whole
	deadline := time.Now().Add(30 * time.Second)
	is.T().Logf("%s ------ test started, deadline %s", time.Now(), deadline)

	// generate a sentinel collection guarantee
	sentinel := unittest.CollectionGuaranteeFixture()
	sentinel.SignerIDs = []flow.Identifier{is.collID}
	sentinel.ReferenceBlockID = is.net.Root().ID()
	colID := sentinel.CollectionID

	is.waitUntilSeenProposal(deadline)

	is.T().Logf("seen a proposal")

	// send collection to one consensus node
	is.sendCollectionToConsensus(deadline, sentinel, is.conIDs[0])

	proposal := is.waitUntilCollectionIncludeInProposal(deadline, sentinel)

	is.T().Logf("collection guarantee %x included in a proposal %x\n", colID, proposal.Header.ID())

	is.waitUntilProposalConfirmed(deadline, sentinel, proposal)

	is.T().Logf("collection guarantee %x is confirmed 3 times\n", colID)
}

func (is *InclusionSuite) waitUntilSeenProposal(deadline time.Time) {
	for time.Now().Before(deadline) {

		// we read the next message until we reach deadline
		originID, msg, err := is.reader.Next()
		if err != nil {
			is.T().Logf("could not read message: %s\n", err)
			continue
		}

		// we only care about block proposals at the moment
		proposal, ok := msg.(*messages.BlockProposal)
		if !ok {
			continue
		}

		is.T().Logf("receive block proposal from %v, height %v", originID, proposal.Header.Height)
		// wait until proposal finalized
		if proposal.Header.Height >= 1 {
			return
		}
	}
	is.T().Fatalf("%s timeout (deadline %s) waiting to see proposal", time.Now(), deadline)
}

func (is *InclusionSuite) sendCollectionToConsensus(deadline time.Time, sentinel *flow.CollectionGuarantee, conID flow.Identifier) {
	colID := sentinel.CollectionID

	// keep trying to send collection guarantee to at least one consensus node
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		is.T().Logf("%s sending collection %x to consensus node %v\n", time.Now(), colID, conID)
		err := is.Collection().Send(ctx, engine.PushGuarantees, sentinel, conID)
		cancel()
		if err != nil {
			is.T().Logf("could not send collection guarantee: %s\n", err)
			continue
		}

		is.T().Logf("%v sent collection %x to consensus %v", time.Now(), colID, conID)
		return
	}

	is.T().Fatalf("%v timeout (deadline %s) sendng collection %x to consensus", time.Now(), deadline, colID)
}

func (is *InclusionSuite) waitUntilCollectionIncludeInProposal(deadline time.Time, sentinel *flow.CollectionGuarantee) *messages.BlockProposal {
	colID := sentinel.CollectionID
	// we try to find a block with the guarantee included
	for time.Now().Before(deadline) {

		// we read the next message until we reach deadline
		originID, msg, err := is.reader.Next()
		if err != nil {
			is.T().Logf("could not read message: %s\n", err)
			continue
		}

		// we only care about block proposals at the moment
		proposal, ok := msg.(*messages.BlockProposal)
		if !ok {
			continue
		}

		guarantees := proposal.Payload.Guarantees
		height := proposal.Header.Height
		is.T().Logf("receive block proposal height %v from %v, %v guarantees included in the payload!", height, originID, len(guarantees))

		// check if the collection guarantee is included
		for _, guarantee := range guarantees {
			if guarantee.CollectionID == sentinel.CollectionID {
				proposalID := proposal.Header.ID()
				is.T().Logf("%x: collection guarantee %x included!\n", proposalID, colID)
				return proposal
			}
		}
	}

	is.T().Fatalf("%s timeout (deadline %s) checking collection guarantee %x included\n", time.Now(), deadline, colID)
	return nil
}

// checkingProposalConfirmed returns whether it has seen 3 blocks confirmations on the block
// that contains the guarantee
func (is *InclusionSuite) waitUntilProposalConfirmed(deadline time.Time, sentinel *flow.CollectionGuarantee, proposal *messages.BlockProposal) {
	colID := sentinel.CollectionID
	// we try to find a block with the guarantee included and three confirmations
	confirmations := make(map[flow.Identifier]uint)
	// add the proposal that includes the guarantee
	confirmations[proposal.Header.ID()] = 0

	for time.Now().Before(deadline) {

		// we read the next message until we reach deadline
		originID, msg, err := is.reader.Next()
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
		is.T().Logf("proposal %v received from %v", proposalID, originID)

		_, processed := confirmations[proposalID]
		if processed {
			continue
		}

		// if the parent is in the map, it is on a chain that included the
		// guarantee; take parent confirmatians plus one as the confirmations
		// for the follow-up block
		n, ok := confirmations[proposal.Header.ParentID]
		if ok {
			confirmations[proposalID] = n + 1
			is.T().Logf("%x: collection guarantee %x confirmed! (count: %d)\n", proposalID, colID, n+1)
		}

		// if we reached three or more confirmations, we are done!
		if confirmations[proposalID] >= 3 {
			return
		}
	}

	is.T().Fatalf("%s timeout (deadline %s) collection guarantee %x not confirmed\n", time.Now(), deadline, colID)
}
