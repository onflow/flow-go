package synchronization

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestSyncCore(t *testing.T) {
	suite.Run(t, new(SyncSuite))
}

type SyncSuite struct {
	suite.Suite
	core *Core
}

func (ss *SyncSuite) SetupTest() {
	var err error

	ss.core, err = New(zerolog.New(ioutil.Discard), DefaultConfig())
	ss.Require().Nil(err)
}

func (ss *SyncSuite) QueuedStatus() *Status {
	return &Status{
		Queued: time.Now(),
	}
}

func (ss *SyncSuite) RequestedStatus() *Status {
	return &Status{
		Queued:    time.Now().Add(-time.Second),
		Requested: time.Now(),
		Attempts:  1,
	}
}

func (ss *SyncSuite) ReceivedStatus(header *flow.Header) *Status {
	return &Status{
		Queued:    time.Now().Add(-time.Second * 2),
		Requested: time.Now().Add(-time.Second),
		Attempts:  1,
		Header:    header,
		Received:  time.Now(),
	}
}

func (ss *SyncSuite) TestQueueByHeight() {

	// generate a number of heights
	var heights []uint64
	for n := 0; n < 100; n++ {
		heights = append(heights, rand.Uint64())
	}

	// add all of them to engine
	for _, height := range heights {
		ss.core.queueByHeight(height)
	}

	// check they are all in the map now
	for _, height := range heights {
		status, exists := ss.core.heights[height]
		ss.Assert().True(exists, "status map should contain the block ID")
		ss.Assert().False(status.WasRequested(), "block should have correct status")
	}

	// get current count and add all again
	count := len(ss.core.heights)
	for _, height := range heights {
		ss.core.queueByHeight(height)
	}

	// check that operation was idempotent (size still the same)
	assert.Len(ss.T(), ss.core.heights, count, "height map should be the same")

}

func (ss *SyncSuite) TestQueueByBlockID() {

	// generate a number of block IDs
	var blockHeaders []flow.Header
	for n := 0; n < 100; n++ {
		blockHeaders = append(blockHeaders, unittest.BlockHeaderFixture())
	}

	// add all of them to engine
	for _, blockHeader := range blockHeaders {
		ss.core.queueByBlockID(blockHeader.ID(), blockHeader.Height)
	}

	// check they are all in the map now
	for _, blockHeader := range blockHeaders {
		status, exists := ss.core.blockIDs[blockHeader.ID()]
		ss.Assert().True(exists, "status map should contain the block ID")
		ss.Assert().False(status.WasRequested(), "block should have correct status")
	}

	// get current count and add all again
	count := len(ss.core.blockIDs)
	for _, blockHeader := range blockHeaders {
		ss.core.queueByBlockID(blockHeader.ID(), blockHeader.Height)
	}

	// check that operation was idempotent (size still the same)
	assert.Len(ss.T(), ss.core.blockIDs, count, "block ID map should be the same")
}

func (ss *SyncSuite) TestRequestBlock() {

	queuedHeader := unittest.BlockHeaderFixture()
	requestedHeader := unittest.BlockHeaderFixture()
	received := unittest.BlockHeaderFixture()

	ss.core.blockIDs[queuedHeader.ID()] = &statusWithHeight{ss.QueuedStatus(), queuedHeader.Height}
	ss.core.blockIDs[requestedHeader.ID()] = &statusWithHeight{ss.RequestedStatus(), queuedHeader.Height}
	ss.core.blockIDs[received.ID()] = &statusWithHeight{ss.RequestedStatus(), received.Height}

	// queued status should stay the same
	ss.core.RequestBlock(queuedHeader.ID(), queuedHeader.Height)
	assert.True(ss.T(), ss.core.blockIDs[queuedHeader.ID()].WasQueued())

	// requested status should stay the same
	ss.core.RequestBlock(requestedHeader.ID(), requestedHeader.Height)
	assert.True(ss.T(), ss.core.blockIDs[requestedHeader.ID()].WasRequested())

	// received status should be re-queued by ID
	ss.core.RequestBlock(received.ID(), received.Height)
	assert.True(ss.T(), ss.core.blockIDs[received.ID()].WasQueued())
	assert.False(ss.T(), ss.core.blockIDs[received.ID()].WasReceived())
	assert.False(ss.T(), ss.core.heights[received.Height].WasQueued())
}

