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

// AgreeingExecutorsCountExceededError indicates that the requested number of agreeing executors
// exceeds the total available execution nodes.
type AgreeingExecutorsCountExceededError struct {
	err error
}

func NewAgreeingExecutorsCountExceededError(agreeingExecutorsCount uint, availableExecutorsCount int) *AgreeingExecutorsCountExceededError {
	return &AgreeingExecutorsCountExceededError{
		err: fmt.Errorf("agreeing executors count exceeded: provided %d, available %d", agreeingExecutorsCount, availableExecutorsCount),
	}
}

func (e AgreeingExecutorsCountExceededError) Error() string {
	return e.err.Error()
}

func (e AgreeingExecutorsCountExceededError) Unwrap() error {
	return e.err
}

func IsAgreeingExecutorsCountExceededError(err error) bool {
	var agreeingExecutorsCountExceededError *AgreeingExecutorsCountExceededError
	return errors.As(err, &agreeingExecutorsCountExceededError)
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
		err: fmt.Errorf("the criteria for block %s is not met", blockID),
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

// BlockFinalityMismatchError indicates that the requested block does not match
// the finalized block at the same height. This means the block cannot belong
// to the canonical finalized chain.
type BlockFinalityMismatchError struct {
	err error
}

func NewBlockFinalityMismatchError(blockID flow.Identifier, actualBlockID flow.Identifier) *BlockFinalityMismatchError {
	return &BlockFinalityMismatchError{
		err: fmt.Errorf("block %s is not the finalized block at its height (finalized block is %s)", blockID, actualBlockID),
	}
}

func (e BlockFinalityMismatchError) Error() string {
	return e.err.Error()
}

func (e BlockFinalityMismatchError) Unwrap() error {
	return e.err
}

func IsBlockFinalityMismatchError(err error) bool {
	var blockFinalityMismatchError *BlockFinalityMismatchError
	return errors.As(err, &blockFinalityMismatchError)
}
