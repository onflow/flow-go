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
	var unknownRequiredExecutor *UnknownRequiredExecutor
	return errors.As(err, &unknownRequiredExecutor)
}

// CriteriaNotMetError indicates that the execution result criteria could not be
// satisfied for a given block, when the block is already sealed.
type CriteriaNotMetError struct {
	blockID flow.Identifier
}

func NewCriteriaNotMetError(blockID flow.Identifier) *CriteriaNotMetError {
	return &CriteriaNotMetError{
		blockID: blockID,
	}
}

func (e *CriteriaNotMetError) Error() string {
	return fmt.Sprintf("block %s is already sealed but the criteria is still not met,", e.blockID)
}

func IsCriteriaNotMetError(err error) bool {
	var criteriaNotMetError *CriteriaNotMetError
	return errors.As(err, &criteriaNotMetError)
}
