package types

import (
	"errors"
	"fmt"

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

func GetErrorForCode(errorCode ErrorCode) error {
	switch errorCode {
	case ValidationErrCodeGasUintOverflow:
		return gethVM.ErrGasUintOverflow
	case ValidationErrCodeNonceTooLow:
		return gethCore.ErrNonceTooLow
	case ValidationErrCodeNonceTooHigh:
		return gethCore.ErrNonceTooHigh
	case ValidationErrCodeNonceMax:
		return gethCore.ErrNonceMax
	case ValidationErrCodeGasLimitReached:
		return gethCore.ErrGasLimitReached
	case ValidationErrCodeInsufficientFundsForTransfer:
		return gethCore.ErrInsufficientFundsForTransfer
	case ValidationErrCodeMaxInitCodeSizeExceeded:
		return gethCore.ErrMaxInitCodeSizeExceeded
	case ValidationErrCodeInsufficientFunds:
		return gethCore.ErrInsufficientFunds
	case ValidationErrCodeIntrinsicGas:
		return gethCore.ErrIntrinsicGas
	case ValidationErrCodeTxTypeNotSupported:
		return gethCore.ErrTxTypeNotSupported
	case ValidationErrCodeTipAboveFeeCap:
		return gethCore.ErrTipAboveFeeCap
	case ValidationErrCodeTipVeryHigh:
		return gethCore.ErrTipVeryHigh
	case ValidationErrCodeFeeCapVeryHigh:
		return gethCore.ErrFeeCapVeryHigh
	case ValidationErrCodeFeeCapTooLow:
		return gethCore.ErrFeeCapTooLow
	case ValidationErrCodeSenderNoEOA:
		return gethCore.ErrSenderNoEOA
	case ValidationErrCodeBlobFeeCapTooLow:
		return gethCore.ErrBlobFeeCapTooLow
	case ExecutionErrCodeOutOfGas:
		return gethVM.ErrOutOfGas
	case ExecutionErrCodeCodeStoreOutOfGas:
		return gethVM.ErrCodeStoreOutOfGas
	case ExecutionErrCodeDepth:
		return gethVM.ErrDepth
	case ExecutionErrCodeInsufficientBalance:
		return gethVM.ErrInsufficientBalance
	case ExecutionErrCodeContractAddressCollision:
		return gethVM.ErrContractAddressCollision
	case ExecutionErrCodeExecutionReverted:
		return gethVM.ErrExecutionReverted
	case ExecutionErrCodeMaxInitCodeSizeExceeded:
		return gethVM.ErrMaxInitCodeSizeExceeded
	case ExecutionErrCodeMaxCodeSizeExceeded:
		return gethVM.ErrMaxCodeSizeExceeded
	case ExecutionErrCodeInvalidJump:
		return gethVM.ErrInvalidJump
	case ExecutionErrCodeWriteProtection:
		return gethVM.ErrWriteProtection
	case ExecutionErrCodeReturnDataOutOfBounds:
		return gethVM.ErrReturnDataOutOfBounds
	case ExecutionErrCodeGasUintOverflow:
		return gethVM.ErrGasUintOverflow
	case ExecutionErrCodeInvalidCode:
		return gethVM.ErrInvalidCode
	case ExecutionErrCodeNonceUintOverflow:
		return gethVM.ErrNonceUintOverflow
	case ValidationErrCodeMisc:
		return fmt.Errorf("validation error: %d", errorCode)
	case ExecutionErrCodeMisc:
		return fmt.Errorf("execution error: %d", errorCode)
	}

	return fmt.Errorf("unknown error code: %d", errorCode)
}
