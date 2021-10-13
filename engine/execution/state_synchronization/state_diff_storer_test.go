package state_synchronization

import (
	"encoding/xml"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

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

const BUCKET_URL = "https://flow_public_mainnet14_execution_state.storage.googleapis.com/"

type ListBucketResult struct {
	XMLName  xml.Name  `xml:"ListBucketResult"`
	Contents []Content `xml:"Contents"`
}

type Content struct {
	XMLName xml.Name `xml:"Contents"`
	Key     string   `xml:"Key"`
	Size    uint64   `xml:"Size"`
}

func TestStateDiffStorer(t *testing.T) {
	// this test is intended to be run locally
	t.Skip()

	var bucketContents ListBucketResult
	func() {
		var client http.Client
		resp, err := client.Get(BUCKET_URL)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, resp.StatusCode, http.StatusOK)
		require.NoError(t, xml.NewDecoder(resp.Body).Decode(&bucketContents))
	}()

	var maxFileSize uint64
	var maxCollectionsLength int
	var maxEventsLength int

	for _, content := range bucketContents.Contents {
		func() {
			t.Logf("downloading file: %v [%v bytes]", content.Key, content.Size)

			if content.Size > maxFileSize {
				maxFileSize = content.Size
			}

			var client http.Client
			resp, err := client.Get(BUCKET_URL + content.Key)
			require.NoError(t, err)
			defer resp.Body.Close()

			bstore, cleanup := makeBlockstore(t, "state-diff-storer-test")
			defer cleanup()

			sdp, err := NewStateDiffStorer(&cborcodec.Codec{}, compressor.NewLz4Compressor(), bstore)
			require.NoError(t, err)

			var blockData uploader.BlockData

			decoder := cbor.NewDecoder(resp.Body)
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
			t.Logf("time to store state diff: %v", duration)

			start = time.Now()
			sd2, err := sdp.Load(cid)
			duration = time.Since(start)
			require.NoError(t, err)
			t.Logf("time to load state diff: %v", duration)

			assert.True(t, reflect.DeepEqual(sd, sd2))
		}()
	}

	t.Logf("largest file: %v bytes", maxFileSize)
	t.Logf("max collections length: %v", maxCollectionsLength)
	t.Logf("max events length: %v", maxEventsLength)
}
