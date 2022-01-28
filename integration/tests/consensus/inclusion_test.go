package consensus

import (
	"context"
	"fmt"
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
	is.T().Logf("%s test case setup inclusion", time.Now().UTC())

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
	collConfig := testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.InfoLevel), testnet.WithID(is.collID), testnet.AsGhost())
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
	is.T().Logf("%s test case tear down inclusion", time.Now().UTC())
	is.net.Remove()
	is.cancel()
}

func makeCollection(collectionID flow.Identifier, refBlockID flow.Identifier) *flow.CollectionGuarantee {
	sentinel := unittest.CollectionGuaranteeFixture()
	sentinel.SignerIDs = []flow.Identifier{collectionID}
	sentinel.ReferenceBlockID = refBlockID
	return sentinel
}

func (is *InclusionSuite) TestCollectionGuaranteeIncluded() {
	t := is.T()
	t.Logf("%s ------ test started", time.Now().UTC())

	// generate a sentinel collection guarantee, with root block as reference,
	// as long as the tests can finish within 600 blocks, the collection should
	// not be expired.
	collection := makeCollection(is.collID, is.net.Root().ID())

	t.Logf("%s collection created: %v", time.Now().UTC(), collection.ID())

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	msgs1, msgs2 := dup(ctx, getMsgs(t, ctx, is.reader))

	go func() {
		t.Logf("%s waiting for ghost node %v to see any proposal with height 1", time.Now().UTC(), is.collID)

		err := is.waitUntilSeenProposal(ctx, msgs1)
		if err != nil {
			cancel()
			t.Logf("%s did not see any proposal finalized: %v", time.Now().UTC(), err)
			return
		}

		t.Logf("%s have seen a proposal finalized, all consensus nodes must have been ready.", time.Now().UTC())

		// send collection to one consensus node
		consensusID := is.conIDs[0]
		err = is.sendCollectionToConsensus(ctx, collection, consensusID)
		if err != nil {
			cancel()
			t.Logf("%s failed to send collection %v to consensus %v", time.Now().UTC(), collection.CollectionID, consensusID)
			return
		}

		t.Logf("%s successfully sent collection %v to consensus %v", time.Now().UTC(), collection.CollectionID, consensusID)
	}()

	t.Logf("%s waiting until collection %v included in any proposal", time.Now().UTC(), collection.CollectionID)

	proposal, err := is.waitUntilCollectionIncludeInProposal(ctx, collection.CollectionID, msgs2)
	if err != nil {
		t.Errorf("%s failed to see collection %v included: %v", time.Now().UTC(), collection.CollectionID, err)
		return
	}

	t.Logf("%s collection guarantee %x included is in a proposal %x", time.Now().UTC(), collection.CollectionID, proposal.Header.ID())

	err = is.waitUntilProposalConfirmed(ctx, collection.CollectionID, proposal, msgs2)
	if err != nil {
		t.Errorf("%s failed to see proposal %v confirmed: %v", time.Now().UTC(), proposal.Header.ID(), err)
		return
	}

	t.Logf("%s collection guarantee %x is confirmed 3 times\n", time.Now().UTC(), collection.CollectionID)
}

func dup(ctx context.Context, msgs <-chan *MsgWrap) (<-chan *MsgWrap, <-chan *MsgWrap) {
	a := make(chan *MsgWrap)
	b := make(chan *MsgWrap)

	go func() {
		for {
			select {
			case <-ctx.Done():
				close(a)
				close(b)
				return
			case msg := <-msgs:

				// push message to a, but if no one is listening to a
				// then we drop the message to prevent from being stuck
				select {
				case a <- msg:
				default:
				}

				select {
				case b <- msg:
				default:
				}
			}
		}
	}()
	return a, b
}

func getMsgs(t *testing.T, ctx context.Context, reader *client.FlowMessageStreamReader) <-chan *MsgWrap {
	msgs := make(chan *MsgWrap)
	go func() {
		for {
			select {
			case <-ctx.Done():
				t.Logf("%v stop reading messages from reader", time.Now().UTC())
				close(msgs)
				return
			default:
			}

			originID, msg, err := reader.Next()
			msgs <- &MsgWrap{
				Err:      err,
				OriginID: originID,
				Msg:      msg,
			}
		}
	}()
	return msgs
}

