package accountkeymetadata

import (
	"bytes"
	"encoding/binary"

	"github.com/fxamacker/circlehash"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
)

// FindDuplicateKey returns true with duplicate key index if duplicate key
// of the given key is found.  However, detection rate is intentionally
// not 100% in order to limit the number of digests we store on chain.
// If a hash collision happens with given digest, this function returns
// SentinelFastDigest64 digest and duplicate key not found.
// Specifically, a duplicate key is found when these conditions are met:
// - computed digest isn't the predefined sentinel digest (0),
// - computed digest matches one of the stored digests in key metadata, and
// - given encodedKey also matches the stored key with the same digest.
func FindDuplicateKey(
	keyMetadata *KeyMetadataAppender,
	encodedKey []byte,
	getKeyDigest func([]byte) uint64,
	getStoredKey func(uint32) ([]byte, error),
) (digest uint64, found bool, duplicateStoredKeyIndex uint32, _ error) {

	// To balance tradeoffs, it is OK to have detection rate less than 100%, for the
	// same reasons compression programs/libraries don't use max compression by default.

	// We use a fast non-cryptographic hash algorithm for efficiency, so
	// we need to handle hash collisions (same digest from different hash inputs).
	// When a hash collision is detected, sentinel digest (0) is stored in place
	// of new key digest, and subsequent digest comparison excludes stored sentinel digest.
	// This means keys with the sentinel digest will not be deduplicated and that is OK.

	digest = getKeyDigest(encodedKey)

	if digest == SentinelFastDigest64 {
		// The new key digest matches the sentinel digest by coincidence or attack.
		// Return early so the key will be stored without using deduplication.
		return SentinelFastDigest64, false, 0, nil
	}

	// Find duplicate stored digest by comparing computed digest against stored digests in key metadata section.
	found, duplicateStoredKeyIndex = keyMetadata.findDuplicateDigest(digest)

	// If no duplicate digest is found, we return duplicate not found.
	if !found {
		return digest, false, 0, nil
	}

	// A duplicate digest is found, so we need to compare the stored key to
	// the given encodedKey.

	// Get encoded key with duplicate digest.
	encodedKeyWithDuplicateDigest, err := getStoredKey(duplicateStoredKeyIndex)
	if err != nil {
		return digest, false, 0, err
	}

	// Compare the given encodedKey with stored key.
	if bytes.Equal(encodedKeyWithDuplicateDigest, encodedKey) {
		return digest, true, duplicateStoredKeyIndex, nil
	}

	// Found hash collision. The given encodedKey and the stored key are different but
	// they both produce the same 64-bit fast hash digest.
	// Return SentinelFastDigest64, duplicate key not found, and no error.
	return SentinelFastDigest64, false, 0, nil
}

func GetPublicKeyDigest(owner flow.Address, encodedPublicKey []byte) uint64 {
	seed := binary.BigEndian.Uint64(owner[:])
	return circlehash.Hash64(encodedPublicKey, seed)
}

// SentinelFastDigest64 is the sentinel digest used for 64-bit fast hash collision handling.  SentinelFastDigest64
// is stored in key metadata's digest list as a placeholder.
const SentinelFastDigest64 uint64 = 0 // don't change this value (instead, declare new constant with new value if needed)

// Utility functions for tests and validation

// DecodeDigests decodes raw bytes of digest list in account key metadata.
func DecodeDigests(b []byte) ([]uint64, error) {
	if len(b)%digestSize != 0 {
		return nil, errors.NewKeyMetadataUnexpectedLengthError(
			"failed to decode digest list",
			digestSize,
			len(b),
		)
	}

	storedDigestCount := len(b) / digestSize

	digests := make([]uint64, 0, storedDigestCount)

	for i := 0; i < len(b); i += digestSize {
		digests = append(digests, binary.BigEndian.Uint64(b[i:]))
	}

	return digests, nil
}
