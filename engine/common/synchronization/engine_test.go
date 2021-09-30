package synchronization

import (
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	synccore "github.com/onflow/flow-go/module/synchronization"
	netint "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/p2p"
	protocolint "github.com/onflow/flow-go/state/protocol"
	protocolEvents "github.com/onflow/flow-go/state/protocol/events"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storerr "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestSyncEngine(t *testing.T) {
	suite.Run(t, new(SyncSuite))
}

type SyncSuite struct {
	suite.Suite
	myID         flow.Identifier
	participants flow.IdentityList
	head         *flow.Header
	heights      map[uint64]*flow.Block
	blockIDs     map[flow.Identifier]*flow.Block
	net          *mocknetwork.Network
	con          *mocknetwork.Conduit
	me           *module.Local
	state        *protocol.State
	snapshot     *protocol.Snapshot
	blocks       *storage.Blocks
	comp         *mocknetwork.Engine
	core         *module.SyncCore
	e            *Engine
}

func (ss *SyncSuite) SetupTest() {
	// seed the RNG
	rand.Seed(time.Now().UnixNano())

	// generate own ID
	ss.participants = unittest.IdentityListFixture(3, unittest.WithRole(flow.RoleConsensus))
	keys, err := unittest.NetworkingKeys(len(ss.participants))
	require.NoError(ss.T(), err)

	for i, p := range ss.participants {
		p.NetworkPubKey = keys[i].PublicKey()
	}
	ss.myID = ss.participants[0].NodeID

	// generate a header for the final state
	header := unittest.BlockHeaderFixture()
	ss.head = &header

	// create maps to enable block returns
	ss.heights = make(map[uint64]*flow.Block)
	ss.blockIDs = make(map[flow.Identifier]*flow.Block)

	// set up the network module mock
	ss.net = &mocknetwork.Network{}
	ss.net.On("Register", mock.Anything, mock.Anything).Return(
		func(channel netint.Channel, engine netint.Engine) netint.Conduit {
			return ss.con
		},
		nil,
	)

	// set up the network conduit mock
	ss.con = &mocknetwork.Conduit{}

	// set up the local module mock
	ss.me = &module.Local{}
	ss.me.On("NodeID").Return(
		func() flow.Identifier {
			return ss.myID
		},
	)

	// set up the protocol state mock
	ss.state = &protocol.State{}
	ss.state.On("Final").Return(
		func() protocolint.Snapshot {
			return ss.snapshot
		},
	)
	ss.state.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) protocolint.Snapshot {
			if ss.head.ID() == blockID {
				return ss.snapshot
			} else {
				return unittest.StateSnapshotForUnknownBlock()
			}
		},
	).Maybe()

	// set up the snapshot mock
	ss.snapshot = &protocol.Snapshot{}
	ss.snapshot.On("Head").Return(
		func() *flow.Header {
			return ss.head
		},
		nil,
	)
	ss.snapshot.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter) flow.IdentityList {
			return ss.participants.Filter(selector)
		},
		nil,
	)

	// set up blocks storage mock
	ss.blocks = &storage.Blocks{}
	ss.blocks.On("ByHeight", mock.Anything).Return(
		func(height uint64) *flow.Block {
			return ss.heights[height]
		},
		func(height uint64) error {
			_, enabled := ss.heights[height]
			if !enabled {
				return storerr.ErrNotFound
			}
			return nil
		},
	)
	ss.blocks.On("ByID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.Block {
			return ss.blockIDs[blockID]
		},
		func(blockID flow.Identifier) error {
			_, enabled := ss.blockIDs[blockID]
			if !enabled {
				return storerr.ErrNotFound
			}
			return nil
		},
	)

	// set up compliance engine mock
	ss.comp = &mocknetwork.Engine{}
	ss.comp.On("SubmitLocal", mock.Anything).Return()

	// set up sync core
	ss.core = &module.SyncCore{}

	// initialize the engine
	log := zerolog.New(ioutil.Discard)
	metrics := metrics.NewNoopCollector()

	finalizedHeader, err := NewFinalizedHeaderCache(log, ss.state, pubsub.NewFinalizationDistributor())
	require.NoError(ss.T(), err, "could not create finalized snapshot cache")

	idCache, err := p2p.NewProtocolStateIDCache(log, ss.state, protocolEvents.NewDistributor())
	require.NoError(ss.T(), err, "could not create protocol state identity cache")
	e, err := New(log, metrics, ss.net, ss.me, ss.blocks, ss.comp, ss.core, finalizedHeader,
		id.NewFilteredIdentifierProvider(
			filter.And(
				filter.HasRole(flow.RoleConsensus),
				filter.Not(filter.HasNodeID(ss.me.NodeID())),
			),
			idCache,
		))
	require.NoError(ss.T(), err, "should pass engine initialization")

	ss.e = e
}

