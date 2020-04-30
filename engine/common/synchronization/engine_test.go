// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package synchronization

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/model/events"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	module "github.com/dapperlabs/flow-go/module/mock"
	netint "github.com/dapperlabs/flow-go/network"
	network "github.com/dapperlabs/flow-go/network/mock"
	protoint "github.com/dapperlabs/flow-go/state/protocol"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	storerr "github.com/dapperlabs/flow-go/storage"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
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
	net          *module.Network
	con          *network.Conduit
	me           *module.Local
	state        *protocol.State
	snapshot     *protocol.Snapshot
	blocks       *storage.Blocks
	comp         *network.Engine
	e            *Engine
}

func (ss *SyncSuite) SetupTest() {
	// seed the RNG
	rand.Seed(time.Now().UnixNano())

	// generate own ID
	ss.participants = unittest.IdentityListFixture(8, unittest.WithRole(flow.RoleConsensus))
	ss.myID = ss.participants[0].NodeID

	// generate a header for the final state
	header := unittest.BlockHeaderFixture()
	ss.head = &header

	// create maps to enable block returns
	ss.heights = make(map[uint64]*flow.Block)
	ss.blockIDs = make(map[flow.Identifier]*flow.Block)

	// set up the network module mock
	ss.net = &module.Network{}
	ss.net.On("Register", mock.Anything, mock.Anything).Return(
		func(code uint8, engine netint.Engine) netint.Conduit {
			return ss.con
		},
		nil,
	)

	// set up the network conduit mock
	ss.con = &network.Conduit{}

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
		func() protoint.Snapshot {
			return ss.snapshot
		},
	)

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
	ss.comp = &network.Engine{}
	ss.comp.On("SubmitLocal", mock.Anything).Return()

	// initialize the engine
	log := zerolog.New(ioutil.Discard)
	e, err := New(log, ss.net, ss.me, ss.state, ss.blocks, ss.comp)
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

	// on same height as final, we should do nothing and have no error
	req.Height = ss.head.Height
	err := ss.e.onSyncRequest(originID, req)
	require.NoError(ss.T(), err, "same height sync request should pass")
	ss.con.AssertNotCalled(ss.T(), "Submit", mock.Anything, mock.Anything)
	assert.Empty(ss.T(), ss.e.heights, "no height should be added to status map")

	// if the request height is higher than head, we should queue the height
	req.Height = ss.head.Height + 1
	err = ss.e.onSyncRequest(originID, req)
	require.NoError(ss.T(), err, "bigger height sync request should pass")
	ss.con.AssertNotCalled(ss.T(), "Submit", mock.Anything, mock.Anything)
	assert.Contains(ss.T(), ss.e.heights, req.Height, "status map should contain request height")

	// if the request height is lower than head, we should submit correct response
	ss.con.On("Submit", mock.Anything, mock.Anything).Return(nil).Run(
		func(args mock.Arguments) {
			res := args.Get(0).(*messages.SyncResponse)
			assert.Equal(ss.T(), ss.head.Height, res.Height, "response should contain head height")
			assert.Equal(ss.T(), req.Nonce, res.Nonce, "response should contain request nonce")
			recipientID := args.Get(1).(flow.Identifier)
			assert.Equal(ss.T(), originID, recipientID, "should send response to original sender")
		},
	)
	req.Height = ss.head.Height - 1
	err = ss.e.onSyncRequest(originID, req)
	require.NoError(ss.T(), err, "smaller height sync request should pass")
}

