package errors

const (
	failureCodeUnknownFailure     = 2000
	failureCodeEncodingFailure    = 2001
	failureCodeLedgerFailure      = 2002
	failureCodeStateMergeFailure  = 2003
	failureCodeBlockFinderFailure = 2004
	failureCodeHasherFailure      = 2005
)

const (
	// tx validation errors 1000 - 1049
	errCodeTxValidationError             = 1000
	errCodeInvalidTxByteSizeError        = 1001
	errCodeInvalidReferenceBlockError    = 1002
	errCodeExpiredTransactionError       = 1003
	errCodeInvalidScriptError            = 1004
	errCodeInvalidGasLimitError          = 1005
	errCodeInvalidProposalSignatureError = 1006
	errCodeInvalidProposalSeqNumberError = 1007
	errCodeInvalidPayloadSignatureError  = 1008
	errCodeInvalidEnvelopeSignatureError = 1009

	// base errors 1050 - 1100
	errCodeValueError                  = 1051
	errCodeInvalidArgumentError        = 1052
	errCodeInvalidAddressError         = 1053
	errCodeInvalidLocationError        = 1054
	errCodeAccountAuthorizationError   = 1055
	errCodeOperationAuthorizationError = 1056
	errCodeOperationNotSupportedError  = 1057

	// execution errors 1100 - 1200
	errCodeExecutionError                     = 1000
	errCodeCadenceRunTimeError                = 1101
	errCodeEncodingUnsupportedValue           = 1102
	errCodeStorageCapacityExceeded            = 1103
	errCodeGasLimitExceededError              = 1104
	errCodeEventLimitExceededError            = 1105
	errCodeLedgerIntractionLimitExceededError = 1106
	errCodeStateKeySizeLimitError             = 1107
	errCodeStateValueSizeLimitError           = 1108

	// accounts errors 1200 - 1250
	errCodeAccountError                  = 1200
	errCodeAccountNotFoundError          = 1201
	errCodeAccountPublicKeyNotFoundError = 1202
	errCodeAccountAlreadyExistsError     = 1203
	errCodeFrozenAccountError            = 1204

	// contract errors 1250 - 1300
	errCodeContractError              = 1250
	errCodeContractNotFoundError      = 1251
	errCodeContractNamesNotFoundError = 1252
)
