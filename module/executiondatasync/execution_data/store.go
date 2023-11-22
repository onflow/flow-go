package execution_data

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/ipfs/go-cid"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/blobs"
)

// ExecutionDataGetter handles getting execution data from a blobstore
type ExecutionDataGetter interface {
	// Get gets the BlockExecutionData for the given root ID from the blobstore.
	// Expected errors during normal operations:
	// - BlobNotFoundError if some CID in the blob tree could not be found from the blobstore
	// - MalformedDataError if some level of the blob tree cannot be properly deserialized
	// - BlobSizeLimitExceededError if some blob in the blob tree exceeds the maximum allowed size
	Get(ctx context.Context, rootID flow.Identifier) (*BlockExecutionData, error)
}

// ExecutionDataStore handles adding / getting execution data to / from a blobstore
type ExecutionDataStore interface {
	ExecutionDataGetter

	// Add constructs a blob tree for the given BlockExecutionData, adds it to the blobstore,
	// then returns the root CID.
	// No errors are expected during normal operation.
	Add(ctx context.Context, executionData *BlockExecutionData) (flow.Identifier, error)
}

type ExecutionDataStoreOption func(*store)

// WithMaxBlobSize configures the maximum blob size of the store
func WithMaxBlobSize(size int) ExecutionDataStoreOption {
	return func(s *store) {
		s.maxBlobSize = size
	}
}

var _ ExecutionDataStore = (*store)(nil)

type store struct {
	blobstore   blobs.Blobstore
	serializer  Serializer
	maxBlobSize int
}

