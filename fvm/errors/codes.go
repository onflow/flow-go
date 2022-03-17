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
	FailureCodeUnknownFailure         FailureCode = 2000
	FailureCodeEncodingFailure        FailureCode = 2001
	FailureCodeLedgerFailure          FailureCode = 2002
	FailureCodeStateMergeFailure      FailureCode = 2003
	FailureCodeBlockFinderFailure     FailureCode = 2004
	FailureCodeHasherFailure          FailureCode = 2005
	FailureCodeMetaTransactionFailure FailureCode = 2100
)

const (
	// tx validation errors 1000 - 1049
	// ErrCodeTxValidationError         ErrorCode = 1000 - reserved
	ErrCodeInvalidTxByteSizeError     ErrorCode = 1001
	ErrCodeInvalidReferenceBlockError ErrorCode = 1002
	// Deprecated: ErrCodeExpiredTransactionError
	ErrCodeExpiredTransactionError       ErrorCode = 1003
	ErrCodeInvalidScriptError            ErrorCode = 1004
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
	// ErrCodeExecutionError                 ErrorCode = 1100 - reserved
	ErrCodeCadenceRunTimeError      ErrorCode = 1101
	ErrCodeEncodingUnsupportedValue ErrorCode = 1102
	ErrCodeStorageCapacityExceeded  ErrorCode = 1103
	//  Deprecated: ErrCodeGasLimitExceededError  ErrorCode = 1104
	ErrCodeEventLimitExceededError            ErrorCode = 1105
	ErrCodeLedgerIntractionLimitExceededError ErrorCode = 1106
	ErrCodeStateKeySizeLimitError             ErrorCode = 1107
	ErrCodeStateValueSizeLimitError           ErrorCode = 1108
	ErrCodeTransactionFeeDeductionFailedError ErrorCode = 1109
	ErrCodeComputationLimitExceededError      ErrorCode = 1110
	ErrCodeMemoryLimitExceededError           ErrorCode = 1111

	// accounts errors 1200 - 1250
	// ErrCodeAccountError              ErrorCode = 1200 - reserved
	ErrCodeAccountNotFoundError          ErrorCode = 1201
	ErrCodeAccountPublicKeyNotFoundError ErrorCode = 1202
	ErrCodeAccountAlreadyExistsError     ErrorCode = 1203
	ErrCodeFrozenAccountError            ErrorCode = 1204

	// contract errors 1250 - 1300
	// ErrCodeContractError          ErrorCode = 1250 - reserved
	ErrCodeContractNotFoundError      ErrorCode = 1251
	ErrCodeContractNamesNotFoundError ErrorCode = 1252
)
