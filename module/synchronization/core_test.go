package synchronization

import (
	"io/ioutil"
	"math/rand"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
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
	var blockIDs []flow.Identifier
	for n := 0; n < 100; n++ {
		blockIDs = append(blockIDs, unittest.IdentifierFixture())
	}

	// add all of them to engine
	for _, blockID := range blockIDs {
		ss.core.queueByBlockID(blockID)
	}

	// check they are all in the map now
	for _, blockID := range blockIDs {
		status, exists := ss.core.blockIDs[blockID]
		ss.Assert().True(exists, "status map should contain the block ID")
		ss.Assert().False(status.WasRequested(), "block should have correct status")
	}

	// get current count and add all again
	count := len(ss.core.blockIDs)
	for _, blockID := range blockIDs {
		ss.core.queueByBlockID(blockID)
	}

	// check that operation was idempotent (size still the same)
	assert.Len(ss.T(), ss.core.blockIDs, count, "block ID map should be the same")
}

func (ss *SyncSuite) TestRequestBlock() {

	queuedID := unittest.IdentifierFixture()
	requestedID := unittest.IdentifierFixture()
	received := unittest.BlockHeaderFixture()

	ss.core.blockIDs[queuedID] = ss.QueuedStatus()
	ss.core.blockIDs[requestedID] = ss.RequestedStatus()
	ss.core.blockIDs[received.ID()] = ss.RequestedStatus()

	// queued status should stay the same
	ss.core.RequestBlock(queuedID)
	assert.True(ss.T(), ss.core.blockIDs[queuedID].WasQueued())

	// requested status should stay the same
	ss.core.RequestBlock(requestedID)
	assert.True(ss.T(), ss.core.blockIDs[requestedID].WasRequested())

	// received status should be re-queued by ID
	ss.core.RequestBlock(received.ID())
	assert.True(ss.T(), ss.core.blockIDs[received.ID()].WasQueued())
	assert.False(ss.T(), ss.core.blockIDs[received.ID()].WasReceived())
	assert.False(ss.T(), ss.core.heights[received.Height].WasQueued())
}

func (ss *SyncSuite) TestHandleBlock() {

	unrequested := unittest.BlockHeaderFixture()
	queuedByHeight := unittest.BlockHeaderFixture()
	ss.core.heights[queuedByHeight.Height] = ss.QueuedStatus()
	requestedByID := unittest.BlockHeaderFixture()
	ss.core.blockIDs[requestedByID.ID()] = ss.RequestedStatus()
	received := unittest.BlockHeaderFixture()
	ss.core.heights[received.Height] = ss.ReceivedStatus(&received)
	ss.core.blockIDs[received.ID()] = ss.ReceivedStatus(&received)

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
		ss.core.blockIDs[block.ID()] = ss.ReceivedStatus(block.Header)
		prunableBlockIDs = append(prunableBlockIDs, block)
	}
	// add some un-finalized, received blocks by block ID
	for i := 0; i < 3; i++ {
		block := unittest.BlockFixture()
		block.Header.Height = 100 + uint64(i+1)
		ss.core.blockIDs[block.ID()] = ss.ReceivedStatus(block.Header)
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
