package state_synchronization

import (
	"bytes"
	"context"
	"math"
	"math/rand"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/encoding/cbor"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/compressor"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/unittest"
)

func mockBlobService(bs blockstore.Blockstore) network.BlobService {
	bex := new(mocknetwork.BlobService)

	bex.On("GetBlobs", mock.Anything, mock.AnythingOfType("[]cid.Cid")).
		Return(func(ctx context.Context, cids []cid.Cid) <-chan blobs.Blob {
			ch := make(chan blobs.Blob)

			var wg sync.WaitGroup
			wg.Add(len(cids))

			for _, c := range cids {
				c := c
				go func() {
					defer wg.Done()

					blob, err := bs.Get(ctx, c)

					if err != nil {
						// In the real implementation, Bitswap would keep trying to get the blob from
						// the network indefinitely, sending requests to more and more peers until it
						// eventually finds the blob, or the context is canceled. Here, we know that
						// if the blob is not already in the blobstore, then we will never appear, so
						// we just wait for the context to be canceled.
						<-ctx.Done()

						return
					}

					ch <- blob
				}()
			}

			go func() {
				wg.Wait()
				close(ch)
			}()

			return ch
		})

	bex.On("AddBlobs", mock.Anything, mock.AnythingOfType("[]blocks.Block")).Return(bs.PutMany)

	return bex
}

func testBlobstore() blockstore.Blockstore {
	return blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore()))
}

func executionData(t *testing.T, s *serializer, minSerializedSize uint64) (*ExecutionData, []byte) {
	ed := &ExecutionData{
		BlockID: unittest.IdentifierFixture(),
	}

	size := 1

	for {
		buf := &bytes.Buffer{}
		require.NoError(t, s.Serialize(buf, ed))

		if buf.Len() >= int(minSerializedSize) {
			t.Logf("Execution data size: %d", buf.Len())
			return ed, buf.Bytes()
		}

		if len(ed.TrieUpdates) == 0 {
			ed.TrieUpdates = append(ed.TrieUpdates, &ledger.TrieUpdate{
				Payloads: []*ledger.Payload{{}},
			})
		}

		v := make([]byte, size)
		rand.Read(v)
		ed.TrieUpdates[0].Payloads[0].Value = v
		size *= 2
	}
}

func getExecutionData(eds ExecutionDataService, rootID flow.Identifier, timeout time.Duration) (*ExecutionData, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return eds.Get(ctx, rootID)
}

func addExecutionData(eds ExecutionDataService, ed *ExecutionData, timeout time.Duration) (flow.Identifier, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	id, _, err := eds.Add(ctx, ed)

	return id, err
}

func putBlob(bs blockstore.Blockstore, data []byte, timeout time.Duration) (cid.Cid, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	blob := blobs.NewBlob(data)

	return blob.Cid(), bs.Put(ctx, blob)
}

func deleteBlob(bs blockstore.Blockstore, c cid.Cid, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return bs.DeleteBlock(ctx, c)
}

func allKeys(t *testing.T, bs blockstore.Blockstore, timeout time.Duration) []cid.Cid {
	cidChan, err := bs.AllKeysChan(context.Background())
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var cids []cid.Cid

	for {
		select {
		case c, ok := <-cidChan:
			if !ok {
				return cids
			}

			cids = append(cids, c)
		case <-ctx.Done():
			require.Fail(t, "timed out waiting for all keys", ctx.Err())
		}
	}
}

func executionDataService(bs network.BlobService) *executionDataServiceImpl {
	codec := new(cbor.Codec)
	compressor := compressor.NewLz4Compressor()
	return NewExecutionDataService(codec, compressor, bs, metrics.NewNoopCollector(), zerolog.Nop())
}

