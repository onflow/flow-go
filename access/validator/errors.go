package validator

import (
	"errors"
	"fmt"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/model/flow"
)

// ErrUnknownReferenceBlock indicates that a transaction references an unknown block.
var ErrUnknownReferenceBlock = errors.New("unknown reference block")

// IndexReporterNotInitialized is returned when indexReporter is nil because
// execution data syncing and indexing is disabled
var IndexReporterNotInitialized = errors.New("index reported not initialized")

// IncompleteTransactionError indicates that a transaction is missing one or more required fields.
type IncompleteTransactionError struct {
	MissingFields []string
}

func (e IncompleteTransactionError) Error() string {
	return fmt.Sprintf("transaction is missing required fields: %s", e.MissingFields)
}

// ExpiredTransactionError indicates that a transaction has expired.
type ExpiredTransactionError struct {
	RefHeight, FinalHeight uint64
}

func (e ExpiredTransactionError) Error() string {
	return fmt.Sprintf("transaction is expired: ref_height=%d final_height=%d", e.RefHeight, e.FinalHeight)
}

// InvalidScriptError indicates that a transaction contains an invalid Cadence script.
type InvalidScriptError struct {
	ParserErr error
}

func (e InvalidScriptError) Error() string {
	return fmt.Sprintf("failed to parse transaction Cadence script: %s", e.ParserErr)
}

func (e InvalidScriptError) Unwrap() error {
	return e.ParserErr
}

// InvalidGasLimitError indicates that a transaction specifies a gas limit that exceeds the maximum.
type InvalidGasLimitError struct {
	Maximum uint64
	Actual  uint64
}

func (e InvalidGasLimitError) Error() string {
	return fmt.Sprintf("transaction gas limit (%d) is not in the acceptable range (min: 1, max: %d)", e.Actual, e.Maximum)
}

// InvalidAddressError indicates that a transaction references an invalid flow Address
// in either the Authorizers or Payer field.
type InvalidAddressError struct {
	Address flow.Address
}

func (e InvalidAddressError) Error() string {
	return fmt.Sprintf("invalid address: %s", e.Address)
}

// DuplicatedSignatureError indicates that two signatures havs been provided for a key (combination of account and key index)
type DuplicatedSignatureError struct {
	Address  flow.Address
	KeyIndex uint32
}

func (e DuplicatedSignatureError) Error() string {
	return fmt.Sprintf("duplicated signature for key (address: %s, index: %d)", e.Address.String(), e.KeyIndex)
}

// MissingSignatureError indicates that a signature is missing for a payer, proposer, or authorizer.
type MissingSignatureError struct {
	Address flow.Address
	Message string
}

func (e MissingSignatureError) Error() string {
	return fmt.Sprintf("%s: missing signature is from account %s", e.Message, e.Address)
}

// UnrelatedAccountSignatureError indicates that a signature has been provided by an account
// that is neither the proposer, payer, nor an authorizer of the transaction.
type UnrelatedAccountSignatureError struct {
	Address flow.Address
}

func (e UnrelatedAccountSignatureError) Error() string {
	return fmt.Sprintf("unrelated account signature from address: %s", e.Address.String())
}

// InvalidRawSignatureError indicates that a transaction contains a cryptographic raw signature
// with a wrong format.
type InvalidRawSignatureError struct {
	Signature flow.TransactionSignature
}

func (e InvalidRawSignatureError) Error() string {
	return fmt.Sprintf("the cryptographic signature within the transaction signature has an invalid format: %s", e.Signature)
}

// InvalidAuthenticationSchemeFormatError indicates that a transaction contains a signature
// with a wrong format.
type InvalidAuthenticationSchemeFormatError struct {
	Signature flow.TransactionSignature
}

func (e InvalidAuthenticationSchemeFormatError) Error() string {
	return fmt.Sprintf("the transaction signature has invalid extension data: %s", e.Signature)
}

// InvalidTxByteSizeError indicates that a transaction byte size exceeds the maximum.
type InvalidTxByteSizeError struct {
	Maximum uint64
	Actual  uint64
}

func (e InvalidTxByteSizeError) Error() string {
	return fmt.Sprintf("transaction byte size (%d) exceeds the maximum byte size allowed for a transaction (%d)", e.Actual, e.Maximum)
}

type InvalidTxRateLimitedError struct {
	Payer flow.Address
}

func (e InvalidTxRateLimitedError) Error() string {
	return fmt.Sprintf("transaction rate limited for payer (%s)", e.Payer)
}

type InsufficientBalanceError struct {
	Payer           flow.Address
	RequiredBalance cadence.UFix64
}

func (e InsufficientBalanceError) Error() string {
	return fmt.Sprintf("transaction payer (%s) has insufficient balance to pay transaction fee. "+
		"Required balance: (%s). ", e.Payer, e.RequiredBalance.String())
}

func IsInsufficientBalanceError(err error) bool {
	var balanceError InsufficientBalanceError
	return errors.As(err, &balanceError)
}

// IndexedHeightFarBehindError indicates that a node is far behind on indexing.
type IndexedHeightFarBehindError struct {
	SealedHeight  uint64
	IndexedHeight uint64
}

func (e IndexedHeightFarBehindError) Error() string {
	return fmt.Sprintf("the difference between the latest sealed height (%d) and indexed height (%d) exceeds the maximum gap allowed",
		e.SealedHeight, e.IndexedHeight)
}
