package unittest

import (
	"github.com/stretchr/testify/mock"

	"github.com/dapperlabs/flow-go/model/flow"
	protint "github.com/dapperlabs/flow-go/state/protocol"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
)

// FinalizedProtocolStateWithParticipants returns a protocol state with finalized participants
func FinalizedProtocolStateWithParticipants(participants flow.IdentityList) (
	*flow.Block, *protocol.Snapshot, *protocol.State) {
	block := BlockFixture()
	head := block.Header

	// set up protocol snapshot mock
	snapshot := &protocol.Snapshot{}
	snapshot.On("Identities", mock.Anything).Return(
		func(filter flow.IdentityFilter) flow.IdentityList {
			return participants.Filter(filter)
		},
		nil,
	)
	snapshot.On("Identity", mock.Anything).Return(func(id flow.Identifier) *flow.Identity {
		for _, n := range participants {
			if n.ID() == id {
				return n
			}
		}
		return nil
	}, nil)
	snapshot.On("Head").Return(
		func() *flow.Header {
			return head
		},
		nil,
	)

	// set up protocol state mock
	state := &protocol.State{}
	state.On("Final").Return(
		func() protint.Snapshot {
			return snapshot
		},
	)
	state.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) protint.Snapshot {
			return snapshot
		},
	)
	return &block, snapshot, state
}
