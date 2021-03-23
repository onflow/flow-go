package errors

import (
	"fmt"
)

// UnknownFailure captures an unknown vm fatal error
type UnknownFailure struct {
	Err error
}

func (e *UnknownFailure) Error() string {
	return fmt.Sprintf("unknown failure: %s", e.Err.Error())
}

// FailureCode returns the failure code
func (e *UnknownFailure) FailureCode() uint16 {
	return failureCodeUnknownFailure
}

// EncodingFailure captures an fatal error sourced from encoding issues
type EncodingFailure struct {
	Err error
}

func (e *EncodingFailure) Error() string {
	return fmt.Sprintf("encoding failed: %s", e.Err.Error())
}

// FailureCode returns the failure code
func (e *EncodingFailure) FailureCode() uint16 {
	return failureCodeEncodingFailure
}

// LedgerFailure captures a fatal error cause by ledger failures
type LedgerFailure struct {
	Err error
}

func (e *LedgerFailure) Error() string {
	return fmt.Sprintf("ledger returns unsuccessful: %s", e.Err.Error())
}

// FailureCode returns the failure code
func (e *LedgerFailure) FailureCode() uint16 {
	return failureCodeLedgerFailure
}

// StateMergeFailure captures a fatal caused by state merge
type StateMergeFailure struct {
	Err error
}

func (e *StateMergeFailure) Error() string {
	return fmt.Sprintf("can not merge the state: %s", e.Err.Error())
}

// FailureCode returns the failure code
func (e *StateMergeFailure) FailureCode() uint16 {
	return failureCodeStateMergeFailure
}

// BlockFinderFailure captures a fatal caused by block finder
type BlockFinderFailure struct {
	Err error
}

func (e *BlockFinderFailure) Error() string {
	return fmt.Sprintf("can not retrieve the block: %s", e.Err.Error())
}

// FailureCode returns the failure code
func (e *BlockFinderFailure) FailureCode() uint16 {
	return failureCodeBlockFinderFailure
}
