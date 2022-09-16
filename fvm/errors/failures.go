package errors

import (
	"errors"
	"fmt"
)

// UnknownFailure captures an unknown vm fatal error
type UnknownFailure struct {
	errorWrapper
}

// NewUnknownFailure constructs a new UnknownFailure
func NewUnknownFailure(err error) UnknownFailure {
	return UnknownFailure{
		errorWrapper: errorWrapper{err: err},
	}
}

func (e UnknownFailure) Error() string {
	return fmt.Sprintf("%s unknown failure: %s", e.FailureCode().String(), e.err.Error())
}

// FailureCode returns the failure code
func (e UnknownFailure) FailureCode() FailureCode {
	return FailureCodeUnknownFailure
}

// EncodingFailure captures an fatal error sourced from encoding issues
type EncodingFailure struct {
	errorWrapper
}

// NewEncodingFailuref formats and returns a new EncodingFailure
func NewEncodingFailuref(msg string, err error) EncodingFailure {
	return EncodingFailure{
		errorWrapper: errorWrapper{err: fmt.Errorf(msg, err)},
	}
}

func (e EncodingFailure) Error() string {
	return fmt.Sprintf("%s encoding failed: %s", e.FailureCode().String(), e.err.Error())
}

// FailureCode returns the failure code
func (e EncodingFailure) FailureCode() FailureCode {
	return FailureCodeEncodingFailure
}

// LedgerFailure captures a fatal error cause by ledger failures
type LedgerFailure struct {
	errorWrapper
}

// NewLedgerFailure constructs a new LedgerFailure
func NewLedgerFailure(err error) LedgerFailure {
	return LedgerFailure{
		errorWrapper: errorWrapper{err: err},
	}
}

func (e LedgerFailure) Error() string {
	return fmt.Sprintf("%s ledger returns unsuccessful: %s", e.FailureCode().String(), e.err.Error())
}

// FailureCode returns the failure code
func (e LedgerFailure) FailureCode() FailureCode {
	return FailureCodeLedgerFailure
}

// IsALedgerFailure returns true if the error or any of the wrapped errors is a ledger failure
func IsALedgerFailure(err error) bool {
	var t LedgerFailure
	return errors.As(err, &t)
}

// StateMergeFailure captures a fatal caused by state merge
type StateMergeFailure struct {
	errorWrapper
}

// NewStateMergeFailure constructs a new StateMergeFailure
func NewStateMergeFailure(err error) StateMergeFailure {
	return StateMergeFailure{
		errorWrapper: errorWrapper{err: err},
	}
}

func (e StateMergeFailure) Error() string {
	return fmt.Sprintf("%s can not merge the state: %s", e.FailureCode().String(), e.err.Error())
}

// FailureCode returns the failure code
func (e StateMergeFailure) FailureCode() FailureCode {
	return FailureCodeStateMergeFailure
}

// BlockFinderFailure captures a fatal caused by block finder
type BlockFinderFailure struct {
	errorWrapper
}

// NewBlockFinderFailure constructs a new BlockFinderFailure
func NewBlockFinderFailure(err error) BlockFinderFailure {
	return BlockFinderFailure{
		errorWrapper: errorWrapper{err: err},
	}
}

func (e BlockFinderFailure) Error() string {
	return fmt.Sprintf("%s can not retrieve the block: %s", e.FailureCode().String(), e.err.Error())
}

// FailureCode returns the failure code
func (e BlockFinderFailure) FailureCode() FailureCode {
	return FailureCodeBlockFinderFailure
}
