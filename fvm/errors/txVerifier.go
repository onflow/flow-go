package errors

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// InvalidProposalSignatureError indicates that no valid signature is provided for the proposal key.
type InvalidProposalSignatureError struct {
	errorWrapper

	address  flow.Address
	keyIndex uint64
}

// NewInvalidProposalSignatureError constructs a new InvalidProposalSignatureError
func NewInvalidProposalSignatureError(address flow.Address, keyIndex uint64, err error) InvalidProposalSignatureError {
	return InvalidProposalSignatureError{
		address:  address,
		keyIndex: keyIndex,
		errorWrapper: errorWrapper{
			err: err,
		},
	}
}

func (e InvalidProposalSignatureError) Error() string {
	return fmt.Sprintf(
		"%s invalid proposal key: public key %d on account %s does not have a valid signature: %s",
		e.Code().String(),
		e.keyIndex,
		e.address,
		e.err.Error(),
	)
}

// Code returns the error code for this error type
func (e InvalidProposalSignatureError) Code() ErrorCode {
	return ErrCodeInvalidProposalSignatureError
}

// InvalidProposalSeqNumberError indicates that proposal key sequence number does not match the on-chain value.
type InvalidProposalSeqNumberError struct {
	address           flow.Address
	keyIndex          uint64
	currentSeqNumber  uint64
	providedSeqNumber uint64
}

// NewInvalidProposalSeqNumberError constructs a new InvalidProposalSeqNumberError
func NewInvalidProposalSeqNumberError(address flow.Address, keyIndex uint64, currentSeqNumber uint64, providedSeqNumber uint64) InvalidProposalSeqNumberError {
	return InvalidProposalSeqNumberError{address: address,
		keyIndex:          keyIndex,
		currentSeqNumber:  currentSeqNumber,
		providedSeqNumber: providedSeqNumber}
}

// CurrentSeqNumber returns the current sequence number
func (e InvalidProposalSeqNumberError) CurrentSeqNumber() uint64 {
	return e.currentSeqNumber
}

// ProvidedSeqNumber returns the provided sequence number
func (e InvalidProposalSeqNumberError) ProvidedSeqNumber() uint64 {
	return e.providedSeqNumber
}

func (e InvalidProposalSeqNumberError) Error() string {
	return fmt.Sprintf(
		"%s invalid proposal key: public key %d on account %s has sequence number %d, but given %d",
		e.Code().String(),
		e.keyIndex,
		e.address.String(),
		e.currentSeqNumber,
		e.providedSeqNumber,
	)
}

// Code returns the error code for this error type
func (e InvalidProposalSeqNumberError) Code() ErrorCode {
	return ErrCodeInvalidProposalSeqNumberError
}

// InvalidPayloadSignatureError indicates that signature verification for a key in this transaction has failed.
// this error is the result of failure in any of the following conditions:
// - provided hashing method is not supported
// - signature size or format is invalid
// - signature verification failed
type InvalidPayloadSignatureError struct {
	errorWrapper

	address  flow.Address
	keyIndex uint64
}

// NewInvalidPayloadSignatureError constructs a new InvalidPayloadSignatureError
func NewInvalidPayloadSignatureError(address flow.Address, keyIndex uint64, err error) InvalidPayloadSignatureError {
	return InvalidPayloadSignatureError{
		address:  address,
		keyIndex: keyIndex,
		errorWrapper: errorWrapper{
			err: err,
		},
	}
}

func (e InvalidPayloadSignatureError) Error() string {
	return fmt.Sprintf(
		"%s invalid payload signature: public key %d on account %s does not have a valid signature: %s",
		e.Code().String(),
		e.keyIndex,
		e.address,
		e.err.Error(),
	)
}

// Code returns the error code for this error type
func (e InvalidPayloadSignatureError) Code() ErrorCode {
	return ErrCodeInvalidPayloadSignatureError
}

func IsInvalidPayloadSignatureError(err error) bool {
	var t InvalidPayloadSignatureError
	return As(err, &t)
}

// InvalidEnvelopeSignatureError indicates that signature verification for a envelope key in this transaction has failed.
// this error is the result of failure in any of the following conditions:
// - provided hashing method is not supported
// - signature size or format is invalid
// - signature verification failed
type InvalidEnvelopeSignatureError struct {
	errorWrapper

	address  flow.Address
	keyIndex uint64
}

// NewInvalidEnvelopeSignatureError constructs a new InvalidEnvelopeSignatureError
func NewInvalidEnvelopeSignatureError(address flow.Address, keyIndex uint64, err error) InvalidEnvelopeSignatureError {
	return InvalidEnvelopeSignatureError{
		address:  address,
		keyIndex: keyIndex,
		errorWrapper: errorWrapper{
			err: err,
		},
	}
}

func (e InvalidEnvelopeSignatureError) Error() string {
	return fmt.Sprintf(
		"%s invalid envelope key: public key %d on account %s does not have a valid signature: %s",
		e.Code().String(),
		e.keyIndex,
		e.address,
		e.err.Error(),
	)
}

// Code returns the error code for this error type
func (e InvalidEnvelopeSignatureError) Code() ErrorCode {
	return ErrCodeInvalidEnvelopeSignatureError
}

func IsInvalidEnvelopeSignatureError(err error) bool {
	var t InvalidEnvelopeSignatureError
	return As(err, &t)
}
