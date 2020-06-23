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

	model "github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module/metrics"
	module "github.com/dapperlabs/flow-go/module/mock"
	netint "github.com/dapperlabs/flow-go/network"
	network "github.com/dapperlabs/flow-go/network/mock"
	clusterint "github.com/dapperlabs/flow-go/state/cluster"
	cluster "github.com/dapperlabs/flow-go/state/cluster/mock"
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
	heights      map[uint64]*model.Block
	blockIDs     map[flow.Identifier]*model.Block
	net          *module.Network
	con          *network.Conduit
	me           *module.Local
	state        *cluster.State
	snapshot     *cluster.Snapshot
	blocks       *storage.ClusterBlocks
	comp         *network.Engine
	core         *module.SyncCore
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
	ss.heights = make(map[uint64]*model.Block)
	ss.blockIDs = make(map[flow.Identifier]*model.Block)

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
	ss.state = &cluster.State{}
	ss.state.On("Final").Return(
		func() clusterint.Snapshot {
			return ss.snapshot
		},
	)

	// set up the snapshot mock
	ss.snapshot = &cluster.Snapshot{}
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
	ss.blocks = &storage.ClusterBlocks{}
	ss.blocks.On("ByHeight", mock.Anything).Return(
		func(height uint64) *model.Block {
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
		func(blockID flow.Identifier) *model.Block {
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

	// set up sync core
	ss.core = &module.SyncCore{}

	// initialize the engine
	log := zerolog.New(ioutil.Discard)
	metrics := metrics.NewNoopCollector()
	e, err := New(log, metrics, ss.net, ss.me, ss.participants, ss.state, ss.blocks, ss.comp, ss.core)
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
	err := ss.e.onSyncRequest(originID, req)
	ss.Assert().NoError(err, "same height sync request should pass")
	ss.con.AssertNotCalled(ss.T(), "Submit", mock.Anything, mock.Anything)

	// if request height is higher than local finalized, we should not respond
	req.Height = ss.head.Height + 1
	ss.core.On("HandleHeight", ss.head, req.Height)
	ss.core.On("WithinTolerance", ss.head, req.Height).Return(false)
	err = ss.e.onSyncRequest(originID, req)
	ss.Assert().NoError(err, "same height sync request should pass")
	ss.con.AssertNotCalled(ss.T(), "Submit", mock.Anything, mock.Anything)

	// if the request height is lower than head and outside tolerance, we should submit correct response
	req.Height = ss.head.Height - 1
	ss.core.On("HandleHeight", ss.head, req.Height)
	ss.core.On("WithinTolerance", ss.head, req.Height).Return(false)
	ss.con.On("Submit", mock.Anything, mock.Anything).Return(nil).Run(
		func(args mock.Arguments) {
			res := args.Get(0).(*messages.SyncResponse)
			assert.Equal(ss.T(), ss.head.Height, res.Height, "response should contain head height")
			assert.Equal(ss.T(), req.Nonce, res.Nonce, "response should contain request nonce")
			recipientID := args.Get(1).(flow.Identifier)
			assert.Equal(ss.T(), originID, recipientID, "should send response to original sender")
		},
	)
	err = ss.e.onSyncRequest(originID, req)
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
	err := ss.e.onSyncResponse(originID, res)
	ss.Assert().Nil(err)
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
		block := unittest.ClusterBlockFixture()
		block.Header.Height = height
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
			res := args.Get(0).(*messages.ClusterBlockResponse)
			expected := []*model.Block{ss.heights[ref-1]}
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
			res := args.Get(0).(*messages.ClusterBlockResponse)
			expected := []*model.Block{ss.heights[ref-2], ss.heights[ref-1], ss.heights[ref]}
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
			res := args.Get(0).(*messages.ClusterBlockResponse)
			expected := []*model.Block{ss.heights[ref-2], ss.heights[ref-1], ss.heights[ref]}
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
	req.BlockIDs = unittest.IdentifierListFixture(1)
	err = ss.e.onBatchRequest(originID, req)
	require.NoError(ss.T(), err, "should pass request for missing block")
	ss.con.AssertNumberOfCalls(ss.T(), "Submit", 0)

	// a non-empty request for existing block IDs should send right response
	block := unittest.ClusterBlockFixture()
	block.Header.Height = ss.head.Height - 1
	req.BlockIDs = []flow.Identifier{block.ID()}
	ss.blockIDs[block.ID()] = &block
	ss.con.On("Submit", mock.Anything, mock.Anything).Return(nil).Run(
		func(args mock.Arguments) {
			res := args.Get(0).(*messages.ClusterBlockResponse)
			assert.ElementsMatch(ss.T(), []*model.Block{&block}, res.Blocks, "response should contain right block")
			assert.Equal(ss.T(), req.Nonce, res.Nonce, "response should contain request nonce")
			recipientID := args.Get(1).(flow.Identifier)
			assert.Equal(ss.T(), originID, recipientID, "response should be send to original requester")
		},
	)
	err = ss.e.onBatchRequest(originID, req)
	require.NoError(ss.T(), err, "should pass request with valid block")
}
