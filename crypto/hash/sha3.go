package hash

import (
	"github.com/onflow/flow-go/crypto/hash/keccak"
)

const (
	rateSha3_256 = 136
	rateSha3_384 = 104
)

// NewSHA3_256 returns a new instance of SHA3-256 hasher.
func NewSHA3_256() Hasher {
	return &sha3State{
		rate:      rateSha3_256,
		outputLen: HashLenSha3_256,
	}
}

// NewSHA3_384 returns a new instance of SHA3-384 hasher.
func NewSHA3_384() Hasher {
	return &sha3State{
		rate:      rateSha3_384,
		outputLen: HashLenSha3_384,
	}
}

// Size returns the output size of the hash function in bytes.
func (d *sha3State) Size() int {
	return d.outputLen
}

// Algorithm returns the hashing algorithm of the instance.
func (s *sha3State) Algorithm() HashingAlgorithm {
	switch s.outputLen {
	case HashLenSha3_256:
		return SHA3_256
	case HashLenSha3_384:
		return SHA3_384
	default:
		panic("failed to return the hashing algorithm because of an incompatible output length")
	}
}

// ComputeHash calculates and returns the SHA3 digest of the input.
// It updates the state (and therefore not thread-safe) and doesn't allow
// further writing without calling Reset().
func (s *sha3State) ComputeHash(data []byte) Hash {
	s.Reset()
	s.write(data)
	return s.sum()
}

// SumHash returns the SHA3-256 digest of the data written to the state.
// It updates the state and doesn't allow further writing without
// calling Reset().
func (s *sha3State) SumHash() Hash {
	return s.sum()
}

// Write absorbs more data into the hash's state.
// It returns the number of bytes written and never errors.
func (d *sha3State) Write(p []byte) (int, error) {
	d.write(p)
	return len(p), nil
}

// The functions below were copied and modified from golang.org/x/crypto/sha3.
//
// Copyright (c) 2009 The Go Authors. All rights reserved.

// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:

//    * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//    * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//    * Neither the name of Google Inc. nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.

// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

const (
	// maxRate is the maximum size of the internal buffer. SHA3-256
	// currently needs the largest buffer.
	maxRate = 1088 / 8

	// dsbyte contains the "domain separation" bits and the first bit of
	// the padding.
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
	a         [25]uint64 // main state of the hash
	buf       []byte     // points into storage
	storage   storageBuf
	rate      int // the number of bytes of state to use
	outputLen int // the default output size in bytes
}

// Reset clears the internal state.
func (d *sha3State) Reset() {
	// Zero the permutation's state.
	for i := range d.a {
		d.a[i] = 0
	}
	d.buf = d.storage.asBytes()[:0]
}

// permute applies the KeccakF-1600 permutation.
func (d *sha3State) permute() {
	// xor the input into the state before applying the permutation.
	xorIn(d, d.buf)
	d.buf = d.storage.asBytes()[:0]
	keccak.KeccakF1600(&d.a)
}

func (d *sha3State) write(p []byte) {
	if d.buf == nil {
		d.buf = d.storage.asBytes()[:0]
	}

	for len(p) > 0 {
		if len(d.buf) == 0 && len(p) >= d.rate {
			// The fast path; absorb a full "rate" bytes of input and apply the permutation.
			xorIn(d, p[:d.rate])
			p = p[d.rate:]
			keccak.KeccakF1600(&d.a)
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
}

// pads appends the domain separation bits in dsbyte, applies
// the multi-bitrate 10..1 padding rule, and permutes the state.
func (d *sha3State) padAndPermute() {
	if d.buf == nil {
		d.buf = d.storage.asBytes()[:0]
	}
	// Pad with this instance with dsbyte. We know that there's
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
