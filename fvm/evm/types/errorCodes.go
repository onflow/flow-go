package types

import (
	"errors"

	gethCore "github.com/ethereum/go-ethereum/core"
	gethVM "github.com/ethereum/go-ethereum/core/vm"
)

type ErrorCode uint64

// TODO reorder to the org ones
// TODO: include other flow EVM errors

const (
	// code reserved for no error
	ErrCodeNoError ErrorCode = 0
	// covers all other validation codes that doesn't have an specific code
	ValidationErrCodeMisc ErrorCode = 100
	// the transaction is not supported in the current network configuration.
	ValidationErrCodeTxTypeNotSupported ErrorCode = 101
	// the sender of a transaction is a contract
	ValidationErrCodeSenderNoEOA ErrorCode = 102
	// the nonce of the tx is lower than the expected
	ValidationErrCodeNonceTooLow ErrorCode = 103
	// the nonce of the tx is higher than the expected
	ValidationErrCodeNonceTooHigh ErrorCode = 104
	// tx sender account has reached to the maximum nonce
	ValidationErrCodeNonceMax ErrorCode = 105
	// not enough gas is available on the block to include this transaction
	ValidationErrCodeGasLimitReached ErrorCode = 106
	// the transaction is specified to use less gas than required to start the invocation.
	ValidationErrCodeIntrinsicGas ErrorCode = 107
	// overflow detected when calculating the gas usage
	ValidationErrCodeGasUintOverflow ErrorCode = 108
	// tip was set to higher than the total fee cap
	ValidationErrCodeTipAboveFeeCap ErrorCode = 109
	// an extremely big numbers is set for the tip field
	ValidationErrCodeTipVeryHigh ErrorCode = 110
	// the transaction fee cap is less than the base fee of the block
	ValidationErrCodeFeeCapTooLow ErrorCode = 111
	// an extremely big numbers is set for the fee cap field
	ValidationErrCodeFeeCapVeryHigh ErrorCode = 112
	// the transaction fee cap is less than the blob gas fee of the block.
	ValidationErrCodeBlobFeeCapTooLow ErrorCode = 113
	// the transaction sender doesn't have enough funds for transfer(topmost call only).
	ValidationErrCodeInsufficientFundsForTransfer ErrorCode = 114
	// the total cost of executing a transaction is higher than the balance of the user's account.
	ValidationErrCodeInsufficientFunds ErrorCode = 115
	// creation transaction provides the init code bigger than init code size limit.
	ValidationErrCodeMaxInitCodeSizeExceeded ErrorCode = 116

	// general execution error returned for cases that don't have an specific code
	ExecutionErrCodeMisc ErrorCode = 200
	// execution ran out of gas
	ExecutionErrCodeOutOfGas ErrorCode = 201
	// contract creation code storage out of gas
	ExecutionErrCodeCodeStoreOutOfGas ErrorCode = 202
	// max call depth exceeded
	ExecutionErrCodeDepth ErrorCode = 203
	// insufficient balance for transfer
	ExecutionErrCodeInsufficientBalance ErrorCode = 204
	// contract address collision"
	ExecutionErrCodeContractAddressCollision ErrorCode = 205
	// execution reverted
	ExecutionErrCodeExecutionReverted ErrorCode = 206
	// max initcode size exceeded
	ExecutionErrCodeMaxInitCodeSizeExceeded ErrorCode = 207
	// max code size exceeded
	ExecutionErrCodeMaxCodeSizeExceeded ErrorCode = 208
	// invalid jump destination
	ExecutionErrCodeInvalidJump ErrorCode = 209
	// write protection
	ExecutionErrCodeWriteProtection ErrorCode = 210
	// return data out of bounds
	ExecutionErrCodeReturnDataOutOfBounds ErrorCode = 211
	// gas uint64 overflow
	ExecutionErrCodeGasUintOverflow ErrorCode = 212
	// invalid code: must not begin with 0xef
	ExecutionErrCodeInvalidCode ErrorCode = 213
	// nonce uint64 overflow
	ExecutionErrCodeNonceUintOverflow ErrorCode = 214
)

func ValidationErrorCode(err error) ErrorCode {
	nested := errors.Unwrap(err)
	switch nested {
	case gethCore.ErrTxTypeNotSupported:
		return ValidationErrCodeTxTypeNotSupported
	case gethCore.ErrSenderNoEOA:
		return ValidationErrCodeSenderNoEOA
	case gethCore.ErrNonceTooLow:
		return ValidationErrCodeNonceTooLow
	case gethCore.ErrNonceTooHigh:
		return ValidationErrCodeNonceTooHigh
	case gethCore.ErrNonceMax:
		return ValidationErrCodeNonceMax
	case gethCore.ErrGasLimitReached:
		return ValidationErrCodeGasLimitReached
	case gethCore.ErrGasUintOverflow:
		return ValidationErrCodeGasUintOverflow
	case gethCore.ErrIntrinsicGas:
		return ValidationErrCodeIntrinsicGas
	case gethCore.ErrTipAboveFeeCap:
		return ValidationErrCodeTipAboveFeeCap
	case gethCore.ErrTipVeryHigh:
		return ValidationErrCodeTipVeryHigh
	case gethCore.ErrFeeCapVeryHigh:
		return ValidationErrCodeFeeCapVeryHigh
	case gethCore.ErrFeeCapTooLow:
		return ValidationErrCodeFeeCapTooLow
	case gethCore.ErrBlobFeeCapTooLow:
		return ValidationErrCodeBlobFeeCapTooLow
	case gethCore.ErrInsufficientFundsForTransfer:
		return ValidationErrCodeInsufficientFundsForTransfer
	case gethCore.ErrInsufficientFunds:
		return ValidationErrCodeInsufficientFunds
	case gethCore.ErrMaxInitCodeSizeExceeded:
		return ValidationErrCodeMaxInitCodeSizeExceeded
	default:
		return ValidationErrCodeMisc
	}
}

func ExecutionErrorCode(err error) ErrorCode {
	switch err {
	case gethVM.ErrOutOfGas:
		return ExecutionErrCodeOutOfGas
	case gethVM.ErrCodeStoreOutOfGas:
		return ExecutionErrCodeCodeStoreOutOfGas
	case gethVM.ErrDepth:
		return ExecutionErrCodeDepth
	case gethVM.ErrInsufficientBalance:
		return ExecutionErrCodeInsufficientBalance
	case gethVM.ErrContractAddressCollision:
		return ExecutionErrCodeContractAddressCollision
	case gethVM.ErrExecutionReverted:
		return ExecutionErrCodeExecutionReverted
	case gethVM.ErrMaxInitCodeSizeExceeded:
		return ExecutionErrCodeMaxInitCodeSizeExceeded
	case gethVM.ErrMaxCodeSizeExceeded:
		return ExecutionErrCodeMaxCodeSizeExceeded
	case gethVM.ErrInvalidJump:
		return ExecutionErrCodeInvalidJump
	case gethVM.ErrWriteProtection:
		return ExecutionErrCodeWriteProtection
	case gethVM.ErrReturnDataOutOfBounds:
		return ExecutionErrCodeReturnDataOutOfBounds
	case gethVM.ErrGasUintOverflow:
		return ExecutionErrCodeGasUintOverflow
	case gethVM.ErrInvalidCode:
		return ExecutionErrCodeInvalidCode
	case gethVM.ErrNonceUintOverflow:
		return ExecutionErrCodeNonceUintOverflow
	default:
		return ExecutionErrCodeMisc
	}
}
