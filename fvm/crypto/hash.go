package crypto

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/flow"
)

// prefixedHashing embeds a crypto hasher and implements
// hashing with a prefix : prefixedHashing(data) = hasher(prefix || data)
//
// Prefixes are padded tags till 32 bytes to guarantee prefixedHashers are independant
// hashers.
// Prefixes are disabled with the particular tag value ""
type prefixedHashing struct {
	hash.Hasher
	usePrefix bool
	tag       [flow.DomainTagLength]byte
}

// paddedDomainTag converts a string into a padded byte array.
func paddedDomainTag(s string) ([flow.DomainTagLength]byte, error) {
	var tag [flow.DomainTagLength]byte
	if len(s) > flow.DomainTagLength {
		return tag, fmt.Errorf("domain tag %s cannot be longer than %d characters", s, flow.DomainTagLength)
	}
	copy(tag[:], s)
	return tag, nil
}

// NewPrefixedHashing returns a new hasher that prefixes the tag for all
// hash computations (only when tag is not empty).
// Only SHA2 and SHA3 algorithms are supported.
func NewPrefixedHashing(shaAlgo hash.HashingAlgorithm, tag string) (hash.Hasher, error) {

	var hasher hash.Hasher
	switch shaAlgo {
	case hash.SHA2_256:
		hasher = hash.NewSHA2_256()
	case hash.SHA3_256:
		hasher = hash.NewSHA3_256()
	case hash.SHA2_384:
		hasher = hash.NewSHA2_384()
	case hash.SHA3_384:
		hasher = hash.NewSHA3_384()
	case hash.Keccak_256:
		hasher = hash.NewKeccak_256()
	default:
		return nil, errors.New("hashing algorithm is not a supported for prefixed algorithm")
	}

	paddedTag, err := paddedDomainTag(tag)
	if err != nil {
		return nil, fmt.Errorf("prefixed hashing failed: %w", err)
	}

	return &prefixedHashing{
		Hasher: hasher,
		// if tag is empty, do not use any prefix (standard hashing)
		usePrefix: tag != "",
		tag:       paddedTag,
	}, nil
}

// ComputeHash calculates and returns the digest of input byte array prefixed by a tag.
// Not thread-safe
func (s *prefixedHashing) ComputeHash(data []byte) hash.Hash {
	s.Reset()
	_, _ = s.Write(data)
	return s.Hasher.SumHash()
}

// Reset gets the hasher back to its original state.
func (s *prefixedHashing) Reset() {
	s.Hasher.Reset()
	// include the tag only when using a prefix is enabled
	if s.usePrefix {
		_, _ = s.Write(s.tag[:])
	}
}
