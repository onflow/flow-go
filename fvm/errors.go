package fvm

import (
	"errors"
	"fmt"

	"github.com/onflow/cadence/runtime"

	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/model/flow"
)

const (
	errCodeInvalidSignaturePublicKey = 1
	errCodeMissingSignature          = 2
	errCodeMissingPayer              = 3
	errCodeInvalidSignatureAccount   = 4

	errCodeProposalKeyDoesNotExist          = 5
	errCodeProposalKeyInvalidSequenceNumber = 6
	errCodeProposalKeyMissingSignature      = 7

	errCodeInvalidHashAlgorithm  = 8
	errCodePublicKeyVerification = 9

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

// An InvalidSignaturePublicKeyError indicates that signature uses an invalid public key.
type InvalidSignaturePublicKeyError struct {
	Address flow.Address
	KeyID   uint64
}

func (e *InvalidSignaturePublicKeyError) Error() string {
	return fmt.Sprintf("invalid signature for key %d on account %s", e.KeyID, e.Address)
}

func (e *InvalidSignaturePublicKeyError) Code() uint32 {
	return errCodeInvalidSignaturePublicKey
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
type MissingPayerError struct {
}

func (e *MissingPayerError) Error() string {
	return "no payer address provided"
}

func (e *MissingPayerError) Code() uint32 {
	return errCodeMissingPayer
}

// An InvalidSignatureAccountError indicates that a signature references a nonexistent account.
type InvalidSignatureAccountError struct {
	Address flow.Address
}

func (e *InvalidSignatureAccountError) Error() string {
	return fmt.Sprintf("account with address %s does not exist", e.Address)
}

func (e *InvalidSignatureAccountError) Code() uint32 {
	return errCodeInvalidSignatureAccount
}

// A ProposalKeyDoesNotExistError indicates that proposal key references a non-existent public key.
type ProposalKeyDoesNotExistError struct {
	Address flow.Address
	KeyID   uint64
}

func (e *ProposalKeyDoesNotExistError) Error() string {
	return fmt.Sprintf("invalid proposal key %d on account %s", e.KeyID, e.Address)
}

func (e *ProposalKeyDoesNotExistError) Code() uint32 {
	return errCodeProposalKeyDoesNotExist
}

// An InvalidHashAlgorithmError indicates that a given key has an invalid hash algorithm.
type InvalidHashAlgorithmError struct {
	Address  flow.Address
	KeyID    uint64
	HashAlgo hash.HashingAlgorithm
}

func (e *InvalidHashAlgorithmError) Error() string {
	return fmt.Sprintf("invalid hash algorithm %d for key %d on account %s", e.HashAlgo, e.KeyID, e.Address)
}

func (e *InvalidHashAlgorithmError) Code() uint32 {
	return errCodeInvalidHashAlgorithm
}

// An PublicKeyVerificationError indicates unexpected error while verifying signature using given key
type PublicKeyVerificationError struct {
	Address flow.Address
	KeyID   uint64
	Err     error
}

func (e *PublicKeyVerificationError) Error() string {
	return fmt.Sprintf("unexpected error while veryfing signature using key %d on account %s: %s", e.KeyID, e.Address, e.Err)
}

func (e *PublicKeyVerificationError) Code() uint32 {
	return errCodePublicKeyVerification
}

// An InvalidProposalKeySequenceNumberError indicates that proposal key sequence number does not match the on-chain value.
type InvalidProposalKeySequenceNumberError struct {
	Address           flow.Address
	KeyID             uint64
	CurrentSeqNumber  uint64
	ProvidedSeqNumber uint64
}

func (e *InvalidProposalKeySequenceNumberError) Error() string {
	return fmt.Sprintf("invalid proposal key sequence number: key %d on account %s has sequence number %d, but given %d", e.KeyID, e.Address, e.CurrentSeqNumber, e.ProvidedSeqNumber)
}

func (e *InvalidProposalKeySequenceNumberError) Code() uint32 {
	return errCodeProposalKeyInvalidSequenceNumber
}

// A MissingSignatureForProposalKeyError indicates that a transaction is missing a required signature for proposal key.
type MissingSignatureForProposalKeyError struct {
	Address flow.Address
	KeyID   uint64
}

func (e *MissingSignatureForProposalKeyError) Error() string {
	return fmt.Sprintf("key %d on account %s does not have sufficient signatures for proposal key", e.KeyID, e.Address)
}

func (e *MissingSignatureForProposalKeyError) Code() uint32 {
	return errCodeProposalKeyMissingSignature
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
