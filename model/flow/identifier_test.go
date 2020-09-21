package flow_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
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
	for i, _ := range ids {
		ids[i] = unittest.IdentifierFixture()
	}

	t.Run("Sample creates a random sample", func(t *testing.T) {
		sampleSize := uint(5)
		sample, err := flow.Sample(sampleSize, ids...)
		require.NoError(t, err)
		require.Len(t, sample, int(sampleSize))
		require.NotEqual(t, sample, ids[:sampleSize])
	})

	t.Run("sample size greater than total size", func(t *testing.T) {
		sampleSize := uint(len(ids) + 1)
		_, err := flow.Sample(sampleSize, ids...)
		require.Error(t, err)
	})
}