func (ss *SyncSuite) TestOnSyncResponse() {

	// generate origin ID and response message
	originID := unittest.IdentifierFixture()
	res := &messages.SyncResponse{
		Nonce:  rand.Uint64(),
		Height: 0,
	}

	// if the response height is below or equal to our finalized, do nothing
	res.Height = ss.head.Height
	err := ss.e.onSyncResponse(originID, res)
	require.NoError(ss.T(), err, "sync response with equal height should pass")
	assert.Empty(ss.T(), ss.e.heights, "no heights should be added to the status map")

	// if the response is higher than our finalized, queue the height
	res.Height = ss.head.Height + 1
	err = ss.e.onSyncResponse(originID, res)
	require.NoError(ss.T(), err, "sync response with higher height should pass")
	assert.Contains(ss.T(), ss.e.heights, res.Height, "status map should contain response height")
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
		block.Height = height
		ss.heights[height] = &block
	}

	// empty range should be a no-op
	req.FromHeight = ref
	req.ToHeight = ref - 1
	err := ss.e.onRangeRequest(originID, req)
	require.NoError(ss.T(), err, "empty range request should pass")
	ss.con.AssertNumberOfCalls(ss.T(), "Submit", 0)

	// range with only unknown block should be a no-op
	req.FromHeight = ref + 1
	req.ToHeight = ref + 3
	err = ss.e.onRangeRequest(originID, req)
	require.NoError(ss.T(), err, "unknown range request should pass")
	ss.con.AssertNumberOfCalls(ss.T(), "Submit", 0)

	// a request for same from and to should send single block
	req.FromHeight = ref - 1
	req.ToHeight = ref - 1
	ss.con.On("Submit", mock.Anything, mock.Anything).Return(nil).Once().Run(
		func(args mock.Arguments) {
			res := args.Get(0).(*messages.BlockResponse)
			expected := []*flow.Block{ss.heights[ref-1]}
			assert.ElementsMatch(ss.T(), expected, res.Blocks, "response should contain right blocks")
			assert.Equal(ss.T(), req.Nonce, res.Nonce, "response should contain request nonce")
			recipientID := args.Get(1).(flow.Identifier)
			assert.Equal(ss.T(), originID, recipientID, "should send response to original requester")
		},
	)
	err = ss.e.onRangeRequest(originID, req)
	require.NoError(ss.T(), err, "range request with higher to height should pass")

	// a request for a range that we partially have should send partial response
	req.FromHeight = ref - 2
	req.ToHeight = ref + 2
	ss.con.On("Submit", mock.Anything, mock.Anything).Return(nil).Once().Run(
		func(args mock.Arguments) {
			res := args.Get(0).(*messages.BlockResponse)
			expected := []*flow.Block{ss.heights[ref-2], ss.heights[ref-1], ss.heights[ref]}
			assert.ElementsMatch(ss.T(), expected, res.Blocks, "response should contain right blocks")
			assert.Equal(ss.T(), req.Nonce, res.Nonce, "response should contain request nonce")
			recipientID := args.Get(1).(flow.Identifier)
			assert.Equal(ss.T(), originID, recipientID, "should send response to original requester")
		},
	)
	err = ss.e.onRangeRequest(originID, req)
	require.NoError(ss.T(), err, "valid range with missing blocks should fail")

	// a request for a range we entirely have should send all blocks
	req.FromHeight = ref - 2
	req.ToHeight = ref
	ss.con.On("Submit", mock.Anything, mock.Anything).Return(nil).Once().Run(
		func(args mock.Arguments) {
			res := args.Get(0).(*messages.BlockResponse)
			expected := []*flow.Block{ss.heights[ref-2], ss.heights[ref-1], ss.heights[ref]}
			assert.ElementsMatch(ss.T(), expected, res.Blocks, "response should contain right blocks")
			assert.Equal(ss.T(), req.Nonce, res.Nonce, "response should contain request nonce")
			recipientID := args.Get(1).(flow.Identifier)
			assert.Equal(ss.T(), originID, recipientID, "should send response to original requester")
		},
	)
	err = ss.e.onRangeRequest(originID, req)
	require.NoError(ss.T(), err, "valid range request should pass")
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
	err := ss.e.onBatchRequest(originID, req)
	require.NoError(ss.T(), err, "should pass empty request")
	ss.con.AssertNumberOfCalls(ss.T(), "Submit", 0)

	// a non-empty request for missing block ID should be a no-op
	req.BlockIDs = []flow.Identifier{ss.head.ID()}
	err = ss.e.onBatchRequest(originID, req)
	require.NoError(ss.T(), err, "should pass request for missing block")
	ss.con.AssertNumberOfCalls(ss.T(), "Submit", 0)

	// a non-empty request for existing block IDs should send right response
	block := unittest.BlockFixture()
	block.Height = ss.head.Height + 1 // this should work, even if it should never happen
	req.BlockIDs = []flow.Identifier{block.ID()}
	ss.blockIDs[block.ID()] = &block
	ss.con.On("Submit", mock.Anything, mock.Anything).Return(nil).Run(
		func(args mock.Arguments) {
			res := args.Get(0).(*messages.BlockResponse)
			assert.ElementsMatch(ss.T(), []*flow.Block{&block}, res.Blocks, "response should contain right block")
			assert.Equal(ss.T(), req.Nonce, res.Nonce, "response should contain request nonce")
			recipientID := args.Get(1).(flow.Identifier)
			assert.Equal(ss.T(), originID, recipientID, "response should be send to original requester")
		},
	)
	err = ss.e.onBatchRequest(originID, req)
	require.NoError(ss.T(), err, "should pass request with valid block")
}

