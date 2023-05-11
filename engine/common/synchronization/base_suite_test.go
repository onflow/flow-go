package synchronization

import (
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	netint "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/mocknetwork"
	protocolint "github.com/onflow/flow-go/state/protocol"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	storerr "github.com/onflow/flow-go/storage"
	mockstorage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type SyncBaseSuite struct {
	suite.Suite

	myID     flow.Identifier
	head     *flow.Header
	heights  map[uint64]*flow.Block
	blockIDs map[flow.Identifier]*flow.Block

	metrics  module.EngineMetrics
	net      *mocknetwork.Network
	con      *mocknetwork.Conduit
	me       *mockmodule.Local
	state    *mockprotocol.State
	snapshot *mockprotocol.Snapshot
	blocks   *mockstorage.Blocks
	core     *mockmodule.SyncCore
}

func (s *SyncBaseSuite) SetupTest() {
	s.myID = unittest.IdentifierFixture()
	s.head = unittest.BlockHeaderFixture()
	s.heights = make(map[uint64]*flow.Block)
	s.blockIDs = make(map[flow.Identifier]*flow.Block)

	s.metrics = metrics.NewNoopCollector()

	// set up the network module mock
	s.net = mocknetwork.NewNetwork(s.T())
	s.net.On("Register", mock.Anything, mock.Anything).Return(
		func(channel channels.Channel, engine netint.MessageProcessor) netint.Conduit {
			return s.con
		},
		nil,
	)

	// set up the network conduit mock
	s.con = mocknetwork.NewConduit(s.T())

	// set up the local module mock
	s.me = mockmodule.NewLocal(s.T())
	s.me.On("NodeID").Return(
		func() flow.Identifier {
			return s.myID
		},
	).Maybe()

	s.state = mockprotocol.NewState(s.T())
	s.state.On("Final").Return(
		func() protocolint.Snapshot {
			return s.snapshot
		},
	)
	s.state.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) protocolint.Snapshot {
			if s.head.ID() == blockID {
				return s.snapshot
			} else {
				return unittest.StateSnapshotForUnknownBlock()
			}
		},
	).Maybe()

	// set up the snapshot mock
	s.snapshot = mockprotocol.NewSnapshot(s.T())
	s.snapshot.On("Head").Return(
		func() *flow.Header {
			return s.head
		},
		nil,
	)

	// set up blocks storage mock
	s.blocks = mockstorage.NewBlocks(s.T())
	s.blocks.On("ByHeight", mock.Anything).Return(
		func(height uint64) *flow.Block {
			return s.heights[height]
		},
		func(height uint64) error {
			_, enabled := s.heights[height]
			if !enabled {
				return storerr.ErrNotFound
			}
			return nil
		},
	).Maybe()
	s.blocks.On("ByID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.Block {
			return s.blockIDs[blockID]
		},
		func(blockID flow.Identifier) error {
			_, enabled := s.blockIDs[blockID]
			if !enabled {
				return storerr.ErrNotFound
			}
			return nil
		},
	).Maybe()

	s.core = mockmodule.NewSyncCore(s.T())
}
