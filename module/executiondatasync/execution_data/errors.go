package execution_data

import (
	"errors"
	"fmt"

	"github.com/ipfs/go-cid"
)

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

// IsMalformedDataError returns whether an error is MalformedDataError
func IsMalformedDataError(err error) bool {
	var malformedDataErr *MalformedDataError
	return errors.As(err, &malformedDataErr)
}

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

// IsBlobNotFoundError returns whether an error is BlobNotFoundError
func IsBlobNotFoundError(err error) bool {
	var blobNotFoundError *BlobNotFoundError
	return errors.As(err, &blobNotFoundError)
}

// BlobSizeLimitExceededError is returned when a blob exceeds the maximum size allowed.
type BlobSizeLimitExceededError struct {
	cid cid.Cid
}

func (e *BlobSizeLimitExceededError) Error() string {
	return fmt.Sprintf("blob %v exceeds maximum blob size", e.cid.String())
}

// IsBlobSizeLimitExceededError returns whether an error is BlobSizeLimitExceededError
func IsBlobSizeLimitExceededError(err error) bool {
	var blobSizeLimitExceededError *BlobSizeLimitExceededError
	return errors.As(err, &blobSizeLimitExceededError)
}
