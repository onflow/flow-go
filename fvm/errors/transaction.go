package errors

import (
	"fmt"

	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/model/flow"
)

// Notes (ramtin)
// when runtime errors are retured, we check the internal errors and if
// type is external means that we have cause the error in the first place
// probably inside the env (so we put it back???)

const (
	// tx validation errors
	errCodeInvalidTxByteSizeError     = 1
	errCodeInvalidReferenceBlockError = 2
	errCodeExpiredTransactionError    = 3
	errCodeInvalidScriptError         = 4
	errCodeInvalidGasLimitError       = 5
	errCodeInvalidAddressError        = 6
	errCodeInvalidArgumentError       = 7

	// execution errors
	errCodeProposalMissingSignatureError  = 50
	errCodeProposalSeqNumberMismatchError = 51

	errCodePayloadSignatureError           = 60
	errCodePayloadSignatureKeyError        = 61
	errCodeRevokedPayloadSignatureKeyError = 62

	errCodeEnvelopeSignatureError           = 70
	errCodeEnvelopeSignatureKeyError        = 71
	errCodeRevokedEnvelopeSignatureKeyError = 72

	errCodeAuthorizationError = 80

	errCodeCadenceRunTimeError = 100

	errCodeInsufficientStorageError              = 200
	errCodeInsufficientTokenBalanceError         = 201
	errCodeMaxGasExceededError                   = 202
	errCodeMaxEventLimitExceededError            = 203
	errCodeMaxLedgerIntractionLimitExceededError = 204
)

// TransactionValidationError captures a transaction validation error
// A transaction having this error (in most cases) is rejected by access/collection nodes
// and later in the pipeline be verified by execution and verification nodes.
type TransactionValidationError interface {
	// Code returns the code for this error
	Code() uint32
	// Error returns an string describing the details of the error
	Error() string
	// and anything else that an error needs
	error
}

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

func (e InvalidScriptError) Code() uint32 {
	return errCodeInvalidScriptError
}

func (e InvalidScriptError) Unwrap() error {
	return e.ParserErr
}

// InvalidGasLimitError indicates that a transaction specifies a gas limit that exceeds the maximum allowed by the network.
type InvalidGasLimitError struct {
	Maximum uint64
	Actual  uint64
}

func (e InvalidGasLimitError) Code() uint32 {
	return errCodeInvalidGasLimitError
}

func (e InvalidGasLimitError) Error() string {
	return fmt.Sprintf("transaction gas limit (%d) exceeds the maximum gas limit (%d)", e.Actual, e.Maximum)
}

// InvalidAddressError indicates that a transaction references an invalid flow Address
// in either the Authorizers or Payer field.
type InvalidAddressError struct {
	Address flow.Address
}

func (e InvalidAddressError) Code() uint32 {
	return errCodeInvalidAddressError
}

func (e InvalidAddressError) Error() string {
	return fmt.Sprintf("invalid address: %s", e.Address)
}

// InvalidArgumentError indicates that a transaction includes invalid arguments.
// this error is the result of failure in any of the following conditions:
// - number of arguments doesn't match the template
// TODO add more cases like argument size
type InvalidArgumentError struct {
	Issue string
}

func (e InvalidArgumentError) Code() uint32 {
	return errCodeInvalidArgumentError
}

func (e InvalidArgumentError) Error() string {
	return fmt.Sprintf("transaction arguments are invalid: (%s)", e.Issue)
}

// TxExecutionError captures errors when executing a transaction.
// A transaction having this error has already passed validation and is included in a collection.
// the transaction will be executed by execution nodes but the result is reverted
// and in some cases there will be a penalty (or fees) for the payer, access nodes or collection nodes.
type TransactionExecutionError interface {
	// TxHash returns the hash of the transaction content
	TxHash() flow.Identifier
	// Code returns the code for this error
	Code() uint32
	// Error returns an string describing the details of the error
	Error() string
}

// ProposalMissingSignatureError indicates that no valid signature is provided for the proposal key.
type ProposalMissingSignatureError struct {
	txHash   flow.Identifier
	address  flow.Address
	keyIndex uint64
}

