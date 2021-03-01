package epochs

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/state/protocol"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func Test_StateStaker(t *testing.T) {

	t.Run("zero stake means unstaked", func(t *testing.T) {
		runWithMockedEnv(t, 0, nil, func(t *testing.T, sss *SealedStateStaker) {

			stakedAt := sss.AmIStakedAt(unittest.IdentifierFixture())
			require.False(t, stakedAt)
		})
	})

	t.Run("positive stake means staked", func(t *testing.T) {
		runWithMockedEnv(t, 10, nil, func(t *testing.T, sss *SealedStateStaker) {

			stakedAt := sss.AmIStakedAt(unittest.IdentifierFixture())
			require.True(t, stakedAt)
		})
	})

	t.Run("errors while querying means false", func(t *testing.T) {
		runWithMockedEnv(t, 100, protocol.IdentityNotFoundError{}, func(t *testing.T, sss *SealedStateStaker) {
			stakedAt := sss.AmIStakedAt(unittest.IdentifierFixture())
			require.False(t, stakedAt)
		})
	})
}

func runWithMockedEnv(t *testing.T, stake uint64, identityError error, f func(t *testing.T, sss *SealedStateStaker)) {
	nodeIdentifier := flow.Identifier{0, 1, 2, 3}

	state := new(mockprotocol.State)
	local := new(module.Local)
	local.On("NodeID").Return(nodeIdentifier)

	ss := new(mockprotocol.Snapshot)

	state.On("AtBlockID", mock.Anything).Return(ss)
	ss.On("Identity", mock.Anything).
		Return(unittest.IdentityFixture(unittest.WithStake(stake)), identityError)

	fresh := NewSealedStateStaker(zerolog.Nop(), state, local)

	f(t, fresh)

	ss.AssertExpectations(t)

	state.AssertExpectations(t)
	local.AssertExpectations(t)
}
