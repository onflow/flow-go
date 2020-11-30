package trie_test

import (
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/stretchr/testify/require"
	"testing"
)

var (
	keyDo = ledger.Key{KeyParts: []ledger.KeyPart{
		ledger.NewKeyPart(0, []byte("do")),
	}}
	keyRe = ledger.Key{KeyParts: []ledger.KeyPart{
		ledger.NewKeyPart(0, []byte("re")),
	}}

	// we don't really expect errors here
	pathDo, _ = pathfinder.KeyToPath(keyDo, complete.DefaultPathFinderVersion)
	pathRe, _ = pathfinder.KeyToPath(keyRe, complete.DefaultPathFinderVersion)

	payloadDo = *ledger.NewPayload(keyDo, []byte{1, 2, 3})
	payloadRe = *ledger.NewPayload(keyRe, []byte{3, 3})
)

func Test_EmptyIndexVsNotSetupIndex(t *testing.T) {

	emptyTrie, err := trie.NewEmptyMTrie(ReferenceImplPathByteSize, []ledger.Key{
		keyDo,
	})
	require.NoError(t, err)

	indexed, err := emptyTrie.GetIndexed(keyDo)
	require.NoError(t, err)
	require.Empty(t, indexed)

	indexed, err = emptyTrie.GetIndexed(keyRe)
	require.Error(t, err)
	require.Empty(t, indexed)
}

func Test_BasicIndexing(t *testing.T) {

	emptyTrie, err := trie.NewEmptyMTrie(ReferenceImplPathByteSize, []ledger.Key{
		keyDo,
	})
	require.NoError(t, err)

	newTrie, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, []ledger.Path{pathDo, pathRe}, []ledger.Payload{payloadDo, payloadRe})
	require.NoError(t, err)

	indexed, err := newTrie.GetIndexed(keyDo)
	require.NoError(t, err)
	require.NotEmpty(t, indexed)
	require.Len(t, indexed, 1)
	require.Equal(t, payloadDo, *indexed[0])

	indexed, err = newTrie.GetIndexed(keyRe)
	require.Error(t, err)
	require.Empty(t, indexed)
}
