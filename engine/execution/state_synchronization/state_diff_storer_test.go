package state_synchronization

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger/common/utils"
	cborcodec "github.com/onflow/flow-go/model/encoding/cbor"
	"github.com/onflow/flow-go/network/compressor"
	"github.com/onflow/flow-go/network/test"
	"github.com/onflow/flow-go/utils/unittest"
)

func generateExecutionStateDiff(t *testing.T, n int) *ExecutionStateDiff {
	est := &ExecutionStateDiff{}
	for i := 0; i < n; i++ {
		est.TrieUpdates = append(est.TrieUpdates, utils.TrieUpdateFixture(1, 1<<16, 1<<20))
		collection := unittest.CollectionFixture(1)
		est.Collections = append(est.Collections, &collection)
	}
	return est
}

func TestStateDiffStorer(t *testing.T) {
	t.Parallel()

	bstore, cleanup := test.MakeBlockstore(t, "state-diff-storer-test")
	defer cleanup()

	sdp, err := NewStateDiffStorer(&cborcodec.Codec{}, compressor.NewLz4Compressor(), bstore)
	require.NoError(t, err)

	sd := generateExecutionStateDiff(t, 0)
	cid, err := sdp.Store(sd)
	require.NoError(t, err)
	sd2, err := sdp.Load(cid)
	require.NoError(t, err)
	assert.True(t, reflect.DeepEqual(sd, sd2))

	sd = generateExecutionStateDiff(t, 10)
	cid, err = sdp.Store(sd)
	require.NoError(t, err)
	sd2, err = sdp.Load(cid)
	require.NoError(t, err)
	assert.True(t, reflect.DeepEqual(sd, sd2))
}
