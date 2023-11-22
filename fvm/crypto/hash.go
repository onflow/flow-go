package crypto

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/flow"
)

// constant tag length
const tagLength = flow.DomainTagLength

// prefixedHashing embeds a crypto hasher and implements
// hashing with a prefix : prefixedHashing(data) = hasher(prefix || data)
//
// Prefixes are padded tags till 32 bytes to guarantee prefixedHashers are independent
// hashers.
// Prefixes are disabled with the particular tag value ""
type prefixedHashing struct {
	hash.Hasher
	usePrefix bool
	tag       [tagLength]byte
}

// paddedDomainTag converts a string into a padded byte array.
func paddedDomainTag(s string) ([tagLength]byte, error) {
	var tag [tagLength]byte
	if len(s) > tagLength {
		return tag, fmt.Errorf("domain tag cannot be longer than %d characters, got %s", tagLength, s)
	}
	copy(tag[:], s)
	return tag, nil
}

var hasherCreators = map[hash.HashingAlgorithm](func() hash.Hasher){
	hash.SHA2_256:   hash.NewSHA2_256,
	hash.SHA3_256:   hash.NewSHA3_256,
	hash.SHA2_384:   hash.NewSHA2_384,
	hash.SHA3_384:   hash.NewSHA3_384,
	hash.Keccak_256: hash.NewKeccak_256,
}

// NewPrefixedHashing returns a new hasher that prefixes the tag for all
// hash computations (only when tag is not empty).
//
// Supported algorithms are SHA2-256, SHA2-384, SHA3-256, SHA3-384 and Keccak256.
func NewPrefixedHashing(algo hash.HashingAlgorithm, tag string) (hash.Hasher, error) {

	hasherCreator, hasCreator := hasherCreators[algo]
	if !hasCreator {
		return nil, errors.New("hashing algorithm is not supported for prefixed algorithm")
	}

	paddedTag, err := paddedDomainTag(tag)
	if err != nil {
		return nil, fmt.Errorf("prefixed hashing failed: %w", err)
	}

	return &prefixedHashing{
		Hasher: hasherCreator(),
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
