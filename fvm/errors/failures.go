package errors

import (
	"github.com/onflow/flow-go/module/trace"
)

func NewUnknownFailure(err error) CodedError {
	return WrapCodedFailure(
		FailureCodeUnknownFailure,
		err,
		"unknown failure")
}

// NewEncodingFailuref formats and returns a new EncodingFailure
func NewEncodingFailuref(
	err error,
	msg string,
	args ...interface{},
) CodedError {
	return WrapCodedFailure(
		FailureCodeEncodingFailure,
		err,
		"encoding failed: "+msg,
		args...)
}

// NewLedgerFailure constructs a new CodedError which captures a fatal error
// cause by ledger failures.
func NewLedgerFailure(err error) CodedError {
	return WrapCodedFailure(
		FailureCodeLedgerFailure,
		err,
		"ledger returns unsuccessful")
}

// IsLedgerFailure returns true if the error or any of the wrapped errors is
// a ledger failure
func IsLedgerFailure(err error) bool {
	return HasErrorCode(err, FailureCodeLedgerFailure)
}

// NewStateMergeFailure constructs a new CodedError which captures a fatal
// caused by state merge.
func NewStateMergeFailure(err error) CodedError {
	return WrapCodedFailure(
		FailureCodeStateMergeFailure,
		err,
		"can not merge the state")
}

// NewBlockFinderFailure constructs a new CodedError which captures a fatal
// caused by block finder.
func NewBlockFinderFailure(err error) CodedError {
	return WrapCodedFailure(
		FailureCodeBlockFinderFailure,
		err,
		"can not retrieve the block")
}

// NewParseRestrictedModeInvalidAccessFailure constructs a CodedError which
// captures a fatal caused by Cadence accessing an unexpected environment
// operation while it is parsing programs.
func NewParseRestrictedModeInvalidAccessFailure(
	spanName trace.SpanName,
) CodedError {
	return NewCodedFailure(
		FailureCodeParseRestrictedModeInvalidAccessFailure,
		"cannot access %s while cadence is in parse restricted mode",
		spanName)
}