func (ss *SyncSuite) TestOnSyncRequest() {

	// generate origin and request message
	originID := unittest.IdentifierFixture()
	req := &messages.SyncRequest{
		Nonce:  rand.Uint64(),
		Height: 0,
	}

	// regardless of request height, if within tolerance, we should not respond
	ss.core.On("HandleHeight", ss.head, req.Height)
	ss.core.On("WithinTolerance", ss.head, req.Height).Return(true)
	err := ss.e.requestHandler.onSyncRequest(originID, req)
	ss.Assert().NoError(err, "same height sync request should pass")
	ss.con.AssertNotCalled(ss.T(), "Unicast", mock.Anything, mock.Anything)

	// if request height is higher than local finalized, we should not respond
	req.Height = ss.head.Height + 1
	ss.core.On("HandleHeight", ss.head, req.Height)
	ss.core.On("WithinTolerance", ss.head, req.Height).Return(false)
	err = ss.e.requestHandler.onSyncRequest(originID, req)
	ss.Assert().NoError(err, "same height sync request should pass")
	ss.con.AssertNotCalled(ss.T(), "Unicast", mock.Anything, mock.Anything)

	// if the request height is lower than head and outside tolerance, we should submit correct response
	req.Height = ss.head.Height - 1
	ss.core.On("HandleHeight", ss.head, req.Height)
	ss.core.On("WithinTolerance", ss.head, req.Height).Return(false)
	ss.con.On("Unicast", mock.Anything, mock.Anything).Return(nil).Run(
		func(args mock.Arguments) {
			res := args.Get(0).(*messages.SyncResponse)
			assert.Equal(ss.T(), ss.head.Height, res.Height, "response should contain head height")
			assert.Equal(ss.T(), req.Nonce, res.Nonce, "response should contain request nonce")
			recipientID := args.Get(1).(flow.Identifier)
			assert.Equal(ss.T(), originID, recipientID, "should send response to original sender")
		},
	)
	err = ss.e.requestHandler.onSyncRequest(originID, req)
	require.NoError(ss.T(), err, "smaller height sync request should pass")

	ss.core.AssertExpectations(ss.T())
}

func (ss *SyncSuite) TestOnSyncResponse() {

	// generate origin ID and response message
	originID := unittest.IdentifierFixture()
	res := &messages.SyncResponse{
		Nonce:  rand.Uint64(),
		Height: rand.Uint64(),
	}

	// the height should be handled
	ss.core.On("HandleHeight", ss.head, res.Height)
	ss.e.onSyncResponse(originID, res)
	ss.core.AssertExpectations(ss.T())
}

