package protocol_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestIsSporkRootSnapshot(t *testing.T) {
	t.Run("spork root", func(t *testing.T) {
		snapshot := unittest.RootSnapshotFixture(unittest.IdentityListFixture(10, unittest.WithAllRoles()))
		isSporkRoot, err := protocol.IsSporkRootSnapshot(snapshot)
		require.NoError(t, err)
		assert.True(t, isSporkRoot)
	})

	t.Run("other snapshot", func(t *testing.T) {
		snapshot := unittest.RootSnapshotFixture(unittest.IdentityListFixture(10, unittest.WithAllRoles()))
		snapshot.Encodable().SealingSegment.Blocks = unittest.BlockFixtures(5)
		isSporkRoot, err := protocol.IsSporkRootSnapshot(snapshot)
		require.NoError(t, err)
		assert.False(t, isSporkRoot)
	})
}
