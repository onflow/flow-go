package errors

import (
	"errors"
	"fmt"
)

func NewUnknownFailure(err error) *CodedFailure {
	return WrapCodedFailure(
		FailureCodeUnknownFailure,
		err,
		"unknown failure")
}

// NewEncodingFailure formats and returns a new CodedFailure which
// captures an fatal error sourced from encoding issues
func NewEncodingFailuref(
	err error,
	msg string,
	args ...interface{},
) *CodedFailure {
	return WrapCodedFailure(
		FailureCodeEncodingFailure,
		err,
		"encoding failed: "+msg,
		args...)
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
func (e LedgerFailure) FailureCode() ErrorCode {
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
func (e StateMergeFailure) FailureCode() ErrorCode {
	return FailureCodeStateMergeFailure
}

// NewBlockFinderFailure constructs a new CodedFailure which captures a fatal
// caused by block finder.
func NewBlockFinderFailure(err error) *CodedFailure {
	return WrapCodedFailure(
		FailureCodeBlockFinderFailure,
		err,
		"can not retrieve the block")
}
