package compliance

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/irrecoverable"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/util"
	netint "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/mocknetwork"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storerr "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestComplianceEngine(t *testing.T) {
	suite.Run(t, new(EngineSuite))
}

type EngineSuite struct {
	CommonSuite

	clusterID  flow.ChainID
	myID       flow.Identifier
	cluster    flow.IdentityList
	me         *module.Local
	net        *mocknetwork.Network
	payloads   *storage.ClusterPayloads
	protoState *protocol.MutableState
	con        *mocknetwork.Conduit

	payloadDB map[flow.Identifier]*cluster.Payload

	ctx    irrecoverable.SignalerContext
	cancel context.CancelFunc
	errs   <-chan error

	engine *Engine
}

func (cs *EngineSuite) SetupTest() {
	cs.CommonSuite.SetupTest()

	// initialize the paramaters
	cs.cluster = unittest.IdentityListFixture(3,
		unittest.WithRole(flow.RoleCollection),
		unittest.WithWeight(1000),
	)
	cs.myID = cs.cluster[0].NodeID

	protoEpoch := &protocol.Epoch{}
	clusters := flow.ClusterList{cs.cluster}
	protoEpoch.On("Clustering").Return(clusters, nil)

	protoQuery := &protocol.EpochQuery{}
	protoQuery.On("Current").Return(protoEpoch)

	protoSnapshot := &protocol.Snapshot{}
	protoSnapshot.On("Epochs").Return(protoQuery)
	protoSnapshot.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter) flow.IdentityList {
			return cs.cluster.Filter(selector)
		},
		nil,
	)

	cs.protoState = &protocol.MutableState{}
	cs.protoState.On("Final").Return(protoSnapshot)

	cs.clusterID = "cluster-id"
	clusterParams := &protocol.Params{}
	clusterParams.On("ChainID").Return(cs.clusterID, nil)

	cs.state.On("Params").Return(clusterParams)

	// set up local module mock
	cs.me = &module.Local{}
	cs.me.On("NodeID").Return(
		func() flow.Identifier {
			return cs.myID
		},
	)

	cs.payloadDB = make(map[flow.Identifier]*cluster.Payload)

	// set up payload storage mock
	cs.payloads = &storage.ClusterPayloads{}
	cs.payloads.On("Store", mock.Anything, mock.Anything).Return(
		func(blockID flow.Identifier, payload *cluster.Payload) error {
			cs.payloadDB[blockID] = payload
			return nil
		},
	)
	cs.payloads.On("ByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) *cluster.Payload {
			return cs.payloadDB[blockID]
		},
		func(blockID flow.Identifier) error {
			_, exists := cs.payloadDB[blockID]
			if !exists {
				return storerr.ErrNotFound
			}
			return nil
		},
	)

	// set up network conduit mock
	cs.con = &mocknetwork.Conduit{}
	cs.con.On("Publish", mock.Anything, mock.Anything).Return(nil)
	cs.con.On("Publish", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	cs.con.On("Publish", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	cs.con.On("Unicast", mock.Anything, mock.Anything).Return(nil)

	// set up network module mock
	cs.net = &mocknetwork.Network{}
	cs.net.On("Register", mock.Anything, mock.Anything).Return(
		func(channel channels.Channel, engine netint.MessageProcessor) netint.Conduit {
			return cs.con
		},
		nil,
	)

	cs.hotstuff.On("Start", mock.Anything)
	cs.hotstuff.On("Done", mock.Anything).Return(unittest.ClosedChannel()).Maybe()
	cs.hotstuff.On("Ready", mock.Anything).Return(unittest.ClosedChannel()).Maybe()

	e, err := NewEngine(unittest.Logger(), cs.net, cs.me, cs.protoState, cs.payloads, cs.core)
	require.NoError(cs.T(), err)
	e.WithConsensus(cs.hotstuff)
	e.WithSync(cs.sync)
	cs.engine = e

	cs.ctx, cs.cancel, cs.errs = irrecoverable.WithSignallerAndCancel(context.Background())
	cs.engine.Start(cs.ctx)
	go unittest.FailOnIrrecoverableError(cs.T(), cs.ctx.Done(), cs.errs)

	unittest.AssertClosesBefore(cs.T(), cs.engine.Ready(), time.Second)
}

// TearDownTest stops the engine and checks there are no errors thrown to the SignallerContext.
func (cs *EngineSuite) TearDownTest() {
	cs.cancel()
	unittest.RequireCloseBefore(cs.T(), cs.engine.Done(), time.Second, "engine failed to stop")
	select {
	case err := <-cs.errs:
		assert.NoError(cs.T(), err)
	default:
	}
}

// TestSendVote tests that single vote can be sent and properly processed
func (cs *EngineSuite) TestSendVote() {
	// create parameters to send a vote
	blockID := unittest.IdentifierFixture()
	view := rand.Uint64()
	sig := unittest.SignatureFixture()
	recipientID := unittest.IdentifierFixture()

	// check it was called with right params
	vote := &messages.ClusterBlockVote{
		BlockID: blockID,
		View:    view,
		SigData: sig,
	}

	done := make(chan struct{})
	*cs.con = *mocknetwork.NewConduit(cs.T())
	cs.con.On("Unicast", vote, recipientID).
		Run(func(_ mock.Arguments) { close(done) }).
		Return(nil).
		Once()

	// submit the vote
	err := cs.engine.SendVote(blockID, view, sig, recipientID)
	require.NoError(cs.T(), err, "should pass send vote")

	// wait for the vote to be sent
	unittest.AssertClosesBefore(cs.T(), done, time.Second)
}

// TestBroadcastProposalWithDelay tests broadcasting proposals with different inputs
func (cs *EngineSuite) TestBroadcastProposalWithDelay() {

	// generate a parent with height and chain ID set
	parent := unittest.ClusterBlockFixture()
	parent.Header.ChainID = "test"
	parent.Header.Height = 10
	cs.headerDB[parent.ID()] = &parent

	// create a block with the parent and store the payload with correct ID
	block := unittest.ClusterBlockWithParent(&parent)
	block.Header.ProposerID = cs.myID
	cs.payloadDB[block.ID()] = block.Payload

	cs.Run("should fail with wrong proposer", func() {
		header := *block.Header
		header.ProposerID = unittest.IdentifierFixture()
		err := cs.engine.BroadcastProposalWithDelay(&header, 0)
		require.Error(cs.T(), err, "should fail with wrong proposer")
		header.ProposerID = cs.myID
	})

	// should fail with changed (missing) parent
	cs.Run("should fail with changed/missing parent", func() {
		header := *block.Header
		header.ParentID[0]++
		err := cs.engine.BroadcastProposalWithDelay(&header, 0)
		require.Error(cs.T(), err, "should fail with missing parent")
		header.ParentID[0]--
	})

	// should fail with wrong block ID (payload unavailable)
	cs.Run("should fail with wrong block ID", func() {
		header := *block.Header
		header.View++
		err := cs.engine.BroadcastProposalWithDelay(&header, 0)
		require.Error(cs.T(), err, "should fail with missing payload")
		header.View--
	})

	cs.Run("should broadcast proposal and pass to HotStuff for valid proposals", func() {
		// unset chain and height to make sure they are correctly reconstructed
		headerFromHotstuff := *block.Header // copy header
		headerFromHotstuff.ChainID = ""
		headerFromHotstuff.Height = 0

		// keep a duplicate of the correct header to check against leader
		header := block.Header
		// make sure chain ID and height were reconstructed and we broadcast to correct nodes
		header.ChainID = "test"
		header.Height = 11
		expectedBroadcastMsg := &messages.ClusterBlockProposal{
			Header:  header,
			Payload: block.Payload,
		}

		submitted := make(chan struct{}) // closed when proposal is submitted to hotstuff
		cs.hotstuff.On("SubmitProposal", &headerFromHotstuff, parent.Header.View).
			Run(func(args mock.Arguments) { close(submitted) }).
			Once()

		broadcasted := make(chan struct{}) // closed when proposal is broadcast
		*cs.con = *mocknetwork.NewConduit(cs.T())
		cs.con.On("Publish", expectedBroadcastMsg, cs.cluster[1].NodeID, cs.cluster[2].NodeID).
			Run(func(_ mock.Arguments) { close(broadcasted) }).
			Return(nil).
			Once()

		// submit to broadcast proposal
		err := cs.engine.BroadcastProposalWithDelay(&headerFromHotstuff, 0)
		require.NoError(cs.T(), err, "header broadcast should pass")

		unittest.AssertClosesBefore(cs.T(), util.AllClosed(broadcasted, submitted), time.Second)
	})
}

// TestSubmittingMultipleVotes tests that we can send multiple votes and they
// are queued and processed in expected way
func (cs *EngineSuite) TestSubmittingMultipleEntries() {
	// create a vote
	originID := unittest.IdentifierFixture()
	voteCount := 15

	channel := channels.ConsensusCluster(cs.clusterID)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for i := 0; i < voteCount; i++ {
			vote := messages.ClusterBlockVote{
				BlockID: unittest.IdentifierFixture(),
				View:    rand.Uint64(),
				SigData: unittest.SignatureFixture(),
			}
			cs.voteAggregator.On("AddVote", &model.Vote{
				View:     vote.View,
				BlockID:  vote.BlockID,
				SignerID: originID,
				SigData:  vote.SigData,
			}).Return().Once()
			// execute the vote submission
			_ = cs.engine.Process(channel, originID, &vote)
		}
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		// create a proposal that directly descends from the latest finalized header
		originID := cs.cluster[1].NodeID
		block := unittest.ClusterBlockWithParent(cs.head)
		proposal := &messages.ClusterBlockProposal{
			Header:  block.Header,
			Payload: block.Payload,
		}

		// store the data for retrieval
		cs.headerDB[block.Header.ParentID] = cs.head
		cs.hotstuff.On("SubmitProposal", block.Header, cs.head.Header.View)
		cs.validator.On("ValidateProposal", model.ProposalFromFlow(block.Header, cs.head.Header.View)).Return(nil)
		err := cs.engine.Process(channel, originID, proposal)
		cs.Assert().NoError(err)
		wg.Done()
	}()

	wg.Wait()

	// wait for the votes queue to drain
	assert.Eventually(cs.T(), func() bool {
		return cs.engine.pendingVotes.(*engine.FifoMessageStore).Len() == 0
	}, time.Second, time.Millisecond*10)
}

