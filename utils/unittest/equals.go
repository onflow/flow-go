package unittest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/inmem"
)

func toHex(ids []flow.Identifier) []string {
	hex := make([]string, 0, len(ids))
	for _, id := range ids {
		hex = append(hex, id.String())
	}
	return hex
}

func IDEqual(t *testing.T, id1, id2 flow.Identifier) {
	require.Equal(t, id1.String(), id2.String())
}

func IDsEqual(t *testing.T, id1, id2 []flow.Identifier) {
	require.Equal(t, toHex(id1), toHex(id2))
}

func AssertSnapshotsEqual(t *testing.T, snap1, snap2 protocol.Snapshot) {
	inmem1, err := inmem.FromSnapshot(snap1)
	require.NoError(t, err)
	inmem2, err := inmem.FromSnapshot(snap2)
	require.NoError(t, err)

	encoded1, err := json.Marshal(inmem1)
	require.NoError(t, err)
	encoded2, err := json.Marshal(inmem2)
	require.NoError(t, err)

	assert.Equal(t, encoded1, encoded2)
}