func writeBlobTree(t *testing.T, s *serializer, data []byte, bs blockstore.Blockstore, timeout time.Duration) flow.Identifier {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		numBlobs := len(data) / defaultMaxBlobSize
		if len(data)%defaultMaxBlobSize > 0 {
			numBlobs++
		}

		batch := make([]blobs.Blob, numBlobs)
		for i := 0; i < numBlobs; i++ {
			end := (i + 1) * defaultMaxBlobSize
			if end > len(data) {
				end = len(data)
			}

			batch[i] = blobs.NewBlob(data[i*defaultMaxBlobSize : end])
		}

		require.NoError(t, bs.PutMany(ctx, batch))

		if numBlobs <= 1 {
			id, err := flow.CidToId(batch[0].Cid())
			require.NoError(t, err)

			return id
		}

		cids := make([]cid.Cid, numBlobs)
		for i, b := range batch {
			cids[i] = b.Cid()
		}

		buf := &bytes.Buffer{}
		require.NoError(t, s.Serialize(buf, cids))

		data = buf.Bytes()
	}
}

func TestHappyPath(t *testing.T) {
	t.Parallel()

	eds := executionDataService(mockBlobService(testBlobstore()))

	test := func(minSerializedSize uint64) {
		expected, _ := executionData(t, eds.serializer, minSerializedSize)
		rootCid, err := addExecutionData(eds, expected, time.Second)
		require.NoError(t, err)
		actual, err := getExecutionData(eds, rootCid, time.Second)
		require.NoError(t, err)
		assert.Equal(t, true, reflect.DeepEqual(expected, actual))
	}

	test(0)                       // small execution data (single level blob tree)
	test(10 * defaultMaxBlobSize) // large execution data (multi level blob tree)
}

func TestMalformedData(t *testing.T) {
	t.Parallel()

	bs := testBlobstore()
	eds := executionDataService(mockBlobService(bs))

	test := func(data []byte) {
		rootID := writeBlobTree(t, eds.serializer, data, bs, time.Second)
		_, err := getExecutionData(eds, rootID, time.Second)
		var malformedDataError *MalformedDataError
		assert.ErrorAs(t, err, &malformedDataError)
	}

	// random bytes
	data := make([]byte, 1024)
	rand.Read(data)
	test(data)

	// serialized execution data with random bytes inserted at the end
	_, serializedBytes := executionData(t, eds.serializer, 10*defaultMaxBlobSize)
	rand.Read(serializedBytes[len(serializedBytes)-1024:])
	test(serializedBytes)
}

func TestOversizedBlob(t *testing.T) {
	t.Parallel()

	bs := testBlobstore()
	eds := executionDataService(mockBlobService(bs))

	test := func(data []byte) {
		cid, err := putBlob(bs, data, time.Second)
		require.NoError(t, err)
		fid, err := flow.CidToId(cid)
		require.NoError(t, err)
		_, err = getExecutionData(eds, fid, time.Second)
		var blobSizeLimitExceededError *BlobSizeLimitExceededError
		assert.ErrorAs(t, err, &blobSizeLimitExceededError)
	}

	// blob of random data
	data := make([]byte, defaultMaxBlobSize*2)
	rand.Read(data)
	test(data)

	// blob of serialized execution data
	_, data = executionData(t, eds.serializer, defaultMaxBlobSize+1)
	test(data)

	// multiple oversized blobs of serialized execution data
	_, data = executionData(t, eds.serializer, defaultMaxBlobSize*5)
	blobSize := int(math.Ceil(float64(len(data)) / 4))
	var cids []cid.Cid
	for i := 0; i < 4; i++ {
		blob := data[i*blobSize : (i+1)*blobSize]
		if i == 3 {
			blob = data[i*blobSize:]
		}
		cid, err := putBlob(bs, blob, time.Second)
		require.NoError(t, err)
		cids = append(cids, cid)
	}
	buf := &bytes.Buffer{}
	require.NoError(t, eds.serializer.Serialize(buf, cids))
	test(buf.Bytes())

	// multiple blobs of serialized execution data with one oversized
	_, data = executionData(t, eds.serializer, defaultMaxBlobSize*5)
	cids = nil
	for i := 0; i < 5; i++ {
		blob := data[i*defaultMaxBlobSize : (i+1)*defaultMaxBlobSize]
		if i == 4 {
			blob = data[i*defaultMaxBlobSize:]
		}
		cid, err := putBlob(bs, blob, time.Second)
		require.NoError(t, err)
		cids = append(cids, cid)
	}
	buf = &bytes.Buffer{}
	require.NoError(t, eds.serializer.Serialize(buf, cids))
	test(buf.Bytes())
}

