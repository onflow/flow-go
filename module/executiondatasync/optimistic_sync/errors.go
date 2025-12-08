package optimistic_sync

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// RequiredExecutorsCountExceededError indicates that the requested number of required executors
// exceeds the total available execution nodes.
type RequiredExecutorsCountExceededError struct {
	err error
}

func NewRequiredExecutorsCountExceededError(requiredExecutorsCount int, availableExecutorsCount int) *RequiredExecutorsCountExceededError {
	return &RequiredExecutorsCountExceededError{
		err: fmt.Errorf("required executors count exceeded: required %d, available %d", requiredExecutorsCount, availableExecutorsCount),
	}
}

func (e RequiredExecutorsCountExceededError) Error() string {
	return e.err.Error()
}

func (e RequiredExecutorsCountExceededError) Unwrap() error {
	return e.err
}

func IsRequiredExecutorsCountExceededError(err error) bool {
	var requiredExecutorsCountExceededError *RequiredExecutorsCountExceededError
	return errors.As(err, &requiredExecutorsCountExceededError)
}

// UnknownRequiredExecutorError indicates that a required executor ID is not present
// in the list of active execution nodes.
type UnknownRequiredExecutorError struct {
	err error
}

func NewUnknownRequiredExecutorError(executorID flow.Identifier) *UnknownRequiredExecutorError {
	return &UnknownRequiredExecutorError{
		err: fmt.Errorf("unknown required executor ID %s", executorID.String()),
	}
}

func (e UnknownRequiredExecutorError) Error() string {
	return e.err.Error()
}

func (e UnknownRequiredExecutorError) Unwrap() error {
	return e.err
}

func IsUnknownRequiredExecutorError(err error) bool {
	var unknownRequiredExecutor *UnknownRequiredExecutorError
	return errors.As(err, &unknownRequiredExecutor)
}

// CriteriaNotMetError indicates that the execution result criteria could not be
// satisfied for a given block, when the block is already sealed.
type CriteriaNotMetError struct {
	err error
}

func NewCriteriaNotMetError(blockID flow.Identifier) *CriteriaNotMetError {
	return &CriteriaNotMetError{
		err: fmt.Errorf("block %s is already sealed but the criteria is still not met", blockID),
	}
}

func (e CriteriaNotMetError) Error() string {
	return e.err.Error()
}

func (e CriteriaNotMetError) Unwrap() error {
	return e.err
}

func IsCriteriaNotMetError(err error) bool {
	var criteriaNotMetError *CriteriaNotMetError
	return errors.As(err, &criteriaNotMetError)
}
