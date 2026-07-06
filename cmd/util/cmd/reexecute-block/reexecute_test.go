package reexecute_block

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// mapRegisterGetter is an in-memory RegisterGetter keyed by height, mirroring the not-found / height-
// not-indexed semantics of the production Pebble register store.
type mapRegisterGetter struct {
	values      map[uint64]map[flow.RegisterID]flow.RegisterValue
	first, last uint64
}

func (m mapRegisterGetter) Get(id flow.RegisterID, height uint64) (flow.RegisterValue, error) {
	if height < m.first || height > m.last {
		return nil, fmt.Errorf("height %d not indexed: %w", height, storage.ErrHeightNotIndexed)
	}
	v, ok := m.values[height][id]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return v, nil
}

func TestStorehouseSnapshotAtHeight(t *testing.T) {
	owner := flow.BytesToAddress([]byte{0x01})
	present := flow.NewRegisterID(owner, "present")
	absent := flow.NewRegisterID(owner, "absent")

	getter := mapRegisterGetter{
		first: 10,
		last:  20,
		values: map[uint64]map[flow.RegisterID]flow.RegisterValue{
			15: {present: []byte("value-at-15")},
		},
	}

	t.Run("returns the value present at the height", func(t *testing.T) {
		snap := StorehouseSnapshotAtHeight(getter, 15)
		value, err := snap.Get(present)
		require.NoError(t, err)
		require.Equal(t, flow.RegisterValue("value-at-15"), value)
	})

	t.Run("absent register reads back as empty (nil), not an error", func(t *testing.T) {
		snap := StorehouseSnapshotAtHeight(getter, 15)
		value, err := snap.Get(absent)
		require.NoError(t, err)
		require.Nil(t, value)
	})

	t.Run("height outside the indexed range is a hard error", func(t *testing.T) {
		snap := StorehouseSnapshotAtHeight(getter, 99)
		_, err := snap.Get(present)
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrHeightNotIndexed)
	})
}

func TestAssembleExecutableBlock(t *testing.T) {
	// Build two collections and a block whose payload references them via guarantees.
	coll1 := unittest.CollectionFixture(2)
	coll2 := unittest.CollectionFixture(3)

	guarantee1 := unittest.CollectionGuaranteeFixture(func(g *flow.CollectionGuarantee) {
		g.CollectionID = coll1.ID()
	})
	guarantee2 := unittest.CollectionGuaranteeFixture(func(g *flow.CollectionGuarantee) {
		g.CollectionID = coll2.ID()
	})

	block := unittest.BlockWithGuaranteesFixture([]*flow.CollectionGuarantee{guarantee1, guarantee2})
	parentCommit := unittest.StateCommitmentFixture()

	collections := storagemock.NewCollections(t)
	collections.On("ByID", coll1.ID()).Return(&coll1, nil)
	collections.On("ByID", coll2.ID()).Return(&coll2, nil)

	executable, err := AssembleExecutableBlock(block, collections, parentCommit)
	require.NoError(t, err)

	// StartState anchors on the parent's final state commitment.
	require.NotNil(t, executable.StartState)
	require.Equal(t, parentCommit, *executable.StartState)
	require.True(t, executable.HasStartState())

	// Every guarantee has a matching complete collection, keyed by collection ID.
	require.Len(t, executable.CompleteCollections, 2)
	require.Contains(t, executable.CompleteCollections, coll1.ID())
	require.Contains(t, executable.CompleteCollections, coll2.ID())
	require.Equal(t, &coll1, executable.CompleteCollections[coll1.ID()].Collection)
	require.Equal(t, guarantee1, executable.CompleteCollections[coll1.ID()].Guarantee)

	// The assembled block round-trips its identity and parent.
	require.Equal(t, block.ID(), executable.BlockID())
	require.Equal(t, block.ParentID, executable.ParentID())
}

func TestAssembleExecutableBlock_MissingCollection(t *testing.T) {
	coll := unittest.CollectionFixture(1)
	guarantee := unittest.CollectionGuaranteeFixture(func(g *flow.CollectionGuarantee) {
		g.CollectionID = coll.ID()
	})
	block := unittest.BlockWithGuaranteesFixture([]*flow.CollectionGuarantee{guarantee})

	collections := storagemock.NewCollections(t)
	collections.On("ByID", coll.ID()).Return(nil, storage.ErrNotFound)

	_, err := AssembleExecutableBlock(block, collections, unittest.StateCommitmentFixture())
	require.ErrorIs(t, err, storage.ErrNotFound)
}

// TestThrowawaySigner confirms the throwaway signer can be built, so the receipt/SPOCK path in the
// block computer has a valid signer during re-execution.
func TestThrowawaySigner(t *testing.T) {
	me, err := throwawaySigner()
	require.NoError(t, err)
	require.Equal(t, flow.RoleExecution, me.Role())
}
