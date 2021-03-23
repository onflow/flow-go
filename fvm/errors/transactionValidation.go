package errors

import (
	"fmt"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/flow"
)

// TransactionValidationError captures a transaction validation error
// A transaction having this error (in most cases) is rejected by access/collection nodes
// and later in the pipeline be verified by execution and verification nodes.
type TransactionValidationError interface {
	TransactionError
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

func (e InvalidTxByteSizeError) Is(target error) bool {
	_, ok := target.(*InvalidTxByteSizeError)
	return ok
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

func (e InvalidReferenceBlockError) Is(target error) bool {
	_, ok := target.(*InvalidReferenceBlockError)
	return ok
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

func (e ExpiredTransactionError) Is(target error) bool {
	_, ok := target.(*ExpiredTransactionError)
	return ok
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

func (e InvalidScriptError) Is(target error) bool {
	_, ok := target.(*InvalidScriptError)
	return ok
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

func (e InvalidGasLimitError) Is(target error) bool {
	_, ok := target.(*InvalidGasLimitError)
	return ok
}

// InvalidAddressError indicates that a transaction references an invalid flow Address
// in either the Authorizers or Payer field.
type InvalidAddressError struct {
	Address flow.Address
	Err     error
}

func (e InvalidAddressError) Code() uint32 {
	return errCodeInvalidAddressError
}

func (e InvalidAddressError) Error() string {
	return fmt.Sprintf("invalid address (%s): %s", e.Address, e.Err.Error())
}

func (e InvalidAddressError) Is(target error) bool {
	_, ok := target.(*InvalidAddressError)
	return ok
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

func (e InvalidArgumentError) Is(target error) bool {
	_, ok := target.(*InvalidArgumentError)
	return ok
}

// InvalidProposalSignatureError indicates that no valid signature is provided for the proposal key.
type InvalidProposalSignatureError struct {
	Address  flow.Address
	KeyIndex uint64
	Err      error
}

func (e *InvalidProposalSignatureError) Code() uint32 {
	return errCodeInvalidProposalSignatureError
}

func (e *InvalidProposalSignatureError) Error() string {
	return fmt.Sprintf(
		"invalid proposal key: public key %d on account %s does not have a valid signature: %s",
		e.KeyIndex,
		e.Address,
		e.Err.Error(),
	)
}

func (e InvalidProposalSignatureError) Is(target error) bool {
	_, ok := target.(*InvalidProposalSignatureError)
	return ok
}

// ProposalSeqNumberMismatchError indicates that proposal key sequence number does not match the on-chain value.
type ProposalSeqNumberMismatchError struct {
	Address           flow.Address
	KeyIndex          uint64
	CurrentSeqNumber  uint64
	ProvidedSeqNumber uint64
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

func (e ProposalSeqNumberMismatchError) Is(target error) bool {
	_, ok := target.(*ProposalSeqNumberMismatchError)
	return ok
}

// InvalidPayloadSignatureError indicates that signature verification for a key in this transaction has failed.
// this error is the result of failure in any of the following conditions:
// - provided hashing method is not supported
// - signature size is wrong
// - signature verification failed
// - public key doesn't match the one in the signature
type InvalidPayloadSignatureError struct {
	Address  flow.Address
	KeyIndex uint64
	Err      error
}

func (e *InvalidPayloadSignatureError) Code() uint32 {
	return errCodeInvalidPayloadSignatureError
}

func (e *InvalidPayloadSignatureError) Error() string {
	return fmt.Sprintf(
		"invalid payload signature: public key %d on account %s does not have a valid signature",
		e.KeyIndex,
		e.Address,
	)
}

func (e InvalidPayloadSignatureError) Is(target error) bool {
	_, ok := target.(*InvalidPayloadSignatureError)
	return ok
}

// InvalidEnvelopeSignatureError indicates that signature verification for a envelope key in this transaction has failed.
// this error is the result of failure in any of the following conditions:
// - provided hashing method is not supported
// - signature size is wrong
// - signature verification failed
// - public key doesn't match the one in the signature
type InvalidEnvelopeSignatureError struct {
	Address  flow.Address
	KeyIndex uint64
	Err      error
}

func (e *InvalidEnvelopeSignatureError) Code() uint32 {
	return errCodeInvalidEnvelopeSignatureError
}

func (e *InvalidEnvelopeSignatureError) Error() string {
	return fmt.Sprintf(
		"invalid envelope key: public key %d on account %s does not have a valid signature",
		e.KeyIndex,
		e.Address,
	)
}

func (e InvalidEnvelopeSignatureError) Is(target error) bool {
	_, ok := target.(*InvalidEnvelopeSignatureError)
	return ok
}

// An InvalidHashAlgorithmError indicates invalid hash algorithm.
type InvalidHashAlgorithmError struct {
	HashAlgo hash.HashingAlgorithm
}

func (e *InvalidHashAlgorithmError) Error() string {
	return fmt.Sprintf("invalid hash algorithm: %s", e.HashAlgo)
}

func (e *InvalidHashAlgorithmError) Code() uint32 {
	return errCodeInvalidHashAlgorithmError
}

func (e InvalidHashAlgorithmError) Is(target error) bool {
	_, ok := target.(*InvalidHashAlgorithmError)
	return ok
}

// An InvalidSignatureAlgorithmError indicates invalid signature algorithm
type InvalidSignatureAlgorithmError struct {
	SigningAlgo crypto.SigningAlgorithm
}

func (e *InvalidSignatureAlgorithmError) Error() string {
	return fmt.Sprintf("invalid signature algorithm: %s", e.SigningAlgo)
}

func (e *InvalidSignatureAlgorithmError) Code() uint32 {
	return errCodeInvalidSignatureAlgorithmError
}

func (e InvalidSignatureAlgorithmError) Is(target error) bool {
	_, ok := target.(*InvalidSignatureAlgorithmError)
	return ok
}

// An InvalidPublicKeyValueError indicates invalid byte content for a public key.
type InvalidPublicKeyValueError struct {
	Err error
}

func (e *InvalidPublicKeyValueError) Error() string {
	return fmt.Sprintf("invalid public key value %s", e.Err.Error())
}

func (e *InvalidPublicKeyValueError) Code() uint32 {
	return errCodeInvalidPublicKeyValueError
}

func (e InvalidPublicKeyValueError) Is(target error) bool {
	_, ok := target.(*InvalidPublicKeyValueError)
	return ok
}

// AuthorizationError indicates that a transaction is missing a required signature to
// authorize access to an account.
// this error is the result of failure in any of the following conditions:
// - no signature provided for an account
// - not enough key weight in total for this account
type AuthorizationError struct {
	Address      flow.Address
	SignedWeight uint32
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

func (e AuthorizationError) Is(target error) bool {
	_, ok := target.(*AuthorizationError)
	return ok
}
