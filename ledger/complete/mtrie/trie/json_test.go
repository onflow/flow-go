package trie_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/module/metrics"
)

func Test_DumpJSONEmpty(t *testing.T) {

	trie := trie.NewEmptyMTrie()

	var buffer bytes.Buffer
	err := trie.DumpAsJSON(&buffer)
	require.NoError(t, err)

	json := buffer.String()
	assert.Empty(t, json)
}

func Test_DumpJSONNonEmpty(t *testing.T) {

	forest, err := mtrie.NewForest(complete.DefaultCacheSize, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	emptyRootHash := forest.GetEmptyRootHash()

	kps := ledger.NewKeyPart(0, []byte("si"))
	kpv := ledger.NewKeyPart(5, []byte("vis"))
	kpp := ledger.NewKeyPart(3, []byte("pacem"))
	key1 := ledger.NewKey([]*ledger.KeyPart{&kps, &kpv, &kpp})

	kpe := ledger.NewKeyPart(3, []byte("ex"))
	kpna := ledger.NewKeyPart(6, []byte("navicula"))
	kpn := ledger.NewKeyPart(9, []byte("navis"))
	key2 := ledger.NewKey([]*ledger.KeyPart{&kpe, &kpna, &kpn})

	kpl := ledger.NewKeyPart(9, []byte("lorem"))
	kpi := ledger.NewKeyPart(0, []byte("ipsum"))
	kpd := ledger.NewKeyPart(5, []byte("dolor"))
	key3 := ledger.NewKey([]*ledger.KeyPart{&kpl, &kpi, &kpd})

	update, err := ledger.NewUpdate(ledger.State(emptyRootHash), []ledger.Key{key1, key2, key3}, []ledger.Value{{1}, {2}, {3}})
	require.NoError(t, err)

	trieUpdate, err := pathfinder.UpdateToTrieUpdate(update, 0)
	require.NoError(t, err)

	newHash, err := forest.Update(trieUpdate)
	require.NoError(t, err)

	newTrie, err := forest.GetTrie(newHash)
	require.NoError(t, err)

	var buffer bytes.Buffer

	err = newTrie.DumpAsJSON(&buffer)
	require.NoError(t, err)

	json := buffer.String()
	split := strings.Split(json, "\n")

	//filter out empty strings
	jsons := make([]string, 0)
	for _, s := range split {
		if len(s) > 0 {
			jsons = append(jsons, s)
		}
	}

	require.Len(t, jsons, 3)

	// key 1
	require.Contains(t, jsons, "{\"Key\":{\"KeyParts\":[{\"Type\":0,\"Value\":\"7369\"},{\"Type\":5,\"Value\":\"766973\"},{\"Type\":3,\"Value\":\"706163656d\"}]},\"Value\":\"01\"}")

	// key 2
	require.Contains(t, jsons, "{\"Key\":{\"KeyParts\":[{\"Type\":3,\"Value\":\"6578\"},{\"Type\":6,\"Value\":\"6e61766963756c61\"},{\"Type\":9,\"Value\":\"6e61766973\"}]},\"Value\":\"02\"}")

	// key 3
	require.Contains(t, jsons, "{\"Key\":{\"KeyParts\":[{\"Type\":9,\"Value\":\"6c6f72656d\"},{\"Type\":0,\"Value\":\"697073756d\"},{\"Type\":5,\"Value\":\"646f6c6f72\"}]},\"Value\":\"03\"}")
}