func (ss *SyncSuite) TestOnBlockResponse() {

	// generate origin and block response
	originID := unittest.IdentifierFixture()
	res := &messages.BlockResponse{
		Nonce:  rand.Uint64(),
		Blocks: []*flow.Block{},
	}

	// check if we want the block by height
	block1 := unittest.BlockFixture()
	ss.e.heights[block1.Height] = nil
	res.Blocks = []*flow.Block{&block1}
	err := ss.e.onBlockResponse(originID, res)
	require.NoError(ss.T(), err, "should pass block response (by height)")
	assert.Empty(ss.T(), ss.e.heights, "heights map should now be empty")
	ss.comp.AssertCalled(ss.T(), "SubmitLocal", &events.SyncedBlock{OriginID: originID, Block: &block1})

	// check if we want the block by block ID
	block2 := unittest.BlockFixture()
	ss.e.blockIDs[block2.ID()] = nil
	res.Blocks = []*flow.Block{&block2}
	err = ss.e.onBlockResponse(originID, res)
	require.NoError(ss.T(), err, "should pass block response (by block ID)")
	assert.Empty(ss.T(), ss.e.blockIDs, "block IDs map should be empty")
	ss.comp.AssertCalled(ss.T(), "SubmitLocal", &events.SyncedBlock{OriginID: originID, Block: &block2})

	// check if we want the block by neither height or block ID
	block3 := unittest.BlockFixture()
	res.Blocks = []*flow.Block{&block3}
	err = ss.e.onBlockResponse(originID, res)
	require.NoError(ss.T(), err, "should pass on unwanted block")
	ss.comp.AssertNotCalled(ss.T(), "SubmitLocal", &events.SyncedBlock{OriginID: originID, Block: &block3})
}

func (ss *SyncSuite) TestQueueByHeight() {

	// generate a number of heights
	var heights []uint64
	for n := 0; n < 100; n++ {
		heights = append(heights, rand.Uint64())
	}

	// add all of them to engine
	for _, height := range heights {
		ss.e.queueByHeight(height)
	}

	// check they are all in the map now
	for _, height := range heights {
		assert.Contains(ss.T(), ss.e.heights, height, "status map should contain the height")
	}

	// get current count and add all again
	count := len(ss.e.heights)
	for _, height := range heights {
		ss.e.queueByHeight(height)
	}

	// check that operation was idempotent (size still the same)
	assert.Len(ss.T(), ss.e.heights, count, "height map should be the same")

}

