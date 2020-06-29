package ingest

import (
	"errors"
	"fmt"
)

// ErrUnknownReferenceBlock indicates a transaction references an unknown block.
var ErrUnknownReferenceBlock = errors.New("unknown reference block")

// IncompleteTransactionError returned when transactions are missing fields.
type IncompleteTransactionError struct {
	Missing []string // the missing fields
}

func (e IncompleteTransactionError) Error() string {
	return fmt.Sprint("incomplete transaction missing fields: ", e.Missing)
}

// ExpiredTransactionError indicates a transaction has expired.
type ExpiredTransactionError struct {
	RefHeight, FinalHeight uint64
}

func (e ExpiredTransactionError) Error() string {
	return fmt.Sprintf("expired transaction: ref_height=%d final_height=%d", e.RefHeight, e.FinalHeight)
}

// InvalidScriptError indicates a transaction with an invalid scipt.
type InvalidScriptError struct {
	ParserErr error
}

func (e InvalidScriptError) Error() string {
	return fmt.Sprintf("invalid transaction script: %s", e.ParserErr)
}

func (e InvalidScriptError) Unwrap() error {
	return e.ParserErr
}

type GasLimitExceededError struct {
	Maximum uint64
	Actual  uint64
}

func (e GasLimitExceededError) Error() string {
	return fmt.Sprintf("transaction's gas limit (%d) exceeds the allowed max gas limit (%d)", e.Actual, e.Maximum)
}
