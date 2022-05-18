package signature

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"

	"github.com/onflow/flow-go/model/flow"
)

// CheckSumLen is fixed to be 4 bytes
const CheckSumLen = 4

func checksum(data []byte) [CheckSumLen]byte {
	// since the checksum is only for detecting honest mistake,
	// crc32 is enough
	sum := crc32.ChecksumIEEE(data)
	// converting the uint32 checksum value into [4]byte
	var sumBytes [CheckSumLen]byte
	binary.BigEndian.PutUint32(sumBytes[:], sum)
	return sumBytes
}

// CheckSumFromIdentities returns checksum for the given identities
func CheckSumFromIdentities(identities []flow.Identifier) [CheckSumLen]byte {
	return checksum(EncodeIdentities(identities))
}

// EncodeIdentities will concatenation all the identities into bytes
func EncodeIdentities(identities []flow.Identifier) []byte {
	// a simple concatenation is deterministic, since each identifier has fixed length.
	encoded := make([]byte, 0, len(identities)*flow.IdentifierLen)
	for _, id := range identities {
		encoded = append(encoded, id[:]...)
	}
	return encoded
}

// PrefixCheckSum prefix the given data with the checksum of the given identifier list
func PrefixCheckSum(canonicalList []flow.Identifier, signrIndices []byte) []byte {
	sum := CheckSumFromIdentities(canonicalList)
	prefixed := make([]byte, 0, len(sum)+len(signrIndices))
	prefixed = append(prefixed, sum[:]...)
	prefixed = append(prefixed, signrIndices[:]...)
	return prefixed
}

// SplitCheckSum splits the given bytes into two parts:
// - prefixed checksum of the canonical identifier list
// - the signerIndices
// Expected error during normal operations:
//  * ErrInvalidChecksum if the input is shorter than the expected checksum contained therein
func SplitCheckSum(checkSumPrefixedSignerIndices []byte) ([CheckSumLen]byte, []byte, error) {
	if len(checkSumPrefixedSignerIndices) < CheckSumLen {
		return [CheckSumLen]byte{}, nil,
			fmt.Errorf("expect checkSumPrefixedSignerIndices to have at least %v bytes, but got %v: %w",
				CheckSumLen, len(checkSumPrefixedSignerIndices), ErrInvalidChecksum)
	}

	var sum [CheckSumLen]byte
	copy(sum[:], checkSumPrefixedSignerIndices[:CheckSumLen])
	signerIndices := checkSumPrefixedSignerIndices[CheckSumLen:]

	return sum, signerIndices, nil
}

// CompareAndExtract reads the checksum from the given `checkSumPrefixedSignerIndices`
// and compares it with the checksum of the given identifier list.
// It returns the signer indices if the checksum matches.
// Inputs:
// - canonicalList is the canonical list from decoder's view
// - checkSumPrefixedSignerIndices is the signer indices created by the encoder,
//   and prefixed with the checksum of the canonical list from encoder's view.
// Expected error during normal operations:
//  * ErrInvalidChecksum if the input is shorter than the expected checksum contained therein
func CompareAndExtract(canonicalList []flow.Identifier, checkSumPrefixedSignerIndices []byte) ([]byte, error) {
	// the checkSumPrefixedSignerIndices bytes contains two parts:
	// 1. the checksum of the canonical identifier list from encoder's view
	// 2. the signer indices
	// so split them
	encoderChecksum, signerIndices, err := SplitCheckSum(checkSumPrefixedSignerIndices)
	if err != nil {
		return nil, fmt.Errorf("could not split checksum: %w", err)
	}

	// this canonicalList here is from decoder's view.
	// by comparing the checksum of the canonical list from encoder's view
	// and the full canonical list from decoder's view, we can tell if the encoder
	// encodes the signer indices using the same list as decoder.
	decoderChecksum := CheckSumFromIdentities(canonicalList)
	match := bytes.Equal(encoderChecksum[:], decoderChecksum[:])
	if !match {
		return nil, fmt.Errorf("decoder sees a canonical list %v, which has a different checksum %x than the encoder's checksum %x: %w",
			canonicalList, decoderChecksum, encoderChecksum, ErrInvalidChecksum)
	}

	return signerIndices, nil
}
