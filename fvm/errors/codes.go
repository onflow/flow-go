package errors

import "fmt"

type ErrorCode uint16

func (ec ErrorCode) String() string {
	return fmt.Sprintf("[Error Code: %d]", ec)
}

type FailureCode uint16

func (fc FailureCode) String() string {
	return fmt.Sprintf("[Failure Code: %d]", fc)
}

const (
	FailureCodeUnknownFailure     FailureCode = 2000
	FailureCodeEncodingFailure    FailureCode = 2001
	FailureCodeLedgerFailure      FailureCode = 2002
	FailureCodeStateMergeFailure  FailureCode = 2003
	FailureCodeBlockFinderFailure FailureCode = 2004
	// Deprecated: No longer used.
	FailureCodeHasherFailure FailureCode = 2005
	// Deprecated: No longer used.
	FailureCodeMetaTransactionFailure FailureCode = 2100
)

const (
	// tx validation errors 1000 - 1049
	// Deprecated: no longer in use
	ErrCodeTxValidationError ErrorCode = 1000
	// Deprecated: No longer used.
	ErrCodeInvalidTxByteSizeError ErrorCode = 1001
	// Deprecated: No longer used.
	ErrCodeInvalidReferenceBlockError ErrorCode = 1002
	// Deprecated: No longer used.
	ErrCodeExpiredTransactionError ErrorCode = 1003
	// Deprecated: No longer used.
	ErrCodeInvalidScriptError ErrorCode = 1004
	// Deprecated: No longer used.
	ErrCodeInvalidGasLimitError          ErrorCode = 1005
	ErrCodeInvalidProposalSignatureError ErrorCode = 1006
	ErrCodeInvalidProposalSeqNumberError ErrorCode = 1007
	ErrCodeInvalidPayloadSignatureError  ErrorCode = 1008
	ErrCodeInvalidEnvelopeSignatureError ErrorCode = 1009

	// base errors 1050 - 1100
	ErrCodeFVMInternalError            ErrorCode = 1050
	ErrCodeValueError                  ErrorCode = 1051
	ErrCodeInvalidArgumentError        ErrorCode = 1052
	ErrCodeInvalidAddressError         ErrorCode = 1053
	ErrCodeInvalidLocationError        ErrorCode = 1054
	ErrCodeAccountAuthorizationError   ErrorCode = 1055
	ErrCodeOperationAuthorizationError ErrorCode = 1056
	ErrCodeOperationNotSupportedError  ErrorCode = 1057

	// execution errors 1100 - 1200
	// Deprecated: No longer used.
	ErrCodeExecutionError      ErrorCode = 1100
	ErrCodeCadenceRunTimeError ErrorCode = 1101
	// Deprecated: No longer used.
	ErrCodeEncodingUnsupportedValue ErrorCode = 1102
	ErrCodeStorageCapacityExceeded  ErrorCode = 1103
	// Deprecated: No longer used.
	ErrCodeGasLimitExceededError                     ErrorCode = 1104
	ErrCodeEventLimitExceededError                   ErrorCode = 1105
	ErrCodeLedgerInteractionLimitExceededError       ErrorCode = 1106
	ErrCodeStateKeySizeLimitError                    ErrorCode = 1107
	ErrCodeStateValueSizeLimitError                  ErrorCode = 1108
	ErrCodeTransactionFeeDeductionFailedError        ErrorCode = 1109
	ErrCodeComputationLimitExceededError             ErrorCode = 1110
	ErrCodeMemoryLimitExceededError                  ErrorCode = 1111
	ErrCodeCouldNotDecodeExecutionParameterFromState ErrorCode = 1112
	ErrCodeScriptExecutionCancelledError             ErrorCode = 1114
	ErrCodeScriptExecutionTimedOutError              ErrorCode = 1113

	// accounts errors 1200 - 1250
	// Deprecated: No longer used.
	ErrCodeAccountError                  ErrorCode = 1200
	ErrCodeAccountNotFoundError          ErrorCode = 1201
	ErrCodeAccountPublicKeyNotFoundError ErrorCode = 1202
	ErrCodeAccountAlreadyExistsError     ErrorCode = 1203
	ErrCodeFrozenAccountError            ErrorCode = 1204
	// Deprecated: No longer used.
	ErrCodeAccountStorageNotInitializedError ErrorCode = 1205
	ErrCodeAccountPublicKeyLimitError        ErrorCode = 1206

	// contract errors 1250 - 1300
	// Deprecated: No longer used.
	ErrCodeContractError         ErrorCode = 1250
	ErrCodeContractNotFoundError ErrorCode = 1251
	// Deprecated: No longer used.
	ErrCodeContractNamesNotFoundError ErrorCode = 1252
)