func (ss *SyncSuite) TestOnRangeRequest() {

	// generate originID and range request
	originID := unittest.IdentifierFixture()
	req := &messages.RangeRequest{
		Nonce:      rand.Uint64(),
		FromHeight: 0,
		ToHeight:   0,
	}

	// fill in blocks at heights -1 to -4 from head
	ref := ss.head.Height
	for height := ref; height >= ref-4; height-- {
		block := unittest.BlockFixture()
		block.Header.Height = height
		ss.heights[height] = &block
	}

	// empty range should be a no-op
	req.FromHeight = ref
	req.ToHeight = ref - 1
	err := ss.e.requestHandler.onRangeRequest(originID, req)
	require.NoError(ss.T(), err, "empty range request should pass")
	ss.con.AssertNumberOfCalls(ss.T(), "Unicast", 0)

	// range with only unknown block should be a no-op
	req.FromHeight = ref + 1
	req.ToHeight = ref + 3
	err = ss.e.requestHandler.onRangeRequest(originID, req)
	require.NoError(ss.T(), err, "unknown range request should pass")
	ss.con.AssertNumberOfCalls(ss.T(), "Unicast", 0)

	// a request for same from and to should send single block
	req.FromHeight = ref - 1
	req.ToHeight = ref - 1
	ss.con.On("Unicast", mock.Anything, mock.Anything).Return(nil).Once().Run(
		func(args mock.Arguments) {
			res := args.Get(0).(*messages.BlockResponse)
			expected := []*flow.Block{ss.heights[ref-1]}
			assert.ElementsMatch(ss.T(), expected, res.Blocks, "response should contain right blocks")
			assert.Equal(ss.T(), req.Nonce, res.Nonce, "response should contain request nonce")
			recipientID := args.Get(1).(flow.Identifier)
			assert.Equal(ss.T(), originID, recipientID, "should send response to original requester")
		},
	)
	err = ss.e.requestHandler.onRangeRequest(originID, req)
	require.NoError(ss.T(), err, "range request with higher to height should pass")

	// a request for a range that we partially have should send partial response
	req.FromHeight = ref - 2
	req.ToHeight = ref + 2
	ss.con.On("Unicast", mock.Anything, mock.Anything).Return(nil).Once().Run(
		func(args mock.Arguments) {
			res := args.Get(0).(*messages.BlockResponse)
			expected := []*flow.Block{ss.heights[ref-2], ss.heights[ref-1], ss.heights[ref]}
			assert.ElementsMatch(ss.T(), expected, res.Blocks, "response should contain right blocks")
			assert.Equal(ss.T(), req.Nonce, res.Nonce, "response should contain request nonce")
			recipientID := args.Get(1).(flow.Identifier)
			assert.Equal(ss.T(), originID, recipientID, "should send response to original requester")
		},
	)
	err = ss.e.requestHandler.onRangeRequest(originID, req)
	require.NoError(ss.T(), err, "valid range with missing blocks should fail")

	// a request for a range we entirely have should send all blocks
	req.FromHeight = ref - 2
	req.ToHeight = ref
	ss.con.On("Unicast", mock.Anything, mock.Anything).Return(nil).Once().Run(
		func(args mock.Arguments) {
			res := args.Get(0).(*messages.BlockResponse)
			expected := []*flow.Block{ss.heights[ref-2], ss.heights[ref-1], ss.heights[ref]}
			assert.ElementsMatch(ss.T(), expected, res.Blocks, "response should contain right blocks")
			assert.Equal(ss.T(), req.Nonce, res.Nonce, "response should contain request nonce")
			recipientID := args.Get(1).(flow.Identifier)
			assert.Equal(ss.T(), originID, recipientID, "should send response to original requester")
		},
	)
	err = ss.e.requestHandler.onRangeRequest(originID, req)
	require.NoError(ss.T(), err, "valid range request should pass")

	// a request for a range larger than MaxSize should be clamped
	req.FromHeight = ref - uint64(synccore.DefaultConfig().MaxSize) - 1
	req.ToHeight = ref
	ss.con.On("Unicast", mock.Anything, mock.Anything).Return(nil).Once().Run(
		func(args mock.Arguments) {
			res := args.Get(0).(*messages.BlockResponse)
			assert.Len(ss.T(), res.Blocks, int(synccore.DefaultConfig().MaxSize))
			assert.Equal(ss.T(), req.Nonce, res.Nonce, "response should contain request nonce")
			recipientID := args.Get(1).(flow.Identifier)
			assert.Equal(ss.T(), originID, recipientID, "should send response to original requester")
		},
	)
	err = ss.e.requestHandler.onRangeRequest(originID, req)
	require.NoError(ss.T(), err, "valid range request exceeding max size should still pass")
}