func TestGetContextCanceled(t *testing.T) {
	t.Parallel()

	bs := testBlobstore()
	eds := executionDataService(mockBlobService(bs))

	ed, _ := executionData(t, eds.serializer, 10*defaultMaxBlobSize)
	rootCid, err := addExecutionData(eds, ed, time.Second)
	require.NoError(t, err)

	cids := allKeys(t, bs, time.Second)
	t.Logf("%d blobs in blob tree", len(cids))

	require.NoError(t, deleteBlob(bs, cids[rand.Intn(len(cids))], time.Second))

	_, err = getExecutionData(eds, rootCid, time.Second)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestAddContextCanceled(t *testing.T) {
	t.Parallel()

	bs := testBlobstore()
	bex := mockBlobService(bs).(*mocknetwork.BlobService)
	eds := executionDataService(bex)

	ed, _ := executionData(t, eds.serializer, 10*defaultMaxBlobSize)
	_, err := addExecutionData(eds, ed, time.Second)
	require.NoError(t, err)

	cids := allKeys(t, bs, time.Second)
	t.Logf("%d blobs in blob tree", len(cids))

	blockingCid := cids[rand.Intn(len(cids))]

	bex.ExpectedCalls = bex.ExpectedCalls[:len(bex.ExpectedCalls)-1]
	bex.On("AddBlobs", mock.Anything, mock.AnythingOfType("[]blocks.Block")).
		Return(func(ctx context.Context, blobs []blobs.Blob) error {
			for _, b := range blobs {
				if b.Cid() == blockingCid {
					<-ctx.Done()
					return ctx.Err()
				}
			}

			return bs.PutMany(ctx, blobs)
		})

	_, err = addExecutionData(eds, ed, time.Second)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestGetIncompleteData(t *testing.T) {
	t.Parallel()

	bs := testBlobstore()
	bex := mockBlobService(bs).(*mocknetwork.BlobService)
	eds := executionDataService(bex)

	ed, _ := executionData(t, eds.serializer, 10*defaultMaxBlobSize)
	rootCid, err := addExecutionData(eds, ed, time.Second)
	require.NoError(t, err)

	cids := allKeys(t, bs, time.Second)
	t.Logf("%d blobs in blob tree", len(cids))

	missingCid := cids[rand.Intn(len(cids))]

	bex.ExpectedCalls = bex.ExpectedCalls[1:]
	bex.On("GetBlobs", mock.Anything, mock.AnythingOfType("[]cid.Cid")).
		Return(func(ctx context.Context, cids []cid.Cid) <-chan blobs.Blob {
			ch := make(chan blobs.Blob)

			go func() {
				defer close(ch)

				for _, c := range cids {
					if c == missingCid {
						continue
					}

					if blob, err := bs.Get(ctx, c); err == nil {
						ch <- blob
					}
				}
			}()

			return ch
		})

	_, err = getExecutionData(eds, rootCid, time.Second)
	var blobNotFoundError *BlobNotFoundError
	assert.ErrorAs(t, err, &blobNotFoundError)
}

func createBlobService(ctx irrecoverable.SignalerContext, t *testing.T, ds datastore.Batching, name string, dhtOpts ...dht.Option) (network.BlobService, host.Host) {
	h, err := libp2p.New()
	require.NoError(t, err)

	cr, err := dht.New(ctx, h, dhtOpts...)
	require.NoError(t, err)

	service := p2p.NewBlobService(h, cr, name, ds)
	service.Start(ctx)
	<-service.Ready()

	return service, h
}

func closeHost(t *testing.T, h host.Host) {
	if err := h.Close(); err != nil {
		require.FailNow(t, "failed to close host", err.Error())
	}
}

func TestWithNetwork(t *testing.T) {
	t.Parallel()

	parent, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx, errChan := irrecoverable.WithSignaler(parent)

	service1, h1 := createBlobService(ctx, t, dssync.MutexWrap(datastore.NewMapDatastore()), "test-create-store-request")
	defer closeHost(t, h1)
	service2, h2 := createBlobService(ctx, t, dssync.MutexWrap(datastore.NewMapDatastore()), "test-create-store-request")
	defer closeHost(t, h2)

	done := make(chan struct{})

	go func() {
		defer close(done)

		select {
		case err := <-errChan:
			t.Errorf("unexpected error: %v", err)
		case <-util.AllDone(service1, service2):
			select {
			case err := <-errChan:
				t.Errorf("unexpected error: %v", err)
			default:
			}
		}
	}()

	defer func() {
		cancel()
		<-done
	}()

	require.NoError(t, h1.Connect(ctx, *host.InfoFromHost(h2)))

	eds1 := executionDataService(service1)
	eds2 := executionDataService(service2)

	expected, _ := executionData(t, eds1.serializer, 10*defaultMaxBlobSize)
	rootCid, err := addExecutionData(eds1, expected, time.Second)
	require.NoError(t, err)

	actual, err := getExecutionData(eds2, rootCid, time.Second)
	require.NoError(t, err)

	assert.Equal(t, true, reflect.DeepEqual(expected, actual))
}

func TestReprovider(t *testing.T) {
	t.Parallel()

	parent, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx, errChan := irrecoverable.WithSignaler(parent)

	ds := dssync.MutexWrap(datastore.NewMapDatastore())
	mockBs := mockBlobService(blockstore.NewBlockstore(ds))
	mockEds := executionDataService(mockBs)
	expected, _ := executionData(t, mockEds.serializer, 10*defaultMaxBlobSize)
	rootCid, err := addExecutionData(mockEds, expected, time.Second)
	require.NoError(t, err)

	h1, err := libp2p.New()
	defer closeHost(t, h1)
	require.NoError(t, err)
	cr1, err := dht.New(ctx, h1, dht.Mode(dht.ModeServer))
	require.NoError(t, err)

	dhtOpts := []dht.Option{
		dht.RoutingTableFilter(func(dht interface{}, p peer.ID) bool {
			return p == h1.ID()
		}),
		dht.Mode(dht.ModeServer),
		dht.DisableAutoRefresh(),
	}

	service2, h2 := createBlobService(ctx, t, ds, "test-reprovider", dhtOpts...)
	defer closeHost(t, h2)
	service3, h3 := createBlobService(ctx, t, dssync.MutexWrap(datastore.NewMapDatastore()), "test-reprovider", dhtOpts...)
	defer closeHost(t, h3)

	require.NoError(t, h2.Connect(ctx, *host.InfoFromHost(h1)))
	require.NoError(t, h3.Connect(ctx, *host.InfoFromHost(h1)))

	done := make(chan struct{})

	go func() {
		defer close(done)

		select {
		case err := <-errChan:
			t.Errorf("unexpected error: %v", err)
		case <-util.AllDone(service2, service3):
			select {
			case err := <-errChan:
				t.Errorf("unexpected error: %v", err)
			default:
			}
		}
	}()

	defer func() {
		cancel()
		assert.NoError(t, cr1.Close())
		<-done
	}()

	reprovideCtx, reprovideCancel := context.WithTimeout(ctx, 5*time.Second)
	defer reprovideCancel()
	require.NoError(t, service2.TriggerReprovide(reprovideCtx))

	eds := executionDataService(service3)

	actual, err := getExecutionData(eds, rootCid, time.Second)
	require.NoError(t, err)

	assert.Equal(t, true, reflect.DeepEqual(expected, actual))
}
