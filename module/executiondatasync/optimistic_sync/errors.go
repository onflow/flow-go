package optimistic_sync

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// ErrForkAbandoned is returned if the execution fork of an execution node from which we were getting the execution
// results was abandoned.
var ErrForkAbandoned = errors.New("current execution fork has been abandoned")

// ErrRequiredExecutorNotFound is returned if the criteria's required executor is not in the group of execution nodes
// that produced the execution result.
var ErrRequiredExecutorNotFound = errors.New("required executor not found")

// ErrNotEnoughAgreeingExecutors is returned if there are not enough execution nodes that produced the execution result.
var ErrNotEnoughAgreeingExecutors = errors.New("not enough agreeing executors found")

// ErrBlockBeforeNodeHistory is returned when the requested block predates what the node has in storage
// (for example, requesting the spork root block while the node was bootstrapped from a newer block).
var ErrBlockBeforeNodeHistory = errors.New("requested block is before node history")

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
