package synchronization

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	synccore "github.com/onflow/flow-go/module/chainsync"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	netint "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/utils/rand"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestOnSyncRequest_LowerThanReceiver_WithinTolerance tests that a sync request that's within tolerance of the receiver doesn't trigger
// a response, even if request height is lower than receiver.
func (ss *SyncSuite) TestOnSyncRequest_LowerThanReceiver_WithinTolerance() {
	nonce, err := rand.Uint64()
	require.NoError(ss.T(), err, "should generate nonce")
	// generate origin and request message
	originID := unittest.IdentifierFixture()
	req := &flow.SyncRequest{
		Nonce:  nonce,
		Height: 0,
	}

	// regardless of request height, if within tolerance, we should not respond
	ss.core.On("HandleHeight", ss.head, req.Height)
	ss.core.On("WithinTolerance", ss.head, req.Height).Return(true)
	ss.Assert().NoError(ss.e.requestHandler.onSyncRequest(originID, req))
	ss.con.AssertNotCalled(ss.T(), "Unicast", mock.Anything, mock.Anything)
	ss.core.AssertExpectations(ss.T())
}

// TestOnSyncRequest_HigherThanReceiver_OutsideTolerance tests that a sync request that's higher
// than the receiver's height doesn't trigger a response, even if outside tolerance.
func (ss *SyncSuite) TestOnSyncRequest_HigherThanReceiver_OutsideTolerance() {
	nonce, err := rand.Uint64()
	require.NoError(ss.T(), err, "should generate nonce")
	// generate origin and request message
	originID := unittest.IdentifierFixture()
	req := &flow.SyncRequest{
		Nonce:  nonce,
		Height: 0,
	}

	// if request height is higher than local finalized, we should not respond
	req.Height = ss.head.Height + 1

	ss.core.On("HandleHeight", ss.head, req.Height)
	ss.core.On("WithinTolerance", ss.head, req.Height).Return(false)
	ss.Assert().NoError(ss.e.requestHandler.onSyncRequest(originID, req))
	ss.con.AssertNotCalled(ss.T(), "Unicast", mock.Anything, mock.Anything)
	ss.core.AssertExpectations(ss.T())
}

// TestOnSyncRequest_LowerThanReceiver_OutsideTolerance tests that a sync request that's outside tolerance and
// lower than the receiver's height triggers a response.
func (ss *SyncSuite) TestOnSyncRequest_LowerThanReceiver_OutsideTolerance() {
	nonce, err := rand.Uint64()
	require.NoError(ss.T(), err, "should generate nonce")

	// generate origin and request message
	originID := unittest.IdentifierFixture()
	req := &flow.SyncRequest{
		Nonce:  nonce,
		Height: 0,
	}

	// if the request height is lower than head and outside tolerance, we should expect correct response
	req.Height = ss.head.Height - 1
	ss.core.On("HandleHeight", ss.head, req.Height)
	ss.core.On("WithinTolerance", ss.head, req.Height).Return(false)
	ss.con.On("Unicast", mock.Anything, mock.Anything).Return(nil).Run(
		func(args mock.Arguments) {
			res := args.Get(0).(*messages.SyncResponse)
			assert.Equal(ss.T(), *ss.head, res.Header, "response should contain header")
			assert.Equal(ss.T(), req.Nonce, res.Nonce, "response should contain request nonce")
			assert.Equal(ss.T(), *ss.qc, res.CertifyingQC, "response should contain QC")
			assert.Equal(ss.T(), ss.head.ID(), res.CertifyingQC.BlockID, "response QC should correspond to response Header")
			recipientID := args.Get(1).(flow.Identifier)
			assert.Equal(ss.T(), originID, recipientID, "should send response to original sender")
		},
	)
	ss.metrics.On("MessageSent", metrics.EngineSynchronization, metrics.MessageSyncResponse).Once()
	err = ss.e.requestHandler.onSyncRequest(originID, req)
	require.NoError(ss.T(), err, "smaller height sync request should pass")

	ss.core.AssertExpectations(ss.T())
	ss.metrics.AssertExpectations(ss.T())
}

