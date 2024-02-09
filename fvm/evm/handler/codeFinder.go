package handler

import (
	"errors"

	gethCore "github.com/ethereum/go-ethereum/core"
	gethVM "github.com/ethereum/go-ethereum/core/vm"
	"github.com/onflow/flow-go/fvm/evm/types"
)

func ValidationErrorCode(err error) types.ErrorCode {
	// if is a EVM validation error first unwrap one level
	if types.IsEVMValidationError(err) {
		// check local errors
		switch err {
		case types.ErrInvalidBalance:
			return types.ValidationErrCodeInvalidBalance
		case types.ErrInsufficientComputation:
			return types.ValidationErrCodeInsufficientComputation
		case types.ErrUnAuthroizedMethodCall:
			return types.ValidationErrCodeUnAuthroizedMethodCall
		case types.ErrWithdrawBalanceRounding:
			return types.ValidationErrCodeWithdrawBalanceRounding
		}
		err = errors.Unwrap(err)
	}

	// direct errors that are returned by the evm
	switch err {
	case gethVM.ErrGasUintOverflow:
		return types.ValidationErrCodeGasUintOverflow
	}

	// wrapped errors return from the evm
	nested := errors.Unwrap(err)
	switch nested {
	case gethCore.ErrNonceTooLow:
		return types.ValidationErrCodeNonceTooLow
	case gethCore.ErrNonceTooHigh:
		return types.ValidationErrCodeNonceTooHigh
	case gethCore.ErrNonceMax:
		return types.ValidationErrCodeNonceMax
	case gethCore.ErrGasLimitReached:
		return types.ValidationErrCodeGasLimitReached
	case gethCore.ErrInsufficientFundsForTransfer:
		return types.ValidationErrCodeInsufficientFundsForTransfer
	case gethCore.ErrMaxInitCodeSizeExceeded:
		return types.ValidationErrCodeMaxInitCodeSizeExceeded
	case gethCore.ErrInsufficientFunds:
		return types.ValidationErrCodeInsufficientFunds
	case gethCore.ErrIntrinsicGas:
		return types.ValidationErrCodeIntrinsicGas
	case gethCore.ErrTxTypeNotSupported:
		return types.ValidationErrCodeTxTypeNotSupported
	case gethCore.ErrTipAboveFeeCap:
		return types.ValidationErrCodeTipAboveFeeCap
	case gethCore.ErrTipVeryHigh:
		return types.ValidationErrCodeTipVeryHigh
	case gethCore.ErrFeeCapVeryHigh:
		return types.ValidationErrCodeFeeCapVeryHigh
	case gethCore.ErrFeeCapTooLow:
		return types.ValidationErrCodeFeeCapTooLow
	case gethCore.ErrSenderNoEOA:
		return types.ValidationErrCodeSenderNoEOA
	case gethCore.ErrBlobFeeCapTooLow:
		return types.ValidationErrCodeBlobFeeCapTooLow
	default:
		return types.ValidationErrCodeMisc
	}
}

func ExecutionErrorCode(err error) types.ErrorCode {
	// execution VM errors are never wrapped
	switch err {
	case gethVM.ErrOutOfGas:
		return types.ExecutionErrCodeOutOfGas
	case gethVM.ErrCodeStoreOutOfGas:
		return types.ExecutionErrCodeCodeStoreOutOfGas
	case gethVM.ErrDepth:
		return types.ExecutionErrCodeDepth
	case gethVM.ErrInsufficientBalance:
		return types.ExecutionErrCodeInsufficientBalance
	case gethVM.ErrContractAddressCollision:
		return types.ExecutionErrCodeContractAddressCollision
	case gethVM.ErrExecutionReverted:
		return types.ExecutionErrCodeExecutionReverted
	case gethVM.ErrMaxInitCodeSizeExceeded:
		return types.ExecutionErrCodeMaxInitCodeSizeExceeded
	case gethVM.ErrMaxCodeSizeExceeded:
		return types.ExecutionErrCodeMaxCodeSizeExceeded
	case gethVM.ErrInvalidJump:
		return types.ExecutionErrCodeInvalidJump
	case gethVM.ErrWriteProtection:
		return types.ExecutionErrCodeWriteProtection
	case gethVM.ErrReturnDataOutOfBounds:
		return types.ExecutionErrCodeReturnDataOutOfBounds
	case gethVM.ErrGasUintOverflow:
		return types.ExecutionErrCodeGasUintOverflow
	case gethVM.ErrInvalidCode:
		return types.ExecutionErrCodeInvalidCode
	case gethVM.ErrNonceUintOverflow:
		return types.ExecutionErrCodeNonceUintOverflow
	default:
		return types.ExecutionErrCodeMisc
	}
}