func (e *ProposalMissingSignatureError) TxHash() flow.Identifier {
	return e.txHash
}

func (e *ProposalMissingSignatureError) Code() uint32 {
	return errCodeProposalMissingSignatureError
}

func (e *ProposalMissingSignatureError) Error() string {
	return fmt.Sprintf(
		"invalid proposal key: public key %d on account %s does not have a valid signature",
		e.keyIndex,
		e.address,
	)
}

// ProposalSeqNumberMismatchError indicates that proposal key sequence number does not match the on-chain value.
type ProposalSeqNumberMismatchError struct {
	txHash            flow.Identifier
	Address           flow.Address
	KeyIndex          uint64
	CurrentSeqNumber  uint64
	ProvidedSeqNumber uint64
}

func (e *ProposalSeqNumberMismatchError) TxHash() flow.Identifier {
	return e.txHash
}

func (e *ProposalSeqNumberMismatchError) Code() uint32 {
	return errCodeProposalSeqNumberMismatchError
}

func (e *ProposalSeqNumberMismatchError) Error() string {
	return fmt.Sprintf(
		"invalid proposal key: public key %d on account %s has sequence number %d, but given %d",
		e.KeyIndex,
		e.Address,
		e.CurrentSeqNumber,
		e.ProvidedSeqNumber,
	)
}

// PayloadSignatureError indicates that signature verification for a key in this transaction has failed.
// this error is the result of failure in any of the following conditions:
// - provided hashing method is not supported
// - signature size is wrong
// - signature verification failed
// - public key doesn't match the one in the signature
type PayloadSignatureError struct {
	txHash   flow.Identifier
	Address  flow.Address
	KeyIndex uint64
}

func (e *PayloadSignatureError) TxHash() flow.Identifier {
	return e.txHash
}

func (e *PayloadSignatureError) Code() uint32 {
	return errCodePayloadSignatureError
}

func (e *PayloadSignatureError) Error() string {
	return fmt.Sprintf(
		"invalid proposal key: public key %d on account %s does not have a valid signature",
		e.KeyIndex,
		e.Address,
	)
}

// PayloadSignatureKeyError indicates an issue with a payload key in the transaction.
// this error is the result of failure in any of the following conditions:
// - keyIndex doesn't exist at this address
type PayloadSignatureKeyError struct {
	txHash   flow.Identifier
	Address  flow.Address
	KeyIndex uint64
}

func (e *PayloadSignatureKeyError) TxHash() flow.Identifier {
	return e.txHash
}

func (e *PayloadSignatureKeyError) Code() uint32 {
	return errCodePayloadSignatureKeyError
}

func (e *PayloadSignatureKeyError) Error() string {
	return fmt.Sprintf(
		"invalid payload key: key index %d doesn't exist on account %s",
		e.KeyIndex,
		e.Address,
	)
}

// RevokedPayloadSignatureKeyError indicates a transaction payload key is revoked.
// this error is the result of failure in any of the following conditions:
// - key Index is revoked from this account
// TODO maybe merge with the one above
type RevokedPayloadSignatureKeyError struct {
	txHash   flow.Identifier
	Address  flow.Address
	KeyIndex uint64
}

func (e *RevokedPayloadSignatureKeyError) TxHash() flow.Identifier {
	return e.txHash
}

func (e *RevokedPayloadSignatureKeyError) Code() uint32 {
	return errCodeRevokedPayloadSignatureKeyError
}

func (e *RevokedPayloadSignatureKeyError) Error() string {
	return fmt.Sprintf(
		"invalid payload key: key index %d doesn't exist on account %s",
		e.KeyIndex,
		e.Address,
	)
}

// EnvelopeSignatureError indicates that signature verification for a envelope key in this transaction has failed.
// this error is the result of failure in any of the following conditions:
// - provided hashing method is not supported
// - signature size is wrong
// - signature verification failed
// - public key doesn't match the one in the signature
type EnvelopeSignatureError struct {
	txHash   flow.Identifier
	Address  flow.Address
	KeyIndex uint64
}

func (e *EnvelopeSignatureError) TxHash() flow.Identifier {
	return e.txHash
}

