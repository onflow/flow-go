package virtualmachine

import (
	"fmt"

	"github.com/onflow/cadence/runtime"

	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/model/flow"
)

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
	return 1
}

// A MissingSignatureError indicates that a transaction is missing a required signature.
type MissingSignatureError struct {
	Address flow.Address
}

func (e *MissingSignatureError) ErrorMessage() string {
	return fmt.Sprintf("account %s does not have sufficient signatures", e.Address)
}

func (e *MissingSignatureError) StatusCode() uint32 {
	return 2
}

// A MissingPayerError indicates that a transaction is missing a payer.
type MissingPayerError struct {
}

func (e *MissingPayerError) ErrorMessage() string {
	return "no payer address provided"
}

func (e *MissingPayerError) StatusCode() uint32 {
	return 3
}

// An InvalidSignatureAccountError indicates that a signature references a nonexistent account.
type InvalidSignatureAccountError struct {
	Address flow.Address
}

func (e *InvalidSignatureAccountError) ErrorMessage() string {
	return fmt.Sprintf("account with address %s does not exist", e.Address)
}

func (e *InvalidSignatureAccountError) StatusCode() uint32 {
	return 4
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
	return 4
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
	return 5
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
	return 6
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
	return 7
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
	return 8
}

// A MissingSignatureForProposalKeyError indicates that a transaction is missing a required signature for proposal key.
type CodeExecutionError struct {
	RuntimeError runtime.Error
}

func (e *CodeExecutionError) ErrorMessage() string {
	return fmt.Sprintf("code execution failed: %s", e.RuntimeError.Error())
}

func (e *CodeExecutionError) StatusCode() uint32 {
	return 9
}
