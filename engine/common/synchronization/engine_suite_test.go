package synchronization

import (
	"io"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	mockconsensus "github.com/onflow/flow-go/engine/consensus/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	netint "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/p2p/cache"
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
	comp         *mockconsensus.Compliance
	core         *module.SyncCore
	e            *Engine
}

func (ss *SyncSuite) SetupTest() {
	// generate own ID
	ss.participants = unittest.IdentityListFixture(3, unittest.WithRole(flow.RoleConsensus))
	keys := unittest.NetworkingKeys(len(ss.participants))

	for i, p := range ss.participants {
		p.NetworkPubKey = keys[i].PublicKey()
	}
	ss.myID = ss.participants[0].NodeID

	// generate a header for the final state
	header := unittest.BlockHeaderFixture()
	ss.head = header

	// create maps to enable block returns
	ss.heights = make(map[uint64]*flow.Block)
	ss.blockIDs = make(map[flow.Identifier]*flow.Block)

	// set up the network module mock
	ss.net = &mocknetwork.Network{}
	ss.net.On("Register", mock.Anything, mock.Anything).Return(
		func(channel channels.Channel, engine netint.MessageProcessor) netint.Conduit {
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
	ss.comp = mockconsensus.NewCompliance(ss.T())
	ss.comp.On("Process", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	// set up sync core
	ss.core = &module.SyncCore{}

	// initialize the engine
	log := zerolog.New(io.Discard)
	metrics := metrics.NewNoopCollector()

	idCache, err := cache.NewProtocolStateIDCache(log, ss.state, protocolEvents.NewDistributor())
	require.NoError(ss.T(), err, "could not create protocol state identity cache")
	spamConfig, err := NewSpamDetectionConfig()
	require.NoError(ss.T(), err, "could not create spam detection config")
	e, err := New(log, metrics, ss.net, ss.me, ss.state, ss.blocks, ss.comp, ss.core,
		id.NewIdentityFilterIdentifierProvider(
			filter.And(
				filter.HasRole(flow.RoleConsensus),
				filter.Not(filter.HasNodeID(ss.me.NodeID())),
			),
			idCache,
		),
		spamConfig)
	require.NoError(ss.T(), err, "should pass engine initialization")
	ss.e = e
}
