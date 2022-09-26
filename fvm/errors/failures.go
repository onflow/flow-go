package errors

func NewUnknownFailure(err error) *CodedFailure {
	return WrapCodedFailure(
		FailureCodeUnknownFailure,
		err,
		"unknown failure")
}

// EncodingFailure captures an fatal error sourced from encoding issues
type EncodingFailure struct {
	errorWrapper
}

// NewEncodingFailuref formats and returns a new EncodingFailure
func NewEncodingFailuref(msg string, err error) *CodedFailure {
	return WrapCodedFailure(
		FailureCodeEncodingFailure,
		err,
		"encoding failed: "+msg)
}

// NewLedgerFailure constructs a new CodedFailure which captures a fatal error
// cause by ledger failures.
func NewLedgerFailure(err error) *CodedFailure {
	return WrapCodedFailure(
		FailureCodeLedgerFailure,
		err,
		"ledger returns unsuccessful")
}

// IsALedgerFailure returns true if the error or any of the wrapped errors is
// a ledger failure
func IsALedgerFailure(err error) bool {
	return HasErrorCode(err, FailureCodeLedgerFailure)
}

// NewStateMergeFailure constructs a new CodedFailure which captures a fatal
// caused by state merge.
func NewStateMergeFailure(err error) *CodedFailure {
	return WrapCodedFailure(
		FailureCodeStateMergeFailure,
		err,
		"can not merge the state")
}

// NewBlockFinderFailure constructs a new CodedFailure which captures a fatal
// caused by block finder.
func NewBlockFinderFailure(err error) *CodedFailure {
	return WrapCodedFailure(
		FailureCodeBlockFinderFailure,
		err,
		"can not retrieve the block")
}