func (e *EnvelopeSignatureError) Code() uint32 {
	return errCodeEnvelopeSignatureError
}

func (e *EnvelopeSignatureError) Error() string {
	return fmt.Sprintf(
		"invalid envelope key: public key %d on account %s does not have a valid signature",
		e.KeyIndex,
		e.Address,
	)
}

// EnvelopeSignatureKeyError indicates an issue with a envelope key in the transaction.
// this error is the result of failure in any of the following conditions:
// - keyIndex doesn't exist at this address
type EnvelopeSignatureKeyError struct {
	txHash   flow.Identifier
	Address  flow.Address
	KeyIndex uint64
}

func (e *EnvelopeSignatureKeyError) TxHash() flow.Identifier {
	return e.txHash
}

func (e *EnvelopeSignatureKeyError) Code() uint32 {
	return errCodeEnvelopeSignatureKeyError
}

func (e *EnvelopeSignatureKeyError) Error() string {
	return fmt.Sprintf(
		"invalid envelope key: key index %d doesn't exist on account %s",
		e.KeyIndex,
		e.Address,
	)
}

// RevokedEnvelopeSignatureKeyError indicates a transaction payload key is revoked.
// this error is the result of failure in any of the following conditions:
// - key Index is revoked from this account
// TODO maybe merge with the one above
type RevokedEnvelopeSignatureKeyError struct {
	txHash   flow.Identifier
	Address  flow.Address
	KeyIndex uint64
}

func (e *RevokedEnvelopeSignatureKeyError) TxHash() flow.Identifier {
	return e.txHash
}

func (e *RevokedEnvelopeSignatureKeyError) Code() uint32 {
	return errCodeRevokedEnvelopeSignatureKeyError
}

func (e *RevokedEnvelopeSignatureKeyError) Error() string {
	return fmt.Sprintf(
		"invalid envelope key: key index %d doesn't exist on account %s",
		e.KeyIndex,
		e.Address,
	)
}

// AuthorizationError indicates that a transaction is missing a required signature to
// authorize access to an account.
// this error is the result of failure in any of the following conditions:
// - no signature provided for an account
// - not enough key weight in total for this account
type AuthorizationError struct {
	txHash       flow.Identifier
	Address      flow.Address
	SignedWeight uint32
}

func (e *AuthorizationError) TxHash() flow.Identifier {
	return e.txHash
}

func (e *AuthorizationError) Code() uint32 {
	return errCodeAuthorizationError
}

func (e *AuthorizationError) Error() string {
	return fmt.Sprintf(
		"account %s does not have sufficient signatures (unauthorized access)",
		e.Address,
	)
}

// CadenceError captures a collection of errors provided by cadence runtime
// it cover cadence errors such as
// NotDeclaredError, NotInvokableError, ArgumentCountError, TransactionNotDeclaredError,
// ConditionError, RedeclarationError, DereferenceError,
// OverflowError, UnderflowError, DivisionByZeroError,
// DestroyedCompositeError,  ForceAssignmentToNonNilResourceError, ForceNilError,
// TypeMismatchError, InvalidPathDomainError, OverwriteError, CyclicLinkError,
// ArrayIndexOutOfBoundsError, ...
type CadenceRunTimeError struct {
	txHash flow.Identifier
	error  runtime.Error
}

// TODO add unwrap

// account doesn't have enough storage
type InsufficientStorageError struct {
	txHash   flow.Identifier
	Capacity uint64
	Used     uint64
	Needed   uint64
}

func (e InsufficientStorageError) TxHash() flow.Identifier {
	return e.txHash
}

func (e InsufficientStorageError) Error() string {
	return fmt.Sprintf("insufficient account storage, available space is %d but %d is needed %s", e.Capacity, e.Used, e.Needed)
}

func (e InsufficientStorageError) Code() uint32 {
	return errCodeInsufficientStorageError
}

// - if user can pay for the tx fees
type InsufficientTokenBalanceError struct {
}

type MaxGasExceededError struct {
}

type MaxEventLimitExceededError struct {
}

type MaxLedgerIntractionLimitExceededError struct {
}
