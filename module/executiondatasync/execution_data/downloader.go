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

// BlobSizeLimitExceededError is returned when a blob exceeds the maximum size allowed.
type BlobSizeLimitExceededError struct {
	cid cid.Cid
}

func (e *BlobSizeLimitExceededError) Error() string {
	return fmt.Sprintf("blob %v exceeds maximum blob size", e.cid.String())
}

// Downloader is used to download execution data blobs from the network via a blob service.
type Downloader interface {
	module.ReadyDoneAware

	// Download downloads and returns a Block Execution Data from the network.
	// The returned error will be:
	// - MalformedDataError if some level of the blob tree cannot be properly deserialized
	// - BlobNotFoundError if some CID in the blob tree could not be found from the blob service
	// - BlobSizeLimitExceededError if some blob in the blob tree exceeds the maximum allowed size
	Download(ctx context.Context, executionDataID flow.Identifier) (*BlockExecutionData, error)
}

type downloader struct {
	blobService network.BlobService
	maxBlobSize int
	serializer  Serializer
}

type DownloaderOption func(*downloader)

func WithSerializer(serializer Serializer) DownloaderOption {
	return func(d *downloader) {
		d.serializer = serializer
	}
}

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

func (d *downloader) Ready() <-chan struct{} {
	return d.blobService.Ready()
}
func (d *downloader) Done() <-chan struct{} {
	return d.blobService.Done()
}

// Download downloads a blob tree identified by executionDataID from the network and returns the deserialized BlockExecutionData struct
// During normal operation, the returned error will be:
// - MalformedDataError if some level of the blob tree cannot be properly deserialized
// - BlobNotFoundError if some CID in the blob tree could not be found from the blob service
// - BlobSizeLimitExceededError if some blob in the blob tree exceeds the maximum allowed size
func (d *downloader) Download(ctx context.Context, executionDataID flow.Identifier) (*BlockExecutionData, error) {
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

// TODO(state-sync): add documentation+errors
func (d *downloader) getExecutionDataRoot(
	ctx context.Context,
	rootID flow.Identifier,
	blobGetter network.BlobGetter,
) (*BlockExecutionDataRoot, error) {
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

	edRoot, ok := v.(*BlockExecutionDataRoot)
	if !ok {
		return nil, NewMalformedDataError(fmt.Errorf("execution data root blob does not deserialize to a BlockExecutionDataRoot, got %T instead", v))
	}

	return edRoot, nil
}

// TODO(state-sync): add documentation+errors
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
// TODO(state-sync): add errors
func (d *downloader) getBlobs(ctx context.Context, blobGetter network.BlobGetter, cids []cid.Cid) (interface{}, error) {
	blobCh, errCh := d.retrieveBlobs(ctx, blobGetter, cids)
	bcr := blobs.NewBlobChannelReader(blobCh)
	v, deserializeErr := d.serializer.Deserialize(bcr)
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
// TODO(state-sync): add errors
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

		for _, c := range cids {
			cidCounts[c] += 1
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

			cidCounts[c] -= 1

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
// TODO(state-sync): add errors
func (d *downloader) findBlob(
	blobChan <-chan blobs.Blob,
	target cid.Cid,
	cache map[cid.Cid]blobs.Blob,
) (blobs.Blob, error) {
	// Note: blobs are returned as they are found, in no particular order
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
