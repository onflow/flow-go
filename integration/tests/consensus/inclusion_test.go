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

func (is *InclusionSuite) TearDownTest() {
	is.net.Remove()
	is.cancel()
}

func (is *InclusionSuite) TestCollectionGuaranteeIncluded() {
	is.T().Logf("------ test started")
	// fix the deadline for the test as a whole
	deadline := time.Now().Add(20 * time.Second)

	// generate a sentinel collection guarantee
	sentinel := unittest.CollectionGuaranteeFixture()
	sentinel.SignerIDs = []flow.Identifier{is.collID}
	sentinel.ReferenceBlockID = is.net.Root().ID()
	colID := sentinel.CollectionID

	is.waitUntilSeenProposal(deadline)

	is.T().Logf("seen a proposal")

	conID := is.sendCollectionToConsensus(deadline, sentinel)

	is.T().Logf("sent collection %x to consensus %v", colID, conID)

	proposal := is.waitUntilCollectionIncludeInProposal(deadline, sentinel)

	is.T().Logf("collection guarantee %x included in a proposal %x\n", colID, proposal.Header.ID())

	is.waitUntilProposalConfirmed(deadline, sentinel, proposal)

	is.T().Logf("collection guarantee %x is confirmed 3 times\n", colID)
}

func (is *InclusionSuite) waitUntilSeenProposal(deadline time.Time) {
	for time.Now().Before(deadline) {

		// we read the next message until we reach deadline
		_, msg, err := is.reader.Next()
		if err != nil {
			is.T().Logf("could not read message: %s\n", err)
			continue
		}

		// we only care about block proposals at the moment
		_, ok := msg.(*messages.BlockProposal)
		if !ok {
			continue
		}
		return
	}
	is.T().Fatal("timeout waiting to see proposal")
}

func (is *InclusionSuite) sendCollectionToConsensus(deadline time.Time, sentinel *flow.CollectionGuarantee) flow.Identifier {
	colID := sentinel.CollectionID

	conID := is.conIDs[0]
	// keep trying to send collection guarantee to at least one consensus node
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		is.T().Logf("sending collection %x to consensus node %v\n", colID, conID)
		err := is.Collection().Send(ctx, engine.PushGuarantees, sentinel, conID)
		cancel()
		if err != nil {
			is.T().Logf("could not send collection guarantee: %s\n", err)
			continue
		}

		return conID
	}
	is.T().Fatalf("timeout sendng collection %x to consensus", colID)
	return flow.ZeroID
}

func (is *InclusionSuite) waitUntilCollectionIncludeInProposal(deadline time.Time, sentinel *flow.CollectionGuarantee) *messages.BlockProposal {
	colID := sentinel.CollectionID
	// we try to find a block with the guarantee included
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

		guarantees := proposal.Payload.Guarantees

		// check if the collection guarantee is included
		for _, guarantee := range guarantees {
			if guarantee.CollectionID == sentinel.CollectionID {
				proposalID := proposal.Header.ID()
				is.T().Logf("%x: collection guarantee %x included!\n", proposalID, colID)
				return proposal
			}
		}
	}

	is.T().Fatalf("timeout checking collection guarantee %x included\n", colID)
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

	is.T().Fatalf("timeout, collection guarantee %x not confirmed\n", colID)
}
