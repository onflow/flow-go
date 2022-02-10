package hash

// Size returns the output size of the hash function in bytes.
func (d *spongeState) Size() int {
	return d.outputLen
}

// Algorithm returns the hashing algorithm of the instance.
func (s *spongeState) Algorithm() HashingAlgorithm {
	return s.algo
}

// ComputeHash calculates and returns the digest of the input.
// It updates the state (and therefore not thread-safe) and doesn't allow
// further writing without calling Reset().
func (s *spongeState) ComputeHash(data []byte) Hash {
	s.Reset()
	s.write(data)
	return s.sum()
}

// SumHash returns the digest of the data written to the state.
// It updates the state and doesn't allow further writing without
// calling Reset().
func (s *spongeState) SumHash() Hash {
	return s.sum()
}

// Write absorbs more data into the hash's state.
// It returns the number of bytes written and never errors.
func (d *spongeState) Write(p []byte) (int, error) {
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

type spongeState struct {
	// the hashing algorithm name
	algo HashingAlgorithm

	a       [25]uint64 // main state of the hash
	storage storageBuf // constant size array
	// `buf` is a sub-slice that points into `storage` using `bufIndex` and `bufSize`:
	// - `bufIndex` is the index of the first element of buf
	// - `bufSize` is the size of buf
	bufIndex int
	bufSize  int
	rate     int // the number of bytes of state to use
	// dsbyte contains the domain separation bits (if any are defined)
	// and the first bit of the 10*1 padding.
	// Using a little-endian bit-ordering convention, it is 0b01 for SHA-3
	// and not defined for legacy Keccak.
	// The padding 10*1 is applied to pad the message to a multiple
	// of the rate, which involves adding a "1" bit, zero or more "0" bits, and
	// a final "1" bit. We merge the first "1" bit from the padding into dsbyte,
	// ( giving 0b00000110 for SHA-3 and 0b00000001 for legacy Keccak)
	// [1] https://keccak.team/sponge_duplex.html
	//     "The sponge and duplex constructions"
	dsByte    byte // the domain separation byte with one bit padding
	outputLen int  // the default output size in bytes
}

const (
	// maxRate is the maximum size of the internal buffer. SHA3-256
	// currently needs the largest buffer among supported sponge-based
	// algorithms.
	maxRate = rateSHA3_256

	// initialization value of the buffer index
	bufNilValue = -1
)

// returns the current buf
func (d *spongeState) buf() []byte {
	return d.storage.asBytes()[d.bufIndex : d.bufIndex+d.bufSize]
}

// setBuf assigns `buf` (sub-slice of `storage`) to a sub-slice of `storage`
// defined by a starting index and size.
func (d *spongeState) setBuf(start, size int) {
	d.bufIndex = start
	d.bufSize = size
}

// checks if `buf` is nil (not yet set)
func (d *spongeState) bufIsNil() bool {
	return d.bufSize == bufNilValue
}

// appendBuf appends a slice to `buf` (sub-slice of `storage`)
// The function assumes the appended buffer still fits into `storage`.
func (d *spongeState) appendBuf(slice []byte) {
	copy(d.storage.asBytes()[d.bufIndex+d.bufSize:], slice)
	d.bufSize += len(slice)
}

// Reset clears the internal state.
func (d *spongeState) Reset() {
	// Zero the permutation's state.
	for i := range d.a {
		d.a[i] = 0
	}
	d.setBuf(0, 0)
}

// permute applies the KeccakF-1600 permutation.
func (d *spongeState) permute() {
	// xor the input into the state before applying the permutation.
	xorIn(d, d.buf())
	d.setBuf(0, 0)
	keccakF1600(&d.a)
}

func (d *spongeState) write(p []byte) {
	if d.bufIsNil() {
		d.setBuf(0, 0)
	}

	for len(p) > 0 {
		if d.bufSize == 0 && len(p) >= d.rate {
			// The fast path; absorb a full "rate" bytes of input and apply the permutation.
			xorIn(d, p[:d.rate])
			p = p[d.rate:]
			keccakF1600(&d.a)
		} else {
			// The slow path; buffer the input until we can fill the sponge, and then xor it in.
			todo := d.rate - d.bufSize
			if todo > len(p) {
				todo = len(p)
			}
			d.appendBuf(p[:todo])
			p = p[todo:]

			// If the sponge is full, apply the permutation.
			if d.bufSize == d.rate {
				d.permute()
			}
		}
	}
}

// pads appends the domain separation bits in dsbyte, applies
// the multi-bitrate 10..1 padding rule, and permutes the state.
func (d *spongeState) padAndPermute() {
	if d.bufIsNil() {
		d.setBuf(0, 0)
	}
	// Pad with this instance with dsbyte. We know that there's
	// at least one byte of space in d.buf because, if it were full,
	// permute would have been called to empty it. dsbyte also contains the
	// first one bit for the padding. See the comment in the state struct.
	d.appendBuf([]byte{d.dsByte})
	zerosStart := d.bufSize
	d.setBuf(0, d.rate)
	buf := d.buf()
	for i := zerosStart; i < d.rate; i++ {
		buf[i] = 0
	}
	// This adds the final one bit for the padding. Because of the way that
	// bits are numbered from the LSB upwards, the final bit is the MSB of
	// the last byte.
	buf[d.rate-1] ^= 0x80
	// Apply the permutation
	d.permute()
	d.setBuf(0, d.rate)
}

// Sum applies padding to the hash state and then squeezes out the desired
// number of output bytes.
func (d *spongeState) sum() []byte {
	hash := make([]byte, d.outputLen)
	d.padAndPermute()
	copyOut(hash, d)
	return hash
}
