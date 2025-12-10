package optimistic_sync

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// RequiredExecutorsCountExceededError indicates that the requested number of required executors
// exceeds the total available execution nodes.
type RequiredExecutorsCountExceededError struct {
	requiredExecutorsCount  int
	availableExecutorsCount int
}

func NewRequiredExecutorsCountExceededError(requiredExecutorsCount int, availableExecutorsCount int) *RequiredExecutorsCountExceededError {
	return &RequiredExecutorsCountExceededError{
		requiredExecutorsCount:  requiredExecutorsCount,
		availableExecutorsCount: availableExecutorsCount,
	}
}

func (e *RequiredExecutorsCountExceededError) Error() string {
	return fmt.Sprintf("required executors count exceeded: required %d, available %d",
		e.requiredExecutorsCount, e.availableExecutorsCount,
	)
}

func IsRequiredExecutorsCountExceededError(err error) bool {
	var requiredExecutorsCountExceededError *RequiredExecutorsCountExceededError
	return errors.As(err, &requiredExecutorsCountExceededError)
}

// AgreeingExecutorsCountExceededError indicates that the requested number of agreeing executors
// exceeds the total available execution nodes.
type AgreeingExecutorsCountExceededError struct {
	agreeingExecutorsCount  uint
	availableExecutorsCount int
}

func NewAgreeingExecutorsCountExceededError(agreeingExecutorsCount uint, availableExecutorsCount int) *AgreeingExecutorsCountExceededError {
	return &AgreeingExecutorsCountExceededError{
		agreeingExecutorsCount:  agreeingExecutorsCount,
		availableExecutorsCount: availableExecutorsCount,
	}
}

func (e *AgreeingExecutorsCountExceededError) Error() string {
	return fmt.Sprintf("agreeing executors count exceeded: provided %d, available %d", e.agreeingExecutorsCount, e.availableExecutorsCount)
}

func IsAgreeingExecutorsCountExceededError(err error) bool {
	var agreeingExecutorsCountExceededError *AgreeingExecutorsCountExceededError
	return errors.As(err, &agreeingExecutorsCountExceededError)
}

// UnknownRequiredExecutorError indicates that a required executor ID is not present
// in the list of active execution nodes.
type UnknownRequiredExecutorError struct {
	executorID flow.Identifier
}

func NewUnknownRequiredExecutorError(executorID flow.Identifier) *UnknownRequiredExecutorError {
	return &UnknownRequiredExecutorError{
		executorID: executorID,
	}
}

func (e *UnknownRequiredExecutorError) Error() string {
	return fmt.Sprintf("unknown required executor ID: %s", e.executorID.String())
}

func IsUnknownRequiredExecutorError(err error) bool {
	var unknownRequiredExecutor *UnknownRequiredExecutorError
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
	return fmt.Sprintf("block %s is already sealed and no execution result satisfies the criteria", e.blockID)
}

func IsCriteriaNotMetError(err error) bool {
	var criteriaNotMetError *CriteriaNotMetError
	return errors.As(err, &criteriaNotMetError)
}