func (ss *SyncSuite) TestOnSyncResponse() {
	nonce, err := rand.Uint64()
	require.NoError(ss.T(), err, "should generate nonce")

	height, err := rand.Uint64()
	require.NoError(ss.T(), err, "should generate height")
	header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))

	// generate origin ID and response message
	originID := unittest.IdentifierFixture()
	res := &flow.SyncResponse{
		Nonce:        nonce,
		Header:       *header,
		CertifyingQC: *unittest.CertifyBlock(header),
	}

	// the height should be handled
	ss.core.On("HandleHeight", ss.head, res.Header.Height)
	ss.e.onSyncResponse(originID, res)
	ss.core.AssertExpectations(ss.T())
}

func (ss *SyncSuite) TestInvalidSyncResponse() {
	ss.T().Skip() // TODO(8174) - implement this test
	nonce, err := rand.Uint64()
	require.NoError(ss.T(), err, "should generate nonce")

	height, err := rand.Uint64()
	require.NoError(ss.T(), err, "should generate height")
	header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))

	// generate origin ID and response message
	originID := unittest.IdentifierFixture()
	res := &flow.SyncResponse{
		Nonce:        nonce,
		Header:       *header,
		CertifyingQC: flow.QuorumCertificate{},
	}
	// TODO(8174): the response should be rejected and/or a violation should be logged
	ss.e.onSyncResponse(originID, res)
	ss.core.AssertExpectations(ss.T())
}

