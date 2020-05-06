package ingest

import (
	"errors"
	"fmt"
)

var (
	ErrUnknownReferenceBlock = errors.New("unknown reference block")
)

// IncompleteTransactionError returned when transactions are missing fields.
type IncompleteTransactionError struct {
	Missing []string // the missing fields
}

func (e IncompleteTransactionError) Error() string {
	return fmt.Sprint("incomplete transaction missing fields: ", e.Missing)
}

func (e IncompleteTransactionError) Is(other error) bool {
	_, ok := other.(IncompleteTransactionError)
	return ok
}

// ExpiredTransactionError indicates a transaction has expired.
type ExpiredTransactionError struct {
	RefHeight, FinalHeight uint64
}

func (e ExpiredTransactionError) Error() string {
	return fmt.Sprintf("expired transaction: ref_height=%d final_height=%d", e.RefHeight, e.FinalHeight)
}

func (e ExpiredTransactionError) Is(other error) bool {
	_, ok := other.(ExpiredTransactionError)
	return ok
}

// InvalidScriptError indicates a transaction with an invalid scipt.
type InvalidScriptError struct {
	ParserErr error
}

func (e InvalidScriptError) Error() string {
	return fmt.Sprintf("invalid transaction script: %s", e.ParserErr)
}

func (e InvalidScriptError) Is(other error) bool {
	_, ok := other.(InvalidScriptError)
	return ok
}

func (e InvalidScriptError) Unwrap() error {
	return e.ParserErr
}