func (ss *SyncSuite) TestQueueByBlockID() {

	// generate a number of block IDs
	var blockIDs []flow.Identifier
	for n := 0; n < 100; n++ {
		blockIDs = append(blockIDs, unittest.IdentifierFixture())
	}

	// add all of them to engine
	for _, blockID := range blockIDs {
		ss.e.queueByBlockID(blockID)
	}

	// check they are all in the map now
	for _, blockID := range blockIDs {
		assert.Contains(ss.T(), ss.e.blockIDs, blockID, "status map should contain the block ID")
	}

	// get current count and add all again
	count := len(ss.e.blockIDs)
	for _, blockID := range blockIDs {
		ss.e.queueByBlockID(blockID)
	}

	// check that operation was idempotent (size still the same)
	assert.Len(ss.T(), ss.e.blockIDs, count, "block ID map should be the same")
}

func (ss *SyncSuite) TestProcessIncomingBlock() {

	var blocks []*flow.Block
	originID := unittest.IdentifierFixture()

	// generate 3 by height blocks
	for n := 0; n < 3; n++ {
		block := unittest.BlockFixture()
		blocks = append(blocks, &block)
		ss.e.heights[block.Height] = nil
	}

	// generate 3 by block ID blocks
	for n := 0; n < 3; n++ {
		block := unittest.BlockFixture()
		blocks = append(blocks, &block)
		ss.e.blockIDs[block.ID()] = nil
	}

	// generate 3 unwanted blocks
	for n := 0; n < 3; n++ {
		block := unittest.BlockFixture()
		blocks = append(blocks, &block)
	}

	// add 2 random heights and block IDs to the height maps
	ss.e.heights[rand.Uint64()] = nil
	ss.e.heights[rand.Uint64()] = nil
	ss.e.blockIDs[unittest.IdentifierFixture()] = nil
	ss.e.blockIDs[unittest.IdentifierFixture()] = nil

	// process all of the blocks
	for _, block := range blocks {
		ss.e.processIncomingBlock(originID, block)
	}

	// assert the maps have right size and elements
	assert.Len(ss.T(), ss.e.heights, 2)
	for _, block := range blocks[0:3] {
		assert.NotContains(ss.T(), ss.e.heights, block.Height, "should not contain blocks by height anymore")
	}
	assert.Len(ss.T(), ss.e.blockIDs, 2)
	for _, block := range blocks[3:6] {
		assert.NotContains(ss.T(), ss.e.blockIDs, block.ID(), "should not contain blocks by ID anymore")
	}

	// assert that submit local was called with the right blocks
	if ss.comp.AssertNumberOfCalls(ss.T(), "SubmitLocal", 6) {
		for _, block := range blocks[0:6] {
			ss.comp.AssertCalled(ss.T(), "SubmitLocal", &events.SyncedBlock{OriginID: originID, Block: block})
		}
	}
}

func (ss *SyncSuite) TestPollHeight() {

	// check that we send to three nodes from our total list
	consensus := ss.participants.Filter(filter.HasNodeID(ss.participants[1:].NodeIDs()...))
	ss.con.On("Submit", mock.Anything, mock.Anything).Return(nil).Run(
		func(args mock.Arguments) {
			req := args.Get(0).(*messages.SyncRequest)
			require.Equal(ss.T(), ss.head.Height, req.Height, "request should contain finalized height")
			targetID := args.Get(1).(flow.Identifier)
			require.Contains(ss.T(), consensus.NodeIDs(), targetID, "target should be in participants")
		},
	)
	err := ss.e.pollHeight()
	require.NoError(ss.T(), err, "should pass poll height")
	ss.con.AssertNumberOfCalls(ss.T(), "Submit", 3)
}

