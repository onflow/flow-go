package crypto

import (
	"encoding/binary"

	"golang.org/x/crypto/sha3"
)

// implements the interface sha3.ShakeHash
type kmac128 struct {
	// Common hasher
	// includes the output size of KMAC
	*commonHasher
	// embeds ShakeHash
	// stores the encoding of the function name and customization string
	// Using the io.Writer interface changes the internal state
	// of the KMAC
	sha3.ShakeHash
	// the block initialized by NewKmac128
	// stores the encoding of the key
	initBlock []byte
}

// the cSHAKE128 rate as defined in NIST SP 800-185
const cSHAKE128BlockSize = 168

// NewKmac128 returns a new KMAC instance
// - key is the KMAC key (in this function, the length is not compared
// against the security level, this function should not be used in
// the general KMAC case)
// - customizer is the customization string
func NewKmac128(key []byte, customizer []byte, outputSize int) Hasher {
	var k kmac128
	k.commonHasher = &commonHasher{
		algo:       KMAC128,
		outputSize: outputSize}
	// initialize the cSHAKE128 instance
	k.ShakeHash = sha3.NewCShake128([]byte("KMAC"), customizer)
	// key length should be larger than the bit-security level
	// This is not checked here as the key is public and only used
	// as a domain tag
	// store the encoding of the key
	k.initBlock = bytepad(encodeString(key), cSHAKE128BlockSize)
	k.Write(k.initBlock)
	return &k
}

// endocee_string function as defined in NIST SP 800-185 (for value < 2^64)
func encodeString(s []byte) []byte {
	// leftEncode returns max 9 bytes
	out := make([]byte, 0, 9+len(s))
	out = append(out, leftEncode(uint64(len(s)*8))...)
	out = append(out, s...)
	return out
}

// left_Encode function as defined in NIST SP 800-185 (for value < 2^64)
// copied from golang.org/x/crypto/sha3
func leftEncode(value uint64) []byte {
	var b [9]byte
	binary.BigEndian.PutUint64(b[1:], value)
	// Trim all but last leading zero bytes
	i := byte(1)
	for i < 8 && b[i] == 0 {
		i++
	}
	// Prepend number of encoded bytes
	b[i-1] = 9 - i
	return b[i-1:]
}

// bytepad function as defined in NIST SP 800-185
// copied from golang.org/x/crypto/sha3
func bytepad(input []byte, w int) []byte {
	// leftEncode always returns max 9 bytes
	buf := make([]byte, 0, 9+len(input)+w)
	buf = append(buf, leftEncode(uint64(w))...)
	buf = append(buf, input...)
	padlen := w - (len(buf) % w)
	return append(buf, make([]byte, padlen)...)
}

// right_Encode function as defined in NIST SP 800-185 (for value < 2^64)
func rightEncode(value uint64) []byte {
	var b [9]byte
	binary.BigEndian.PutUint64(b[:8], value)
	// Trim all but last leading zero bytes
	i := byte(0)
	for i < 7 && b[i] == 0 {
		i++
	}
	// Append number of encoded bytes
	b[8] = 8 - i
	return b[i:]
}

// Reset resets the hash to initial state.
func (k *kmac128) Reset() {
	k.ShakeHash.Reset()
	k.Write(k.initBlock)
}

// ComputeHash adds the input data to the a mac state copy
// and returns the mac output
// It does not change the underlying hash state.
func (k *kmac128) ComputeHash(data []byte) Hash {
	cshake := k.ShakeHash.Clone()
	cshake.Write(data)
	cshake.Write(rightEncode(uint64(k.outputSize * 8)))
	// read the cshake output
	h := make([]byte, k.outputSize)
	cshake.Read(h)
	return h
}

// SumHash finalizes the mac computations using a state copy,
// and returns the hash output
// It does not change the underlying hash state.
func (k *kmac128) SumHash() Hash {
	cshake := k.ShakeHash.Clone()
	cshake.Write(rightEncode(uint64(k.outputSize * 8)))
	// read the cshake output
	h := make([]byte, k.outputSize)
	cshake.Read(h)
	return h
}

// Size returns the output length of the KMAC instance
func (k *kmac128) Size() int {
	return k.outputSize
}
