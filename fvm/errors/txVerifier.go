package errors

import (
	"github.com/onflow/flow-go/model/flow"
)

// NewInvalidProposalSignatureError constructs a new CodedError which indicates
// that no valid signature is provided for the proposal key.
func NewInvalidProposalSignatureError(
	address flow.Address,
	keyIndex uint64,
	err error,
) CodedError {
	return WrapCodedError(
		ErrCodeInvalidProposalSignatureError,
		err,
		"invalid proposal key: public key %d on account %s does not have a "+
			"valid signature",
		keyIndex,
		address)
}

// InvalidProposalSeqNumberError indicates that proposal key sequence number does not match the on-chain value.
type InvalidProposalSeqNumberError struct {
	currentSeqNumber  uint64
	providedSeqNumber uint64

	CodedError
}

// NewInvalidProposalSeqNumberError constructs a new InvalidProposalSeqNumberError
func NewInvalidProposalSeqNumberError(
	address flow.Address,
	keyIndex uint64,
	currentSeqNumber uint64,
	providedSeqNumber uint64,
) CodedError {
	return InvalidProposalSeqNumberError{
		currentSeqNumber:  currentSeqNumber,
		providedSeqNumber: providedSeqNumber,
		CodedError: NewCodedError(
			ErrCodeInvalidProposalSeqNumberError,
			"invalid proposal key: public key %d on account %s has sequence "+
				"number %d, but given %d",
			keyIndex,
			address.String(),
			currentSeqNumber,
			providedSeqNumber),
	}
}

// CurrentSeqNumber returns the current sequence number
func (e InvalidProposalSeqNumberError) CurrentSeqNumber() uint64 {
	return e.currentSeqNumber
}

// ProvidedSeqNumber returns the provided sequence number
func (e InvalidProposalSeqNumberError) ProvidedSeqNumber() uint64 {
	return e.providedSeqNumber
}

// NewInvalidPayloadSignatureError constructs a new CodedError which indicates
// that signature verification for a key in this transaction has failed. This
// error is the result of failure in any of the following conditions:
// - provided hashing method is not supported
// - signature size or format is invalid
// - signature verification failed
func NewInvalidPayloadSignatureError(
	address flow.Address,
	keyIndex uint64,
	err error,
) CodedError {
	return WrapCodedError(
		ErrCodeInvalidPayloadSignatureError,
		err,
		"invalid payload signature: public key %d on account %s does not have"+
			" a valid signature",
		keyIndex,
		address)
}

func IsInvalidPayloadSignatureError(err error) bool {
	return HasErrorCode(err, ErrCodeInvalidPayloadSignatureError)
}

// NewInvalidEnvelopeSignatureError constructs a new CodedError which indicates
// that signature verification for a envelope key in this transaction has
// failed. Tthis error is the result of failure in any of the following
// conditions:
// - provided hashing method is not supported
// - signature size or format is invalid
// - signature verification failed
func NewInvalidEnvelopeSignatureError(
	address flow.Address,
	keyIndex uint64,
	err error,
) CodedError {
	return WrapCodedError(
		ErrCodeInvalidEnvelopeSignatureError,
		err,
		"invalid envelope key: public key %d on account %s does not have a "+
			"valid signature",
		keyIndex,
		address)
}

func IsInvalidEnvelopeSignatureError(err error) bool {
	return HasErrorCode(err, ErrCodeInvalidEnvelopeSignatureError)
}
