package flow_test

import (
	"encoding/json"
	"fmt"
	"testing"

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
	require.Equal(t, referenceMerkleRoot(ids...), flow.MerkleRoot(ids...))

}

func referenceMerkleRoot(ids ...flow.Identifier) flow.Identifier {
	var root flow.Identifier
	tree := merkle.NewTree()
	for _, id := range ids {
		idCopy := id
		tree.Put(idCopy[:], nil)
	}
	hash := tree.Hash()
	copy(root[:], hash)
	return root
}
