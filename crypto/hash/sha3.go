package hash

import (
	"hash"

	"golang.org/x/crypto/sha3"
)

// sha3_256Algo, embeds commonHasher
type sha3_256Algo struct {
	hash.Hash
}

// NewSHA3_256 returns a new instance of SHA3-256 hasher
func NewSHA3_256() Hasher {
	return &sha3_256Algo{
		Hash: sha3.New256()}
}

func (s *sha3_256Algo) Algorithm() HashingAlgorithm {
	return SHA3_256
}

// ComputeHash calculates and returns the SHA3-256 output of input byte array.
// It does not reset the state to allow further writing.
func (s *sha3_256Algo) ComputeHash(data []byte) Hash {
	s.Reset()
	_, _ = s.Write(data)
	return s.Sum(nil)
}

// SumHash returns the SHA3-256 output.
// It does not reset the state to allow further writing.
func (s *sha3_256Algo) SumHash() Hash {
	return s.Sum(nil)
}

// sha3_384Algo, embeds commonHasher
type sha3_384Algo struct {
	hash.Hash
}

// NewSHA3_384 returns a new instance of SHA3-384 hasher
func NewSHA3_384() Hasher {
	return &sha3_384Algo{
		Hash: sha3.New384()}
}

func (s *sha3_384Algo) Algorithm() HashingAlgorithm {
	return SHA3_384
}

// ComputeHash calculates and returns the SHA3-384 output of input byte array.
// It does not reset the state to allow further writing.
func (s *sha3_384Algo) ComputeHash(data []byte) Hash {
	s.Reset()
	_, _ = s.Write(data)
	return s.Sum(nil)
}

// SumHash returns the SHA3-384 output.
// It does not reset the state to allow further writing.
func (s *sha3_384Algo) SumHash() Hash {
	return s.Sum(nil)
}

///////////////////////////////////////////////////////////

// NewSHA3_256 returns a new instance of SHA3-256 hasher
func NewSHA3_256_opt() Hasher {
	return &sha3State{
		rate:      136,
		outputLen: HashLenSha3_256,
	}
}

// NewSHA3_384 returns a new instance of SHA3-384 hasher
func NewSHA3_384_opt() Hasher {
	return &sha3State{
		rate:      104,
		outputLen: HashLenSha3_384,
	}
}

// Size returns the output size of the hash function in bytes.
func (d *sha3State) Size() int {
	return d.outputLen
}

func (s *sha3State) Algorithm() HashingAlgorithm {
	switch s.outputLen {
	case HashLenSha3_256:
		return HashLenSha3_256
	case HashLenSha3_384:
		return HashLenSha3_384
	}
	return UnknownHashingAlgorithm
}

// ComputeHash calculates and returns the SHA3-256 output of input byte array.
// It does not reset the state to allow further writing.
func (s *sha3State) ComputeHash(data []byte) Hash {
	_, _ = s.Write(data)
	return s.sum()
}

// SumHash returns the SHA3-256 output.
// It does not reset the state to allow further writing.
func (s *sha3State) SumHash() Hash {
	return s.sum()
}

const (
	// maxRate is the maximum size of the internal buffer. SHA3-256
	// currently needs the largest buffer.
	maxRate = 1088 / 8

	// dsbyte contains the "domain separation" bits and the first bit of
	// the padding. Sections 6.1 and 6.2 of [1] separate the outputs of the
	// SHA-3 and SHAKE functions by appending bitstrings to the message.
	// Using a little-endian bit-ordering convention, it is "01" for SHA-3.
	// The padding rule from section 5.1 is applied to pad the message to a multiple
	// of the rate, which involves adding a "1" bit, zero or more "0" bits, and
	// a final "1" bit. We merge the first "1" bit from the padding into dsbyte,
	// giving 00000110b (0x06).
	// [1] http://csrc.nist.gov/publications/drafts/fips-202/fips_202_draft.pdf
	//     "Draft FIPS 202: SHA-3 Standard: Permutation-Based Hash and
	//      Extendable-Output Functions (May 2014)"
	dsbyte = byte(0x6)
)

type sha3State struct {
	// Generic sponge components.
	a         [25]uint64 // main state of the hash
	buf       []byte     // points into storage
	storage   storageBuf
	rate      int // the number of bytes of state to use
	outputLen int // the default output size in bytes
}

// Reset clears the internal state by zeroing the sponge state and
// the byte buffer, and setting Sponge.state to absorbing.
func (d *sha3State) Reset() {
	// Zero the permutation's state.
	d.buf = d.storage.asBytes()[:0]
}

// permute applies the KeccakF-1600 permutation. It handles
// any input-output buffering.
func (d *sha3State) permute() {
	// xor the input into the state before applying the permutation.
	xorIn(d, d.buf)
	d.buf = d.storage.asBytes()[:0]
	keccakF1600(&d.a)
}

// Write absorbs more data into the hash's state. It produces an error
// if more data is written to the ShakeHash after writing
func (d *sha3State) Write(p []byte) (written int, err error) {
	if d.buf == nil {
		d.buf = d.storage.asBytes()[:0]
	}
	written = len(p)

	for len(p) > 0 {
		if len(d.buf) == 0 && len(p) >= d.rate {
			// The fast path; absorb a full "rate" bytes of input and apply the permutation.
			xorIn(d, p[:d.rate])
			p = p[d.rate:]
			keccakF1600(&d.a)
		} else {
			// The slow path; buffer the input until we can fill the sponge, and then xor it in.
			todo := d.rate - len(d.buf)
			if todo > len(p) {
				todo = len(p)
			}
			d.buf = append(d.buf, p[:todo]...)
			p = p[todo:]

			// If the sponge is full, apply the permutation.
			if len(d.buf) == d.rate {
				d.permute()
			}
		}
	}
	return
}

// pads appends the domain separation bits in dsbyte, applies
// the multi-bitrate 10..1 padding rule, and permutes the state.
func (d *sha3State) padAndPermute() {
	if d.buf == nil {
		d.buf = d.storage.asBytes()[:0]
	}
	// Pad with this instance's domain-separator bits. We know that there's
	// at least one byte of space in d.buf because, if it were full,
	// permute would have been called to empty it. dsbyte also contains the
	// first one bit for the padding. See the comment in the state struct.
	d.buf = append(d.buf, dsbyte)
	zerosStart := len(d.buf)
	d.buf = d.storage.asBytes()[:d.rate]
	for i := zerosStart; i < d.rate; i++ {
		d.buf[i] = 0
	}
	// This adds the final one bit for the padding. Because of the way that
	// bits are numbered from the LSB upwards, the final bit is the MSB of
	// the last byte.
	d.buf[d.rate-1] ^= 0x80
	// Apply the permutation
	d.permute()
	d.buf = d.storage.asBytes()[:d.rate]
}

// Sum applies padding to the hash state and then squeezes out the desired
// number of output bytes.
func (d *sha3State) sum() []byte {
	hash := make([]byte, d.outputLen)
	d.padAndPermute()
	copyOut(hash, d)
	return hash
}
