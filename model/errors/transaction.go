package errors

import "fmt"

const (
	// tx validation errors
	errCodeInvalidTxByteSizeError     = 1
	errCodeInvalidReferenceBlockError = 2
	errCodeInvalidScriptError         = 3

	errCodeMissingPayer                          = 2
	errCodeInvalidSignaturePublicKeyDoesNotExist = 3
	errCodeInvalidSignaturePublicKeyRevoked      = 4
	errCodeInvalidSignatureVerification          = 5

	errCodeInvalidProposalKeyPublicKeyDoesNotExist = 6
	errCodeInvalidProposalKeyPublicKeyRevoked      = 7
	errCodeInvalidProposalKeySequenceNumber        = 8
	errCodeInvalidProposalKeyMissingSignature      = 9

	errCodeInvalidHashAlgorithm = 10

	// tx execution errors

	errCodeExecution = 100
)

// TxValidationError captures a transaction validation error
// A transaction having this error (in most cases) is rejected by access/collection nodes
// and later in the pipeline be verified by execution and verification nodes.
type TxValidationError interface {
	// Code returns the code for this error
	Code() uint32
	// Error returns an string describing the details of the error
	Error() string
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
	return fmt.Sprintf("transaction byte size (%d) exceeds the maximum byte size allowed for a transaction (%d)", e.TxByteSize, e.Maximum)
}

func (e InvalidReferenceBlockError) Code() uint32 {
	return errCodeInvalidReferenceBlockError
}

// ExpiredTxError indicates that a transaction has expired.
// this error is the result of failure in any of the following conditions:
// - ReferenceBlock.Height - CurrentBlock.Height < Expiry Limit (Transaction is Expired)
type ExpiredTxError struct {
	RefHeight, FinalHeight uint64
}

func (e ExpiredTxError) Error() string {
	return fmt.Sprintf("transaction is expired: ref_height=%d final_height=%d", e.RefHeight, e.FinalHeight)
}

func (e ExpiredTxError) Code() uint32 {
	return errCodeInvalidReferenceBlockError
}

// InvalidScriptError indicates that a transaction contains an invalid Cadence script.
// this error is the result of failure in any of the following conditions:
// - script is empty
// - script can not be parsed by the cadence parser
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

// 		- InvalidArgumentError
// 			- check
// 				- # of args not matching the script template

// 		- InvalidPayloadSignatureError
// 			(role : payer)
// 			- PublicKeyDoesNotExist
// 			- PublicKeyRevoked
// 			- Verification failed?
// 			- not enough weight ?

// 		- InvalidEnvelopeSignatureError
// 				(role : payer, proposer, authroizer)
// 			- PublicKeyDoesNotExist
// 			- PublicKeyRevoked
// 			- VerificationFailed?
// 			- not enough weight ?
// 			- InvalidHashAlgorithm

// 		- InvalidSequenceNumber
// 			- check
// 				- proposal sequence number not match

// // TxExecutionError captures a transaction execution error
// type TxExecutionError interface {
// 	// TxHash returns the hash of the transaction content
// 	TxHash() string
// 	// Code returns the code for this error
// 	Code() uint32
// 	// Error returns an string describing the details of the error
// 	Error() string
// }

// type TxExecutionError

// 	// run time error
// 	cadence.Errors []

// // ErrUnknownReferenceBlock indicates that a transaction references an unknown block.
// var ErrUnknownReferenceBlock = errors.New("unknown reference block")

// // IncompleteTransactionError indicates that a transaction is missing one or more required fields.
// type IncompleteTransactionError struct {
// 	MissingFields []string
// }

// func (e IncompleteTransactionError) Error() string {
// 	return fmt.Sprintf("transaction is missing required fields: %s", e.MissingFields)
// }

// // InvalidScriptError indicates that a transaction contains an invalid Cadence script.
// type InvalidScriptError struct {
// 	ParserErr error
// }

// func (e InvalidScriptError) Error() string {
// 	return fmt.Sprintf("failed to parse transaction Cadence script: %s", e.ParserErr)
// }

// func (e InvalidScriptError) Unwrap() error {
// 	return e.ParserErr
// }

// // InvalidGasLimitError indicates that a transaction specifies a gas limit that exceeds the maximum.
// type InvalidGasLimitError struct {
// 	Maximum uint64
// 	Actual  uint64
// }

// func (e InvalidGasLimitError) Error() string {
// 	return fmt.Sprintf("transaction gas limit (%d) exceeds the maximum gas limit (%d)", e.Actual, e.Maximum)
// }

// // InvalidAddressError indicates that a transaction references an invalid flow Address
// // in either the Authorizers or Payer field.
// type InvalidAddressError struct {
// 	Address flow.Address
// }

// func (e InvalidAddressError) Error() string {
// 	return fmt.Sprintf("invalid address: %s", e.Address)
// }
