package errors

const (
	// tx validation errors
	errCodeInvalidTxByteSizeError     = 1
	errCodeInvalidReferenceBlockError = 2
	errCodeExpiredTransactionError    = 3
	errCodeInvalidScriptError         = 4
	errCodeInvalidGasLimitError       = 5
	errCodeInvalidAddressError        = 6
	errCodeInvalidArgumentError       = 7

	errCodeInvalidHashAlgorithmError      = 10
	errCodeInvalidSignatureAlgorithmError = 11
	errCodeInvalidPublicKeyValueError     = 12

	// execution errors
	errCodeInvalidProposalSignatureError  = 50
	errCodeProposalSeqNumberMismatchError = 51

	errCodeInvalidPayloadSignatureError = 60
	errCodePayloadSignatureKeyError     = 61

	errCodeInvalidEnvelopeSignatureError = 70
	errCodeEnvelopeSignatureKeyError     = 71

	errCodeAuthorizationError = 80

	errCodeCadenceRunTimeError        = 100
	errCodeEncodingUnsupportedValue   = 120
	errCodeOperationNotSupportedError = 121
	errCodeInvalidBlockHeightError    = 122

	// account errors
	errCodeAccountNotFoundError          = 150
	errCodeAccountPublicKeyNotFoundError = 151
	errCodeAccountAlreadyExistsError     = 152
	errCodeFrozenAccountError            = 153

	errCodeContractNotFoundError      = 160
	errCodeContractNamesNotFoundError = 161

	// limit errors
	errCodeStorageCapacityExceeded = 200
	// errCodeInsufficientTokenBalanceError      = 201
	errCodeGasLimitExceededError              = 202
	errCodeEventLimitExceededError            = 203
	errCodeLedgerIntractionLimitExceededError = 204
	errCodeStateKeySizeLimitError             = 205
	errCodeStateValueSizeLimitError           = 206
)

const (
	failureCodeUnknownFailure     = 1000
	failureCodeEncodingFailure    = 1001
	failureCodeLedgerFailure      = 1002
	failureCodeStateMergeFailure  = 1003
	failureCodeBlockFinderFailure = 1004
)