func (ss *SyncSuite) TestHandleBlock() {

	unrequested := unittest.BlockHeaderFixture()
	queuedByHeight := unittest.BlockHeaderFixture()
	ss.core.heights[queuedByHeight.Height] = ss.QueuedStatus()
	requestedByID := unittest.BlockHeaderFixture()
	ss.core.blockIDs[requestedByID.ID()] = &statusWithHeight{ss.RequestedStatus(), requestedByID.Height}
	received := unittest.BlockHeaderFixture()
	ss.core.heights[received.Height] = ss.ReceivedStatus(&received)
	ss.core.blockIDs[received.ID()] = &statusWithHeight{ss.ReceivedStatus(&received), received.Height}

	// should ignore un-requested blocks
	shouldProcess := ss.core.HandleBlock(&unrequested)
	ss.Assert().False(shouldProcess, "should not process un-requested block")
	ss.Assert().NotContains(ss.core.heights, unrequested.Height)
	ss.Assert().NotContains(ss.core.blockIDs, unrequested.ID())

	// should mark queued blocks as received, and process them
	shouldProcess = ss.core.HandleBlock(&queuedByHeight)
	ss.Assert().True(shouldProcess, "should process queued block")
	ss.Assert().True(ss.core.blockIDs[queuedByHeight.ID()].WasReceived(), "status should be reflected in block ID map")
	ss.Assert().True(ss.core.heights[queuedByHeight.Height].WasReceived(), "status should be reflected in height map")

	// should mark requested block as received, and process them
	shouldProcess = ss.core.HandleBlock(&requestedByID)
	ss.Assert().True(shouldProcess, "should process requested block")
	ss.Assert().True(ss.core.blockIDs[requestedByID.ID()].WasReceived(), "status should be reflected in block ID map")
	ss.Assert().True(ss.core.heights[requestedByID.Height].WasReceived(), "status should be reflected in height map")

	// should leave received blocks, and not process them
	shouldProcess = ss.core.HandleBlock(&received)
	ss.Assert().False(shouldProcess, "should not process already received block")
	ss.Assert().True(ss.core.blockIDs[received.ID()].WasReceived(), "status should remain reflected in block ID map")
	ss.Assert().True(ss.core.heights[received.Height].WasReceived(), "status should remain reflected in height map")
}

func (ss *SyncSuite) TestHandleHeight() {

	final := unittest.BlockHeaderFixture()
	lower := final.Height - uint64(ss.core.Config.Tolerance)
	aboveWithinTolerance := final.Height + 1
	aboveOutsideTolerance := final.Height + uint64(ss.core.Config.Tolerance+1)

	// a height lower than finalized should be a no-op
	ss.core.HandleHeight(&final, lower)
	ss.Assert().Len(ss.core.heights, 0)

	// a height higher than finalized, but within tolerance, should be a no-op
	ss.core.HandleHeight(&final, aboveWithinTolerance)
	ss.Assert().Len(ss.core.heights, 0)

	// a height higher than finalized and outside tolerance should queue missing heights
	ss.core.HandleHeight(&final, aboveOutsideTolerance)
	ss.Assert().Len(ss.core.heights, int(aboveOutsideTolerance-final.Height))
	for height := final.Height + 1; height <= aboveOutsideTolerance; height++ {
		ss.Assert().Contains(ss.core.heights, height)
	}
}

