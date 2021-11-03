package state_synchronization

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/fxamacker/cbor/v2"
	datastore "github.com/ipfs/go-datastore/examples"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

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

const BUCKET_NAME = "flow_public_mainnet14_execution_state"

func TestStateDiffStorer(t *testing.T) {
	// this test is intended to be run locally
	t.Skip()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := storage.NewClient(ctx, option.WithoutAuthentication())
	require.NoError(t, err)
	bucket := client.Bucket(BUCKET_NAME)

	var maxFileSize int64
	var maxCollectionsLength int
	var maxEventsLength int
	var maxStoreTime time.Duration
	var maxLoadTime time.Duration

	it := bucket.Objects(ctx, &storage.Query{Prefix: ""})
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		require.NoError(t, err)

		t.Logf("reading object: %v [%v bytes]\n", attrs.Name, attrs.Size)
		if attrs.Size > maxFileSize {
			maxFileSize = attrs.Size
		}

		reader, err := bucket.Object(attrs.Name).NewReader(ctx)
		require.NoError(t, err)
		defer reader.Close()

		bstore, cleanup := makeBlockstore(t, "state-diff-storer-test")
		defer cleanup()

		sdp, err := NewStateDiffStorer(&cborcodec.Codec{}, compressor.NewLz4Compressor(), bstore)
		require.NoError(t, err)

		var blockData uploader.BlockData

		decoder := cbor.NewDecoder(reader)
		require.NoError(t, decoder.Decode(&blockData))

		var collections []*flow.Collection
		for _, c := range blockData.Collections {
			collections = append(collections, &flow.Collection{Transactions: c.Transactions})
		}

		if len(collections) > maxCollectionsLength {
			maxCollectionsLength = len(collections)
		}
		if len(blockData.Events) > maxEventsLength {
			maxEventsLength = len(blockData.Events)
		}

		sd := &ExecutionStateDiff{
			Collections:        collections,
			TransactionResults: blockData.TxResults,
			Events:             blockData.Events,
			TrieUpdate:         blockData.TrieUpdates,
		}

		start := time.Now()
		cid, err := sdp.Store(sd)
		duration := time.Since(start)
		require.NoError(t, err)
		t.Logf("time to store state diff: %v\n", duration)
		if duration > maxStoreTime {
			maxStoreTime = duration
		}

		start = time.Now()
		sd2, err := sdp.Load(cid)
		duration = time.Since(start)
		require.NoError(t, err)
		t.Logf("time to load state diff: %v\n", duration)
		if duration > maxLoadTime {
			maxLoadTime = duration
		}

		assert.True(t, reflect.DeepEqual(sd, sd2))
	}

	t.Log()
	t.Logf("largest file: %v bytes\n", maxFileSize)
	t.Logf("max collections length: %v\n", maxCollectionsLength)
	t.Logf("max events length: %v\n", maxEventsLength)
	t.Logf("max store time: %v\n", maxStoreTime)
	t.Logf("max load time: %v\n", maxLoadTime)
}
