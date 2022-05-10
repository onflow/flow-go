package execution_data_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	goassert "gotest.tools/assert"

	"github.com/onflow/flow-go/ledger"
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

func generateChunkExecutionData(t *testing.T, minSerializedSize uint64) *execution_data.ChunkExecutionData {
	ced := &execution_data.ChunkExecutionData{
		TrieUpdate: &ledger.TrieUpdate{
			Payloads: []*ledger.Payload{
				{
					Value: nil,
				},
			},
		},
	}

	size := 1

	for {
		buf := &bytes.Buffer{}
		require.NoError(t, execution_data.DefaultSerializer.Serialize(buf, ced))

		if buf.Len() >= int(minSerializedSize) {
			t.Logf("Chunk execution data size: %d", buf.Len())
			return ced
		}

		v := make([]byte, size)
		rand.Read(v)
		ced.TrieUpdate.Payloads[0].Value = v
		size *= 2
	}
}

func generateBlockExecutionData(t *testing.T, numChunks int, minSerializedSizePerChunk uint64) *execution_data.BlockExecutionData {
	bed := &execution_data.BlockExecutionData{
		BlockID:             unittest.IdentifierFixture(),
		ChunkExecutionDatas: make([]*execution_data.ChunkExecutionData, numChunks),
	}

	for i := 0; i < numChunks; i++ {
		bed.ChunkExecutionDatas[i] = generateChunkExecutionData(t, minSerializedSizePerChunk)
	}

	return bed
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

func TestHappyPath(t *testing.T) {
	t.Parallel()

	eds := getExecutionDataStore(getBlobstore(), execution_data.DefaultSerializer)

	test := func(numChunks int, minSerializedSizePerChunk uint64) {
		expected := generateBlockExecutionData(t, numChunks, minSerializedSizePerChunk)
		rootId, err := eds.AddExecutionData(context.Background(), expected)
		require.NoError(t, err)
		actual, err := eds.GetExecutionData(context.Background(), rootId)
		require.NoError(t, err)
		goassert.DeepEqual(t, expected, actual)
	}

	test(1, 0)                                   // small execution data (single level blob tree)
	test(1, 5*execution_data.DefaultMaxBlobSize) // large execution data (multi level blob tree)
}

type randomSerializer struct{}

func (rs *randomSerializer) Serialize(w io.Writer, v interface{}) error {
	data := make([]byte, 1024)
	rand.Read(data)
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
		corruptedChunk: rand.Intn(numChunks) + 1,
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
			rand.Read(data[len(data)-1024:])

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
		rootID, err := malformedEds.AddExecutionData(context.Background(), bed)
		require.NoError(t, err)
		_, err = defaultEds.GetExecutionData(context.Background(), rootID)
		var malformedDataErr *execution_data.MalformedDataError
		assert.ErrorAs(t, err, &malformedDataErr)
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
	rootID, err := eds.AddExecutionData(context.Background(), bed)
	require.NoError(t, err)

	cids := getAllKeys(t, blobstore)
	t.Logf("%d blobs in blob tree", len(cids))

	cidToDelete := cids[rand.Intn(len(cids))]
	blobstore.DeleteBlob(context.Background(), cidToDelete)

	_, err = eds.GetExecutionData(context.Background(), rootID)
	var blobNotFoundError *execution_data.BlobNotFoundError
	assert.ErrorAs(t, err, &blobNotFoundError)
}