func (ss *SyncSuite) TestOnBatchRequest() {

	// generate origin ID and batch request
	originID := unittest.IdentifierFixture()
	req := &messages.BatchRequest{
		Nonce:    rand.Uint64(),
		BlockIDs: nil,
	}

	// an empty request should not lead to response
	req.BlockIDs = []flow.Identifier{}
	err := ss.e.requestHandler.onBatchRequest(originID, req)
	require.NoError(ss.T(), err, "should pass empty request")
	ss.con.AssertNumberOfCalls(ss.T(), "Unicast", 0)

	// a non-empty request for missing block ID should be a no-op
	req.BlockIDs = unittest.IdentifierListFixture(1)
	err = ss.e.requestHandler.onBatchRequest(originID, req)
	require.NoError(ss.T(), err, "should pass request for missing block")
	ss.con.AssertNumberOfCalls(ss.T(), "Unicast", 0)

	// a non-empty request for existing block IDs should send right response
	block := unittest.BlockFixture()
	block.Header.Height = ss.head.Height - 1
	req.BlockIDs = []flow.Identifier{block.ID()}
	ss.blockIDs[block.ID()] = &block
	ss.con.On("Unicast", mock.Anything, mock.Anything).Return(nil).Run(
		func(args mock.Arguments) {
			res := args.Get(0).(*messages.BlockResponse)
			assert.ElementsMatch(ss.T(), []*flow.Block{&block}, res.Blocks, "response should contain right block")
			assert.Equal(ss.T(), req.Nonce, res.Nonce, "response should contain request nonce")
			recipientID := args.Get(1).(flow.Identifier)
			assert.Equal(ss.T(), originID, recipientID, "response should be send to original requester")
		},
	).Once()
	err = ss.e.requestHandler.onBatchRequest(originID, req)
	require.NoError(ss.T(), err, "should pass request with valid block")

	// a request for too many blocks should be clamped
	for i := 0; i < int(synccore.DefaultConfig().MaxSize); i++ {
		block := unittest.BlockFixture()
		block.Header.Height = ss.head.Height - 2 - uint64(i)
		req.BlockIDs = append(req.BlockIDs, block.ID())
		ss.blockIDs[block.ID()] = &block
	}
	ss.con.On("Unicast", mock.Anything, mock.Anything).Return(nil).Run(
		func(args mock.Arguments) {
			res := args.Get(0).(*messages.BlockResponse)
			assert.Len(ss.T(), res.Blocks, int(synccore.DefaultConfig().MaxSize))
			assert.Equal(ss.T(), req.Nonce, res.Nonce, "response should contain request nonce")
			recipientID := args.Get(1).(flow.Identifier)
			assert.Equal(ss.T(), originID, recipientID, "response should be send to original requester")
		},
	)
	err = ss.e.requestHandler.onBatchRequest(originID, req)
	require.NoError(ss.T(), err, "valid batch request exceeding max size should still pass")
}

func (ss *SyncSuite) TestOnBlockResponse() {

	// generate origin and block response
	originID := unittest.IdentifierFixture()
	res := &messages.BlockResponse{
		Nonce:  rand.Uint64(),
		Blocks: []*flow.Block{},
	}

	// add one block that should be processed
	processable := unittest.BlockFixture()
	ss.core.On("HandleBlock", processable.Header).Return(true)
	res.Blocks = append(res.Blocks, &processable)

	// add one block that should not be processed
	unprocessable := unittest.BlockFixture()
	ss.core.On("HandleBlock", unprocessable.Header).Return(false)
	res.Blocks = append(res.Blocks, &unprocessable)

	ss.comp.On("SubmitLocal", mock.Anything).Run(func(args mock.Arguments) {
		res := args.Get(0).(*events.SyncedBlock)
		ss.Assert().Equal(&processable, res.Block)
		ss.Assert().Equal(originID, res.OriginID)
	},
	)

	ss.e.onBlockResponse(originID, res)
	ss.comp.AssertExpectations(ss.T())
	ss.core.AssertExpectations(ss.T())
}

