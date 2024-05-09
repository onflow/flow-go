package execution_data_test

import (
	"context"
	"math/rand"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/network/mocknetwork"
)

func TestCIDNotFound(t *testing.T) {
	blobstore := blobs.NewBlobstore(dssync.MutexWrap(datastore.NewMapDatastore()))
	blobService := new(mocknetwork.BlobService)
	downloader := execution_data.NewDownloader(blobService)
	edStore := execution_data.NewExecutionDataStore(blobstore, execution_data.DefaultSerializer)
	bed := generateBlockExecutionData(t, 10, 3*execution_data.DefaultMaxBlobSize)
	edID, err := edStore.Add(context.Background(), bed)
	require.NoError(t, err)

	blobGetter := new(mocknetwork.BlobGetter)
	blobService.On("GetSession", mock.Anything).Return(blobGetter, nil)
	blobGetter.On("GetBlob", mock.Anything, mock.AnythingOfType("cid.Cid")).Return(
		func(ctx context.Context, c cid.Cid) blobs.Blob {
			blob, _ := blobstore.Get(ctx, c)
			return blob
		},
		func(ctx context.Context, c cid.Cid) error {
			_, err := blobstore.Get(ctx, c)
			return err
		},
	)
	blobGetter.On("GetBlobs", mock.Anything, mock.AnythingOfType("[]cid.Cid")).Return(
		func(ctx context.Context, cids []cid.Cid) <-chan blobs.Blob {
			blobCh := make(chan blobs.Blob, len(cids))
			missingIdx := rand.Intn(len(cids))
			for i, c := range cids {
				if i != missingIdx {
					blob, err := blobstore.Get(ctx, c)
					assert.NoError(t, err)
					blobCh <- blob
				}
			}
			close(blobCh)
			return blobCh
		},
	)

	_, err = downloader.Get(context.Background(), edID)
	var blobNotFoundError *execution_data.BlobNotFoundError
	assert.ErrorAs(t, err, &blobNotFoundError)
}
