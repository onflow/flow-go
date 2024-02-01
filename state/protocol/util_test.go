package protocol_test

import (
	"errors"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestIsSporkRootSnapshot(t *testing.T) {
	t.Run("spork root", func(t *testing.T) {
		snapshot := unittest.RootSnapshotFixture(unittest.IdentityListFixture(10, unittest.WithAllRoles()))
		isSporkRoot, err := protocol.IsSporkRootSnapshot(snapshot)
		require.NoError(t, err)
		assert.True(t, isSporkRoot)
	})

	t.Run("other snapshot", func(t *testing.T) {
		snapshot := unittest.RootSnapshotFixture(unittest.IdentityListFixture(10, unittest.WithAllRoles()))
		snapshot.Encodable().Head.Height += 1 // modify head height to break equivalence with spork root block height
		isSporkRoot, err := protocol.IsSporkRootSnapshot(snapshot)
		require.NoError(t, err)
		assert.False(t, isSporkRoot)
	})
}

// TestOrderedSeals tests that protocol.OrderedSeals returns a list of ordered seals for a payload.
func TestOrderedSeals(t *testing.T) {
	t.Run("empty payload", func(t *testing.T) {
		payload := flow.EmptyPayload()
		headers := storagemock.NewHeaders(t)

		ordered, err := protocol.OrderedSeals(payload.Seals, headers)
		require.NoError(t, err)
		require.Empty(t, ordered)
	})
	t.Run("block not found error", func(t *testing.T) {
		headers := storagemock.NewHeaders(t)
		seals := unittest.Seal.Fixtures(10)
		payload := unittest.PayloadFixture(unittest.WithSeals(seals...))
		headers.On("ByBlockID", mock.Anything).Return(nil, storage.ErrNotFound)

		ordered, err := protocol.OrderedSeals(payload.Seals, headers)
		require.ErrorIs(t, err, storage.ErrNotFound)
		require.Empty(t, ordered)
	})
	t.Run("unexpected error", func(t *testing.T) {
		headers := storagemock.NewHeaders(t)
		seals := unittest.Seal.Fixtures(10)
		payload := unittest.PayloadFixture(unittest.WithSeals(seals...))
		exception := errors.New("exception")
		headers.On("ByBlockID", mock.Anything).Return(nil, exception)

		ordered, err := protocol.OrderedSeals(payload.Seals, headers)
		require.ErrorIs(t, err, exception)
		require.Empty(t, ordered)
	})
	t.Run("already ordered", func(t *testing.T) {
		headers := storagemock.NewHeaders(t)

		blocks := unittest.ChainFixtureFrom(10, unittest.BlockHeaderFixture())
		seals := unittest.Seal.Fixtures(10)
		for i, seal := range seals {
			seal.BlockID = blocks[i].ID()
			headers.On("ByBlockID", seal.BlockID).Return(blocks[i].Header, nil)
		}
		payload := unittest.PayloadFixture(unittest.WithSeals(seals...))

		ordered, err := protocol.OrderedSeals(payload.Seals, headers)
		require.NoError(t, err)
		require.Equal(t, seals, ordered)
	})
	t.Run("unordered", func(t *testing.T) {
		headers := storagemock.NewHeaders(t)

		blocks := unittest.ChainFixtureFrom(10, flow.Genesis(flow.Localnet).Header)
		orderedSeals := unittest.Seal.Fixtures(len(blocks))
		for i, seal := range orderedSeals {
			seal.BlockID = blocks[i].ID()
			headers.On("ByBlockID", seal.BlockID).Return(blocks[i].Header, nil)
		}
		unorderedSeals := make([]*flow.Seal, len(orderedSeals))
		copy(unorderedSeals, orderedSeals)
		// randomly re-order seals
		rand.Shuffle(len(unorderedSeals), func(i, j int) {
			unorderedSeals[i], unorderedSeals[j] = unorderedSeals[j], unorderedSeals[i]
		})
		payload := unittest.PayloadFixture(unittest.WithSeals(unorderedSeals...))

		ordered, err := protocol.OrderedSeals(payload.Seals, headers)
		require.NoError(t, err)
		require.Equal(t, orderedSeals, ordered)
	})
}
