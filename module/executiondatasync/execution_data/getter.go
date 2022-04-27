package execution_data

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/onflow/flow-go/model/flow"
)

// ExecutionDataGetter handles getting execution data from a blobstore
type ExecutionDataGetter interface {
	// GetExecutionData gets the BlockExecutionData for the given root ID from the blobstore.
	// The returned error will be:
	// - MalformedDataError if some level of the blob tree cannot be properly deserialized
	// - BlobNotFoundError if some CID in the blob tree could not be found from the blobstore
	GetExecutionData(ctx context.Context, rootID flow.Identifier) (*BlockExecutionData, error)
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
