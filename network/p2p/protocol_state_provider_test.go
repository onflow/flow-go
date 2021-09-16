package p2p

import (
	"testing"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type ProtocolStateProviderTestSuite struct {
	suite.Suite
	provider     *ProtocolStateIDCache
	distributor  *events.Distributor
	state        protocol.State
	snapshot     protocol.Snapshot
	head         *flow.Header
	participants flow.IdentityList
	epochNum     uint64
}

func (suite *ProtocolStateProviderTestSuite) SetupTest() {
	suite.distributor = events.NewDistributor()

	// set up protocol state mock
	state := &mockprotocol.State{}
	state.On("Final").Return(
		func() protocol.Snapshot {
			return suite.snapshot
		},
	)
	state.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) protocol.Snapshot {
			if suite.head.ID() == blockID {
				return suite.snapshot
			} else {
				return unittest.StateSnapshotForUnknownBlock()
			}
		},
	)
	suite.state = state
	suite.epochNum = 0

	suite.triggerUpdate()

	provider, err := NewProtocolStateIDCache(zerolog.Logger{}, state, suite.distributor)
	require.NoError(suite.T(), err)

	suite.provider = provider
}

// triggerUpdate simulates an epoch transition
func (suite *ProtocolStateProviderTestSuite) triggerUpdate() {
	suite.participants = unittest.IdentityListFixture(5, unittest.WithAllRoles(), unittest.WithKeys)

	block := unittest.BlockFixture()
	suite.head = block.Header

	// set up protocol snapshot mock
	snapshot := &mockprotocol.Snapshot{}
	snapshot.On("Identities", mock.Anything).Return(
		func(filter flow.IdentityFilter) flow.IdentityList {
			return suite.participants.Filter(filter)
		},
		nil,
	)
	snapshot.On("Identity", mock.Anything).Return(func(id flow.Identifier) *flow.Identity {
		for _, n := range suite.participants {
			if n.ID() == id {
				return n
			}
		}
		return nil
	}, nil)
	snapshot.On("Head").Return(
		func() *flow.Header {
			return suite.head
		},
		nil,
	)
	suite.snapshot = snapshot
	suite.epochNum += 1

	suite.distributor.EpochTransition(suite.epochNum, suite.head)
}

func TestProtocolStateProvider(t *testing.T) {
	suite.Run(t, new(ProtocolStateProviderTestSuite))
}

// checkStateTransition triggers an epoch transition and checks that the updated
// state is reflected by the provider being tested.
func (suite *ProtocolStateProviderTestSuite) checkStateTransition() {
	oldParticipants := suite.participants

	suite.triggerUpdate()

	assert.ElementsMatch(suite.T(), suite.participants, suite.provider.Identities(filter.Any))
	for _, participant := range suite.participants {
		pid, err := suite.provider.GetPeerID(participant.NodeID)
		require.NoError(suite.T(), err)
		fid, err := suite.provider.GetFlowID(pid)
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), fid, participant.NodeID)
	}
	for _, participant := range oldParticipants {
		_, err := suite.provider.GetPeerID(participant.NodeID)
		require.Error(suite.T(), err)
	}
}

func (suite *ProtocolStateProviderTestSuite) TestUpdateState() {
	for i := 0; i < 10; i++ {
		suite.checkStateTransition()
	}
}

func (suite *ProtocolStateProviderTestSuite) TestIDTranslation() {
	for _, participant := range suite.participants {
		pid, err := suite.provider.GetPeerID(participant.NodeID)
		require.NoError(suite.T(), err)
		key, err := LibP2PPublicKeyFromFlow(participant.NetworkPubKey)
		require.NoError(suite.T(), err)
		expectedPid, err := peer.IDFromPublicKey(key)
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), expectedPid, pid)
		fid, err := suite.provider.GetFlowID(pid)
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), fid, participant.NodeID)
	}
}