func (ss *SyncSuite) TestScanPending() {

	// get current timestamp and zero timestamp
	now := time.Now().UTC()
	zero := time.Time{}

	// fill in a height status that should be skipped
	skipHeight := uint64(rand.Uint64())
	ss.e.heights[skipHeight] = &Status{
		Queued:    now,
		Requested: now,
		Attempts:  0,
	}

	// fill in a height status that should be deleted
	dropHeight := uint64(rand.Uint64())
	ss.e.heights[dropHeight] = &Status{
		Queued:    now,
		Requested: zero,
		Attempts:  ss.e.maxAttempts,
	}

	// fill in a height status that should be requested
	reqHeight := uint64(rand.Uint64())
	ss.e.heights[reqHeight] = &Status{
		Queued:    now,
		Requested: zero,
		Attempts:  0,
	}

	// fill in a block ID that should be skipped
	skipBlockID := unittest.IdentifierFixture()
	ss.e.blockIDs[skipBlockID] = &Status{
		Queued:    now,
		Requested: now,
		Attempts:  0,
	}

	// fill in a block ID that should be deleted
	dropBlockID := unittest.IdentifierFixture()
	ss.e.blockIDs[dropBlockID] = &Status{
		Queued:    now,
		Requested: zero,
		Attempts:  ss.e.maxAttempts,
	}

	// fill in a block ID that should be requested
	reqBlockID := unittest.IdentifierFixture()
	ss.e.blockIDs[reqBlockID] = &Status{
		Queued:    now,
		Requested: zero,
		Attempts:  0,
	}

	// execute the pending scan
	heights, blockIDs, err := ss.e.scanPending()
	require.NoError(ss.T(), err, "should pass pending scan")

	// check only the request height is in heights
	require.NotContains(ss.T(), heights, skipHeight, "output should not contain skip height")
	require.NotContains(ss.T(), heights, dropHeight, "output should not contain drop height")
	require.Contains(ss.T(), heights, reqHeight, "output should contain request height")

	// check only the request block ID is in block IDs
	require.NotContains(ss.T(), blockIDs, skipBlockID, "output should not contain skip blockID")
	require.NotContains(ss.T(), blockIDs, dropBlockID, "output should not contain drop blockID")
	require.Contains(ss.T(), blockIDs, reqBlockID, "output should contain request blockID")

	// check only delete  height was deleted
	require.Contains(ss.T(), ss.e.heights, skipHeight, "status should not contain skip height")
	require.NotContains(ss.T(), ss.e.heights, dropHeight, "status should not contain drop height")
	require.Contains(ss.T(), ss.e.heights, reqHeight, "status should contain request height")

	// check only the delete block ID was deleted
	require.Contains(ss.T(), ss.e.blockIDs, skipBlockID, "status should not contain skip blockID")
	require.NotContains(ss.T(), ss.e.blockIDs, dropBlockID, "status should not contain drop blockID")
	require.Contains(ss.T(), ss.e.blockIDs, reqBlockID, "status should contain request blockID")

}

func (ss *SyncSuite) TestSendRequestsContinuousRangeMaxSizeExact() {

	// configure request parameters
	ss.e.maxSize = 10
	ss.e.maxRequests = 100

	// create a maximum sized batch of heights
	var maximum []uint64
	maxSize := uint64(ss.e.maxSize)
	start := uint64(100)
	end := start + maxSize
	for n := start; n < end; n++ {
		maximum = append(maximum, n)
		ss.e.heights[n] = &Status{}
	}

	// set up checks for desired sequence of submit calls
	ss.con.On("Submit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Once().Return(nil).Run(
		func(args mock.Arguments) {
			req := args.Get(0).(*messages.RangeRequest)
			assert.Equal(ss.T(), start, req.FromHeight, "request should start at start")
			assert.Equal(ss.T(), end-1, req.ToHeight, "request should end at end")
		},
	)

	// set up one additional catch-all to log info on superfluous calls
	ss.con.On("Submit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(
		func(args mock.Arguments) {
			req := args.Get(0).(*messages.RangeRequest)
			assert.Fail(ss.T(), fmt.Sprintf("additional request (from: %d, to: %d)", req.FromHeight, req.ToHeight))
		},
	)

	// send requests and do checks
	err := ss.e.sendRequests(maximum, nil)
	require.NoError(ss.T(), err, "should pass maximum height batch")
	ss.con.AssertNumberOfCalls(ss.T(), "Submit", 1)
}

