package accountkeymetadata

import "bytes"

func FindDuplicateKey(
	keyMetadata *KeyMetadataAppender,
	encodedKey []byte,
	digest uint64,
	getStoredKey func(uint32) ([]byte, error),
) (found bool, duplicateStoredKeyIndex uint32, _ error) {
	if digest == SentinelFastDigest64 {
		// The new key digest matches the sentinel digest by coincidence or attack.
		// Return early so the key will be stored without using deduplication.
		return false, 0, nil
	}

	// Find duplicate digest
	found, duplicateStoredKeyIndex = keyMetadata.FindDuplicateDigest(digest)

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