func (ss *SyncSuite) TestOnRangeRequest() {
	nonce, err := rand.Uint64()
	require.NoError(ss.T(), err, "should generate nonce")

	// generate originID and range request
	originID := unittest.IdentifierFixture()
	req := &flow.RangeRequest{
		Nonce:      nonce,
		FromHeight: 0,
		ToHeight:   0,
	}

	// fill in blocks at heights -1 to -4 from head
	ref := ss.head.Height
	for height := ref; height >= ref-4; height-- {
		block := unittest.BlockFixture(
			unittest.Block.WithHeight(height),
		)
		ss.heights[height] = unittest.ProposalFromBlock(block)
		ss.blockIDs[block.ID()] = ss.heights[height]
	}

	// empty range should be a no-op
	ss.T().Run("empty range", func(t *testing.T) {
		req.FromHeight = ref
		req.ToHeight = ref - 1
		err := ss.e.requestHandler.onRangeRequest(originID, req)
		require.NoError(ss.T(), err, "empty range request should pass")
		ss.con.AssertNotCalled(ss.T(), "Unicast", mock.Anything, mock.Anything)
	})

	// range with only unknown block should be a no-op
	ss.T().Run("range with unknown block", func(t *testing.T) {
		req.FromHeight = ref + 1
		req.ToHeight = ref + 3
		err := ss.e.requestHandler.onRangeRequest(originID, req)
		require.NoError(ss.T(), err, "unknown range request should pass")
		ss.con.AssertNotCalled(ss.T(), "Unicast", mock.Anything, mock.Anything)
	})

	// a request for same from and to should send single block
	ss.T().Run("from == to", func(t *testing.T) {
		req.FromHeight = ref - 1
		req.ToHeight = ref - 1
		ss.con.On("Unicast", mock.Anything, mock.Anything).Return(nil).Once().Run(
			func(args mock.Arguments) {
				res := args.Get(0).(*messages.BlockResponse)
				expected := *ss.heights[ref-1]
				internal, err := res.ToInternal()
				require.NoError(t, err)
				actual, ok := internal.(*flow.BlockResponse)
				require.True(t, ok)
				assert.Equal(ss.T(), expected, actual.Blocks[0], "response should contain right block")
				assert.Equal(ss.T(), req.Nonce, res.Nonce, "response should contain request nonce")
				recipientID := args.Get(1).(flow.Identifier)
				assert.Equal(ss.T(), originID, recipientID, "should send response to original requester")
			},
		)
		ss.metrics.On("MessageSent", metrics.EngineSynchronization, metrics.MessageBlockResponse).Once()
		err := ss.e.requestHandler.onRangeRequest(originID, req)
		require.NoError(ss.T(), err, "range request with higher to height should pass")
		ss.con.AssertNumberOfCalls(ss.T(), "Unicast", 1)
		ss.metrics.AssertExpectations(ss.T())

		// clear any expectations for next test - otherwise, next subtest will fail due to increment of expected calls to Unicast
		ss.con.Mock = mock.Mock{}
	})

	// a request for a range that we partially have should send partial response
	ss.T().Run("have partial range", func(t *testing.T) {
		req.FromHeight = ref - 2
		req.ToHeight = ref + 2
		ss.con.On("Unicast", mock.Anything, mock.Anything).Return(nil).Once().Run(
			func(args mock.Arguments) {
				res := args.Get(0).(*messages.BlockResponse)
				expected := []flow.Proposal{*ss.heights[ref-2], *ss.heights[ref-1], *ss.heights[ref]}
				internal, err := res.ToInternal()
				require.NoError(t, err)
				actual, ok := internal.(*flow.BlockResponse)
				require.True(t, ok)
				assert.ElementsMatch(ss.T(), expected, actual.Blocks, "response should contain right blocks")
				assert.Equal(ss.T(), req.Nonce, res.Nonce, "response should contain request nonce")
				recipientID := args.Get(1).(flow.Identifier)
				assert.Equal(ss.T(), originID, recipientID, "should send response to original requester")
			},
		)
		ss.metrics.On("MessageSent", metrics.EngineSynchronization, metrics.MessageBlockResponse).Once()
		err := ss.e.requestHandler.onRangeRequest(originID, req)
		require.NoError(ss.T(), err, "valid range with missing blocks should fail")
		ss.con.AssertNumberOfCalls(ss.T(), "Unicast", 1)
		ss.metrics.AssertExpectations(ss.T())

		// clear any expectations for next test - otherwise, next subtest will fail due to increment of expected calls to Unicast
		ss.con.Mock = mock.Mock{}
	})

	// a request for a range we entirely have should send all blocks
	ss.T().Run("have entire range", func(t *testing.T) {
		req.FromHeight = ref - 2
		req.ToHeight = ref
		ss.con.On("Unicast", mock.Anything, mock.Anything).Return(nil).Once().Run(
			func(args mock.Arguments) {
				res := args.Get(0).(*messages.BlockResponse)
				expected := []flow.Proposal{*ss.heights[ref-2], *ss.heights[ref-1], *ss.heights[ref]}
				internal, err := res.ToInternal()
				require.NoError(t, err)
				actual, ok := internal.(*flow.BlockResponse)
				require.True(t, ok)
				assert.ElementsMatch(ss.T(), expected, actual.Blocks, "response should contain right blocks")
				assert.Equal(ss.T(), req.Nonce, res.Nonce, "response should contain request nonce")
				recipientID := args.Get(1).(flow.Identifier)
				assert.Equal(ss.T(), originID, recipientID, "should send response to original requester")
			},
		)
		ss.metrics.On("MessageSent", metrics.EngineSynchronization, metrics.MessageBlockResponse).Once()
		err := ss.e.requestHandler.onRangeRequest(originID, req)
		require.NoError(ss.T(), err, "valid range request should pass")
		ss.con.AssertNumberOfCalls(ss.T(), "Unicast", 1)
		ss.metrics.AssertExpectations(ss.T())

		// clear any expectations for next test - otherwise, next subtest will fail due to increment of expected calls to Unicast
		ss.con.Mock = mock.Mock{}
	})

	// a request for a range larger than MaxSize should be clamped
	ss.T().Run("oversized range", func(t *testing.T) {
		req.FromHeight = ref - 4
		req.ToHeight = math.MaxUint64
		ss.con.On("Unicast", mock.Anything, mock.Anything).Return(nil).Once().Run(
			func(args mock.Arguments) {
				res := args.Get(0).(*messages.BlockResponse)
				expected := []flow.Proposal{*ss.heights[ref-4], *ss.heights[ref-3], *ss.heights[ref-2]}
				internal, err := res.ToInternal()
				require.NoError(t, err)
				actual, ok := internal.(*flow.BlockResponse)
				require.True(t, ok)
				assert.ElementsMatch(ss.T(), expected, actual.Blocks, "response should contain right blocks")
				assert.Equal(ss.T(), req.Nonce, res.Nonce, "response should contain request nonce")
				recipientID := args.Get(1).(flow.Identifier)
				assert.Equal(ss.T(), originID, recipientID, "should send response to original requester")
			},
		)

		// Rebuild sync core with a smaller max size
		var err error
		config := synccore.DefaultConfig()
		config.MaxSize = 2
		ss.e.requestHandler.core, err = synccore.New(ss.e.log, config, metrics.NewNoopCollector(), flow.Localnet)
		require.NoError(ss.T(), err)

		ss.metrics.On("MessageSent", metrics.EngineSynchronization, metrics.MessageBlockResponse).Once()
		err = ss.e.requestHandler.onRangeRequest(originID, req)
		require.NoError(ss.T(), err, "valid range request exceeding max size should still pass")
		ss.con.AssertNumberOfCalls(ss.T(), "Unicast", 1)
		ss.metrics.AssertExpectations(ss.T())

		// clear any expectations for next test - otherwise, next subtest will fail due to increment of expected calls to Unicast
		ss.con.Mock = mock.Mock{}
	})
}

