package convert_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestConvertSnapshot(t *testing.T) {
	t.Parallel()

	identities := unittest.CompleteIdentitySet()
	snapshot := unittest.RootSnapshotFixtureWithChainID(identities, flow.Testnet.Chain().ChainID())

	msg, err := convert.SnapshotToBytes(snapshot)
	require.NoError(t, err)

	converted, err := convert.BytesToInmemSnapshot(msg)
	require.NoError(t, err)

	assert.Equal(t, snapshot, converted)
}
