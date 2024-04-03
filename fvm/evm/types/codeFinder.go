package types

import (
	"errors"

	gethCore "github.com/onflow/go-ethereum/core"
	gethVM "github.com/onflow/go-ethereum/core/vm"
)

func ValidationErrorCode(err error) ErrorCode {
	// direct errors that are returned by the evm
	switch err {
	case gethVM.ErrGasUintOverflow:
		return ValidationErrCodeGasUintOverflow
	}

	// wrapped errors return from the evm
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
	// execution VM errors are never wrapped
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
