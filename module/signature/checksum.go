package signature

import (
	"crypto/md5"

	"github.com/onflow/flow-go/model/flow"
)

// CheckSumLen is fixed to be 16 bytes, which is also md5.Size
const CheckSumLen = 16

// CheckSumFromIdentities returns checksum for the given identities
func CheckSumFromIdentities(identities []flow.Identifier) [CheckSumLen]byte {
	return md5.Sum(EncodeIdentities(identities))
}

// EncodeIdentities will concatenation all the identities into bytes
func EncodeIdentities(identities []flow.Identifier) []byte {
	encoded := make([]byte, len(identities)*flow.IdentifierLen)
	for i, id := range identities {
		copy(encoded[i*flow.IdentifierLen:(i+1)*flow.IdentifierLen], id[:])
	}
	return encoded
}
