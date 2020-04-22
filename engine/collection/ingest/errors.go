package ingest

import (
	"fmt"
)

// ErrIncompleteTransaction returned when transactions are missing fields.
type ErrIncompleteTransaction struct {
	Missing []string // the missing fields
}

func (e ErrIncompleteTransaction) Error() string {
	return fmt.Sprint("incomplete transaction missing fields: ", e.Missing)
}

func (e ErrIncompleteTransaction) Is(other error) bool {
	_, ok := other.(ErrIncompleteTransaction)
	return ok
}
