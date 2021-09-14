package access

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// ErrUnknownReferenceBlock indicates that a transaction references an unknown block.
var ErrUnknownReferenceBlock = errors.New("unknown reference block")

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

// DuplicatedSignatureError indicates that two signatures have been provided for a key (combination of account and key index)
type DuplicatedSignatureError struct {
	Address  flow.Address
	KeyIndex uint64
}

func (e DuplicatedSignatureError) Error() string {
	return fmt.Sprintf("duplicated signature for key (address: %s, index: %d)", e.Address.String(), e.KeyIndex)
}

// InvalidSignatureError indicates that a transaction contains a signature
// with a wrong format.
type InvalidSignatureError struct {
	Signature flow.TransactionSignature
}

func (e InvalidSignatureError) Error() string {
	return fmt.Sprintf("invalid signature: %s", e.Signature)
}

// InvalidTxByteSizeError indicates that a transaction byte size exceeds the maximum.
type InvalidTxByteSizeError struct {
	Maximum uint64
	Actual  uint64
}

func (e InvalidTxByteSizeError) Error() string {
	return fmt.Sprintf("transaction byte size (%d) exceeds the maximum byte size allowed for a transaction (%d)", e.Actual, e.Maximum)
}
