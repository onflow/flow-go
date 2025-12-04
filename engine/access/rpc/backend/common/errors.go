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

// RequiredExecutorsCountExceeded indicates that the requested number of required executors
// exceeds the total available execution nodes.
type RequiredExecutorsCountExceeded struct {
	requiredExecutorsCount  int
	availableExecutorsCount int
}

func NewRequiredExecutorsCountExceeded(requiredExecutorsCount int, availableExecutorsCount int) *RequiredExecutorsCountExceeded {
	return &RequiredExecutorsCountExceeded{
		requiredExecutorsCount:  requiredExecutorsCount,
		availableExecutorsCount: availableExecutorsCount,
	}
}

func (e *RequiredExecutorsCountExceeded) Error() string {
	return fmt.Sprintf(
		"required executors count exceeded: required %d, available %d",
		e.requiredExecutorsCount, e.availableExecutorsCount,
	)
}

func IsRequiredExecutorsCountExceeded(err error) bool {
	var target *RequiredExecutorsCountExceeded
	return errors.As(err, &target)
}

// UnknownRequiredExecutor indicates that a required executor ID is not present
// in the list of active execution nodes.
type UnknownRequiredExecutor struct {
	executorID flow.Identifier
}

func NewUnknownRequiredExecutor(executorID flow.Identifier) *UnknownRequiredExecutor {
	return &UnknownRequiredExecutor{
		executorID: executorID,
	}
}

func (e *UnknownRequiredExecutor) Error() string {
	return fmt.Sprintf("unknown required executor ID: %s", e.executorID.String())
}

func IsUnknownRequiredExecutor(err error) bool {
	var target *UnknownRequiredExecutor
	return errors.As(err, &target)
}
