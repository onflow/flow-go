package state_synchronization

import (
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/onflow/flow-go/model/flow"
)

// MalformedDataError is returned when malformed data is found at some level of the requested
// blob tree. It likely indicates that the tree was generated incorrectly, and hence the request
// should not be retried.
type MalformedDataError struct {
	err error
}

func (e *MalformedDataError) Error() string {
	return fmt.Sprintf("malformed data: %v", e.err)
}

func (e *MalformedDataError) Unwrap() error { return e.err }

// BlobSizeLimitExceededError is returned when a blob exceeds the maximum size allowed.
type BlobSizeLimitExceededError struct {
	cid cid.Cid
}

func (e *BlobSizeLimitExceededError) Error() string {
	return fmt.Sprintf("blob %v exceeds maximum blob size", e.cid.String())
}

// BlobNotFoundError is returned when the blobservice failed to find a blob.
type BlobNotFoundError struct {
	cid cid.Cid
}

func (e *BlobNotFoundError) Error() string {
	return fmt.Sprintf("blob %v not found", e.cid.String())
}

type MismatchedBlockIDError struct {
	expected flow.Identifier
	actual   flow.Identifier
}

func (e *MismatchedBlockIDError) Error() string {
	return fmt.Sprintf("execution data root has mismatched block ID:\n"+
		"expected: %q\n"+
		"actual  : %q", e.expected, e.actual)
}
