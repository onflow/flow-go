package access

import (
	"errors"
	"fmt"
)

// ErrUnknownReferenceBlock indicates that a transaction references an unknown block.
var ErrUnknownReferenceBlock = errors.New("unknown reference block")

// IncompleteTransactionError indicates that a transaction is missing one or more required fields.
type IncompleteTransactionError struct {
	MissingFields []string
}

func (e IncompleteTransactionError) Error() string {
	return fmt.Sprintf("transaction is missing required fields: %s", e.MissingFields)
}

// ExpiredTransactionError indicates that a transaction has expired.
type ExpiredTransactionError struct {
	RefHeight, FinalHeight uint64
}

func (e ExpiredTransactionError) Error() string {
	return fmt.Sprintf("transaction is expired: ref_height=%d final_height=%d", e.RefHeight, e.FinalHeight)
}

// InvalidScriptError indicates that a transaction contains an invalid Cadence script.
type InvalidScriptError struct {
	ParserErr error
}

func (e InvalidScriptError) Error() string {
	return fmt.Sprintf("failed to parse transaction Cadence script: %s", e.ParserErr)
}

func (e InvalidScriptError) Unwrap() error {
	return e.ParserErr
}

// InvalidGasLimitError indicates that a transaction specifies a gas limit that exceeds the maximum.
type InvalidGasLimitError struct {
	Maximum uint64
	Actual  uint64
}

func (e InvalidGasLimitError) Error() string {
	return fmt.Sprintf("transaction gas limit (%d) exceeds the maximum gas limit (%d)", e.Actual, e.Maximum)
}
