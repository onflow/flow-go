package inmem_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/unittest"
)

// test that we have the same snapshot after an encode/decode cycle
// in particular with differing public key implementations
func TestEncodeDecode(t *testing.T) {

	participants := unittest.IdentityListFixture(10, unittest.WithAllRoles())
	// add a partner, which has its key represented as an encodable wrapper
	// type rather than the direct crypto type
	partner := unittest.IdentityFixture(unittest.WithKeys, func(identity *flow.Identity) {
		identity.NetworkPubKey = encodable.NetworkPubKey{PublicKey: identity.NetworkPubKey}
	})
	participants = append(participants, partner)
	initialSnapshot := unittest.RootSnapshotFixture(participants)

	// encode then decode the snapshot
	var decodedSnapshot inmem.EncodableSnapshot
	bz, err := json.Marshal(initialSnapshot.Encodable())
	require.NoError(t, err)
	err = json.Unmarshal(bz, &decodedSnapshot)
	require.NoError(t, err)

	// check that the computed and stored result IDs are consistent
	decodedResult, decodedSeal := decodedSnapshot.LatestResult, decodedSnapshot.LatestSeal
	assert.Equal(t, decodedResult.ID(), decodedSeal.ResultID)
}
