package execution_data

import (
	"bytes"
	"context"
	"fmt"

	"github.com/ipfs/go-cid"

	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/network"
)

// ExecutionDataGetter handles getting execution data from a blobstore
type ExecutionDataGetter interface {
	// GetExecutionData gets the BlockExecutionData for the given root ID from the blobstore.
	// The returned error will be:
	// - MalformedDataError if some level of the blob tree cannot be properly deserialized
	// - BlobNotFoundError if some CID in the blob tree could not be found from the blobstore
	GetExecutionData(ctx context.Context, rootID flow.Identifier) (*BlockExecutionData, error)
}

type getter struct {
	blobstore  blobs.Blobstore
	serializer *Serializer
}

func NewExecutionDataGetter(blobstore blobs.Blobstore, codec encoding.Codec, compressor network.Compressor) *getter {
	return &getter{blobstore, NewSerializer(codec, compressor)}
}

func (g *getter) GetExecutionData(ctx context.Context, rootID flow.Identifier) (*BlockExecutionData, error) {
	rootCid := flow.IdToCid(rootID)

	rootBlob, err := g.blobstore.Get(ctx, rootCid)
	if err != nil {
		// TODO: technically, we should check if the error is ErrNotFound
		// otherwise don't wrap it with BlobNotFoundError
		return nil, NewBlobNotFoundError(rootCid)
	}

	rootData, err := g.serializer.Deserialize(bytes.NewBuffer(rootBlob.RawData()))
	if err != nil {
		return nil, NewMalformedDataError(err)
	}

	executionDataRoot, ok := rootData.(*BlockExecutionDataRoot)
	if !ok {
		return nil, NewMalformedDataError(fmt.Errorf("root blob does not deserialize to a BlockExecutionDataRoot"))
	}

	blockExecutionData := &BlockExecutionData{
		BlockID:             executionDataRoot.BlockID,
		ChunkExecutionDatas: make([]*ChunkExecutionData, len(executionDataRoot.ChunkExecutionDataIDs)),
	}

	for i, chunkExecutionDataID := range executionDataRoot.ChunkExecutionDataIDs {
		chunkExecutionData, err := g.getChunkExecutionData(ctx, chunkExecutionDataID)
		if err != nil {
			return nil, fmt.Errorf("could not get chunk execution data at index %d: %w", i, err)
		}

		blockExecutionData.ChunkExecutionDatas[i] = chunkExecutionData
	}

	return blockExecutionData, nil
}

func (g *getter) getChunkExecutionData(ctx context.Context, chunkExecutionDataID cid.Cid) (*ChunkExecutionData, error) {
	cids := []cid.Cid{chunkExecutionDataID}

	for i := 0; ; i++ {
		v, err := g.getBlobs(ctx, cids)
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

func (g *getter) getBlobs(ctx context.Context, cids []cid.Cid) (interface{}, error) {
	buf := new(bytes.Buffer)

	for _, cid := range cids {
		blob, err := g.blobstore.Get(ctx, cid)
		if err != nil {
			// TODO: technically, we should check if the error is ErrNotFound
			// otherwise don't wrap it with BlobNotFoundError
			return nil, NewBlobNotFoundError(cid)
		}

		_, err = buf.Write(blob.RawData())
		if err != nil {
			return nil, fmt.Errorf("failed to write blob %s to deserialization buffer: %w", cid.String(), err)
		}
	}

	v, err := g.serializer.Deserialize(buf)
	if err != nil {
		return nil, NewMalformedDataError(err)
	}

	return v, nil
}

// MalformedDataError is returned when malformed data is found at some level of the requested
// blob tree. It likely indicates that the tree was generated incorrectly, and hence the request
// should not be retried.
type MalformedDataError struct {
	err error
}

func NewMalformedDataError(err error) *MalformedDataError {
	return &MalformedDataError{err: err}
}

func (e *MalformedDataError) Error() string {
	return fmt.Sprintf("malformed data: %v", e.err)
}

func (e *MalformedDataError) Unwrap() error { return e.err }

// BlobNotFoundError is returned when a blob could not be found.
type BlobNotFoundError struct {
	cid cid.Cid
}

func NewBlobNotFoundError(cid cid.Cid) *BlobNotFoundError {
	return &BlobNotFoundError{cid: cid}
}

func (e *BlobNotFoundError) Error() string {
	return fmt.Sprintf("blob %v not found", e.cid.String())
}