func (ss *SyncSuite) TestSendRequestsContinuousRangeMaxSizePlusOne() {

	// configure the request parameters
	ss.e.maxSize = 10
	ss.e.maxRequests = 100

	// create a batch of heights one longer than maximum size
	var oversized []uint64
	maxSize := uint64(ss.e.maxSize)
	start := uint64(200)
	end := start + maxSize + 1
	for n := start; n < end; n++ {
		oversized = append(oversized, n)
		ss.e.heights[n] = &Status{}
	}

	// set up checks for desired sequence of submit calls
	ss.con.On("Submit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Once().Return(nil).Run(
		func(args mock.Arguments) {
			req := args.Get(0).(*messages.RangeRequest)
			assert.Equal(ss.T(), start, req.FromHeight, "first request should start at start")
			assert.Equal(ss.T(), end-2, req.ToHeight, "first request should end at end")
		},
	)
	ss.con.On("Submit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Once().Return(nil).Run(
		func(args mock.Arguments) {
			req := args.Get(0).(*messages.RangeRequest)
			assert.Equal(ss.T(), end-1, req.FromHeight, "second request should start at start")
			assert.Equal(ss.T(), end-1, req.ToHeight, "second request should end at end")
		},
	)

	// set up one additional catch-all to log info on superfluous calls
	ss.con.On("Submit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(
		func(args mock.Arguments) {
			req := args.Get(0).(*messages.RangeRequest)
			assert.Fail(ss.T(), fmt.Sprintf("additional request (from: %d, to: %d)", req.FromHeight, req.ToHeight))
		},
	)

	// send the requests and do checks
	err := ss.e.sendRequests(oversized, nil)
	require.NoError(ss.T(), err, "should pass oversized height batch")
	ss.con.AssertNumberOfCalls(ss.T(), "Submit", 2)
}

func (ss *SyncSuite) TestSendRequestsTwoRangesEachMaxSizeExact() {

	// configure request parameters
	ss.e.maxSize = 1
	ss.e.maxRequests = 100

	// create a slice of two heights, which should be split into two of one each
	batches := []uint64{1, 2}
	for _, n := range batches {
		ss.e.heights[n] = &Status{}
	}

	// set up checks for desired sequence of submit calls
	ss.con.On("Submit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Once().Return(nil).Run(
		func(args mock.Arguments) {
			req := args.Get(0).(*messages.RangeRequest)
			assert.Equal(ss.T(), uint64(1), req.FromHeight, "first request should start at start")
			assert.Equal(ss.T(), uint64(1), req.ToHeight, "first request should end at end")
		},
	)
	ss.con.On("Submit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Once().Return(nil).Run(
		func(args mock.Arguments) {
			req := args.Get(0).(*messages.RangeRequest)
			assert.Equal(ss.T(), uint64(2), req.FromHeight, "second request should start at start")
			assert.Equal(ss.T(), uint64(2), req.ToHeight, "second request should end at end")
		},
	)

	// set up one additional catch-all to log info on superfluous calls
	ss.con.On("Submit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(
		func(args mock.Arguments) {
			req := args.Get(0).(*messages.RangeRequest)
			assert.Fail(ss.T(), fmt.Sprintf("additional request (from: %d, to: %d)", req.FromHeight, req.ToHeight))
		},
	)

	// send the requests and do checks
	err := ss.e.sendRequests(batches, nil)
	require.NoError(ss.T(), err, "should pass two one-height batches")
	ss.con.AssertNumberOfCalls(ss.T(), "Submit", 2)
}

