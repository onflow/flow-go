package errors

func NewUnknownFailure(err error) CodedError {
	return WrapCodedError(
		FailureCodeUnknownFailure,
		err,
		"unknown failure")
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
