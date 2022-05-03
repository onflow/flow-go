package signature

import (
	"bytes"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// CheckSumLen is fixed to be 16 bytes, which is also md5.Size
const CheckSumLen = 32

// CheckSumFromIdentities returns checksum for the given identities
func CheckSumFromIdentities(identities []flow.Identifier) [CheckSumLen]byte {
	return flow.MakeID(EncodeIdentities(identities))
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
func PrefixCheckSum(identities []flow.Identifier, data []byte) []byte {
	sum := CheckSumFromIdentities(identities)
	prefixed := make([]byte, len(sum)+len(data))
	copy(prefixed[0:len(sum)], sum[:])
	copy(prefixed[len(sum):], data[:])
	return prefixed
}

// SplitCheckSum splits the given bytes into two parts: prefixed checksum of the identifier list
// and the data.
func SplitCheckSum(prefixed []byte) ([CheckSumLen]byte, []byte, error) {
	if len(prefixed) < CheckSumLen {
		return [CheckSumLen]byte{}, nil,
			fmt.Errorf("expect at least %v bytes, but got %v", CheckSumLen, len(prefixed))
	}

	var sum [CheckSumLen]byte
	copy(sum[:], prefixed[:CheckSumLen])
	data := prefixed[CheckSumLen:]
	return sum, data, nil
}

// CompareChecksum compares if the given checksum matches with the checksum of the given identifier list
func CompareChecksum(sum [CheckSumLen]byte, identities []flow.Identifier) bool {
	computedSum := CheckSumFromIdentities(identities)
	return bytes.Equal(sum[:], computedSum[:])
}

// CompareAndExtract reads the checksum from the given prefixed data bytes, and compare with the checksum
// of the given identifier list.
// it returns the data if the checksum matches.
// it returns error if the checksum doesn't match
func CompareAndExtract(identities []flow.Identifier, prefixed []byte) ([]byte, error) {
	// the prefixed bytes contains both the checksum and the signer indices, split them
	sum, data, err := SplitCheckSum(prefixed)
	if err != nil {
		return nil, fmt.Errorf("could not split checksum: %w", err)
	}

	match := CompareChecksum(sum, identities)
	if !match {
		return nil, fmt.Errorf("data %x does not match with checksum %x", data, sum)
	}

	return data, nil
}