func (ss *SyncSuite) TestSendRequestsFiveRangesWithMaxRequestsFour() {

	// configure request parameters
	ss.e.maxRequests = 4
	ss.e.maxSize = 100

	// create a slice of 5 contiguous ranges, which is one more than max requests
	batches := []uint64{
		1,
		3, 4,
		6, 7, 8,
		10, 11, 12, 13,
		15, 16, 17, 18, 19,
	}
	for _, n := range batches {
		ss.e.heights[n] = &Status{}
	}

	// set up checks for desired sequence of submit calls
	ss.con.On("Submit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Once().Return(nil).Run(
		func(args mock.Arguments) {
			req := args.Get(0).(*messages.RangeRequest)
			assert.Equal(ss.T(), uint64(1), req.FromHeight, "first request should start at start")
			assert.Equal(ss.T(), uint64(1), req.ToHeight, "first request should end at end")
		},
	)
	ss.con.On("Submit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Once().Return(nil).Run(
		func(args mock.Arguments) {
			req := args.Get(0).(*messages.RangeRequest)
			assert.Equal(ss.T(), uint64(3), req.FromHeight, "second request should start at start")
			assert.Equal(ss.T(), uint64(4), req.ToHeight, "second request should end at end")
		},
	)
	ss.con.On("Submit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Once().Return(nil).Run(
		func(args mock.Arguments) {
			req := args.Get(0).(*messages.RangeRequest)
			assert.Equal(ss.T(), uint64(6), req.FromHeight, "second request should start at start")
			assert.Equal(ss.T(), uint64(8), req.ToHeight, "second request should end at end")
		},
	)
	ss.con.On("Submit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Once().Return(nil).Run(
		func(args mock.Arguments) {
			req := args.Get(0).(*messages.RangeRequest)
			assert.Equal(ss.T(), uint64(10), req.FromHeight, "second request should start at start")
			assert.Equal(ss.T(), uint64(13), req.ToHeight, "second request should end at end")
		},
	)

	// set up one additional catch-all to log info on superfluous calls
	ss.con.On("Submit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(
		func(args mock.Arguments) {
			req := args.Get(0).(*messages.RangeRequest)
			assert.Fail(ss.T(), fmt.Sprintf("additional request (from: %d, to: %d)", req.FromHeight, req.ToHeight))
		},
	)

	// send the requests and do checks
	err := ss.e.sendRequests(batches, nil)
	require.NoError(ss.T(), err, "should pass five height batches")
	ss.con.AssertNumberOfCalls(ss.T(), "Submit", 4)
}

func (ss *SyncSuite) TestSendRequestsBlockIDs() {

	// configure request parameters
	ss.e.maxSize = 1024
	ss.e.maxRequests = 100

	// create a batch twice maximum size plus one, which should lead to 3 requests
	maxSize := uint64(ss.e.maxSize)
	var blockIDs []flow.Identifier
	for n := uint64(0); n < maxSize*2+1; n++ {
		blockID := unittest.IdentifierFixture()
		blockIDs = append(blockIDs, blockID)
		ss.e.blockIDs[blockID] = &Status{}
	}

	// set up checks for desired sequence of submit calls
	ss.con.On("Submit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Once().Return(nil).Run(
		func(args mock.Arguments) {
			req := args.Get(0).(*messages.BatchRequest)
			assert.ElementsMatch(ss.T(), blockIDs[:maxSize], req.BlockIDs, "first batch request should have first slice")
		},
	)
	ss.con.On("Submit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Once().Return(nil).Run(
		func(args mock.Arguments) {
			req := args.Get(0).(*messages.BatchRequest)
			assert.ElementsMatch(ss.T(), blockIDs[maxSize:maxSize*2], req.BlockIDs, "second batch request should have second slice")
		},
	)
	ss.con.On("Submit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Once().Return(nil).Run(
		func(args mock.Arguments) {
			req := args.Get(0).(*messages.BatchRequest)
			assert.ElementsMatch(ss.T(), blockIDs[maxSize*2:], req.BlockIDs, "last batch request should have last slice")
		},
	)

	// set up one additional catch-all to log info on superfluous calls
	ss.con.On("Submit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(
		func(args mock.Arguments) {
			req := args.Get(0).(*messages.BatchRequest)
			assert.Fail(ss.T(), fmt.Sprintf("additional request (blocks: %s)", req.BlockIDs))
		},
	)

	// send the requests and do checks
	err := ss.e.sendRequests(nil, blockIDs)
	require.NoError(ss.T(), err, "should pass three block ID batches")
	ss.con.AssertNumberOfCalls(ss.T(), "Submit", 3)

}
