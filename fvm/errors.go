package fvm

import (
	"errors"
	"fmt"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/flow"
)

const (
	errCodeMissingSignature                      = 1
	errCodeMissingPayer                          = 2
	errCodeInvalidSignaturePublicKeyDoesNotExist = 3
	errCodeInvalidSignaturePublicKeyRevoked      = 4
	errCodeInvalidSignatureVerification          = 5

	errCodeInvalidProposalKeyPublicKeyDoesNotExist = 6
	errCodeInvalidProposalKeyPublicKeyRevoked      = 7
	errCodeInvalidProposalKeySequenceNumber        = 8
	errCodeInvalidProposalKeyMissingSignature      = 9

	errCodeInvalidHashAlgorithm = 10

	errCodeExecution = 100
)

var ErrAccountNotFound = errors.New("account not found")
var ErrInvalidHashAlgorithm = errors.New("invalid hash algorithm")

// An Error represents a non-fatal error that is expected during normal operation of the virtual machine.
//
// VM errors are distinct from fatal errors, which indicate an unexpected failure
// in the VM (i.e. storage, stack overflow).
//
// Each VM error is identified by a unique error code that is returned to the user.
type Error interface {
	Code() uint32
	Error() string
}

// A MissingSignatureError indicates that a transaction is missing a required signature.
type MissingSignatureError struct {
	Address flow.Address
}

func (e *MissingSignatureError) Error() string {
	return fmt.Sprintf("account %s does not have sufficient signatures", e.Address)
}

func (e *MissingSignatureError) Code() uint32 {
	return errCodeMissingSignature
}

// A MissingPayerError indicates that a transaction is missing a payer.
type MissingPayerError struct{}

func (e *MissingPayerError) Error() string {
	return "no payer address provided"
}

func (e *MissingPayerError) Code() uint32 {
	return errCodeMissingPayer
}

// An InvalidSignaturePublicKeyDoesNotExistError indicates that a signature specifies a public key that
// does not exist.
type InvalidSignaturePublicKeyDoesNotExistError struct {
	Address  flow.Address
	KeyIndex uint64
}

func (e *InvalidSignaturePublicKeyDoesNotExistError) Error() string {
	return fmt.Sprintf(
		"invalid signature: public key with index %d does not exist on account %s",
		e.KeyIndex,
		e.Address,
	)
}

func (e *InvalidSignaturePublicKeyDoesNotExistError) Code() uint32 {
	return errCodeInvalidSignaturePublicKeyDoesNotExist
}

// An InvalidSignaturePublicKeyRevokedError indicates that a signature specifies a public key that has been revoked.
type InvalidSignaturePublicKeyRevokedError struct {
	Address  flow.Address
	KeyIndex uint64
}

func (e *InvalidSignaturePublicKeyRevokedError) Error() string {
	return fmt.Sprintf(
		"invalid signature: public key with index %d on account %s has been revoked",
		e.KeyIndex,
		e.Address,
	)
}

func (e *InvalidSignaturePublicKeyRevokedError) Code() uint32 {
	return errCodeInvalidSignaturePublicKeyRevoked
}

// An InvalidSignatureVerificationError indicates that a signature could not be verified using its specified
// public key.
type InvalidSignatureVerificationError struct {
	Address  flow.Address
	KeyIndex uint64
}

func (e *InvalidSignatureVerificationError) Error() string {
	return fmt.Sprintf(
		"invalid signature: signature could not be verified using public key with index %d on account %s",
		e.KeyIndex,
		e.Address,
	)
}

func (e *InvalidSignatureVerificationError) Code() uint32 {
	return errCodeInvalidSignatureVerification
}

// A InvalidProposalKeyPublicKeyDoesNotExistError indicates that proposal key specifies a nonexistent public key.
type InvalidProposalKeyPublicKeyDoesNotExistError struct {
	Address  flow.Address
	KeyIndex uint64
}

func (e *InvalidProposalKeyPublicKeyDoesNotExistError) Error() string {
	return fmt.Sprintf(
		"invalid proposal key: public key with index %d does not exist on account %s",
		e.KeyIndex,
		e.Address,
	)
}