// NewExecutionDataStore creates a new Execution Data Store.
func NewExecutionDataStore(blobstore blobs.Blobstore, serializer Serializer, opts ...ExecutionDataStoreOption) *store {
	s := &store{
		blobstore:   blobstore,
		serializer:  serializer,
		maxBlobSize: DefaultMaxBlobSize,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Add constructs a blob tree for the given BlockExecutionData, adds it to the blobstore,
// then returns the rootID.
// No errors are expected during normal operation.
func (s *store) Add(ctx context.Context, executionData *BlockExecutionData) (flow.Identifier, error) {
	executionDataRoot := &flow.BlockExecutionDataRoot{
		BlockID:               executionData.BlockID,
		ChunkExecutionDataIDs: make([]cid.Cid, len(executionData.ChunkExecutionDatas)),
	}

	for i, chunkExecutionData := range executionData.ChunkExecutionDatas {
		chunkExecutionDataID, err := s.addChunkExecutionData(ctx, chunkExecutionData)
		if err != nil {
			return flow.ZeroID, fmt.Errorf("could not add chunk execution data at index %d: %w", i, err)
		}

		executionDataRoot.ChunkExecutionDataIDs[i] = chunkExecutionDataID
	}

	buf := new(bytes.Buffer)
	if err := s.serializer.Serialize(buf, executionDataRoot); err != nil {
		return flow.ZeroID, fmt.Errorf("could not serialize execution data root: %w", err)
	}

	// this should never happen unless either:
	// - maxBlobSize is set too low
	// - an enormous number of chunks are included in the block
	//   e.g. given a 1MB max size, 32 byte CID and 32 byte blockID:
	//   1MB/32byes - 1 = 32767 chunk CIDs
	// if the number of chunks in a block ever exceeds this, we will need to update the root blob
	// generation to support splitting it up into a tree similar to addChunkExecutionData
	if buf.Len() > s.maxBlobSize {
		return flow.ZeroID, errors.New("root blob exceeds blob size limit")
	}

	rootBlob := blobs.NewBlob(buf.Bytes())
	if err := s.blobstore.Put(ctx, rootBlob); err != nil {
		return flow.ZeroID, fmt.Errorf("could not add execution data root: %w", err)
	}

	rootID, err := flow.CidToId(rootBlob.Cid())
	if err != nil {
		return flow.ZeroID, fmt.Errorf("could not get root ID: %w", err)
	}

	return rootID, nil
}

// addChunkExecutionData constructs a blob tree for the given ChunkExecutionData, adds it to the
// blobstore, and returns the root CID.
// No errors are expected during normal operation.
func (s *store) addChunkExecutionData(ctx context.Context, chunkExecutionData *ChunkExecutionData) (cid.Cid, error) {
	var v interface{} = chunkExecutionData

	// given an arbitrarily large v, split it into blobs of size up to maxBlobSize, adding them to
	// the blobstore. Then, combine the list of CIDs added into a second level of blobs, and repeat.
	// This produces a tree of blobs, where the leaves are the actual data, and each internal node
	// contains a list of CIDs for its children.
	for i := 0; ; i++ {
		// chunk and store the data, then get the list of CIDs added
		cids, err := s.addBlobs(ctx, v)
		if err != nil {
			return cid.Undef, fmt.Errorf("failed to add blob tree level at height %d: %w", i, err)
		}

		// once a single CID is left, we have reached the root of the tree
		if len(cids) == 1 {
			return cids[0], nil
		}

		// the next level is the list of CIDs added in this level
		v = cids
	}
}

// addBlobs splits the given value into blobs of size up to maxBlobSize, adds them to the blobstore,
// then returns the CIDs for each blob added.
// No errors are expected during normal operation.
func (s *store) addBlobs(ctx context.Context, v interface{}) ([]cid.Cid, error) {
	// first, serialize the data into a large byte slice
	buf := new(bytes.Buffer)
	if err := s.serializer.Serialize(buf, v); err != nil {
		return nil, fmt.Errorf("could not serialize execution data root: %w", err)
	}

	data := buf.Bytes()
	var cids []cid.Cid
	var blbs []blobs.Blob

	// next, chunk the data into blobs of size up to maxBlobSize
	for len(data) > 0 {
		blobLen := s.maxBlobSize
		if len(data) < blobLen {
			blobLen = len(data)
		}

		blob := blobs.NewBlob(data[:blobLen])
		data = data[blobLen:]
		blbs = append(blbs, blob)
		cids = append(cids, blob.Cid())
	}

	// finally, add the blobs to the blobstore and return the list of CIDs
	if err := s.blobstore.PutMany(ctx, blbs); err != nil {
		return nil, fmt.Errorf("could not add blobs: %w", err)
	}

	return cids, nil
}

// Get gets the BlockExecutionData for the given root ID from the blobstore.
// Expected errors during normal operations:
// - BlobNotFoundError if some CID in the blob tree could not be found from the blobstore
// - MalformedDataError if some level of the blob tree cannot be properly deserialized
func (s *store) Get(ctx context.Context, rootID flow.Identifier) (*BlockExecutionData, error) {
	rootCid := flow.IdToCid(rootID)

	// first, get the root blob. it will contain a list of blobs, one for each chunk
	rootBlob, err := s.blobstore.Get(ctx, rootCid)
	if err != nil {
		if errors.Is(err, blobs.ErrNotFound) {
			return nil, NewBlobNotFoundError(rootCid)
		}

		return nil, fmt.Errorf("failed to get root blob: %w", err)
	}

	rootData, err := s.serializer.Deserialize(bytes.NewBuffer(rootBlob.RawData()))
	if err != nil {
		return nil, NewMalformedDataError(err)
	}

	executionDataRoot, ok := rootData.(*flow.BlockExecutionDataRoot)
	if !ok {
		return nil, NewMalformedDataError(fmt.Errorf("root blob does not deserialize to a BlockExecutionDataRoot, got %T instead", rootData))
	}

	// next, get each chunk blob and deserialize it
	blockExecutionData := &BlockExecutionData{
		BlockID:             executionDataRoot.BlockID,
		ChunkExecutionDatas: make([]*ChunkExecutionData, len(executionDataRoot.ChunkExecutionDataIDs)),
	}

	for i, chunkExecutionDataID := range executionDataRoot.ChunkExecutionDataIDs {
		chunkExecutionData, err := s.getChunkExecutionData(ctx, chunkExecutionDataID)
		if err != nil {
			return nil, fmt.Errorf("could not get chunk execution data at index %d: %w", i, err)
		}

		blockExecutionData.ChunkExecutionDatas[i] = chunkExecutionData
	}

	return blockExecutionData, nil
}

// getChunkExecutionData gets the ChunkExecutionData for the given CID from the blobstore.
// Expected errors during normal operations:
// - BlobNotFoundError if some CID in the blob tree could not be found from the blobstore
// - MalformedDataError if some level of the blob tree cannot be properly deserialized
func (s *store) getChunkExecutionData(ctx context.Context, chunkExecutionDataID cid.Cid) (*ChunkExecutionData, error) {
	cids := []cid.Cid{chunkExecutionDataID}

	// given a root CID, get the blob tree level by level, until we reach the full ChunkExecutionData
	for i := 0; ; i++ {
		v, err := s.getBlobs(ctx, cids)
		if err != nil {
			return nil, fmt.Errorf("failed to get blob tree level at depth %d: %w", i, err)
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

// getBlobs gets the blobs for the given CIDs from the blobstore, deserializes them, and returns
// the deserialized value.
// - BlobNotFoundError if any of the CIDs could not be found from the blobstore
// - MalformedDataError if any of the blobs cannot be properly deserialized
func (s *store) getBlobs(ctx context.Context, cids []cid.Cid) (interface{}, error) {
	buf := new(bytes.Buffer)

	// get each blob and append the raw data to the buffer
	for _, cid := range cids {
		blob, err := s.blobstore.Get(ctx, cid)
		if err != nil {
			if errors.Is(err, blobs.ErrNotFound) {
				return nil, NewBlobNotFoundError(cid)
			}

			return nil, fmt.Errorf("failed to get blob: %w", err)
		}

		_, err = buf.Write(blob.RawData())
		if err != nil {
			return nil, fmt.Errorf("failed to write blob %s to deserialization buffer: %w", cid.String(), err)
		}
	}

	// deserialize the buffer into a value, and return it
	v, err := s.serializer.Deserialize(buf)
	if err != nil {
		return nil, NewMalformedDataError(err)
	}

	return v, nil
}
