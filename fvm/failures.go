package fvm

import (
	"fmt"
)

const (
	failureCodeLedgerFailure        = 1000
	failureCodeUUIDGeneratorFailure = 1001
	failureCodeHashingFailure       = 1002
	failureCodeStorageFailure       = 1003
	failureCodeCBOREncodingFailure  = 1004
	failureCodeCBORDecodingFailure  = 1005
	failureCodeRLPEncodingFailure   = 1006
	failureCodeRLPDecodingFailure   = 1007
)

// A Failure is an unexpected fatal error in the VM (i.e. storage, stack overflow), halts the execution flow and
// stops the service.
type Failure interface {
	FailureCode() uint32
	Error() string
	Unwrap() error
}

// A LedgerFailure indicates an unexpected issue with the ledger
type LedgerFailure struct {
	err error
}

func (f *LedgerFailure) FailureCode() uint32 {
	return failureCodeLedgerFailure
}

func (f *LedgerFailure) Error() string {
	return fmt.Sprintf("Ledger FAILURE: ledger returned an error: %w", f.err)
}

func (f *LedgerFailure) Unwrap() error {
	return f.err
}

// A UUIDGeneratorFailure indicates an unexpected issue with uuid generator
type UUIDGeneratorFailure struct {
	err error
}

func (f *UUIDGeneratorFailure) FailureCode() uint32 {
	return failureCodeUUIDGeneratorFailure
}

func (f *UUIDGeneratorFailure) Error() string {
	return fmt.Sprintf("UUID Generator FAILURE: %w", f.err)
}

func (f *UUIDGeneratorFailure) Unwrap() error {
	return f.err
}

// A HasherFailure indicates an unexpected issue with crypto hasher
type HasherFailure struct {
	err              error
	details          string
	hashingAlgorithm string
}

func (f *HasherFailure) FailureCode() uint32 {
	return failureCodeHashingFailure
}

func (f *HasherFailure) Error() string {
	return fmt.Sprintf("Hasher FAILURE: %s (hashing algorithm: %s): %w", f.details, f.hashingAlgorithm, f.err)
}

func (f *HasherFailure) Unwrap() error {
	return f.err
}

// A StorageFailure indicates an unexpected issue with the storage (e.g. loading a block)
type StorageFailure struct {
	err     error
	details string
}

func (f *StorageFailure) FailureCode() uint32 {
	return failureCodeStorageFailure
}

func (f *StorageFailure) Error() string {
	return fmt.Sprintf("Storage FAILURE: %s: %w", f.details, f.err)
}

func (f *StorageFailure) Unwrap() error {
	return f.err
}