func (e *InvalidProposalKeyPublicKeyDoesNotExistError) Code() uint32 {
	return errCodeInvalidProposalKeyPublicKeyDoesNotExist
}

// An InvalidProposalKeyPublicKeyRevokedError indicates that proposal key sequence number does not match the on-chain value.
type InvalidProposalKeyPublicKeyRevokedError struct {
	Address  flow.Address
	KeyIndex uint64
}

func (e *InvalidProposalKeyPublicKeyRevokedError) Error() string {
	return fmt.Sprintf(
		"invalid proposal key: public key with index %d on account %s has been revoked",
		e.KeyIndex,
		e.Address,
	)
}

func (e *InvalidProposalKeyPublicKeyRevokedError) Code() uint32 {
	return errCodeInvalidProposalKeyPublicKeyRevoked
}

// An InvalidProposalKeySequenceNumberError indicates that proposal key sequence number does not match the on-chain value.
type InvalidProposalKeySequenceNumberError struct {
	Address           flow.Address
	KeyIndex          uint64
	CurrentSeqNumber  uint64
	ProvidedSeqNumber uint64
}

func (e *InvalidProposalKeySequenceNumberError) Error() string {
	return fmt.Sprintf(
		"invalid proposal key: public key %d on account %s has sequence number %d, but given %d",
		e.KeyIndex,
		e.Address,
		e.CurrentSeqNumber,
		e.ProvidedSeqNumber,
	)
}

func (e *InvalidProposalKeySequenceNumberError) Code() uint32 {
	return errCodeInvalidProposalKeySequenceNumber
}

// A InvalidProposalKeyMissingSignatureError indicates that a proposal key does not have a valid signature.
type InvalidProposalKeyMissingSignatureError struct {
	Address  flow.Address
	KeyIndex uint64
}

func (e *InvalidProposalKeyMissingSignatureError) Error() string {
	return fmt.Sprintf(
		"invalid proposal key: public key %d on account %s does not have a valid signature",
		e.KeyIndex,
		e.Address,
	)
}

func (e *InvalidProposalKeyMissingSignatureError) Code() uint32 {
	return errCodeInvalidProposalKeyMissingSignature
}

// An InvalidHashAlgorithmError indicates that a given key has an invalid hash algorithm.
type InvalidHashAlgorithmError struct {
	Address  flow.Address
	KeyIndex uint64
	HashAlgo hash.HashingAlgorithm
}

func (e *InvalidHashAlgorithmError) Error() string {
	return fmt.Sprintf("invalid hash algorithm %d for key %d on account %s", e.HashAlgo, e.KeyIndex, e.Address)
}

func (e *InvalidHashAlgorithmError) Code() uint32 {
	return errCodeInvalidHashAlgorithm
}

type ExecutionError struct {
	Err runtime.Error
}

func (e *ExecutionError) Error() string {
	return e.Err.Error()
}

func (e *ExecutionError) Code() uint32 {
	return errCodeExecution
}

func handleError(err error) (vmErr Error, fatalErr error) {
	switch typedErr := err.(type) {
	case runtime.Error:
		// If the error originated from the runtime, handle separately
		return handleRuntimeError(typedErr)
	case Error:
		// If the error is an fvm.Error, return as is
		return typedErr, nil
	default:
		// All other errors are considered fatal
		return nil, err
	}
}

func handleRuntimeError(err runtime.Error) (vmErr Error, fatalErr error) {
	innerErr := err.Err

	// External errors are reported by the runtime but originate from the VM.
	//
	// External errors may be fatal or non-fatal, so additional handling
	// is required.
	if externalErr, ok := innerErr.(interpreter.ExternalError); ok {
		if recoveredErr, ok := externalErr.Recovered.(error); ok {
			// If the recovered value is an error, pass it to the original
			// error handler to distinguish between fatal and non-fatal errors.
			return handleError(recoveredErr)
		}

		// If the recovered value is not an error, bubble up the panic.
		panic(externalErr.Recovered)
	}

	// All other errors are non-fatal Cadence errors.
	return &ExecutionError{Err: err}, nil
}