func (ss *SyncSuite) TestOnBatchRequest() {
	nonce, err := rand.Uint64()
	require.NoError(ss.T(), err, "should generate nonce")

	// generate origin ID and batch request
	originID := unittest.IdentifierFixture()
	req := &flow.BatchRequest{
		Nonce:    nonce,
		BlockIDs: nil,
	}

	// an empty request should not lead to response
	ss.T().Run("empty request", func(t *testing.T) {
		req.BlockIDs = []flow.Identifier{}
		err := ss.e.requestHandler.onBatchRequest(originID, req)
		require.NoError(ss.T(), err, "should pass empty request")
		ss.con.AssertNumberOfCalls(ss.T(), "Unicast", 0)
	})

	// a non-empty request for missing block ID should be a no-op
	ss.T().Run("request for missing blocks", func(t *testing.T) {
		req.BlockIDs = unittest.IdentifierListFixture(1)
		err := ss.e.requestHandler.onBatchRequest(originID, req)
		require.NoError(ss.T(), err, "should pass request for missing block")
		ss.con.AssertNumberOfCalls(ss.T(), "Unicast", 0)
	})

	// a non-empty request for existing block IDs should send right response
	ss.T().Run("request for existing blocks", func(t *testing.T) {
		block := unittest.BlockFixture(
			unittest.Block.WithHeight(ss.head.Height - 1),
		)
		proposal := unittest.ProposalFromBlock(block)
		req.BlockIDs = []flow.Identifier{block.ID()}
		ss.blockIDs[block.ID()] = proposal
		ss.con.On("Unicast", mock.Anything, mock.Anything).Return(nil).Run(
			func(args mock.Arguments) {
				res := args.Get(0).(*messages.BlockResponse)
				internal, err := res.ToInternal()
				require.NoError(t, err)
				actual, ok := internal.(*flow.BlockResponse)
				require.True(t, ok)
				assert.Equal(ss.T(), proposal, &actual.Blocks[0], "response should contain right block")
				assert.Equal(ss.T(), req.Nonce, res.Nonce, "response should contain request nonce")
				recipientID := args.Get(1).(flow.Identifier)
				assert.Equal(ss.T(), originID, recipientID, "response should be send to original requester")
			},
		).Once()
		ss.metrics.On("MessageSent", metrics.EngineSynchronization, metrics.MessageBlockResponse).Once()
		err := ss.e.requestHandler.onBatchRequest(originID, req)
		require.NoError(ss.T(), err, "should pass request with valid block")

		ss.metrics.AssertExpectations(ss.T())
	})

	// a request for too many blocks should be clamped
	ss.T().Run("oversized range", func(t *testing.T) {
		// setup request for 5 blocks. response should contain the first 2 (MaxSize)
		ss.blockIDs = make(map[flow.Identifier]*flow.Proposal)
		req.BlockIDs = make([]flow.Identifier, 5)
		for i := 0; i < len(req.BlockIDs); i++ {
			b := unittest.BlockFixture(
				unittest.Block.WithHeight(ss.head.Height - uint64(i)),
			)
			req.BlockIDs[i] = b.ID()
			ss.blockIDs[b.ID()] = unittest.ProposalFromBlock(b)
		}
		ss.con.On("Unicast", mock.Anything, mock.Anything).Return(nil).Run(
			func(args mock.Arguments) {
				res := args.Get(0).(*messages.BlockResponse)
				expected := []flow.Proposal{*ss.blockIDs[req.BlockIDs[0]], *ss.blockIDs[req.BlockIDs[1]]}
				internal, err := res.ToInternal()
				require.NoError(t, err)
				actual, ok := internal.(*flow.BlockResponse)
				require.True(t, ok)
				assert.ElementsMatch(ss.T(), expected, actual.Blocks, "response should contain right block")
				assert.Equal(ss.T(), req.Nonce, res.Nonce, "response should contain request nonce")
				recipientID := args.Get(1).(flow.Identifier)
				assert.Equal(ss.T(), originID, recipientID, "response should be send to original requester")
			},
		)

		// Rebuild sync core with a smaller max size
		var err error
		config := synccore.DefaultConfig()
		config.MaxSize = 2
		ss.e.requestHandler.core, err = synccore.New(ss.e.log, config, metrics.NewNoopCollector(), flow.Localnet)
		require.NoError(ss.T(), err)
		ss.metrics.On("MessageSent", metrics.EngineSynchronization, metrics.MessageBlockResponse).Once()

		err = ss.e.requestHandler.onBatchRequest(originID, req)
		require.NoError(ss.T(), err, "valid batch request exceeding max size should still pass")

		ss.metrics.AssertExpectations(ss.T())
	})
}

