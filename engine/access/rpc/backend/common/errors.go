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

// InvalidDataFromExternalNodeError is returned when the data returned by the execution node is invalid.
// For example, event encoding conversion failed.
type InvalidDataFromExternalNodeError struct {
	dataType string
	err      error
	nodeID   flow.Identifier
}

func NewInvalidDataFromExternalNodeError(dataType string, nodeID flow.Identifier, err error) *InvalidDataFromExternalNodeError {
	return &InvalidDataFromExternalNodeError{
		dataType: dataType,
		nodeID:   nodeID,
		err:      err,
	}
}

func (e *InvalidDataFromExternalNodeError) Error() string {
	return fmt.Sprintf("received invalid %s data from external node %s: %v", e.dataType, e.nodeID, e.err)
}

func (e *InvalidDataFromExternalNodeError) Unwrap() error {
	return e.err
}

func IsInvalidDataFromExternalNodeError(err error) bool {
	var target *InvalidDataFromExternalNodeError
	return errors.As(err, &target)
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
