package state_synchronization

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/fxamacker/cbor/v2"
	datastore "github.com/ipfs/go-datastore/examples"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/computation/computer/uploader"
	cborcodec "github.com/onflow/flow-go/model/encoding/cbor"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/compressor"
)

func makeBlockstore(t *testing.T, name string) (blockstore.Blockstore, func()) {
	dsDir := filepath.Join(os.TempDir(), name)
	require.NoError(t, os.RemoveAll(dsDir))
	err := os.Mkdir(dsDir, fs.ModeDir)
	require.NoError(t, err)

	ds, err := datastore.NewDatastore(dsDir)
	require.NoError(t, err)

	return blockstore.NewBlockstore(ds.(*datastore.Datastore)), func() {
		require.NoError(t, os.RemoveAll(dsDir))
	}
}

func TestStateDiffStorer(t *testing.T) {
	bstore, cleanup := makeBlockstore(t, "state-diff-storer-test")
	defer cleanup()

	sdp, err := NewStateDiffStorer(&cborcodec.Codec{}, compressor.NewLz4Compressor(), bstore)
	require.NoError(t, err)

	file, err := os.Open("/Users/smnzhu/Downloads/8aeb01b08a446434ea4746aaaec1c458fd24922e26a13e94842bafb74e681670.cbor")
	require.NoError(t, err)

	var blockData uploader.BlockData

	decoder := cbor.NewDecoder(file)
	require.NoError(t, decoder.Decode(&blockData))

	var collections []*flow.Collection
	for _, c := range blockData.Collections {
		collections = append(collections, &flow.Collection{Transactions: c.Transactions})
	}

	sd := &ExecutionStateDiff{
		Collections:        collections,
		TransactionResults: blockData.TxResults,
		Events:             blockData.Events,
		TrieUpdate:         blockData.TrieUpdates,
	}

	cid, err := sdp.Store(sd)
	require.NoError(t, err)

	sd2, err := sdp.Load(cid)
	require.NoError(t, err)

	assert.True(t, reflect.DeepEqual(sd, sd2))
}
