package execution_data

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/ipfs/go-cid"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/network"
)

// Downloader is used to download execution data blobs from the network via a blob service.
type Downloader interface {
	module.ReadyDoneAware
	ExecutionDataGetter
}

var _ Downloader = (*downloader)(nil)

type downloader struct {
	blobService network.BlobService
	maxBlobSize int
	serializer  Serializer
}

type DownloaderOption func(*downloader)

// WithSerializer configures the serializer for the downloader
func WithSerializer(serializer Serializer) DownloaderOption {
	return func(d *downloader) {
		d.serializer = serializer
	}
}

// NewDownloader creates a new Downloader instance
func NewDownloader(blobService network.BlobService, opts ...DownloaderOption) *downloader {
	d := &downloader{
		blobService,
		DefaultMaxBlobSize,
		DefaultSerializer,
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}

// Ready returns a channel that will be closed when the downloader is ready to be used
func (d *downloader) Ready() <-chan struct{} {
	return d.blobService.Ready()
}

// Done returns a channel that will be closed when the downloader is finished shutting down
func (d *downloader) Done() <-chan struct{} {
	return d.blobService.Done()
}

// Get downloads a blob tree identified by executionDataID from the network and returns the deserialized BlockExecutionData struct
//
// Expected errors during normal operations:
// - BlobNotFoundError if some CID in the blob tree could not be found from the blob service
// - MalformedDataError if some level of the blob tree cannot be properly deserialized
// - BlobSizeLimitExceededError if some blob in the blob tree exceeds the maximum allowed size
func (d *downloader) Get(ctx context.Context, executionDataID flow.Identifier) (*BlockExecutionData, error) {
	blobGetter := d.blobService.GetSession(ctx)

	// First, download the root execution data record which contains a list of chunk execution data
	// blobs included in the original record.
	edRoot, err := d.getExecutionDataRoot(ctx, executionDataID, blobGetter)
	if err != nil {
		return nil, fmt.Errorf("failed to get execution data root: %w", err)
	}

	g, gCtx := errgroup.WithContext(ctx)

	// Next, download each of the chunk execution data blobs
	chunkExecutionDatas := make([]*ChunkExecutionData, len(edRoot.ChunkExecutionDataIDs))
	for i, chunkDataID := range edRoot.ChunkExecutionDataIDs {
		i := i
		chunkDataID := chunkDataID

		g.Go(func() error {
			ced, err := d.getChunkExecutionData(
				gCtx,
				chunkDataID,
				blobGetter,
			)

			if err != nil {
				return fmt.Errorf("failed to get chunk execution data at index %d: %w", i, err)
			}

			chunkExecutionDatas[i] = ced

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Finally, recombine data into original record.
	bed := &BlockExecutionData{
		BlockID:             edRoot.BlockID,
		ChunkExecutionDatas: chunkExecutionDatas,
	}

	return bed, nil
}

// getExecutionDataRoot downloads the root execution data record from the network and returns the
// deserialized flow.BlockExecutionDataRoot struct.
//
// Expected errors during normal operations:
// - BlobNotFoundError if the root blob could not be found from the blob service
// - MalformedDataError if the root blob cannot be properly deserialized
// - BlobSizeLimitExceededError if the root blob exceeds the maximum allowed size
func (d *downloader) getExecutionDataRoot(
	ctx context.Context,
	rootID flow.Identifier,
	blobGetter network.BlobGetter,
) (*flow.BlockExecutionDataRoot, error) {
	rootCid := flow.IdToCid(rootID)

	blob, err := blobGetter.GetBlob(ctx, rootCid)
	if err != nil {
		if errors.Is(err, network.ErrBlobNotFound) {
			return nil, NewBlobNotFoundError(rootCid)
		}

		return nil, fmt.Errorf("failed to get root blob: %w", err)
	}

	blobSize := len(blob.RawData())

	if blobSize > d.maxBlobSize {
		return nil, &BlobSizeLimitExceededError{blob.Cid()}
	}

	v, err := d.serializer.Deserialize(bytes.NewBuffer(blob.RawData()))
	if err != nil {
		return nil, NewMalformedDataError(err)
	}

	edRoot, ok := v.(*flow.BlockExecutionDataRoot)
	if !ok {
		return nil, NewMalformedDataError(fmt.Errorf("execution data root blob does not deserialize to a BlockExecutionDataRoot, got %T instead", v))
	}

	return edRoot, nil
}

// getChunkExecutionData downloads a chunk execution data blob from the network and returns the
// deserialized ChunkExecutionData struct.
//
// Expected errors during normal operations:
// - context.Canceled or context.DeadlineExceeded if the context is canceled or times out
// - BlobNotFoundError if the root blob could not be found from the blob service
// - MalformedDataError if the root blob cannot be properly deserialized
// - BlobSizeLimitExceededError if the root blob exceeds the maximum allowed size
func (d *downloader) getChunkExecutionData(
	ctx context.Context,
	chunkExecutionDataID cid.Cid,
	blobGetter network.BlobGetter,
) (*ChunkExecutionData, error) {
	cids := []cid.Cid{chunkExecutionDataID}

	// iteratively process each level of the blob tree until a ChunkExecutionData is returned or an
	// error is encountered
	for i := 0; ; i++ {
		v, err := d.getBlobs(ctx, blobGetter, cids)
		if err != nil {
			return nil, fmt.Errorf("failed to get level %d of blob tree: %w", i, err)
		}

		switch v := v.(type) {
		case *ChunkExecutionData:
			return v, nil
		case *[]cid.Cid:
			cids = *v
		default:
			return nil, NewMalformedDataError(fmt.Errorf("blob tree contains unexpected type %T at level %d", v, i))
		}
	}
}

// getBlobs gets the given CIDs from the blobservice, reassembles the blobs, and deserializes the reassembled data into an object.
//
// Expected errors during normal operations:
// - context.Canceled or context.DeadlineExceeded if the context is canceled or times out
// - BlobNotFoundError if the root blob could not be found from the blob service
// - MalformedDataError if the root blob cannot be properly deserialized
// - BlobSizeLimitExceededError if the root blob exceeds the maximum allowed size
func (d *downloader) getBlobs(ctx context.Context, blobGetter network.BlobGetter, cids []cid.Cid) (interface{}, error) {
	// this uses an optimization to deserialize the data in a streaming fashion as it is received
	// from the network, reducing the amount of memory required to deserialize large objects.
	blobCh, errCh := d.retrieveBlobs(ctx, blobGetter, cids)
	bcr := blobs.NewBlobChannelReader(blobCh)

	v, deserializeErr := d.serializer.Deserialize(bcr)

	// blocks until all blobs have been retrieved or an error is encountered
	err := <-errCh

	if err != nil {
		return nil, err
	}

	if deserializeErr != nil {
		return nil, NewMalformedDataError(deserializeErr)
	}

	return v, nil
}

// retrieveBlobs asynchronously retrieves the blobs for the given CIDs with the given BlobGetter.
// Blobs corresponding to the requested CIDs are returned in order on the response channel.
//
// Expected errors during normal operations:
// - context.Canceled or context.DeadlineExceeded if the context is canceled or times out
// - BlobNotFoundError if the root blob could not be found from the blob service
// - MalformedDataError if the root blob cannot be properly deserialized
// - BlobSizeLimitExceededError if the root blob exceeds the maximum allowed size
func (d *downloader) retrieveBlobs(parent context.Context, blobGetter network.BlobGetter, cids []cid.Cid) (<-chan blobs.Blob, <-chan error) {
	blobsOut := make(chan blobs.Blob, len(cids))
	errCh := make(chan error, 1)

	go func() {
		var err error

		ctx, cancel := context.WithCancel(parent)
		defer cancel()
		defer close(blobsOut)
		defer func() {
			errCh <- err
			close(errCh)
		}()

		blobChan := blobGetter.GetBlobs(ctx, cids) // initiate a batch request for the given CIDs
		cachedBlobs := make(map[cid.Cid]blobs.Blob)
		cidCounts := make(map[cid.Cid]int) // used to account for duplicate CIDs

		// record the number of times each CID appears in the list. this is later used to determine
		// when it's safe to delete cached blobs during processing
		for _, c := range cids {
			cidCounts[c]++
		}

		// for each cid, find the corresponding blob from the incoming blob channel and send it to
		// the outgoing blob channel in the proper order
		for _, c := range cids {
			blob, ok := cachedBlobs[c]

			if !ok {
				if blob, err = d.findBlob(blobChan, c, cachedBlobs); err != nil {
					// the blob channel may be closed as a result of the context being canceled,
					// in which case we should return the context error.
					if ctxErr := ctx.Err(); ctxErr != nil {
						err = ctxErr
					}

					return
				}
			}

			// remove the blob from the cache if it's no longer needed
			cidCounts[c]--

			if cidCounts[c] == 0 {
				delete(cachedBlobs, c)
				delete(cidCounts, c)
			}

			blobsOut <- blob
		}
	}()

	return blobsOut, errCh
}

// findBlob retrieves blobs from the given channel, caching them along the way, until it either
// finds the target blob or exhausts the channel.
//
// This is necessary to ensure blobs can be reassembled in order from the underlying blobservice
// which provides no guarantees for blob order on the response channel.
//
// Expected errors during normal operations:
// - BlobNotFoundError if the root blob could not be found from the blob service
// - BlobSizeLimitExceededError if the root blob exceeds the maximum allowed size
func (d *downloader) findBlob(
	blobChan <-chan blobs.Blob,
	target cid.Cid,
	cache map[cid.Cid]blobs.Blob,
) (blobs.Blob, error) {
	// pull blobs off the blob channel until the target blob is found or the channel is closed
	// Note: blobs are returned on the blob channel as they are found, in no particular order
	for blob := range blobChan {
		// check blob size
		blobSize := len(blob.RawData())

		if blobSize > d.maxBlobSize {
			return nil, &BlobSizeLimitExceededError{blob.Cid()}
		}

		cache[blob.Cid()] = blob

		if blob.Cid() == target {
			return blob, nil
		}
	}

	return nil, NewBlobNotFoundError(target)
}
