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

// ExecutionDataStore handles adding / getting execution data to / from a blobstore
type ExecutionDataStore interface {
	// GetExecutionData gets the BlockExecutionData for the given root ID from the blobstore.
	// The returned error will be:
	// - MalformedDataError if some level of the blob tree cannot be properly deserialized
	// - BlobNotFoundError if some CID in the blob tree could not be found from the blobstore
	GetExecutionData(ctx context.Context, rootID flow.Identifier) (*BlockExecutionData, error)

	// AddExecutionData constructs a blob tree for the given BlockExecutionData and adds it to the
	// blobstore, and then returns the root CID.
	AddExecutionData(ctx context.Context, executionData *BlockExecutionData) (flow.Identifier, error)
}

type store struct {
	blobstore   blobs.Blobstore
	serializer  Serializer
	maxBlobSize int
}

func NewExecutionDataStore(blobstore blobs.Blobstore, serializer Serializer) *store {
	return &store{
		blobstore:   blobstore,
		serializer:  serializer,
		maxBlobSize: DefaultMaxBlobSize, // TODO: make this configurable
	}
}

func (s *store) AddExecutionData(ctx context.Context, executionData *BlockExecutionData) (flow.Identifier, error) {
	executionDataRoot := &BlockExecutionDataRoot{
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

func (s *store) addChunkExecutionData(ctx context.Context, chunkExecutionData *ChunkExecutionData) (cid.Cid, error) {
	var v interface{} = chunkExecutionData

	for i := 0; ; i++ {
		cids, err := s.addBlobs(ctx, v)
		if err != nil {
			return cid.Undef, fmt.Errorf("failed to add blob tree level at height %d: %w", i, err)
		}

		if len(cids) == 1 {
			return cids[0], nil
		}

		v = cids
	}
}

func (s *store) addBlobs(ctx context.Context, v interface{}) ([]cid.Cid, error) {
	buf := new(bytes.Buffer)
	if err := s.serializer.Serialize(buf, v); err != nil {
		return nil, fmt.Errorf("could not serialize execution data root: %w", err)
	}

	data := buf.Bytes()
	var cids []cid.Cid
	var blbs []blobs.Blob

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

	if err := s.blobstore.PutMany(ctx, blbs); err != nil {
		return nil, fmt.Errorf("could not add blobs: %w", err)
	}

	return cids, nil
}

func (s *store) GetExecutionData(ctx context.Context, rootID flow.Identifier) (*BlockExecutionData, error) {
	rootCid := flow.IdToCid(rootID)

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

	executionDataRoot, ok := rootData.(*BlockExecutionDataRoot)
	if !ok {
		return nil, NewMalformedDataError(fmt.Errorf("root blob does not deserialize to a BlockExecutionDataRoot"))
	}

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

func (s *store) getChunkExecutionData(ctx context.Context, chunkExecutionDataID cid.Cid) (*ChunkExecutionData, error) {
	cids := []cid.Cid{chunkExecutionDataID}

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

func (s *store) getBlobs(ctx context.Context, cids []cid.Cid) (interface{}, error) {
	buf := new(bytes.Buffer)

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

	v, err := s.serializer.Deserialize(buf)
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
