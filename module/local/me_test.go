package local

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

// should be able to initialize with a Identity whose PublicKey matches with
// the given private key's public key
func TestInitializeWithMatchingKey(t *testing.T) {
	stakingPriv := unittest.StakingPrivKeyFixture()
	nodeID := unittest.IdentityFixture()
	nodeID.StakingPubKey = stakingPriv.PublicKey()

	me, err := New(nodeID, stakingPriv)
	require.NoError(t, err)
	require.Equal(t, nodeID.NodeID, me.NodeID())
}

// should fail to initialize with a Identity whose PublicKey mismatch with
// the given private key's public key
func TestInitializeWithMisMatchingKey(t *testing.T) {
	stakingPriv := unittest.StakingPrivKeyFixture()
	badPriv := unittest.StakingPrivKeyFixture()

	nodeID := unittest.IdentityFixture()
	nodeID.StakingPubKey = badPriv.PublicKey()

	_, err := New(nodeID, stakingPriv)
	require.Error(t, err)
}
