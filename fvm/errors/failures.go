package errors

import (
	"errors"
	"fmt"
)

// UnknownFailure captures an unknown vm fatal error
type UnknownFailure struct {
	err error
}

// NewUnknownFailure constructs a new UnknownFailure
func NewUnknownFailure(err error) *UnknownFailure {
	return &UnknownFailure{err: err}
}

func (e *UnknownFailure) Error() string {
	return fmt.Sprintf("%s unknown failure: %s", e.FailureCode().String(), e.err.Error())
}

// FailureCode returns the failure code
func (e *UnknownFailure) FailureCode() FailureCode {
	return FailureCodeUnknownFailure
}

// Unwrap unwraps the error
func (e UnknownFailure) Unwrap() error {
	return e.err
}

// EncodingFailure captures an fatal error sourced from encoding issues
type EncodingFailure struct {
	err error
}

// NewEncodingFailuref formats and returns a new EncodingFailure
func NewEncodingFailuref(msg string, err error) *EncodingFailure {
	return &EncodingFailure{
		err: fmt.Errorf(msg, err),
	}
}

func (e *EncodingFailure) Error() string {
	return fmt.Sprintf("%s encoding failed: %s", e.FailureCode().String(), e.err.Error())
}

// FailureCode returns the failure code
func (e *EncodingFailure) FailureCode() FailureCode {
	return FailureCodeEncodingFailure
}

// Unwrap unwraps the error
func (e EncodingFailure) Unwrap() error {
	return e.err
}

// LedgerFailure captures a fatal error cause by ledger failures
type LedgerFailure struct {
	err error
}

// NewLedgerFailure constructs a new LedgerFailure
func NewLedgerFailure(err error) *LedgerFailure {
	return &LedgerFailure{err: err}
}

func (e *LedgerFailure) Error() string {
	return fmt.Sprintf("%s ledger returns unsuccessful: %s", e.FailureCode().String(), e.err.Error())
}

// FailureCode returns the failure code
func (e *LedgerFailure) FailureCode() FailureCode {
	return FailureCodeLedgerFailure
}

// Unwrap unwraps the error
func (e LedgerFailure) Unwrap() error {
	return e.err
}

// IsALedgerFailure returns true if the error or any of the wrapped errors is a ledger failure
func IsALedgerFailure(err error) bool {
	var t *LedgerFailure
	return errors.As(err, &t)
}

// StateMergeFailure captures a fatal caused by state merge
type StateMergeFailure struct {
	err error
}

// NewStateMergeFailure constructs a new StateMergeFailure
func NewStateMergeFailure(err error) *StateMergeFailure {
	return &StateMergeFailure{err: err}
}

func (e StateMergeFailure) Error() string {
	return fmt.Sprintf("%s can not merge the state: %s", e.FailureCode().String(), e.err.Error())
}

// FailureCode returns the failure code
func (e StateMergeFailure) FailureCode() FailureCode {
	return FailureCodeStateMergeFailure
}

// Unwrap unwraps the error
func (e StateMergeFailure) Unwrap() error {
	return e.err
}

// BlockFinderFailure captures a fatal caused by block finder
type BlockFinderFailure struct {
	err error
}

// NewBlockFinderFailure constructs a new BlockFinderFailure
func NewBlockFinderFailure(err error) *BlockFinderFailure {
	return &BlockFinderFailure{err: err}
}

func (e BlockFinderFailure) Error() string {
	return fmt.Sprintf("%s can not retrieve the block: %s", e.FailureCode().String(), e.err.Error())
}

// FailureCode returns the failure code
func (e BlockFinderFailure) FailureCode() FailureCode {
	return FailureCodeBlockFinderFailure
}

// Unwrap unwraps the error
func (e BlockFinderFailure) Unwrap() error {
	return e.err
}

// HasherFailure captures a fatal caused by hasher
type HasherFailure struct {
	err error
}

// NewHasherFailuref constructs a new hasherFailure
func NewHasherFailuref(msg string, args ...interface{}) *HasherFailure {
	return &HasherFailure{err: fmt.Errorf(msg, args...)}
}

func (e HasherFailure) Error() string {
	return fmt.Sprintf("%s hasher failed: %s", e.FailureCode().String(), e.err.Error())
}

// FailureCode returns the failure code
func (e HasherFailure) FailureCode() FailureCode {
	return FailureCodeHasherFailure
}

// Unwrap unwraps the error
func (e HasherFailure) Unwrap() error {
	return e.err
}

// MetaTransactionFailure captures a fatal caused by invoking a meta transaction
type MetaTransactionFailure struct {
	err error
}

// NewMetaTransactionFailuref constructs a new hasherFailure
func NewMetaTransactionFailuref(msg string, args ...interface{}) *MetaTransactionFailure {
	return &MetaTransactionFailure{err: fmt.Errorf(msg, args...)}
}

func (e MetaTransactionFailure) Error() string {
	return fmt.Sprintf("%s meta transaction failed: %s", e.FailureCode().String(), e.err.Error())
}

// FailureCode returns the failure code
func (e MetaTransactionFailure) FailureCode() FailureCode {
	return FailureCodeMetaTransactionFailure
}

// Unwrap unwraps the error
func (e MetaTransactionFailure) Unwrap() error {
	return e.err
}
