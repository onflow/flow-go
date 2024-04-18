package common

import "fmt"

type TransactionError struct {
	Err error
}

func (m TransactionError) Error() string {
	return fmt.Sprintf("TransactionError: %s", m.Err)
}

func (m TransactionError) Unwrap() error {
	return m.Err
}

func NewTransactionError(err error) *TransactionError {
	return &TransactionError{Err: err}
}
