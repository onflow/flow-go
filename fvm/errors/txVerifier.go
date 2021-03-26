package errors

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// InvalidTxByteSizeError indicates that a transaction byte size exceeds the maximum limit.
// this error is the result of failure in any of the following conditions:
// - the total tx byte size is bigger than the limit set by the network
type InvalidTxByteSizeError struct {
	Maximum    uint64
	TxByteSize uint64
}

func (e InvalidTxByteSizeError) Error() string {
	return fmt.Sprintf("transaction byte size (%d) exceeds the maximum byte size allowed for a transaction (%d)", e.TxByteSize, e.Maximum)
}

// Code returns the error code for this error type
func (e InvalidTxByteSizeError) Code() uint32 {
	return errCodeInvalidTxByteSizeError
}

// InvalidReferenceBlockError indicates that the transaction's ReferenceBlockID is not acceptable.
// this error is the result of failure in any of the following conditions:
// - ReferenceBlockID refer to a non-existing block
// - ReferenceBlockID == ZeroID (if configured by the network)
type InvalidReferenceBlockError struct {
	ReferenceBlockID string
}

func (e InvalidReferenceBlockError) Error() string {
	return fmt.Sprintf("reference block is pointing to an invalid block: %s", e.ReferenceBlockID)
}

// Code returns the error code for this error type
func (e InvalidReferenceBlockError) Code() uint32 {
	return errCodeInvalidReferenceBlockError
}

// ExpiredTransactionError indicates that a transaction has expired.
// this error is the result of failure in any of the following conditions:
// - ReferenceBlock.Height - CurrentBlock.Height < Expiry Limit (Transaction is Expired)
type ExpiredTransactionError struct {
	RefHeight, FinalHeight uint64
}

func (e ExpiredTransactionError) Error() string {
	return fmt.Sprintf("transaction is expired: ref_height=%d final_height=%d", e.RefHeight, e.FinalHeight)
}

// Code returns the error code for this error type
func (e ExpiredTransactionError) Code() uint32 {
	return errCodeInvalidReferenceBlockError
}

// InvalidScriptError indicates that a transaction contains an invalid Cadence script.
// this error is the result of failure in any of the following conditions:
// - script is empty
// - script can not be parsed by the cadence parser
// - comment-only script, len(program.Declarations) == 0
type InvalidScriptError struct {
	ParserErr error
}

func (e InvalidScriptError) Error() string {
	return fmt.Sprintf("failed to parse transaction Cadence script: %s", e.ParserErr)
}

// Code returns the error code for this error type
func (e InvalidScriptError) Code() uint32 {
	return errCodeInvalidScriptError
}

// Unwrap unwraps the error
func (e InvalidScriptError) Unwrap() error {
	return e.ParserErr
}

// InvalidGasLimitError indicates that a transaction specifies a gas limit that exceeds the maximum allowed by the network.
type InvalidGasLimitError struct {
	Maximum uint64
	Actual  uint64
}

func (e InvalidGasLimitError) Error() string {
	return fmt.Sprintf("transaction gas limit (%d) exceeds the maximum gas limit (%d)", e.Actual, e.Maximum)
}

// Code returns the error code for this error type
func (e InvalidGasLimitError) Code() uint32 {
	return errCodeInvalidGasLimitError
}

// InvalidProposalSignatureError indicates that no valid signature is provided for the proposal key.
type InvalidProposalSignatureError struct {
	address  flow.Address
	keyIndex uint64
	err      error
}

// NewInvalidProposalSignatureError constructs a new InvalidProposalSignatureError
func NewInvalidProposalSignatureError(address flow.Address, keyIndex uint64, err error) error {
	return &InvalidProposalSignatureError{address: address, keyIndex: keyIndex, err: err}
}

func (e *InvalidProposalSignatureError) Error() string {
	return fmt.Sprintf(
		"invalid proposal key: public key %d on account %s does not have a valid signature: %s",
		e.keyIndex,
		e.address,
		e.err.Error(),
	)
}

// Code returns the error code for this error type
func (e *InvalidProposalSignatureError) Code() uint32 {
	return errCodeInvalidProposalSignatureError
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
func NewInvalidProposalSeqNumberError(address flow.Address, keyIndex uint64, currentSeqNumber uint64, providedSeqNumber uint64) error {
	return &InvalidProposalSeqNumberError{address: address,
		keyIndex:          keyIndex,
		currentSeqNumber:  currentSeqNumber,
		providedSeqNumber: providedSeqNumber}
}

// CurrentSeqNumber returns the current sequence number
func (e *InvalidProposalSeqNumberError) CurrentSeqNumber() uint64 {
	return e.currentSeqNumber
}

func (e *InvalidProposalSeqNumberError) Error() string {
	return fmt.Sprintf(
		"invalid proposal key: public key %d on account %s has sequence number %d, but given %d",
		e.keyIndex,
		e.address.String(),
		e.currentSeqNumber,
		e.providedSeqNumber,
	)
}

// Code returns the error code for this error type
func (e *InvalidProposalSeqNumberError) Code() uint32 {
	return errCodeProposalSeqNumberMismatchError
}

// InvalidPayloadSignatureError indicates that signature verification for a key in this transaction has failed.
// this error is the result of failure in any of the following conditions:
// - provided hashing method is not supported
// - signature size is wrong
// - signature verification failed
// - public key doesn't match the one in the signature
type InvalidPayloadSignatureError struct {
	address  flow.Address
	keyIndex uint64
	err      error
}

// NewInvalidPayloadSignatureError constructs a new InvalidPayloadSignatureError
func NewInvalidPayloadSignatureError(address flow.Address, keyIndex uint64, err error) error {
	return &InvalidPayloadSignatureError{address: address, keyIndex: keyIndex, err: err}
}

func (e *InvalidPayloadSignatureError) Error() string {
	return fmt.Sprintf(
		"invalid payload signature: public key %d on account %s does not have a valid signature: %s",
		e.keyIndex,
		e.address,
		e.err.Error(),
	)
}

// Code returns the error code for this error type
func (e *InvalidPayloadSignatureError) Code() uint32 {
	return errCodeInvalidPayloadSignatureError
}

// Unwrap unwraps the error
func (e InvalidPayloadSignatureError) Unwrap() error {
	return e.err
}

// InvalidEnvelopeSignatureError indicates that signature verification for a envelope key in this transaction has failed.
// this error is the result of failure in any of the following conditions:
// - provided hashing method is not supported
// - signature size is wrong
// - signature verification failed
// - public key doesn't match the one in the signature
type InvalidEnvelopeSignatureError struct {
	address  flow.Address
	keyIndex uint64
	err      error
}

// NewInvalidEnvelopeSignatureError constructs a new InvalidEnvelopeSignatureError
func NewInvalidEnvelopeSignatureError(address flow.Address, keyIndex uint64, err error) error {
	return &InvalidEnvelopeSignatureError{address: address, keyIndex: keyIndex, err: err}
}

func (e *InvalidEnvelopeSignatureError) Error() string {
	return fmt.Sprintf(
		"invalid envelope key: public key %d on account %s does not have a valid signature: %s",
		e.keyIndex,
		e.address,
		e.err.Error(),
	)
}

// Code returns the error code for this error type
func (e *InvalidEnvelopeSignatureError) Code() uint32 {
	return errCodeInvalidEnvelopeSignatureError
}

// Unwrap unwraps the error
func (e InvalidEnvelopeSignatureError) Unwrap() error {
	return e.err
}
