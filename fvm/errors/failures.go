package errors

import (
	"fmt"
)

func NewUnknownFailure(err error) CodedError {
	return WrapCodedError(
		FailureCodeUnknownFailure,
		err,
		"unknown failure")
}

// EventEncodingError captures an error sourced from encoding issues
type EventEncodingError struct {
	err error
}

// NewEventEncodingErrorf formats and returns a new EventEncodingError
func NewEventEncodingErrorf(msg string, err error) *EventEncodingError {
	return &EventEncodingError{
		err: fmt.Errorf(msg, err),
	}
}

func (e *EventEncodingError) Error() string {
	//return fmt.Sprintf("%s encoding failed: %s", e.ErrorCode().String(), e.err.Error())
	return fmt.Sprintf("%s encoding failed: %s", e.ErrorCode().String(), e.err.Error())
}

// ErrorCode returns the error code
func (e *EventEncodingError) ErrorCode() ErrorCode {
	return ErrCodeEventEncodingError
}

// Unwrap unwraps the error
func (e EventEncodingError) Unwrap() error {
	return e.err
}

// EncodingFailure captures an fatal error sourced from encoding issues
type EncodingFailure struct {
	err error
}

// NewEncodingFailuref formats and returns a new EncodingFailure
func NewEncodingFailuref(
	err error,
	msg string,
	args ...interface{},
) CodedError {
	return WrapCodedError(
		FailureCodeEncodingFailure,
		err,
		"encoding failed: "+msg,
		args...)
}

// NewLedgerFailure constructs a new CodedError which captures a fatal error
// cause by ledger failures.
func NewLedgerFailure(err error) CodedError {
	return WrapCodedError(
		FailureCodeLedgerFailure,
		err,
		"ledger returns unsuccessful")
}

// IsALedgerFailure returns true if the error or any of the wrapped errors is
// a ledger failure
func IsALedgerFailure(err error) bool {
	return HasErrorCode(err, FailureCodeLedgerFailure)
}

// NewStateMergeFailure constructs a new CodedError which captures a fatal
// caused by state merge.
func NewStateMergeFailure(err error) CodedError {
	return WrapCodedError(
		FailureCodeStateMergeFailure,
		err,
		"can not merge the state")
}

// NewBlockFinderFailure constructs a new CodedError which captures a fatal
// caused by block finder.
func NewBlockFinderFailure(err error) CodedError {
	return WrapCodedError(
		FailureCodeBlockFinderFailure,
		err,
		"can not retrieve the block")
}

// NewParseRestrictedModeInvalidAccessFailure constructs a CodedError which
// captures a fatal caused by Cadence accessing an unexpected environment
// operation while it is parsing programs.
func NewParseRestrictedModeInvalidAccessFailure(op string) CodedError {
	return NewCodedError(
		FailureCodeParseRestrictedModeInvalidAccessFailure,
		"cannot access %s while cadence is in parse restricted mode",
		op)
}
