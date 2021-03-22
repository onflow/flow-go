package errors

import (
	"fmt"
)

// VMError captures fatal vm errors
// TODO add failure codes
type VMError interface {
	FailureCode() uint16
	error
}

type UnknownFailure struct {
	Err error
}

func (e *UnknownFailure) Error() string {
	return fmt.Sprintf("unknown failure: %s", e.Err.Error())
}

func (e *UnknownFailure) FailureCode() uint16 {
	return 0
}

type EncodingFailure struct {
	Err error
}

func (e *EncodingFailure) Error() string {
	return fmt.Sprintf("encoding failed: %s", e.Err.Error())
}

func (e *EncodingFailure) FailureCode() uint16 {
	return 1
}

type LedgerFailure struct {
	Err error
}

func (e *LedgerFailure) Error() string {
	return fmt.Sprintf("ledger returns unsuccessful: %s", e.Err.Error())
}

func (e *LedgerFailure) FailureCode() uint16 {
	return 2
}

type StateMergeFailure struct {
	Err error
}

func (e *StateMergeFailure) Error() string {
	return fmt.Sprintf("can not merge the state: %s", e.Err.Error())
}

func (e *StateMergeFailure) FailureCode() uint16 {
	return 3
}