// TestProcessUnsupportedMessageType tests that Process and ProcessLocal correctly handle a case where invalid message type
// was submitted from network layer.
func (cs *EngineSuite) TestProcessUnsupportedMessageType() {
	invalidEvent := uint64(42)
	err := cs.engine.Process("ch", unittest.IdentifierFixture(), invalidEvent)
	// shouldn't result in error since byzantine inputs are expected
	require.NoError(cs.T(), err)
}

// TestOnFinalizedBlock tests if finalized block gets processed when send through `Engine`.
// Tests the whole processing pipeline.
func (cs *EngineSuite) TestOnFinalizedBlock() {
	finalizedBlock := unittest.ClusterBlockFixture()
	cs.head = &finalizedBlock

	*cs.pending = module.PendingClusterBlockBuffer{}
	// wait for both expected calls before ending the test
	wg := new(sync.WaitGroup)
	wg.Add(2)
	cs.pending.On("PruneByView", finalizedBlock.Header.View).
		Run(func(_ mock.Arguments) { wg.Done() }).
		Return(nil).Once()
	cs.pending.On("Size").
		Run(func(_ mock.Arguments) { wg.Done() }).
		Return(uint(0)).Once()

	cs.engine.OnFinalizedBlock(model.BlockFromFlow(finalizedBlock.Header, finalizedBlock.Header.View-1))
	unittest.AssertReturnsBefore(cs.T(), wg.Wait, time.Second, "an expected call to block buffer wasn't made")
}