func (ss *SyncSuite) TestOnValidBlockResponse() {
	// generate origin and block response
	originID := unittest.IdentifierFixture()

	// add one block that should be processed
	response := unittest.BlockResponseFixture(2)
	processable := response.Blocks[0]
	ss.core.On("HandleBlock", processable.Block.ToHeader()).Return(true)

	// add one block that should not be processed
	unprocessable := response.Blocks[1]
	ss.core.On("HandleBlock", unprocessable.Block.ToHeader()).Return(false)

	ss.comp.On("OnSyncedBlocks", mock.Anything).Run(func(args mock.Arguments) {
		res := args.Get(0).(flow.Slashable[[]*flow.Proposal])
		actual := res.Message[0]
		ss.Assert().Equal(processable.Block.HeaderBody, actual.Block.HeaderBody)
		ss.Assert().Equal(processable.Block.Payload, actual.Block.Payload)
		ss.Assert().Equal(originID, res.OriginID)
	})

	ss.e.onBlockResponse(originID, response)
	ss.core.AssertExpectations(ss.T())
}

func (ss *SyncSuite) TestPollHeight() {

	// check that we send to three nodes from our total list
	others := ss.participants.Filter(filter.HasNodeID[flow.Identity](ss.participants[1:].NodeIDs()...))
	ss.con.On("Multicast", mock.Anything, synccore.DefaultPollNodes, others[0].NodeID, others[1].NodeID).Return(nil).Run(
		func(args mock.Arguments) {
			req := args.Get(0).(*messages.SyncRequest)
			require.Equal(ss.T(), ss.head.Height, req.Height, "request should contain finalized height")
		},
	)
	ss.metrics.On("MessageSent", metrics.EngineSynchronization, metrics.MessageSyncRequest).Once()
	ss.e.pollHeight()
	ss.con.AssertExpectations(ss.T())
	ss.metrics.AssertExpectations(ss.T())
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
	ss.metrics.On("MessageSent", metrics.EngineSynchronization, metrics.MessageBatchRequest).Once()
	ss.metrics.On("MessageSent", metrics.EngineSynchronization, metrics.MessageRangeRequest).Once()

	// exclude my node ID
	ss.e.sendRequests(ss.participants[1:].NodeIDs(), ranges, batches)
	ss.con.AssertExpectations(ss.T())
	ss.metrics.AssertExpectations(ss.T())
}

