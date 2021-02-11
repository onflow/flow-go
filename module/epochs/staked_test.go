package epochs

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	module "github.com/onflow/flow-go/module/mock"
	protocol2 "github.com/onflow/flow-go/state/protocol"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
)

func Test_StateStaker(t *testing.T) {

	t.Run("fresh one is un-staked", func(t *testing.T) {

		state := new(protocol.State)
		local := new(module.Local)

		fresh := NewSealedStateStaker(zerolog.Nop(), state, local)

		require.False(t, fresh.AmIStaked())
	})

	t.Run("refresh checks stakes at current seal", func(t *testing.T) {
		runWithMockedEnv(t, 0, 3, nil, func(t *testing.T, sss *SealedStateStaker) {
			err := sss.Refresh()
			require.NoError(t, err)
			require.True(t, sss.AmIStaked())
		})
	})

	t.Run("zero stake means unstaked", func(t *testing.T) {
		runWithMockedEnv(t, 0, 0, nil, func(t *testing.T, sss *SealedStateStaker) {
			err := sss.Refresh()
			require.NoError(t, err)
			require.False(t, sss.AmIStaked())
		})
	})

	t.Run("errors while querying identity mean not staked", func(t *testing.T) {
		runWithMockedEnv(t, 0, 0, protocol2.IdentityNotFoundError{}, func(t *testing.T, sss *SealedStateStaker) {
			err := sss.Refresh()
			require.NoError(t, err)
			require.False(t, sss.AmIStaked())
		})
	})

	t.Run("epoch transition triggers recheck", func(t *testing.T) {

		stake := uint64(2)
		epochNumber := uint64(1)

		header := &flow.Header{}
		nodeIdentifier := flow.Identifier{0, 1, 2, 3}

		//mock all state querying interfaces
		identity := &flow.Identity{
			NodeID: nodeIdentifier,
			Stake:  stake,
		}

		state := new(protocol.State)
		local := new(module.Local)

		local.On("NodeID").Return(nodeIdentifier)

		byBlockSnapshot := new(protocol.Snapshot)
		byBlockSnapshot.On("Identity", nodeIdentifier).Return(identity, nil)

		state.On("AtBlockID", mock.Anything).Return(byBlockSnapshot)

		// actual test
		fresh := NewSealedStateStaker(zerolog.Nop(), state, local)

		require.False(t, fresh.AmIStaked())
		fresh.EpochTransition(epochNumber, header)
		require.True(t, fresh.AmIStaked())

		state.AssertExpectations(t)
		local.AssertExpectations(t)
		byBlockSnapshot.AssertExpectations(t)
	})
}

func runWithMockedEnv(t *testing.T, epochNumber, stake uint64, identityError error, f func(t *testing.T, sss *SealedStateStaker)) {
	header := &flow.Header{}
	nodeIdentifier := flow.Identifier{0, 1, 2, 3}

	//mock all state querying interfaces
	identity := &flow.Identity{
		NodeID: nodeIdentifier,
		Stake:  stake,
	}

	state := new(protocol.State)
	local := new(module.Local)

	local.On("NodeID").Return(nodeIdentifier)

	epoch := new(protocol.Epoch)
	epoch.On("Counter").Return(epochNumber, nil)

	epochQuery := new(protocol.EpochQuery)
	epochQuery.On("Current").Return(epoch)

	sealedSnapshot := new(protocol.Snapshot)
	sealedSnapshot.On("Epochs").Return(epochQuery)
	sealedSnapshot.On("Head").Return(header, nil)

	byBlockSnapshot := new(protocol.Snapshot)
	byBlockSnapshot.On("Identity", nodeIdentifier).Return(identity, identityError)

	state.On("Sealed").Return(sealedSnapshot)
	state.On("AtBlockID", mock.Anything).Return(byBlockSnapshot)

	// actual test
	fresh := NewSealedStateStaker(zerolog.Nop(), state, local)

	f(t, fresh)

	state.AssertExpectations(t)
	local.AssertExpectations(t)
	epoch.AssertExpectations(t)
	epochQuery.AssertExpectations(t)
	byBlockSnapshot.AssertExpectations(t)
	sealedSnapshot.AssertExpectations(t)
}