func (ss *SyncSuite) TestGetRequestableItems() {

	// get current timestamp and zero timestamp
	now := time.Now().UTC()
	zero := time.Time{}

	// fill in a height status that should be skipped
	skipHeight := uint64(rand.Uint64())
	ss.core.heights[skipHeight] = &Status{
		Queued:    now,
		Requested: now,
		Attempts:  0,
	}

	// fill in a height status that should be deleted
	dropHeight := uint64(rand.Uint64())
	ss.core.heights[dropHeight] = &Status{
		Queued:    now,
		Requested: zero,
		Attempts:  ss.core.Config.MaxAttempts,
	}

	// fill in a height status that should be requested
	reqHeight := uint64(rand.Uint64())
	ss.core.heights[reqHeight] = &Status{
		Queued:    now,
		Requested: zero,
		Attempts:  0,
	}

	// fill in a block ID that should be skipped
	skipBlockHeader := unittest.BlockHeaderFixture()
	ss.core.blockIDs[skipBlockHeader.ID()] = &statusWithHeight{
		&Status{
			Queued:    now,
			Requested: now,
			Attempts:  0,
		},
		skipBlockHeader.Height,
	}

	// fill in a block ID that should be deleted
	dropBlockHeader := unittest.BlockHeaderFixture()
	ss.core.blockIDs[dropBlockHeader.ID()] = &statusWithHeight{
		&Status{
			Queued:    now,
			Requested: zero,
			Attempts:  ss.core.Config.MaxAttempts,
		},
		dropBlockHeader.Height,
	}

	// fill in a block ID that should be requested
	reqBlockHeader := unittest.BlockHeaderFixture()
	ss.core.blockIDs[reqBlockHeader.ID()] = &statusWithHeight{
		&Status{
			Queued:    now,
			Requested: zero,
			Attempts:  0,
		},
		reqBlockHeader.Height,
	}

	// execute the pending scan
	heights, blockIDs := ss.core.getRequestableItems()

	// check only the request height is in heights
	require.NotContains(ss.T(), heights, skipHeight, "output should not contain skip height")
	require.NotContains(ss.T(), heights, dropHeight, "output should not contain drop height")
	require.Contains(ss.T(), heights, reqHeight, "output should contain request height")

	// check only the request block ID is in block IDs
	require.NotContains(ss.T(), blockIDs, skipBlockHeader.ID(), "output should not contain skip blockID")
	require.NotContains(ss.T(), blockIDs, dropBlockHeader.ID(), "output should not contain drop blockID")
	require.Contains(ss.T(), blockIDs, reqBlockHeader.ID(), "output should contain request blockID")

	// check only delete height was deleted
	require.Contains(ss.T(), ss.core.heights, skipHeight, "status should not contain skip height")
	require.NotContains(ss.T(), ss.core.heights, dropHeight, "status should not contain drop height")
	require.Contains(ss.T(), ss.core.heights, reqHeight, "status should contain request height")

	// check only the delete block ID was deleted
	require.Contains(ss.T(), ss.core.blockIDs, skipBlockHeader.ID(), "status should not contain skip blockID")
	require.NotContains(ss.T(), ss.core.blockIDs, dropBlockHeader.ID(), "status should not contain drop blockID")
	require.Contains(ss.T(), ss.core.blockIDs, reqBlockHeader.ID(), "status should contain request blockID")
}

func (ss *SyncSuite) TestGetRanges() {

	// use a small max request size for simpler test cases
	ss.core.Config.MaxSize = 4

	ss.Run("contiguous", func() {
		input := []uint64{1, 2, 3, 4, 5, 6, 7, 8}
		expected := []flow.Range{{From: 1, To: 4}, {From: 5, To: 8}}
		ranges := ss.core.getRanges(input)
		ss.Assert().Equal(expected, ranges)
	})

	ss.Run("non-contiguous", func() {
		input := []uint64{1, 3}
		expected := []flow.Range{{From: 1, To: 1}, {From: 3, To: 3}}
		ranges := ss.core.getRanges(input)
		ss.Assert().Equal(expected, ranges)
	})

	ss.Run("with dupes", func() {
		input := []uint64{1, 2, 2, 3, 3, 4}
		expected := []flow.Range{{From: 1, To: 4}}
		ranges := ss.core.getRanges(input)
		ss.Assert().Equal(expected, ranges)
	})
}

func (ss *SyncSuite) TestGetBatches() {

	// use a small max request size for simpler test cases
	ss.core.Config.MaxSize = 4

	ss.Run("less than max size", func() {
		input := unittest.IdentifierListFixture(2)
		expected := []flow.Batch{{BlockIDs: input}}
		batches := ss.core.getBatches(input)
		ss.Assert().Equal(expected, batches)
	})

	ss.Run("greater than max size", func() {
		input := unittest.IdentifierListFixture(6)
		expected := []flow.Batch{{BlockIDs: input[:4]}, {BlockIDs: input[4:]}}
		batches := ss.core.getBatches(input)
		ss.Assert().Equal(expected, batches)
	})
}

