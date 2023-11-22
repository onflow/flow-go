package execution_data_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	mrand "math/rand"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	goassert "gotest.tools/assert"

	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/utils/unittest"
)

func getBlobstore() blobs.Blobstore {
	return blobs.NewBlobstore(dssync.MutexWrap(datastore.NewMapDatastore()))
}

func getExecutionDataStore(blobstore blobs.Blobstore, serializer execution_data.Serializer) execution_data.ExecutionDataStore {
	return execution_data.NewExecutionDataStore(blobstore, serializer)
}

func generateBlockExecutionData(t *testing.T, numChunks int, minSerializedSizePerChunk uint64) *execution_data.BlockExecutionData {
	ceds := make([]*execution_data.ChunkExecutionData, numChunks)
	for i := 0; i < numChunks; i++ {
		ceds[i] = unittest.ChunkExecutionDataFixture(t, int(minSerializedSizePerChunk))
	}

	return unittest.BlockExecutionDataFixture(unittest.WithChunkExecutionDatas(ceds...))
}

func getAllKeys(t *testing.T, bs blobs.Blobstore) []cid.Cid {
	cidChan, err := bs.AllKeysChan(context.Background())
	require.NoError(t, err)

	var cids []cid.Cid

	for cid := range cidChan {
		cids = append(cids, cid)
	}

	return cids
}

func deepEqual(t *testing.T, expected, actual *execution_data.BlockExecutionData) {
	assert.Equal(t, expected.BlockID, actual.BlockID)
	assert.Equal(t, len(expected.ChunkExecutionDatas), len(actual.ChunkExecutionDatas))

	for i, expectedChunk := range expected.ChunkExecutionDatas {
		actualChunk := actual.ChunkExecutionDatas[i]

		goassert.DeepEqual(t, expectedChunk.Collection, actualChunk.Collection)
		goassert.DeepEqual(t, expectedChunk.Events, actualChunk.Events)
		assert.True(t, expectedChunk.TrieUpdate.Equals(actualChunk.TrieUpdate))
	}
}

func TestHappyPath(t *testing.T) {
	t.Parallel()

	eds := getExecutionDataStore(getBlobstore(), execution_data.DefaultSerializer)

	test := func(numChunks int, minSerializedSizePerChunk uint64) {
		expected := generateBlockExecutionData(t, numChunks, minSerializedSizePerChunk)
		rootId, err := eds.Add(context.Background(), expected)
		require.NoError(t, err)
		actual, err := eds.Get(context.Background(), rootId)
		require.NoError(t, err)
		deepEqual(t, expected, actual)
	}

	test(1, 0)                                   // small execution data (single level blob tree)
	test(5, 5*execution_data.DefaultMaxBlobSize) // large execution data (multi level blob tree)
}

type randomSerializer struct{}

func (rs *randomSerializer) Serialize(w io.Writer, v interface{}) error {
	data := make([]byte, 1024)
	_, _ = rand.Read(data)
	_, err := w.Write(data)
	return err
}

func (rs *randomSerializer) Deserialize(r io.Reader) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

type corruptedTailSerializer struct {
	corruptedChunk int
	i              int
}

func newCorruptedTailSerializer(numChunks int) *corruptedTailSerializer {
	return &corruptedTailSerializer{
		corruptedChunk: mrand.Intn(numChunks) + 1,
	}
}

func (cts *corruptedTailSerializer) Serialize(w io.Writer, v interface{}) error {
	if _, ok := v.(*execution_data.ChunkExecutionData); ok {
		cts.i++
		if cts.i == cts.corruptedChunk {
			buf := &bytes.Buffer{}

			err := execution_data.DefaultSerializer.Serialize(buf, v)
			if err != nil {
				return err
			}

			data := buf.Bytes()
			_, _ = rand.Read(data[len(data)-1024:])

			_, err = w.Write(data)
			return err
		}
	}

	return execution_data.DefaultSerializer.Serialize(w, v)
}

func (cts *corruptedTailSerializer) Deserialize(r io.Reader) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

func TestMalformedData(t *testing.T) {
	t.Parallel()

	test := func(bed *execution_data.BlockExecutionData, serializer execution_data.Serializer) {
		blobstore := getBlobstore()
		defaultEds := getExecutionDataStore(blobstore, execution_data.DefaultSerializer)
		malformedEds := getExecutionDataStore(blobstore, serializer)
		rootID, err := malformedEds.Add(context.Background(), bed)
		require.NoError(t, err)
		_, err = defaultEds.Get(context.Background(), rootID)
		assert.True(t, execution_data.IsMalformedDataError(err))
	}

	numChunks := 5
	bed := generateBlockExecutionData(t, numChunks, 10*execution_data.DefaultMaxBlobSize)

	test(bed, &randomSerializer{})                   // random bytes
	test(bed, newCorruptedTailSerializer(numChunks)) // serialized execution data with random bytes replaced at the end of a random chunk
}

func TestGetIncompleteData(t *testing.T) {
	t.Parallel()

	blobstore := getBlobstore()
	eds := getExecutionDataStore(blobstore, execution_data.DefaultSerializer)

	bed := generateBlockExecutionData(t, 5, 10*execution_data.DefaultMaxBlobSize)
	rootID, err := eds.Add(context.Background(), bed)
	require.NoError(t, err)

	cids := getAllKeys(t, blobstore)
	t.Logf("%d blobs in blob tree", len(cids))

	cidToDelete := cids[mrand.Intn(len(cids))]
	require.NoError(t, blobstore.DeleteBlob(context.Background(), cidToDelete))

	_, err = eds.Get(context.Background(), rootID)
	var blobNotFoundError *execution_data.BlobNotFoundError
	assert.ErrorAs(t, err, &blobNotFoundError)
}
