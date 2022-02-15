package flow_test

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"

	blocks "github.com/ipfs/go-block-format"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/merkle"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestIdentifierFormat(t *testing.T) {
	id := unittest.IdentifierFixture()

	// should print hex representation with %x formatting verb
	t.Run("%x", func(t *testing.T) {
		formatted := fmt.Sprintf("%x", id)
		assert.Equal(t, id.String(), formatted)
	})

	// should print hex representation with %s formatting verb
	t.Run("%s", func(t *testing.T) {
		formatted := fmt.Sprintf("%s", id) //nolint:gosimple
		assert.Equal(t, id.String(), formatted)
	})

	// should print hex representation with default formatting verb
	t.Run("%v", func(t *testing.T) {
		formatted := fmt.Sprintf("%v", id)
		assert.Equal(t, id.String(), formatted)
	})

	// should handle unsupported verbs
	t.Run("unsupported formatting verb", func(t *testing.T) {
		formatted := fmt.Sprintf("%d", id)
		expected := fmt.Sprintf("%%!d(flow.Identifier=%s)", id)
		assert.Equal(t, expected, formatted)
	})
}

func TestIdentifierJSON(t *testing.T) {
	id := unittest.IdentifierFixture()
	bz, err := json.Marshal(id)
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("\"%v\"", id), string(bz))
	var actual flow.Identifier
	err = json.Unmarshal(bz, &actual)
	assert.NoError(t, err)
	assert.Equal(t, id, actual)
}

func TestIdentifierSample(t *testing.T) {

	total := 10
	ids := make([]flow.Identifier, total)
	for i := range ids {
		ids[i] = unittest.IdentifierFixture()
	}

	t.Run("Sample creates a random sample", func(t *testing.T) {
		sampleSize := uint(5)
		sample := flow.Sample(sampleSize, ids...)
		require.Len(t, sample, int(sampleSize))
		require.NotEqual(t, sample, ids[:sampleSize])
	})

	t.Run("sample size greater than total size results in the original list", func(t *testing.T) {
		sampleSize := uint(len(ids) + 1)
		sample := flow.Sample(sampleSize, ids...)
		require.Equal(t, sample, ids)
	})

	t.Run("sample size of zero results in an empty list", func(t *testing.T) {
		sampleSize := uint(0)
		sample := flow.Sample(sampleSize, ids...)
		require.Empty(t, sample)
	})
}

func TestMerkleRoot(t *testing.T) {
	total := 10
	ids := make([]flow.Identifier, total)
	for i := range ids {
		ids[i] = unittest.IdentifierFixture()
	}

	idsBad := make([]flow.Identifier, total)
	for i := range idsBad {
		idsBad[i] = ids[len(ids)-1]
	}
	fmt.Println(ids)
	fmt.Println(idsBad)

	require.NotEqual(t, flow.MerkleRoot(ids...), flow.MerkleRoot(idsBad...))
	require.Equal(t, referenceMerkleRoot(t, ids...), flow.MerkleRoot(ids...))
}

// We should ideally replace this with a completely different reference implementation
// Possibly written in another language, such as python, similar to the Ledger Trie Implementation
func referenceMerkleRoot(t *testing.T, ids ...flow.Identifier) flow.Identifier {
	var root flow.Identifier
	tree, err := merkle.NewTree(flow.IdentifierLen)
	assert.NoError(t, err)
	for idx, id := range ids {
		idxVal := make([]byte, 8)
		binary.BigEndian.PutUint64(idxVal, uint64(idx))
		_, err := tree.Put(id[:], idxVal)
		assert.NoError(t, err)
	}
	hash := tree.Hash()
	copy(root[:], hash)
	return root
}

// TestCIDConversion tests that the CID conversion functions are working as expected
// It generates Flow ID / CID fixtures and converts them back and forth to check that
// the conversion is correct.
func TestCIDConversion(t *testing.T) {
	id := unittest.IdentifierFixture()
	cid := flow.IdToCid(id)
	id2, err := flow.CidToId(cid)
	assert.NoError(t, err)
	assert.Equal(t, id, id2)

	// generate random CID
	data := make([]byte, 4)
	rand.Read(data)
	cid = blocks.NewBlock(data).Cid()

	id, err = flow.CidToId(cid)
	cid2 := flow.IdToCid(id)
	assert.NoError(t, err)
	assert.Equal(t, cid, cid2)
}

// TestByteConversionRoundTrip evaluates the round trip of conversion of identifiers to bytes, and back.
// The original identifiers must be recovered from the converted bytes.
func TestByteConversionRoundTrip(t *testing.T) {
	ids := unittest.IdentifierListFixture(10)

	converted, err := flow.ByteSlicesToIds(flow.IdsToBytes(ids))
	require.NoError(t, err)

	require.Equal(t, len(ids), len(converted))
	require.ElementsMatch(t, ids, converted)
}
