package local

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestInitializeWithMatchingKey(t *testing.T) {
	stakingPriv := unittest.StakingPrivKeyFixture()
	nodeID := unittest.IdentityFixture()
	nodeID.StakingPubKey = stakingPriv.PublicKey()

	me, err := New(nodeID, stakingPriv)
	require.NoError(t, err)
	require.Equal(t, nodeID.NodeID, me.NodeID())
}

func TestInitializeWithMisMatchingKey(t *testing.T) {
	stakingPriv := unittest.StakingPrivKeyFixture()
	badPriv := unittest.StakingPrivKeyFixture()

	nodeID := unittest.IdentityFixture()
	nodeID.StakingPubKey = badPriv.PublicKey()

	_, err := New(nodeID, stakingPriv)
	require.Error(t, err)
}
