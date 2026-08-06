package wal

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/payloadless"
	"github.com/onflow/flow-go/utils/unittest"
)

// iterateStats accumulates the statistics produced by IterateCheckpointNodes for testing.
type iterateStats struct {
	total       uint64
	leaf        uint64
	interim     uint64
	payloadSize uint64
}

func collectIterateStats(t *testing.T, dir, fileName string) iterateStats {
	var s iterateStats
	seen := make(map[uint64]struct{})
	err := IterateCheckpointNodes(zerolog.Nop(), dir, fileName, func(n *CheckpointNode) error {
		// Every node is delivered exactly once with a unique global index.
		_, dup := seen[n.Index]
		require.False(t, dup, "node index %d delivered more than once", n.Index)
		seen[n.Index] = struct{}{}

		s.total++
		if n.IsLeaf {
			s.leaf++
			s.payloadSize += uint64(n.PayloadSize)
			require.Zero(t, n.LeftChildIndex)
			require.Zero(t, n.RightChildIndex)
		} else {
			s.interim++
			// descendants-first: children precede the node
			require.Less(t, n.LeftChildIndex, n.Index)
			require.Less(t, n.RightChildIndex, n.Index)
		}
		return nil
	})
	require.NoError(t, err)
	return s
}

// oracleStatsV6 computes the expected statistics by loading the checkpoint into
// memory and iterating the unique nodes of the whole forest (matching how the
// checkpoint dedups shared subtries when storing).
func oracleStatsV6(t *testing.T, tries []*trie.MTrie) iterateStats {
	var s iterateStats
	visited := make(map[*node.Node]uint64)
	visited[nil] = 0
	for _, tr := range tries {
		for itr := flattener.NewUniqueNodeIterator(tr.RootNode(), visited); itr.Next(); {
			n := itr.Value()
			visited[n] = uint64(len(visited))
			s.total++
			if n.IsLeaf() {
				s.leaf++
				s.payloadSize += uint64(ledger.EncodedPayloadLengthWithoutPrefix(n.Payload(), payloadEncodingVersion))
			} else {
				s.interim++
			}
		}
	}
	return s
}

// oracleStatsV7 mirrors oracleStatsV6 for payloadless tries. Payloadless leaves
// store no payload, so payloadSize is always 0.
func oracleStatsV7(t *testing.T, tries []*payloadless.MTrie) iterateStats {
	var s iterateStats
	visited := make(map[*payloadless.Node]uint64)
	visited[nil] = 0
	for _, tr := range tries {
		for itr := payloadless.NewUniqueNodeIterator(tr.RootNode(), visited); itr.Next(); {
			n := itr.Value()
			visited[n] = uint64(len(visited))
			s.total++
			if n.IsLeaf() {
				s.leaf++
			} else {
				s.interim++
			}
		}
	}
	return s
}

func TestIterateCheckpointNodesV6(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("simple trie", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			tries := createSimpleTrie(t)
			fileName := "checkpoint-iterate-v6-simple"
			require.NoError(t, StoreCheckpointV6Concurrently(tries, dir, fileName, logger))

			got := collectIterateStats(t, dir, fileName)
			want := oracleStatsV6(t, tries)
			require.Equal(t, want, got)
		})
	})

	t.Run("multiple random tries", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			tries := createMultipleRandomTries(t)
			fileName := "checkpoint-iterate-v6-multi"
			require.NoError(t, StoreCheckpointV6Concurrently(tries, dir, fileName, logger))

			got := collectIterateStats(t, dir, fileName)
			want := oracleStatsV6(t, tries)
			require.Equal(t, want, got)
			require.Positive(t, got.leaf)
			require.Positive(t, got.interim)
			require.Positive(t, got.payloadSize)
		})
	})
}

func TestIterateCheckpointNodesV7(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("simple trie", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			tries := createSimplePayloadlessTrie(t)
			fileName := "checkpoint-iterate-v7-simple"
			require.NoError(t, StoreCheckpointV7Concurrently(tries, dir, fileName, logger))

			got := collectIterateStats(t, dir, fileName)
			want := oracleStatsV7(t, tries)
			require.Equal(t, want, got)
			require.Zero(t, got.payloadSize, "v7 leaves store no payload")
		})
	})

	t.Run("multiple random tries", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			tries := createMultiplePayloadlessTries(t)
			fileName := "checkpoint-iterate-v7-multi"
			require.NoError(t, StoreCheckpointV7Concurrently(tries, dir, fileName, logger))

			got := collectIterateStats(t, dir, fileName)
			want := oracleStatsV7(t, tries)
			require.Equal(t, want, got)
			require.Positive(t, got.leaf)
			require.Positive(t, got.interim)
		})
	})
}

func TestCheckpointIteratorForwardReference(t *testing.T) {
	it := &checkpointIterator{
		fn:          func(*CheckpointNode) error { return nil },
		isDefault:   newBitset(16),
		referenced:  newBitset(16),
		total:       15,
		logProgress: func(uint64) {},
	}

	// An interim node at global index 3 referencing a child at index 5 violates
	// the descendants-first ordering (the child has not been seen yet).
	err := it.emit(nodeMeta{isLeaf: false, height: 1}, 3, 5, 0)
	require.ErrorIs(t, err, ErrCheckpointIntegrity)

	// A node referencing only already-seen children is accepted and marks them
	// as referenced.
	require.NoError(t, it.emit(nodeMeta{isLeaf: false, height: 2}, 6, 2, 4))
	require.True(t, it.referenced.get(2))
	require.True(t, it.referenced.get(4))
	require.False(t, it.referenced.get(6))
}

func TestCheckpointIteratorDefaultChild(t *testing.T) {
	it := &checkpointIterator{
		fn:          func(*CheckpointNode) error { return nil },
		isDefault:   newBitset(16),
		referenced:  newBitset(16),
		total:       15,
		logProgress: func(uint64) {},
	}

	// Emit a node at index 2 whose hash equals the default hash for its height: it
	// is recorded as a default (completely unallocated) sub-trie.
	const height = 1
	require.NoError(t, it.emit(
		nodeMeta{isLeaf: true, height: height, hash: ledger.GetDefaultHashForHeight(height)},
		2, 0, 0,
	))
	require.True(t, it.isDefault.get(2))

	// An interim node referencing the default child is an integrity violation: a
	// compactified trie collapses default children to nil rather than storing them.
	err := it.emit(nodeMeta{isLeaf: false, height: height + 1}, 3, 2, 0)
	require.ErrorIs(t, err, ErrCheckpointIntegrity)
}

func TestBitset(t *testing.T) {
	b := newBitset(130)
	require.False(t, b.get(0))
	require.False(t, b.get(64))
	require.False(t, b.get(129))

	b.set(0)
	b.set(64)
	b.set(129)
	require.True(t, b.get(0))
	require.True(t, b.get(64))
	require.True(t, b.get(129))
	require.False(t, b.get(1))
	require.False(t, b.get(63))
	require.False(t, b.get(65))
}

func TestIterateCheckpointNodesEmptyTrie(t *testing.T) {
	logger := zerolog.Nop()
	unittest.RunWithTempDir(t, func(dir string) {
		tries := []*trie.MTrie{trie.NewEmptyMTrie()}
		fileName := "checkpoint-iterate-v6-empty"
		require.NoError(t, StoreCheckpointV6Concurrently(tries, dir, fileName, logger))

		got := collectIterateStats(t, dir, fileName)
		require.Equal(t, iterateStats{}, got, "empty trie has no stored nodes")
	})
}
