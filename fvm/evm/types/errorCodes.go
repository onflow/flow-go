package types

import (
	"errors"

	gethCore "github.com/ethereum/go-ethereum/core"
	gethVM "github.com/ethereum/go-ethereum/core/vm"
)

type ErrorCode uint64

// TODO: move this to emulator
// TODO: include other flow EVM errors

// internal error codes
const ( // code reserved for no error
	ErrCodeNoError ErrorCode = 0

	// covers all other validation codes that doesn't have an specific code
	ValidationErrCodeMisc                    ErrorCode = 100
	ValidationErrCodeInvalidBalance          ErrorCode = 101
	ValidationErrCodeInsufficientComputation ErrorCode = 102
	ValidationErrCodeUnAuthroizedMethodCall  ErrorCode = 103

	// general execution error returned for cases that don't have an specific code
	ExecutionErrCodeMisc                         ErrorCode = 150
	ExecutionErrCodeInsufficientTotalSupply      ErrorCode = 151
	ExecutionErrCodeWithdrawBalanceRoundingError ErrorCode = 152
)

// geth evm core errors (reserved range: [200-300) )
const (
	// the nonce of the tx is lower than the expected
	ValidationErrCodeNonceTooLow = iota + 200
	// the nonce of the tx is higher than the expected
	ValidationErrCodeNonceTooHigh
	// tx sender account has reached to the maximum nonce
	ValidationErrCodeNonceMax
	// not enough gas is available on the block to include this transaction
	ValidationErrCodeGasLimitReached
	// the transaction sender doesn't have enough funds for transfer(topmost call only).
	ValidationErrCodeInsufficientFundsForTransfer
	// creation transaction provides the init code bigger than init code size limit.
	ValidationErrCodeMaxInitCodeSizeExceeded
	// the total cost of executing a transaction is higher than the balance of the user's account.
	ValidationErrCodeInsufficientFunds
	// overflow detected when calculating the gas usage
	ValidationErrCodeGasUintOverflow
	// the transaction is specified to use less gas than required to start the invocation.
	ValidationErrCodeIntrinsicGas
	// the transaction is not supported in the current network configuration.
	ValidationErrCodeTxTypeNotSupported
	// tip was set to higher than the total fee cap
	ValidationErrCodeTipAboveFeeCap
	// an extremely big numbers is set for the tip field
	ValidationErrCodeTipVeryHigh
	// an extremely big numbers is set for the fee cap field
	ValidationErrCodeFeeCapVeryHigh
	// the transaction fee cap is less than the base fee of the block
	ValidationErrCodeFeeCapTooLow
	// the sender of a transaction is a contract
	ValidationErrCodeSenderNoEOA
	// the transaction fee cap is less than the blob gas fee of the block.
	ValidationErrCodeBlobFeeCapTooLow
)

// evm execution errors (reserved range: [300-400) )
const (
	// execution ran out of gas
	ExecutionErrCodeOutOfGas ErrorCode = iota + 300
	// contract creation code storage out of gas
	ExecutionErrCodeCodeStoreOutOfGas
	// max call depth exceeded
	ExecutionErrCodeDepth
	// insufficient balance for transfer
	ExecutionErrCodeInsufficientBalance
	// contract address collision"
	ExecutionErrCodeContractAddressCollision
	// execution reverted
	ExecutionErrCodeExecutionReverted
	// max initcode size exceeded
	ExecutionErrCodeMaxInitCodeSizeExceeded
	// max code size exceeded
	ExecutionErrCodeMaxCodeSizeExceeded
	// invalid jump destination
	ExecutionErrCodeInvalidJump
	// write protection
	ExecutionErrCodeWriteProtection
	// return data out of bounds
	ExecutionErrCodeReturnDataOutOfBounds
	// gas uint64 overflow
	ExecutionErrCodeGasUintOverflow
	// invalid code: must not begin with 0xef
	ExecutionErrCodeInvalidCode
	// nonce uint64 overflow
	ExecutionErrCodeNonceUintOverflow
)

func ValidationErrorCode(err error) ErrorCode {
	nested := errors.Unwrap(err)
	switch nested {
	case gethCore.ErrNonceTooLow:
		return ValidationErrCodeNonceTooLow
	case gethCore.ErrNonceTooHigh:
		return ValidationErrCodeNonceTooHigh
	case gethCore.ErrNonceMax:
		return ValidationErrCodeNonceMax
	case gethCore.ErrGasLimitReached:
		return ValidationErrCodeGasLimitReached
	case gethCore.ErrInsufficientFundsForTransfer:
		return ValidationErrCodeInsufficientFundsForTransfer
	case gethCore.ErrMaxInitCodeSizeExceeded:
		return ValidationErrCodeMaxInitCodeSizeExceeded
	case gethCore.ErrInsufficientFunds:
		return ValidationErrCodeInsufficientFunds
	case gethCore.ErrGasUintOverflow:
		return ValidationErrCodeGasUintOverflow
	case gethCore.ErrIntrinsicGas:
		return ValidationErrCodeIntrinsicGas
	case gethCore.ErrTxTypeNotSupported:
		return ValidationErrCodeTxTypeNotSupported
	case gethCore.ErrTipAboveFeeCap:
		return ValidationErrCodeTipAboveFeeCap
	case gethCore.ErrTipVeryHigh:
		return ValidationErrCodeTipVeryHigh
	case gethCore.ErrFeeCapVeryHigh:
		return ValidationErrCodeFeeCapVeryHigh
	case gethCore.ErrFeeCapTooLow:
		return ValidationErrCodeFeeCapTooLow
	case gethCore.ErrSenderNoEOA:
		return ValidationErrCodeSenderNoEOA
	case gethCore.ErrBlobFeeCapTooLow:
		return ValidationErrCodeBlobFeeCapTooLow
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
