package crypto

import (
	"errors"
)

// blsAggregateEmptyListError is returned when a list of BLS objects (e.g. signatures or keys)
// is empty or nil and thereby represents an invalid input.
var blsAggregateEmptyListError = errors.New("list cannot be empty")

// IsBLSAggregateEmptyListError checks if err is an `blsAggregateEmptyListError`.
// blsAggregateEmptyListError is returned when a BLS aggregation function is called with
// an empty list which is not allowed in some aggregation cases to avoid signature equivocation
// issues.
func IsBLSAggregateEmptyListError(err error) bool {
	return errors.Is(err, blsAggregateEmptyListError)
}

// notBLSKeyError is returned when a private or public key
// used is not a BLS on BLS12 381 key.
var notBLSKeyError = errors.New("input key has to be a BLS on BLS12-381 key")

// IsNotBLSKeyError checks if err is an `notBLSKeyError`.
// notBLSKeyError is returned when a private or public key
// used is not a BLS on BLS12 381 key.
func IsNotBLSKeyError(err error) bool {
	return errors.Is(err, notBLSKeyError)
}

// invalidSignatureError is returned when a signature input does not serialize to a
// valid element on E1 of the BLS12-381 curve (but without checking the element is on subgroup G1).
var invalidSignatureError = errors.New("input signature does not deserialize to an E1 element")

// IsInvalidSignatureError checks if err is an `invalidSignatureError`
// invalidSignatureError is returned when a signature input does not serialize to a
// valid element on E1 of the BLS12-381 curve (but without checking the element is on subgroup G1).
func IsInvalidSignatureError(err error) bool {
	return errors.Is(err, invalidSignatureError)
}