func (ss *SyncSuite) TestSelectRequests() {

	ss.core.Config.MaxRequests = 4

	type testcase struct {
		// number of candidate ranges and batches
		nRanges, nBatches int
		// number of each request that should be selected
		expectedNRanges, expectedNBatches int
	}

	cases := []testcase{
		{
			nRanges:          4,
			nBatches:         1,
			expectedNRanges:  4,
			expectedNBatches: 0,
		}, {
			nRanges:          5,
			nBatches:         1,
			expectedNRanges:  4,
			expectedNBatches: 0,
		}, {
			nRanges:          3,
			nBatches:         1,
			expectedNRanges:  3,
			expectedNBatches: 1,
		}, {
			nRanges:          0,
			nBatches:         1,
			expectedNRanges:  0,
			expectedNBatches: 1,
		}, {
			nRanges:          0,
			nBatches:         5,
			expectedNRanges:  0,
			expectedNBatches: 4,
		},
	}

	for _, tcase := range cases {
		ss.Run(fmt.Sprintf("%d ranges / %d batches", tcase.nRanges, tcase.nBatches), func() {
			inputRanges := unittest.RangeListFixture(tcase.nRanges)
			inputBatches := unittest.BatchListFixture(tcase.nBatches)

			ranges, batches := ss.core.selectRequests(inputRanges, inputBatches)
			ss.Assert().Len(ranges, tcase.expectedNRanges)
			if tcase.expectedNRanges > 0 {
				ss.Assert().Equal(ranges, inputRanges[:tcase.expectedNRanges])
			}
			ss.Assert().Len(batches, tcase.expectedNBatches)
			if tcase.expectedNBatches > 0 {
				ss.Assert().Equal(batches, inputBatches[:tcase.expectedNBatches])
			}
		})
	}
}

func (ss *SyncSuite) TestPrune() {

	// our latest finalized height is 100
	final := unittest.BlockHeaderFixture()
	final.Height = 100

	var (
		prunableHeights  []flow.Block
		prunableBlockIDs []flow.Block
		unprunable       []flow.Block
	)

	// add some finalized blocks by height
	for i := 0; i < 3; i++ {
		block := unittest.BlockFixture()
		block.Header.Height = uint64(i + 1)
		ss.core.heights[block.Header.Height] = ss.QueuedStatus()
		prunableHeights = append(prunableHeights, block)
	}
	// add some un-finalized blocks by height
	for i := 0; i < 3; i++ {
		block := unittest.BlockFixture()
		block.Header.Height = final.Height + uint64(i+1)
		ss.core.heights[block.Header.Height] = ss.QueuedStatus()
		unprunable = append(unprunable, block)
	}

	// add some finalized blocks by block ID
	for i := 0; i < 3; i++ {
		block := unittest.BlockFixture()
		block.Header.Height = uint64(i + 1)
		ss.core.blockIDs[block.ID()] = &statusWithHeight{ss.ReceivedStatus(block.Header), block.Header.Height}
		prunableBlockIDs = append(prunableBlockIDs, block)
	}
	// add some un-finalized, received blocks by block ID
	for i := 0; i < 3; i++ {
		block := unittest.BlockFixture()
		block.Header.Height = 100 + uint64(i+1)
		ss.core.blockIDs[block.ID()] = &statusWithHeight{ss.ReceivedStatus(block.Header), block.Header.Height}
		unprunable = append(unprunable, block)
	}

	heightsBefore := len(ss.core.heights)
	blockIDsBefore := len(ss.core.blockIDs)

	// prune the pending requests
	ss.core.prune(&final)

	assert.Equal(ss.T(), heightsBefore-len(prunableHeights), len(ss.core.heights))
	assert.Equal(ss.T(), blockIDsBefore-len(prunableBlockIDs), len(ss.core.blockIDs))

	// ensure the right things were pruned
	for _, block := range prunableBlockIDs {
		_, exists := ss.core.blockIDs[block.ID()]
		assert.False(ss.T(), exists)
	}
	for _, block := range prunableHeights {
		_, exists := ss.core.heights[block.Header.Height]
		assert.False(ss.T(), exists)
	}
	for _, block := range unprunable {
		_, heightExists := ss.core.heights[block.Header.Height]
		_, blockIDExists := ss.core.blockIDs[block.ID()]
		assert.True(ss.T(), heightExists || blockIDExists)
	}
}
