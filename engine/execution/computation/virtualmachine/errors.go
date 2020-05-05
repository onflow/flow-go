package virtualmachine

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

// An InvalidSignaturePublicKeyError indicates that signature uses an invalid public key.
type InvalidSignaturePublicKeyError struct {
	Address flow.Address
	KeyID   uint64
}

func (e *InvalidSignaturePublicKeyError) Error() string {
	return fmt.Sprintf("invalid signature for key %d on account %s", e.KeyID, e.Address)
}

// A MissingSignatureError indicates that a transaction is missing a required signature.
type MissingSignatureError struct {
	Address flow.Address
}

func (e *MissingSignatureError) Error() string {
	return fmt.Sprintf("account %s does not have sufficient signatures", e.Address)
}

// A MissingPayerError indicates that a transaction is missing a payer.
type MissingPayerError struct {
}

func (e *MissingPayerError) Error() string {
	return fmt.Sprintf("no payer address provided")
}

// An InvalidSignatureAccountError indicates that a signature references a nonexistent account.
type InvalidSignatureAccountError struct {
	Address flow.Address
}

func (e *InvalidSignatureAccountError) Error() string {
	return fmt.Sprintf("account with address %s does not exist", e.Address)
}

// An InvalidProposalKeyError indicates that proposal key references an invalid public key.
type InvalidProposalKeyError struct {
	Address flow.Address
	KeyID   uint64
}

func (e *InvalidProposalKeyError) Error() string {
	return fmt.Sprintf("invalid proposal key %d on account %s", e.KeyID, e.Address)
}

// An InvalidProposalSequenceNumberError indicates that proposal key sequence number does not match the on-chain value.
type InvalidProposalSequenceNumberError struct {
	Address           flow.Address
	KeyID             uint64
	CurrentSeqNumber  uint64
	ProvidedSeqNumber uint64
}

func (e *InvalidProposalSequenceNumberError) Error() string {
	return fmt.Sprintf("invalid proposal key sequence number: key %d on account %s has sequence number %d, but given %d", e.KeyID, e.Address, e.CurrentSeqNumber, e.ProvidedSeqNumber)
}
