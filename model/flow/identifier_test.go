package flow_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

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
