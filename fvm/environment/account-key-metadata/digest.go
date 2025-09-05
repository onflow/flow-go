package accountkeymetadata

import "bytes"

// FindDuplicateKey returns true with duplicate key index if duplicate key
// of the given key is found.  However, detection rate is intentionally
// not 100% in order to limit the number of digests we store on chain.
// If a hash collision happens with given digest, CollisionError is returned.
// Specifically, a duplicate key is found when these conditions are met:
// - given digest isn't the predefined sentinel digest (0),
// - given digest matches one of the stored digests in key metadata, and
// - given encodedKey also matches the stored key with the same digest.
func FindDuplicateKey(
	keyMetadata *KeyMetadataAppender,
	encodedKey []byte,
	digest uint64,
	getStoredKey func(uint32) ([]byte, error),
) (found bool, duplicateStoredKeyIndex uint32, _ error) {

	// To balance tradeoffs, it is OK to have detection rate less than 100%, for the
	// same reasons compression programs/libraries don't use max compression by default.

	// We use a fast non-cryptographic hash algorithm for efficiency, so
	// we need to handle hash collisions (same digest from different hash inputs).
	// When a hash collision is detected, sentinel digest (0) is stored in place
	// of new key digest, and subsequent digest comparison excludes stored sentinel digest.
	// This means keys with the sentinel digest will not be deduplicated and that is OK.

	if digest == SentinelFastDigest64 {
		// The new key digest matches the sentinel digest by coincidence or attack.
		// Return early so the key will be stored without using deduplication.
		return false, 0, nil
	}

	// Find duplicate digest
	found, duplicateStoredKeyIndex = keyMetadata.findDuplicateDigest(digest)

	if !found {
		return false, 0, nil
	}

	// Get encoded key with duplicate digest
	encodedKeyWithDuplicateDigest, err := getStoredKey(duplicateStoredKeyIndex)
	if err != nil {
		return false, 0, err
	}

	// Confirm keys are duplicate
	if bytes.Equal(encodedKeyWithDuplicateDigest, encodedKey) {
		return true, duplicateStoredKeyIndex, nil
	}

	// Found a collision
	return false, 0, NewCollisionError(encodedKeyWithDuplicateDigest, encodedKey, digest)
}

// SentinelFastDigest64 is the sentinel digest used for 64-bit fast hash collision handling.  SentinelFastDigest64
// is stored in key metadata's digest list as a placeholder.
const SentinelFastDigest64 uint64 = 0 // don't change this value (instead, declare new constant with new value if needed)
