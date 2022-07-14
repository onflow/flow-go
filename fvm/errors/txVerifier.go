package errors

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// InvalidTxByteSizeError indicates that a transaction byte size exceeds the maximum limit.
// this error is the result of failure in any of the following conditions:
// - the total tx byte size is bigger than the limit set by the network
type InvalidTxByteSizeError struct {
	txByteSize uint64
	maximum    uint64
}

// NewInvalidTxByteSizeError constructs a new InvalidTxByteSizeError
func NewInvalidTxByteSizeError(txByteSize, maximum uint64) *InvalidTxByteSizeError {
	return &InvalidTxByteSizeError{txByteSize: txByteSize, maximum: maximum}
}

func (e InvalidTxByteSizeError) Error() string {
	return fmt.Sprintf("%s transaction byte size (%d) exceeds the maximum byte size allowed for a transaction (%d)", e.Code().String(), e.txByteSize, e.maximum)
}

// Code returns the error code for this error type
func (e InvalidTxByteSizeError) Code() ErrorCode {
	return ErrCodeInvalidTxByteSizeError
}

// InvalidReferenceBlockError indicates that the transaction's ReferenceBlockID is not acceptable.
// this error is the result of failure in any of the following conditions:
// - ReferenceBlockID refer to a non-existing block
// - ReferenceBlockID == ZeroID (if configured by the network)
type InvalidReferenceBlockError struct {
	referenceBlockID string
}

// NewInvalidReferenceBlockError constructs a new InvalidReferenceBlockError
func NewInvalidReferenceBlockError(referenceBlockID string) *InvalidReferenceBlockError {
	return &InvalidReferenceBlockError{referenceBlockID: referenceBlockID}
}

func (e InvalidReferenceBlockError) Error() string {
	return fmt.Sprintf("%s reference block is pointing to an invalid block: %s", e.Code().String(), e.referenceBlockID)
}

// Code returns the error code for this error type
func (e InvalidReferenceBlockError) Code() ErrorCode {
	return ErrCodeInvalidReferenceBlockError
}

// ExpiredTransactionError indicates that a transaction has expired.
// this error is the result of failure in any of the following conditions:
// - ReferenceBlock.Height - CurrentBlock.Height < Expiry Limit (Transaction is Expired)
type ExpiredTransactionError struct {
	refHeight, finalHeight uint64
}

// NewExpiredTransactionError constructs a new ExpiredTransactionError
func NewExpiredTransactionError(refHeight, finalHeight uint64) *ExpiredTransactionError {
	return &ExpiredTransactionError{refHeight: refHeight, finalHeight: finalHeight}
}

func (e ExpiredTransactionError) Error() string {
	return fmt.Sprintf("%s transaction is expired: ref_height=%d final_height=%d", e.Code().String(), e.refHeight, e.finalHeight)
}

// Code returns the error code for this error type
func (e ExpiredTransactionError) Code() ErrorCode {
	return ErrCodeInvalidReferenceBlockError
}

// InvalidScriptError indicates that a transaction contains an invalid Cadence script.
// this error is the result of failure in any of the following conditions:
// - script is empty
// - script can not be parsed by the cadence parser
// - comment-only script, len(program.Declarations) == 0
type InvalidScriptError struct {
	err error
}

// NewInvalidScriptError constructs a new InvalidScriptError
func NewInvalidScriptError(err error) *InvalidScriptError {
	return &InvalidScriptError{err: err}
}

func (e InvalidScriptError) Error() string {
	return fmt.Sprintf("%s failed to parse transaction Cadence script: %s", e.Code().String(), e.err.Error())
}

// Code returns the error code for this error type
func (e InvalidScriptError) Code() ErrorCode {
	return ErrCodeInvalidScriptError
}

// Unwrap unwraps the error
func (e InvalidScriptError) Unwrap() error {
	return e.err
}

// InvalidGasLimitError indicates that a transaction specifies a gas limit that exceeds the maximum allowed by the network.
type InvalidGasLimitError struct {
	actual  uint64
	maximum uint64
}

// NewInvalidGasLimitError constructs a new InvalidGasLimitError
func NewInvalidGasLimitError(actual, maximum uint64) *InvalidGasLimitError {
	return &InvalidGasLimitError{actual: actual, maximum: maximum}
}

func (e InvalidGasLimitError) Error() string {
	return fmt.Sprintf("%s transaction gas limit (%d) exceeds the maximum gas limit (%d)", e.Code().String(), e.actual, e.maximum)
}

// Code returns the error code for this error type
func (e InvalidGasLimitError) Code() ErrorCode {
	return ErrCodeInvalidGasLimitError
}

// InvalidProposalSignatureError indicates that no valid signature is provided for the proposal key.
type InvalidProposalSignatureError struct {
	address  flow.Address
	keyIndex uint64
	err      error
}

// NewInvalidProposalSignatureError constructs a new InvalidProposalSignatureError
func NewInvalidProposalSignatureError(address flow.Address, keyIndex uint64, err error) *InvalidProposalSignatureError {
	return &InvalidProposalSignatureError{address: address, keyIndex: keyIndex, err: err}
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

// Unwrap unwraps the error
func (e InvalidProposalSignatureError) Unwrap() error {
	return e.err
}

// InvalidProposalSeqNumberError indicates that proposal key sequence number does not match the on-chain value.
type InvalidProposalSeqNumberError struct {
	address           flow.Address
	keyIndex          uint64
	currentSeqNumber  uint64
	providedSeqNumber uint64
}

// NewInvalidProposalSeqNumberError constructs a new InvalidProposalSeqNumberError
func NewInvalidProposalSeqNumberError(address flow.Address, keyIndex uint64, currentSeqNumber uint64, providedSeqNumber uint64) *InvalidProposalSeqNumberError {
	return &InvalidProposalSeqNumberError{address: address,
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
	address  flow.Address
	keyIndex uint64
	err      error
}

// NewInvalidPayloadSignatureError constructs a new InvalidPayloadSignatureError
func NewInvalidPayloadSignatureError(address flow.Address, keyIndex uint64, err error) *InvalidPayloadSignatureError {
	return &InvalidPayloadSignatureError{address: address, keyIndex: keyIndex, err: err}
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

// Unwrap unwraps the error
func (e InvalidPayloadSignatureError) Unwrap() error {
	return e.err
}

// InvalidEnvelopeSignatureError indicates that signature verification for a envelope key in this transaction has failed.
// this error is the result of failure in any of the following conditions:
// - provided hashing method is not supported
// - signature size or format is invalid
// - signature verification failed
type InvalidEnvelopeSignatureError struct {
	address  flow.Address
	keyIndex uint64
	err      error
}

// NewInvalidEnvelopeSignatureError constructs a new InvalidEnvelopeSignatureError
func NewInvalidEnvelopeSignatureError(address flow.Address, keyIndex uint64, err error) *InvalidEnvelopeSignatureError {
	return &InvalidEnvelopeSignatureError{address: address, keyIndex: keyIndex, err: err}
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

// Unwrap unwraps the error
func (e InvalidEnvelopeSignatureError) Unwrap() error {
	return e.err
}