// test a synchronization engine can be started and stopped
func (ss *SyncSuite) TestStartStop() {
	ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(ss.T(), context.Background())
	ss.e.Start(ctx)
	unittest.AssertClosesBefore(ss.T(), ss.e.Ready(), time.Second)
	cancel()
	unittest.AssertClosesBefore(ss.T(), ss.e.Done(), time.Second)
}

// TestProcessingMultipleItems tests that items are processed in async way
func (ss *SyncSuite) TestProcessingMultipleItems() {
	ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(ss.T(), context.Background())
	ss.e.Start(ctx)
	unittest.AssertClosesBefore(ss.T(), ss.e.Ready(), time.Second)
	defer cancel()

	originID := unittest.IdentifierFixture()
	for i := 0; i < 5; i++ {
		header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(uint64(1000 + i)))
		msg := &flow.SyncResponse{
			Nonce:        uint64(i),
			Header:       *header,
			CertifyingQC: *unittest.CertifyBlock(header),
		}
		ss.core.On("HandleHeight", mock.Anything, msg.Header.Height).Once()
		ss.metrics.On("MessageSent", metrics.EngineSynchronization, metrics.MessageSyncResponse).Once()
		ss.metrics.On("MessageHandled", metrics.EngineSynchronization, metrics.MessageSyncResponse).Once()
		ss.metrics.On("MessageReceived", metrics.EngineSynchronization, metrics.MessageSyncResponse).Once()

		require.NoError(ss.T(), ss.e.Process(channels.SyncCommittee, originID, msg))
	}

	finalHeight := ss.head.Height
	for i := 0; i < 5; i++ {
		msg := &flow.SyncRequest{
			Nonce:  uint64(i),
			Height: finalHeight - 100,
		}

		originID := unittest.IdentifierFixture()
		ss.core.On("WithinTolerance", mock.Anything, mock.Anything).Return(false)
		ss.core.On("HandleHeight", mock.Anything, msg.Height).Once()
		ss.con.On("Unicast", mock.Anything, mock.Anything).Return(nil)
		ss.metrics.On("MessageReceived", metrics.EngineSynchronization, metrics.MessageSyncRequest).Once()

		// misbehavior might or might not be reported
		ss.con.On("ReportMisbehavior", mock.Anything).Return(mock.Anything).Maybe()

		require.NoError(ss.T(), ss.e.Process(channels.SyncCommittee, originID, msg))
	}

	// give at least some time to process items
	time.Sleep(time.Millisecond * 100)

	ss.core.AssertExpectations(ss.T())
	ss.metrics.AssertExpectations(ss.T())
}

// TestProcessUnsupportedMessageType tests that Process and ProcessLocal correctly handle a case where invalid message type
// was submitted from network layer.
func (ss *SyncSuite) TestProcessUnsupportedMessageType() {
	invalidEvent := uint64(42)
	engines := []netint.MessageProcessor{ss.e, ss.e.requestHandler}
	for _, e := range engines {
		err := e.Process("ch", unittest.IdentifierFixture(), invalidEvent)
		// shouldn't result in error since byzantine inputs are expected
		require.NoError(ss.T(), err)
	}
}
