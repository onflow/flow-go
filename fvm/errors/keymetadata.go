package errors

// NewKeyMetadataEmptyError creates a new CodedFailure. It is returned
// when key metadata cannot be parsed because it is unexpectedly empty.
func NewKeyMetadataEmptyError(msgPrefix string) CodedFailure {
	if len(msgPrefix) > 0 {
		msgPrefix += ": "
	}
	return NewCodedFailure(
		FailureKeyMetadataDecodingFailure,
		msgPrefix+"key metadata is empty")
}

// NewKeyMetadataTooShortError creates a new CodedFailure. It is returned
// when key metadata cannot be parsed because it is truncated.
func NewKeyMetadataTooShortError(msgPrefix string, expectedLength, actualLength int) CodedFailure {
	if len(msgPrefix) > 0 {
		msgPrefix += ": "
	}
	return NewCodedFailure(
		FailureKeyMetadataDecodingFailure,
		msgPrefix+"key metadata is too short: expect %d bytes, got %d bytes",
		expectedLength,
		actualLength,
	)
}

// NewKeyMetadataUnexpectedLengthError creates a new CodedFailure. It is returned
// when key metadata cannot be parsed because its length is unexpected.
func NewKeyMetadataUnexpectedLengthError(msgPrefix string, groupLength, actualLength int) CodedFailure {
	if len(msgPrefix) > 0 {
		msgPrefix += ": "
	}
	return NewCodedFailure(
		FailureKeyMetadataDecodingFailure,
		msgPrefix+"key metadata length is unexpected: expect multiples of %d, got %d bytes",
		groupLength,
		actualLength,
	)
}

// NewKeyMetadataTrailingDataError creates a new CodedFailure. It is returned
// when key metadata cannot be parsed because it has trailing data after expected content.
func NewKeyMetadataTrailingDataError(msgPrefix string, trailingDataLength int) CodedFailure {
	if len(msgPrefix) > 0 {
		msgPrefix += ": "
	}
	return NewCodedFailure(
		FailureKeyMetadataDecodingFailure,
		msgPrefix+"key metadata has trailing data: expect no trailing data, got %d bytes",
		trailingDataLength,
	)
}

// NewKeyMetadataNotFoundError creates a new CodedFailure. It is returned
// when key metadata cannot be found for the given stored key index.
func NewKeyMetadataNotFoundError(msgPrefix string, storedKeyIndex uint32) CodedFailure {
	if len(msgPrefix) > 0 {
		msgPrefix += ": "
	}
	return NewCodedFailure(
		FailureKeyMetadataNotFoundFailure,
		msgPrefix+"key metadata not found at stored key index %d",
		storedKeyIndex,
	)
}

// NewKeyMetadataUnexpectedKeyIndexError creates a new CodedFailure. It is returned
// when key metadata cannot be found for unexpected stored key index.
func NewKeyMetadataUnexpectedKeyIndexError(msgPrefix string, storedKeyIndex uint32) CodedFailure {
	if len(msgPrefix) > 0 {
		msgPrefix += ": "
	}
	return NewCodedFailure(
		FailureKeyMetadataUnexpectedKeyIndexFailure,
		msgPrefix+"unexpected key index %d",
		storedKeyIndex,
	)
}

// IsKeyMetadataDecodingError returns true if error has FailureKeyMetadataDecodingFailure type.
func IsKeyMetadataDecodingError(err error) bool {
	return HasFailureCode(err, FailureKeyMetadataDecodingFailure)
}

// IsKeyMetadataNotFoundError returns true if error has FailureKeyMetadataNotFoundFailure type.
func IsKeyMetadataNotFoundError(err error) bool {
	return HasFailureCode(err, FailureKeyMetadataNotFoundFailure)
}

// IsKeyMetadataUnexpectedKeyIndexError returns true if error has FailureKeyMetadataUnexpectedKeyIndexFailure type.
func IsKeyMetadataUnexpectedKeyIndexError(err error) bool {
	return HasFailureCode(err, FailureKeyMetadataUnexpectedKeyIndexFailure)
}
