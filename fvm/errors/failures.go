package errors

import (
	"fmt"
)

// UnknownFailure captures an unknown vm fatal error
type UnknownFailure struct {
	err error
}

// NewUnknownFailure constructs a new UnknownFailure
func NewUnknownFailure(err error) *UnknownFailure {
	return &UnknownFailure{err: err}
}

func (e *UnknownFailure) Error() string {
	return fmt.Sprintf("unknown failure: %s", e.err.Error())
}

// FailureCode returns the failure code
func (e *UnknownFailure) FailureCode() uint16 {
	return failureCodeUnknownFailure
}

// EncodingFailure captures an fatal error sourced from encoding issues
type EncodingFailure struct {
	err error
}

// NewEncodingFailuref formats and returns a new EncodingFailure
func NewEncodingFailuref(msg string, err error) *EncodingFailure {
	return &EncodingFailure{
		err: fmt.Errorf(msg, err),
	}
}

func (e *EncodingFailure) Error() string {
	return fmt.Sprintf("encoding failed: %s", e.err.Error())
}

// FailureCode returns the failure code
func (e *EncodingFailure) FailureCode() uint16 {
	return failureCodeEncodingFailure
}

// LedgerFailure captures a fatal error cause by ledger failures
type LedgerFailure struct {
	err error
}

// NewLedgerFailure constructs a new LedgerFailure
func NewLedgerFailure(err error) *LedgerFailure {
	return &LedgerFailure{err: err}
}

func (e *LedgerFailure) Error() string {
	return fmt.Sprintf("ledger returns unsuccessful: %s", e.err.Error())
}

// FailureCode returns the failure code
func (e *LedgerFailure) FailureCode() uint16 {
	return failureCodeLedgerFailure
}

// StateMergeFailure captures a fatal caused by state merge
type StateMergeFailure struct {
	err error
}

// NewStateMergeFailure constructs a new StateMergeFailure
func NewStateMergeFailure(err error) *StateMergeFailure {
	return &StateMergeFailure{err: err}
}

func (e *StateMergeFailure) Error() string {
	return fmt.Sprintf("can not merge the state: %s", e.err.Error())
}

// FailureCode returns the failure code
func (e *StateMergeFailure) FailureCode() uint16 {
	return failureCodeStateMergeFailure
}

// BlockFinderFailure captures a fatal caused by block finder
type BlockFinderFailure struct {
	err error
}

// NewBlockFinderFailure constructs a new BlockFinderFailure
func NewBlockFinderFailure(err error) *BlockFinderFailure {
	return &BlockFinderFailure{err: err}
}

func (e *BlockFinderFailure) Error() string {
	return fmt.Sprintf("can not retrieve the block: %s", e.err.Error())
}

// FailureCode returns the failure code
func (e *BlockFinderFailure) FailureCode() uint16 {
	return failureCodeBlockFinderFailure
}

// HasherFailure captures a fatal caused by hasher
type HasherFailure struct {
	err error
}

// NewHasherFailure returns a new hasherFailure
func NewHasherFailure(err error) *HasherFailure {
	return &HasherFailure{
		err: err,
	}
}

// NewHasherFailuref formats and returns a new hasherFailure
func NewHasherFailuref(msg string, err error) error {
	return &HasherFailure{
		err: fmt.Errorf(msg, err),
	}
}

func (e *HasherFailure) Error() string {
	return fmt.Sprintf("hasher failed: %s", e.err.Error())
}

// FailureCode returns the failure code
func (e *HasherFailure) FailureCode() uint16 {
	return failureCodeHasherFailure
}
