package common

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// InsufficientExecutionReceipts indicates that no execution receipts were found for a given block ID
type InsufficientExecutionReceipts struct {
	blockID      flow.Identifier
	receiptCount int
}

func NewInsufficientExecutionReceipts(blockID flow.Identifier, receiptCount int) InsufficientExecutionReceipts {
	return InsufficientExecutionReceipts{blockID: blockID, receiptCount: receiptCount}
}

var _ error = (*InsufficientExecutionReceipts)(nil)

func (e InsufficientExecutionReceipts) Error() string {
	return fmt.Sprintf("insufficient execution receipts found (%d) for block ID: %s", e.receiptCount, e.blockID.String())
}

func IsInsufficientExecutionReceipts(err error) bool {
	var errInsufficientExecutionReceipts InsufficientExecutionReceipts
	return errors.As(err, &errInsufficientExecutionReceipts)
}

// FailedToQueryExternalNodeError is returned when the request to the external node failed.
type FailedToQueryExternalNodeError struct {
	err error
}

func NewFailedToQueryExternalNodeError(err error) *FailedToQueryExternalNodeError {
	return &FailedToQueryExternalNodeError{
		err: err,
	}
}

func (e *FailedToQueryExternalNodeError) Error() string {
	return fmt.Sprintf("failed to query external node: %v", e.err)
}

func (e *FailedToQueryExternalNodeError) Unwrap() error {
	return e.err
}

func IsFailedToQueryExternalNodeError(err error) bool {
	var target *FailedToQueryExternalNodeError
	return errors.As(err, &target)
}
