package fvm

import (
	"fmt"

	"github.com/onflow/cadence/runtime"

	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/model/flow"
)

const InvalidSignaturePublicKeyErrorCode = 1
const MissingSignatureErrorCode = 2
const MissingPayerErrorCode = 3
const SignatureAccountDoesNotExistErrorCode = 4
const InvalidHashingAlgorithmErrorCode = 5
const PublicKeyVerificationErrorCode = 6
const InvalidProposalSequenceNumberErrorCode = 7
const MissingSignatureForProposalKeyErrorCode = 8
const CodeExecutionErrorCode = 9
const InvalidProposalKeyErrorCode = 10
const SignatureAccountKeyDoesNotExistErrorCode = 11

// FlowError represents an expected error inside Flow. Not to be confused with standard Go errors which
// indicate exceptional situation. FlowErrors are really values which should be returned to a client.
type FlowError interface {
	StatusCode() uint32
	ErrorMessage() string
}

// An InvalidSignaturePublicKeyError indicates that signature uses an invalid public key.
type InvalidSignaturePublicKeyError struct {
	Address flow.Address
	KeyID   uint64
}

func (e *InvalidSignaturePublicKeyError) ErrorMessage() string {
	return fmt.Sprintf("invalid signature for key %d on account %s", e.KeyID, e.Address)
}
func (e *InvalidSignaturePublicKeyError) StatusCode() uint32 {
	return InvalidSignaturePublicKeyErrorCode
}

// A MissingSignatureError indicates that a transaction is missing a required signature.
type MissingSignatureError struct {
	Address flow.Address
}

func (e *MissingSignatureError) ErrorMessage() string {
	return fmt.Sprintf("account %s does not have sufficient signatures", e.Address)
}

func (e *MissingSignatureError) StatusCode() uint32 {
	return MissingSignatureErrorCode
}

// A MissingPayerError indicates that a transaction is missing a payer.
type MissingPayerError struct {
}

func (e *MissingPayerError) ErrorMessage() string {
	return "no payer address provided"
}

func (e *MissingPayerError) StatusCode() uint32 {
	return MissingPayerErrorCode
}

// An SignatureAccountDoesNotExist indicates that a signature references a nonexistent account.
type SignatureAccountDoesNotExist struct {
	Address flow.Address
}

func (e *SignatureAccountDoesNotExist) ErrorMessage() string {
	return fmt.Sprintf("account with address %s does not exist", e.Address)
}

func (e *SignatureAccountDoesNotExist) StatusCode() uint32 {
	return SignatureAccountDoesNotExistErrorCode
}

// An SignatureAccountKeyDoesNotExist indicates that a signature references a nonexistent account.
type SignatureAccountKeyDoesNotExist struct {
	Address flow.Address
	KeyID   uint64
}

// An InvalidHashingAlgorithmError indicates that given key has invalid key algorithm
type InvalidHashingAlgorithmError struct {
	Address          flow.Address
	KeyID            uint64
	HashingAlgorithm hash.HashingAlgorithm
}

func (e *InvalidHashingAlgorithmError) ErrorMessage() string {
	return fmt.Sprintf("invalid hashing algorithm %d for key %d on account %s", e.HashingAlgorithm, e.KeyID, e.Address)
}

func (e *InvalidHashingAlgorithmError) StatusCode() uint32 {
	return InvalidHashingAlgorithmErrorCode
}

// An PublicKeyVerificationError indicates unexpected error while verifying signature using given key
type PublicKeyVerificationError struct {
	Address flow.Address
	KeyID   uint64
	Err     error
}

func (e *PublicKeyVerificationError) ErrorMessage() string {
	return fmt.Sprintf("unexpected error while veryfing signature using key %d on account %s: %s", e.KeyID, e.Address, e.Err)
}

func (e *PublicKeyVerificationError) StatusCode() uint32 {
	return PublicKeyVerificationErrorCode
}

// An InvalidProposalSequenceNumberError indicates that proposal key sequence number does not match the on-chain value.
type InvalidProposalSequenceNumberError struct {
	Address           flow.Address
	KeyID             uint64
	CurrentSeqNumber  uint64
	ProvidedSeqNumber uint64
}

func (e *InvalidProposalSequenceNumberError) ErrorMessage() string {
	return fmt.Sprintf("invalid proposal key sequence number: key %d on account %s has sequence number %d, but given %d", e.KeyID, e.Address, e.CurrentSeqNumber, e.ProvidedSeqNumber)
}

func (e *InvalidProposalSequenceNumberError) StatusCode() uint32 {
	return InvalidProposalSequenceNumberErrorCode
}

// A MissingSignatureForProposalKeyError indicates that a transaction is missing a required signature for proposal key.
type MissingSignatureForProposalKeyError struct {
	Address flow.Address
	KeyID   uint64
}

func (e *MissingSignatureForProposalKeyError) ErrorMessage() string {
	return fmt.Sprintf("key %d on account %s does not have sufficient signatures for proposal key", e.KeyID, e.Address)
}

func (e *MissingSignatureForProposalKeyError) StatusCode() uint32 {
	return MissingSignatureForProposalKeyErrorCode
}

// A MissingSignatureForProposalKeyError indicates that a transaction is missing a required signature for proposal key.
type CodeExecutionError struct {
	RuntimeError runtime.Error
}

func (e *CodeExecutionError) ErrorMessage() string {
	return fmt.Sprintf("code execution failed: %s", e.RuntimeError.Error())
}

func (e *CodeExecutionError) StatusCode() uint32 {
	return CodeExecutionErrorCode
}

// An InvalidProposalKeyError indicates that proposal key references an invalid public key.
type InvalidProposalKeyError struct {
	Address flow.Address
	KeyID   uint64
}

func (e *InvalidProposalKeyError) ErrorMessage() string {
	return fmt.Sprintf("invalid proposal key %d on account %s", e.KeyID, e.Address)
}

func (e *InvalidProposalKeyError) StatusCode() uint32 {
	return InvalidProposalKeyErrorCode
}

func (e *SignatureAccountKeyDoesNotExist) ErrorMessage() string {
	return fmt.Sprintf("key %d on account with address %s does not exist", e.KeyID, e.Address)
}

func (e *SignatureAccountKeyDoesNotExist) StatusCode() uint32 {
	return SignatureAccountKeyDoesNotExistErrorCode
}
