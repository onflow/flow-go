package types

import (
	"errors"
	"fmt"

	gethCore "github.com/onflow/go-ethereum/core"
	gethVM "github.com/onflow/go-ethereum/core/vm"
)

func ValidationErrorCode(err error) ErrorCode {
	switch {
	case errors.Is(err, gethVM.ErrGasUintOverflow):
		return ValidationErrCodeGasUintOverflow
	case errors.Is(err, gethCore.ErrNonceTooLow):
		return ValidationErrCodeNonceTooLow
	case errors.Is(err, gethCore.ErrNonceTooHigh):
		return ValidationErrCodeNonceTooHigh
	case errors.Is(err, gethCore.ErrNonceMax):
		return ValidationErrCodeNonceMax
	case errors.Is(err, gethCore.ErrGasLimitReached):
		return ValidationErrCodeGasLimitReached
	case errors.Is(err, gethCore.ErrInsufficientFundsForTransfer):
		return ValidationErrCodeInsufficientFundsForTransfer
	case errors.Is(err, gethCore.ErrMaxInitCodeSizeExceeded):
		return ValidationErrCodeMaxInitCodeSizeExceeded
	case errors.Is(err, gethCore.ErrInsufficientFunds):
		return ValidationErrCodeInsufficientFunds
	case errors.Is(err, gethCore.ErrIntrinsicGas):
		return ValidationErrCodeIntrinsicGas
	case errors.Is(err, gethCore.ErrTxTypeNotSupported):
		return ValidationErrCodeTxTypeNotSupported
	case errors.Is(err, gethCore.ErrTipAboveFeeCap):
		return ValidationErrCodeTipAboveFeeCap
	case errors.Is(err, gethCore.ErrTipVeryHigh):
		return ValidationErrCodeTipVeryHigh
	case errors.Is(err, gethCore.ErrFeeCapVeryHigh):
		return ValidationErrCodeFeeCapVeryHigh
	case errors.Is(err, gethCore.ErrFeeCapTooLow):
		return ValidationErrCodeFeeCapTooLow
	case errors.Is(err, gethCore.ErrSenderNoEOA):
		return ValidationErrCodeSenderNoEOA
	case errors.Is(err, gethCore.ErrBlobFeeCapTooLow):
		return ValidationErrCodeBlobFeeCapTooLow
	default:
		return ValidationErrCodeMisc
	}
}

func ExecutionErrorCode(err error) ErrorCode {
	switch {
	case errors.Is(err, gethVM.ErrOutOfGas):
		return ExecutionErrCodeOutOfGas
	case errors.Is(err, gethVM.ErrCodeStoreOutOfGas):
		return ExecutionErrCodeCodeStoreOutOfGas
	case errors.Is(err, gethVM.ErrDepth):
		return ExecutionErrCodeDepth
	case errors.Is(err, gethVM.ErrInsufficientBalance):
		return ExecutionErrCodeInsufficientBalance
	case errors.Is(err, gethVM.ErrContractAddressCollision):
		return ExecutionErrCodeContractAddressCollision
	case errors.Is(err, gethVM.ErrExecutionReverted):
		return ExecutionErrCodeExecutionReverted
	case errors.Is(err, gethVM.ErrMaxInitCodeSizeExceeded):
		return ExecutionErrCodeMaxInitCodeSizeExceeded
	case errors.Is(err, gethVM.ErrMaxCodeSizeExceeded):
		return ExecutionErrCodeMaxCodeSizeExceeded
	case errors.Is(err, gethVM.ErrInvalidJump):
		return ExecutionErrCodeInvalidJump
	case errors.Is(err, gethVM.ErrWriteProtection):
		return ExecutionErrCodeWriteProtection
	case errors.Is(err, gethVM.ErrReturnDataOutOfBounds):
		return ExecutionErrCodeReturnDataOutOfBounds
	case errors.Is(err, gethVM.ErrGasUintOverflow):
		return ExecutionErrCodeGasUintOverflow
	case errors.Is(err, gethVM.ErrInvalidCode):
		return ExecutionErrCodeInvalidCode
	case errors.Is(err, gethVM.ErrNonceUintOverflow):
		return ExecutionErrCodeNonceUintOverflow
	default:
		return ExecutionErrCodeMisc
	}
}

func ErrorFromCode(errorCode ErrorCode) error {
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
