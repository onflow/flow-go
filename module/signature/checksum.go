package signature

import (
	"bytes"
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
	return [CheckSumLen]byte{
		byte(sum >> 24),
		byte(sum >> 16),
		byte(sum >> 8),
		byte(sum),
	}
}

// CheckSumFromIdentities returns checksum for the given identities
func CheckSumFromIdentities(identities []flow.Identifier) [CheckSumLen]byte {
	return checksum(EncodeIdentities(identities))
}

// EncodeIdentities will concatenation all the identities into bytes
func EncodeIdentities(identities []flow.Identifier) []byte {
	// a simple concatenation is determinsitic, since each identifier has fixed length.
	encoded := make([]byte, len(identities)*flow.IdentifierLen)
	for i, id := range identities {
		copy(encoded[i*flow.IdentifierLen:(i+1)*flow.IdentifierLen], id[:])
	}
	return encoded
}

// PrefixCheckSum prefix the given data with the checksum of the given identifier list
func PrefixCheckSum(canonicalList []flow.Identifier, signrIndices []byte) []byte {
	sum := CheckSumFromIdentities(canonicalList)
	prefixed := make([]byte, len(sum)+len(signrIndices))
	copy(prefixed[0:len(sum)], sum[:])
	copy(prefixed[len(sum):], signrIndices[:])
	return prefixed
}

// SplitCheckSum splits the given bytes into two parts:
// - prefixed checksum of the canonical identifier list
// - the signerIndices
// the prefixed checksum is used to verify if the decoder is decoding the signer indices
// using the same canonical identifier list
func SplitCheckSum(checkSumPrefixedSignerIndices []byte) ([CheckSumLen]byte, []byte, error) {
	if len(checkSumPrefixedSignerIndices) < CheckSumLen {
		return [CheckSumLen]byte{}, nil,
			fmt.Errorf("expect checkSumPrefixedSignerIndices to have at least %v bytes, but got %v",
				CheckSumLen, len(checkSumPrefixedSignerIndices))
	}

	var sum [CheckSumLen]byte
	copy(sum[:], checkSumPrefixedSignerIndices[:CheckSumLen])
	signerIndices := checkSumPrefixedSignerIndices[CheckSumLen:]

	return sum, signerIndices, nil
}

// CompareAndExtract reads the checksum from the given checkSumPrefixedSIgnerIndices
// bytes, and compare with the checksum of the given identifier list.
// it returns the signer indices if the checksum matches.
// it returns error if splitting the checksum fails or
// 										 the splitted checksum doesn't match
// - canonicalList is the canonical list from decoder's view
// - checkSumPrefixedSignerIndices is the signer indices created by the encoder,
//   and prefixed with the checksum of the canonical list from encoder's view.
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
		return nil, fmt.Errorf("decoder sees a canonical list %v, which has a different checksum %x than the encoder's checksum %x",
			canonicalList, decoderChecksum, encoderChecksum)
	}

	return signerIndices, nil
}
