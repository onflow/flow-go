package errors

import "github.com/onflow/flow-go/model/flow"

// NewKeyMetadataEmptyError creates a new CodedFailure. It is returned
// when key metadata cannot be parsed because it is unexpectedly empty.
func NewKeyMetadataEmptyError(msgPrefix string) CodedFailure {
	return NewCodedFailuref(
		FailureCodeKeyMetadataDecodingFailure,
		msgPrefix,
		"key metadata is empty")
}

// NewKeyMetadataTooShortError creates a new CodedFailure. It is returned
// when key metadata cannot be parsed because it is truncated.
func NewKeyMetadataTooShortError(msgPrefix string, expectedLength, actualLength int) CodedFailure {
	return NewCodedFailuref(
		FailureCodeKeyMetadataDecodingFailure,
		msgPrefix,
		"key metadata is too short: expect %d bytes, got %d bytes",
		expectedLength,
		actualLength,
	)
}

// NewKeyMetadataUnexpectedLengthError creates a new CodedFailure. It is returned
// when key metadata cannot be parsed because its length is unexpected.
func NewKeyMetadataUnexpectedLengthError(msgPrefix string, groupLength, actualLength int) CodedFailure {
	return NewCodedFailuref(
		FailureCodeKeyMetadataDecodingFailure,
		msgPrefix,
		"key metadata length is unexpected: expect multiples of %d, got %d bytes",
		groupLength,
		actualLength,
	)
}

// NewKeyMetadataTrailingDataError creates a new CodedFailure. It is returned
// when key metadata cannot be parsed because it has trailing data after expected content.
func NewKeyMetadataTrailingDataError(msgPrefix string, trailingDataLength int) CodedFailure {
	return NewCodedFailuref(
		FailureCodeKeyMetadataDecodingFailure,
		msgPrefix,
		"key metadata has trailing data: expect no trailing data, got %d bytes",
		trailingDataLength,
	)
}

// NewKeyMetadataNotFoundError creates a new CodedFailure. It is returned
// when key metadata cannot be found for the given stored key index.
func NewKeyMetadataNotFoundError(msgPrefix string, storedKeyIndex uint32) CodedFailure {
	return NewCodedFailuref(
		FailureCodeKeyMetadataNotFoundFailure,
		msgPrefix,
		"key metadata not found at stored key index %d",
		storedKeyIndex,
	)
}

// NewKeyMetadataUnexpectedKeyIndexError creates a new CodedFailure. It is returned
// when key metadata cannot be found for unexpected stored key index.
func NewKeyMetadataUnexpectedKeyIndexError(msgPrefix string, storedKeyIndex uint32) CodedFailure {
	return NewCodedFailuref(
		FailureCodeKeyMetadataUnexpectedKeyIndexFailure,
		msgPrefix,
		"unexpected key index %d",
		storedKeyIndex,
	)
}

// NewBatchPublicKeyDecodingError creates a new CodedFailure. It is returned
// when batch public key payload cannot be decoded.
func NewBatchPublicKeyDecodingError(msgPrefix string, address flow.Address, batchIndex uint32) CodedFailure {
	return NewCodedFailuref(
		FailureCodeBatchPublicKeyDecodingFailure,
		msgPrefix,
		"batch public key payload is malformed at batch index %d for address %s",
		batchIndex,
		address,
	)
}

// NewStoredPublicKeyNotFoundError creates a new CodedFailure. It is returned
// when stored public key cannot be found for the given stored key index.
func NewStoredPublicKeyNotFoundError(msgPrefix string, address flow.Address, storedKeyIndex uint32) CodedFailure {
	return NewCodedFailuref(
		FailureCodeStoredPublicKeyNotFoundFailure,
		msgPrefix,
		"stored public key not found at stored key index %d for address %s",
		storedKeyIndex,
		address,
	)
}

// NewStoredPublicKeyUnexpectedIndexError creates a new CodedFailure. It is returned
// when stored public key cannot be found for unexpected stored key index.
func NewStoredPublicKeyUnexpectedIndexError(msgPrefix string, address flow.Address, storedKeyIndex uint32) CodedFailure {
	return NewCodedFailuref(
		FailureCodeStoredPublicKeyUnexpectedIndexFailure,
		msgPrefix,
		"unexpected stored key index %d for address %s",
		storedKeyIndex,
		address,
	)
}

// NewBatchPublicKeyNotFoundError creates a new CodedFailure. It is returned
// when batch public key payload cannot be found for the given batch ndex.
func NewBatchPublicKeyNotFoundError(msgPrefix string, address flow.Address, batchIndex uint32) CodedFailure {
	return NewCodedFailuref(
		FailureCodeBatchPublicKeyNotFoundFailure,
		msgPrefix,
		"batch public key payload not found for address %s and batch index %d",
		address,
		batchIndex,
	)
}

// IsKeyMetadataDecodingError returns true if error has FailureKeyMetadataDecodingFailure type.
func IsKeyMetadataDecodingError(err error) bool {
	return HasFailureCode(err, FailureCodeKeyMetadataDecodingFailure)
}

// IsKeyMetadataNotFoundError returns true if error has FailureKeyMetadataNotFoundFailure type.
func IsKeyMetadataNotFoundError(err error) bool {
	return HasFailureCode(err, FailureCodeKeyMetadataNotFoundFailure)
}

// IsKeyMetadataUnexpectedKeyIndexError returns true if error has FailureKeyMetadataUnexpectedKeyIndexFailure type.
func IsKeyMetadataUnexpectedKeyIndexError(err error) bool {
	return HasFailureCode(err, FailureCodeKeyMetadataUnexpectedKeyIndexFailure)
}

// IsStoredPublicKeyNotFoundError returns true if error has FailureStoredPublicKeyNotFoundFailure type.
func IsStoredPublicKeyNotFoundError(err error) bool {
	return HasFailureCode(err, FailureCodeStoredPublicKeyNotFoundFailure)
}

// IsStoredPublicKeyUnexpectedIndexError returns true if error has FailureStoredPublicKeyUnexpectedIndexFailure type.
func IsStoredPublicKeyUnexpectedIndexError(err error) bool {
	return HasFailureCode(err, FailureCodeStoredPublicKeyUnexpectedIndexFailure)
}

// IsBatchPublicKeyDecodingError returns true if error has FailureCodeBatchPublicKeyDecodingFailure type.
func IsBatchPublicKeyDecodingError(err error) bool {
	return HasFailureCode(err, FailureCodeBatchPublicKeyDecodingFailure)
}

// IsBatchPublicKeyNotFoundError returns true if error has FailureBatchPublicKeyNotFoundFailure type.
func IsBatchPublicKeyNotFoundError(err error) bool {
	return HasFailureCode(err, FailureCodeBatchPublicKeyNotFoundFailure)
}