func (ss *SyncSuite) TestPollHeight() {

	// check that we send to three nodes from our total list
	others := ss.participants.Filter(filter.HasNodeID(ss.participants[1:].NodeIDs()...))
	ss.con.On("Multicast", mock.Anything, synccore.DefaultPollNodes, others[0].NodeID, others[1].NodeID).Return(nil).Run(
		func(args mock.Arguments) {
			req := args.Get(0).(*messages.SyncRequest)
			require.Equal(ss.T(), ss.head.Height, req.Height, "request should contain finalized height")
		},
	)
	ss.e.pollHeight()
	ss.con.AssertExpectations(ss.T())
}

func (ss *SyncSuite) TestSendRequests() {

	ranges := unittest.RangeListFixture(1)
	batches := unittest.BatchListFixture(1)

	// should submit and mark requested all ranges
	ss.con.On("Multicast", mock.AnythingOfType("*messages.RangeRequest"), synccore.DefaultBlockRequestNodes, mock.Anything, mock.Anything).Return(nil).Run(
		func(args mock.Arguments) {
			req := args.Get(0).(*messages.RangeRequest)
			ss.Assert().Equal(ranges[0].From, req.FromHeight)
			ss.Assert().Equal(ranges[0].To, req.ToHeight)
		},
	)
	ss.core.On("RangeRequested", ranges[0])

	// should submit and mark requested all batches
	ss.con.On("Multicast", mock.AnythingOfType("*messages.BatchRequest"), synccore.DefaultBlockRequestNodes, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(
		func(args mock.Arguments) {
			req := args.Get(0).(*messages.BatchRequest)
			ss.Assert().Equal(batches[0].BlockIDs, req.BlockIDs)
		},
	)
	ss.core.On("BatchRequested", batches[0])

	// exclude my node ID
	ss.e.sendRequests(ss.participants[1:].NodeIDs(), ranges, batches)
	ss.con.AssertExpectations(ss.T())
}

// test a synchronization engine can be started and stopped
func (ss *SyncSuite) TestStartStop() {
	unittest.AssertReturnsBefore(ss.T(), func() {
		<-ss.e.Ready()
		<-ss.e.Done()
	}, time.Second)
}

// TestProcessingMultipleItems tests that items are processed in async way
func (ss *SyncSuite) TestProcessingMultipleItems() {
	<-ss.e.Ready()

	originID := unittest.IdentifierFixture()
	for i := 0; i < 5; i++ {
		msg := &messages.SyncResponse{
			Nonce:  uint64(i),
			Height: uint64(1000 + i),
		}
		ss.core.On("HandleHeight", mock.Anything, msg.Height).Once()
		require.NoError(ss.T(), ss.e.Process(engine.SyncCommittee, originID, msg))
	}

	finalHeight := ss.head.Height
	for i := 0; i < 5; i++ {
		msg := &messages.SyncRequest{
			Nonce:  uint64(i),
			Height: finalHeight - 100,
		}

		originID := unittest.IdentifierFixture()
		ss.core.On("WithinTolerance", mock.Anything, mock.Anything).Return(false)
		ss.core.On("HandleHeight", mock.Anything, msg.Height).Once()
		ss.con.On("Unicast", mock.Anything, mock.Anything).Return(nil)

		require.NoError(ss.T(), ss.e.Process(engine.SyncCommittee, originID, msg))
	}

	// give at least some time to process items
	time.Sleep(time.Millisecond * 100)

	ss.core.AssertExpectations(ss.T())
}

// TestOnFinalizedBlock tests that when new finalized block is discovered engine updates cached variables
// to latest state
func (ss *SyncSuite) TestOnFinalizedBlock() {
	finalizedBlock := unittest.BlockHeaderWithParentFixture(ss.head)
	// change head
	ss.head = &finalizedBlock

	err := ss.e.finalizedHeader.updateHeader()
	require.NoError(ss.T(), err)
	actualHeader := ss.e.finalizedHeader.Get()
	require.ElementsMatch(ss.T(), ss.e.participantsProvider.Identifiers(), ss.participants[1:].NodeIDs())
	require.Equal(ss.T(), actualHeader, &finalizedBlock)
}