// processMsg takes in a function to process each message from the given channel, until the function returns true
// which indicates it has found a specific message
func processMsg(t *testing.T, ctx context.Context, msgs <-chan *MsgWrap, taskName string, fn func(msg *MsgWrap) (bool, error)) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("%v failed to %s : %w", time.Now().UTC(), taskName, ctx.Err())
		case msgWrap := <-msgs:
			t.Logf("received message from task %v", taskName)
			if msgWrap.Err != nil {
				t.Logf("could not read message: %s\n", msgWrap.Err)
				continue
			}

			found, err := fn(msgWrap)
			if err != nil {
				return err
			}

			if !found {
				continue
			}

			return nil
		}
	}

	return nil
}

func (is *InclusionSuite) waitUntilSeenProposal(ctx context.Context, msgs <-chan *MsgWrap) error {
	return processMsg(is.T(), ctx, msgs, "see any proposal incorporated", func(msg *MsgWrap) (bool, error) {
		// we only care about block proposals at the moment
		proposal, ok := msg.Msg.(*messages.BlockProposal)
		if !ok {
			return false, nil
		}

		is.T().Logf("%v seen proposal at height %v", time.Now().UTC(), proposal.Header.Height)

		// wait until proposal finalized
		if proposal.Header.Height >= 1 {
			return true, nil
		}

		return false, nil
	})
}

func (is *InclusionSuite) sendCollectionToConsensus(ctx context.Context, collection *flow.CollectionGuarantee, conID flow.Identifier) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	is.T().Logf("%s sending collection %x to consensus node %v\n", time.Now().UTC(), collection.CollectionID, conID)

	err := is.Collection().Send(ctx, engine.PushGuarantees, collection, conID)
	defer cancel()

	if err != nil {
		return fmt.Errorf("could not send collection guarantee: %w", err)
	}

	return nil
}

type MsgWrap struct {
	Err      error
	OriginID flow.Identifier
	Msg      interface{}
}

func (is *InclusionSuite) waitUntilCollectionIncludeInProposal(ctx context.Context, collectionID flow.Identifier, msgs <-chan *MsgWrap) (*messages.BlockProposal, error) {
	var included *messages.BlockProposal
	err := processMsg(is.T(), ctx, msgs, "wait until collection included", func(msgWrap *MsgWrap) (bool, error) {
		// we only care about block proposals at the moment
		proposal, ok := msgWrap.Msg.(*messages.BlockProposal)
		if !ok {
			return false, nil
		}

		guarantees := proposal.Payload.Guarantees
		height := proposal.Header.Height
		originID := msgWrap.OriginID

		is.T().Logf("receive block proposal height %v from %v, %v guarantees included in the payload!", height, originID, len(guarantees))

		// check if the collection guarantee is included
		for _, guarantee := range guarantees {
			if guarantee.CollectionID == collectionID {
				proposalID := proposal.Header.ID()
				is.T().Logf("%x: collection guarantee %x included!\n", proposalID, collectionID)
				included = proposal
				return true, nil
			}
		}
		return false, nil
	})

	if err != nil {
		return nil, err
	}
	return included, nil
}

// checkingProposalConfirmed returns whether it has seen 3 blocks confirmations on the block
// that contains the guarantee
func (is *InclusionSuite) waitUntilProposalConfirmed(ctx context.Context, collectionID flow.Identifier, proposal *messages.BlockProposal, msgs <-chan *MsgWrap) error {
	// we try to find a block with the guarantee included and three confirmations
	confirmations := make(map[flow.Identifier]uint)
	// add the proposal that includes the guarantee
	confirmations[proposal.Header.ID()] = 0

	return processMsg(is.T(), ctx, msgs, "wait until proposal confirmed", func(msgWrap *MsgWrap) (bool, error) {
		// we only care about block proposals at the moment
		proposal, ok := msgWrap.Msg.(*messages.BlockProposal)
		if !ok {
			return false, nil
		}

		// check if the proposal was already processed
		proposalID := proposal.Header.ID()
		originID := msgWrap.OriginID
		is.T().Logf("proposal %v (height %v) received from %v", proposalID, proposal.Header.Height, originID)

		_, processed := confirmations[proposalID]
		if processed {
			return false, nil
		}

		// if the parent is in the map, it is on a chain that included the
		// guarantee; take parent confirmatians plus one as the confirmations
		// for the follow-up block
		n, ok := confirmations[proposal.Header.ParentID]
		if ok {
			confirmations[proposalID] = n + 1
			is.T().Logf("%x: collection guarantee %x confirmed! (count: %d)\n", proposalID, collectionID, n+1)
		}

		// if we reached three or more confirmations, we are done!
		if confirmations[proposalID] < 3 {
			return false, nil
		}

		return true, nil
	})
}
