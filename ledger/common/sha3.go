package common

import (
	"encoding/binary"
)

// All functions are copied and modified from golang.org/x/crypto/sha3
// This is a specific version of sha3 optimized only for the functions in
// this package and must not be used elsewhere
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
	// rate is size of the internal buffer.
	rate = 136

	// dsbyte contains the "domain separation" bits and the first bit of
	// the padding. Sections 6.1 and 6.2 of [1] separate the outputs of the
	// SHA-3 and SHAKE functions by appending bitstrings to the message.
	// Using a little-endian bit-ordering convention, these are "01" for SHA-3
	// and "1111" for SHAKE, or 00000010b and 00001111b, respectively. Then the
	// padding rule from section 5.1 is applied to pad the message to a multiple
	// of the rate, which involves adding a "1" bit, zero or more "0" bits, and
	// a final "1" bit. We merge the first "1" bit from the padding into dsbyte,
	// giving 00000110b (0x06) and 00011111b (0x1f).
	// [1] http://csrc.nist.gov/publications/drafts/fips-202/fips_202_draft.pdf
	//     "Draft FIPS 202: SHA-3 Standard: Permutation-Based Hash and
	//      Extendable-Output Functions (May 2014)"
	dsbyte     = byte(0x06)
	paddingEnd = uint64(1 << 63)
)

type state struct {
	a [25]uint64 // main state of the hash
}

// New256 creates a new SHA3-256 hash.
// Its generic security strength is 256 bits against preimage attacks,
// and 128 bits against collision attacks.
func new256() *state {
	d := &state{}
	return d
}

// copyOut copies ulint64s to a byte buffer.
func (d *state) copyOut(out *[32]byte) {
	for i := 0; i < 4; i++ {
		binary.LittleEndian.PutUint64((*out)[i<<3:], d.a[i])
	}
}

func xorInAtIndex(d *state, buf []byte, index int) {
	n := len(buf) >> 3
	aAtIndex := d.a[index:]

	for i := 0; i < n; i++ {
		a := binary.LittleEndian.Uint64(buf)
		aAtIndex[i] ^= a
		buf = buf[8:]
	}
}

func (d *state) hash256Plus(out *[32]byte, p1, p2 []byte) {
	//xorIn since p1 length is a multiple of 8
	xorInAtIndex(d, p1, 0)
	written := 32 // written uint64s in the state

	for len(p2)+written >= rate {
		xorInAtIndex(d, p2[:rate-written], written>>3)
		keccakF1600(&d.a)
		p2 = p2[rate-written:]
		written = 0 // to avoid
	}

	// xorIn the left over of p2, 64 bits at a time
	for len(p2) >= 8 {
		a := binary.LittleEndian.Uint64(p2[:8])
		d.a[written>>3] ^= a
		p2 = p2[8:]
		written += 8
	}

	var tmp [8]byte
	copy(tmp[:], p2)
	tmp[len(p2)] = dsbyte
	a := binary.LittleEndian.Uint64(tmp[:])
	d.a[written>>3] ^= a

	// the last padding
	d.a[16] ^= paddingEnd

	// permute
	finalKeccakF1600(&d.a)

	// reverse and output
	d.copyOut(out)
}

// hash256plus256 absorbs two 256 bits slices of data into the hash's state
// applies the permutation, and outpute the result in out
func (d *state) hash256plus256(out *[32]byte, p1, p2 []byte) {
	xorIn512(d, p1, p2)
	// permute
	finalKeccakF1600(&d.a)
	// reverese the endianess to the output
	d.copyOut(out)
}

// xorIn256 xors two 32 bytes slices into the state; it
// makes no non-portable assumptions about memory layout
// or alignment.
func xorIn512(d *state, buf1, buf2 []byte) {
	var i int
	for ; i < 4; i++ {
		d.a[i] = binary.LittleEndian.Uint64(buf1)
		buf1 = buf1[8:]
	}
	for ; i < 8; i++ {
		d.a[i] = binary.LittleEndian.Uint64(buf2)
		buf2 = buf2[8:]
	}
	// xor with the dsbyte
	// dsbyte also contains the first one bit for the padding.
	d.a[8] = 0x6
	// xor the last padding bit
	d.a[16] = paddingEnd
}

// rc stores the round constants for use in the ι step.
var rc = [24]uint64{
	0x0000000000000001,
	0x0000000000008082,
	0x800000000000808A,
	0x8000000080008000,
	0x000000000000808B,
	0x0000000080000001,
	0x8000000080008081,
	0x8000000000008009,
	0x000000000000008A,
	0x0000000000000088,
	0x0000000080008009,
	0x000000008000000A,
	0x000000008000808B,
	0x800000000000008B,
	0x8000000000008089,
	0x8000000000008003,
	0x8000000000008002,
	0x8000000000000080,
	0x000000000000800A,
	0x800000008000000A,
	0x8000000080008081,
	0x8000000000008080,
	0x0000000080000001,
	0x8000000080008008,
}

// keccakF1600 applies the Keccak permutation to a 1600b-wide
// state represented as a slice of 25 uint64s.
func keccakF1600(a *[25]uint64) {
	// Implementation translated from Keccak-inplace.c
	// in the keccak reference code.
	var t, bc0, bc1, bc2, bc3, bc4, d0, d1, d2, d3, d4 uint64

	for i := 0; i < 24; i += 4 {
		// Combines the 5 steps in each round into 2 steps.
		// Unrolls 4 rounds per loop and spreads some steps across rounds.

		// Round 1
		bc0 = a[0] ^ a[5] ^ a[10] ^ a[15] ^ a[20]
		bc1 = a[1] ^ a[6] ^ a[11] ^ a[16] ^ a[21]
		bc2 = a[2] ^ a[7] ^ a[12] ^ a[17] ^ a[22]
		bc3 = a[3] ^ a[8] ^ a[13] ^ a[18] ^ a[23]
		bc4 = a[4] ^ a[9] ^ a[14] ^ a[19] ^ a[24]
		d0 = bc4 ^ (bc1<<1 | bc1>>63)
		d1 = bc0 ^ (bc2<<1 | bc2>>63)
		d2 = bc1 ^ (bc3<<1 | bc3>>63)
		d3 = bc2 ^ (bc4<<1 | bc4>>63)
		d4 = bc3 ^ (bc0<<1 | bc0>>63)

		bc0 = a[0] ^ d0
		t = a[6] ^ d1
		bc1 = t<<44 | t>>(64-44)
		t = a[12] ^ d2
		bc2 = t<<43 | t>>(64-43)
		t = a[18] ^ d3
		bc3 = t<<21 | t>>(64-21)
		t = a[24] ^ d4
		bc4 = t<<14 | t>>(64-14)
		a[0] = bc0 ^ (bc2 &^ bc1) ^ rc[i]
		a[6] = bc1 ^ (bc3 &^ bc2)
		a[12] = bc2 ^ (bc4 &^ bc3)
		a[18] = bc3 ^ (bc0 &^ bc4)
		a[24] = bc4 ^ (bc1 &^ bc0)

		t = a[10] ^ d0
		bc2 = t<<3 | t>>(64-3)
		t = a[16] ^ d1
		bc3 = t<<45 | t>>(64-45)
		t = a[22] ^ d2
		bc4 = t<<61 | t>>(64-61)
		t = a[3] ^ d3
		bc0 = t<<28 | t>>(64-28)
		t = a[9] ^ d4
		bc1 = t<<20 | t>>(64-20)
		a[10] = bc0 ^ (bc2 &^ bc1)
		a[16] = bc1 ^ (bc3 &^ bc2)
		a[22] = bc2 ^ (bc4 &^ bc3)
		a[3] = bc3 ^ (bc0 &^ bc4)
		a[9] = bc4 ^ (bc1 &^ bc0)

		t = a[20] ^ d0
		bc4 = t<<18 | t>>(64-18)
		t = a[1] ^ d1
		bc0 = t<<1 | t>>(64-1)
		t = a[7] ^ d2
		bc1 = t<<6 | t>>(64-6)
		t = a[13] ^ d3
		bc2 = t<<25 | t>>(64-25)
		t = a[19] ^ d4
		bc3 = t<<8 | t>>(64-8)
		a[20] = bc0 ^ (bc2 &^ bc1)
		a[1] = bc1 ^ (bc3 &^ bc2)
		a[7] = bc2 ^ (bc4 &^ bc3)
		a[13] = bc3 ^ (bc0 &^ bc4)
		a[19] = bc4 ^ (bc1 &^ bc0)

		t = a[5] ^ d0
		bc1 = t<<36 | t>>(64-36)
		t = a[11] ^ d1
		bc2 = t<<10 | t>>(64-10)
		t = a[17] ^ d2
		bc3 = t<<15 | t>>(64-15)
		t = a[23] ^ d3
		bc4 = t<<56 | t>>(64-56)
		t = a[4] ^ d4
		bc0 = t<<27 | t>>(64-27)
		a[5] = bc0 ^ (bc2 &^ bc1)
		a[11] = bc1 ^ (bc3 &^ bc2)
		a[17] = bc2 ^ (bc4 &^ bc3)
		a[23] = bc3 ^ (bc0 &^ bc4)
		a[4] = bc4 ^ (bc1 &^ bc0)

		t = a[15] ^ d0
		bc3 = t<<41 | t>>(64-41)
		t = a[21] ^ d1
		bc4 = t<<2 | t>>(64-2)
		t = a[2] ^ d2
		bc0 = t<<62 | t>>(64-62)
		t = a[8] ^ d3
		bc1 = t<<55 | t>>(64-55)
		t = a[14] ^ d4
		bc2 = t<<39 | t>>(64-39)
		a[15] = bc0 ^ (bc2 &^ bc1)
		a[21] = bc1 ^ (bc3 &^ bc2)
		a[2] = bc2 ^ (bc4 &^ bc3)
		a[8] = bc3 ^ (bc0 &^ bc4)
		a[14] = bc4 ^ (bc1 &^ bc0)

		// Round 2
		bc0 = a[0] ^ a[5] ^ a[10] ^ a[15] ^ a[20]
		bc1 = a[1] ^ a[6] ^ a[11] ^ a[16] ^ a[21]
		bc2 = a[2] ^ a[7] ^ a[12] ^ a[17] ^ a[22]
		bc3 = a[3] ^ a[8] ^ a[13] ^ a[18] ^ a[23]
		bc4 = a[4] ^ a[9] ^ a[14] ^ a[19] ^ a[24]
		d0 = bc4 ^ (bc1<<1 | bc1>>63)
		d1 = bc0 ^ (bc2<<1 | bc2>>63)
		d2 = bc1 ^ (bc3<<1 | bc3>>63)
		d3 = bc2 ^ (bc4<<1 | bc4>>63)
		d4 = bc3 ^ (bc0<<1 | bc0>>63)

		bc0 = a[0] ^ d0
		t = a[16] ^ d1
		bc1 = t<<44 | t>>(64-44)
		t = a[7] ^ d2
		bc2 = t<<43 | t>>(64-43)
		t = a[23] ^ d3
		bc3 = t<<21 | t>>(64-21)
		t = a[14] ^ d4
		bc4 = t<<14 | t>>(64-14)
		a[0] = bc0 ^ (bc2 &^ bc1) ^ rc[i+1]
		a[16] = bc1 ^ (bc3 &^ bc2)
		a[7] = bc2 ^ (bc4 &^ bc3)
		a[23] = bc3 ^ (bc0 &^ bc4)
		a[14] = bc4 ^ (bc1 &^ bc0)

		t = a[20] ^ d0
		bc2 = t<<3 | t>>(64-3)
		t = a[11] ^ d1
		bc3 = t<<45 | t>>(64-45)
		t = a[2] ^ d2
		bc4 = t<<61 | t>>(64-61)
		t = a[18] ^ d3
		bc0 = t<<28 | t>>(64-28)
		t = a[9] ^ d4
		bc1 = t<<20 | t>>(64-20)
		a[20] = bc0 ^ (bc2 &^ bc1)
		a[11] = bc1 ^ (bc3 &^ bc2)
		a[2] = bc2 ^ (bc4 &^ bc3)
		a[18] = bc3 ^ (bc0 &^ bc4)
		a[9] = bc4 ^ (bc1 &^ bc0)

		t = a[15] ^ d0
		bc4 = t<<18 | t>>(64-18)
		t = a[6] ^ d1
		bc0 = t<<1 | t>>(64-1)
		t = a[22] ^ d2
		bc1 = t<<6 | t>>(64-6)
		t = a[13] ^ d3
		bc2 = t<<25 | t>>(64-25)
		t = a[4] ^ d4
		bc3 = t<<8 | t>>(64-8)
		a[15] = bc0 ^ (bc2 &^ bc1)
		a[6] = bc1 ^ (bc3 &^ bc2)
		a[22] = bc2 ^ (bc4 &^ bc3)
		a[13] = bc3 ^ (bc0 &^ bc4)
		a[4] = bc4 ^ (bc1 &^ bc0)

		t = a[10] ^ d0
		bc1 = t<<36 | t>>(64-36)
		t = a[1] ^ d1
		bc2 = t<<10 | t>>(64-10)
		t = a[17] ^ d2
		bc3 = t<<15 | t>>(64-15)
		t = a[8] ^ d3
		bc4 = t<<56 | t>>(64-56)
		t = a[24] ^ d4
		bc0 = t<<27 | t>>(64-27)
		a[10] = bc0 ^ (bc2 &^ bc1)
		a[1] = bc1 ^ (bc3 &^ bc2)
		a[17] = bc2 ^ (bc4 &^ bc3)
		a[8] = bc3 ^ (bc0 &^ bc4)
		a[24] = bc4 ^ (bc1 &^ bc0)

		t = a[5] ^ d0
		bc3 = t<<41 | t>>(64-41)
		t = a[21] ^ d1
		bc4 = t<<2 | t>>(64-2)
		t = a[12] ^ d2
		bc0 = t<<62 | t>>(64-62)
		t = a[3] ^ d3
		bc1 = t<<55 | t>>(64-55)
		t = a[19] ^ d4
		bc2 = t<<39 | t>>(64-39)
		a[5] = bc0 ^ (bc2 &^ bc1)
		a[21] = bc1 ^ (bc3 &^ bc2)
		a[12] = bc2 ^ (bc4 &^ bc3)
		a[3] = bc3 ^ (bc0 &^ bc4)
		a[19] = bc4 ^ (bc1 &^ bc0)

		// Round 3
		bc0 = a[0] ^ a[5] ^ a[10] ^ a[15] ^ a[20]
		bc1 = a[1] ^ a[6] ^ a[11] ^ a[16] ^ a[21]
		bc2 = a[2] ^ a[7] ^ a[12] ^ a[17] ^ a[22]
		bc3 = a[3] ^ a[8] ^ a[13] ^ a[18] ^ a[23]
		bc4 = a[4] ^ a[9] ^ a[14] ^ a[19] ^ a[24]
		d0 = bc4 ^ (bc1<<1 | bc1>>63)
		d1 = bc0 ^ (bc2<<1 | bc2>>63)
		d2 = bc1 ^ (bc3<<1 | bc3>>63)
		d3 = bc2 ^ (bc4<<1 | bc4>>63)
		d4 = bc3 ^ (bc0<<1 | bc0>>63)

		bc0 = a[0] ^ d0
		t = a[11] ^ d1
		bc1 = t<<44 | t>>(64-44)
		t = a[22] ^ d2
		bc2 = t<<43 | t>>(64-43)
		t = a[8] ^ d3
		bc3 = t<<21 | t>>(64-21)
		t = a[19] ^ d4
		bc4 = t<<14 | t>>(64-14)
		a[0] = bc0 ^ (bc2 &^ bc1) ^ rc[i+2]
		a[11] = bc1 ^ (bc3 &^ bc2)
		a[22] = bc2 ^ (bc4 &^ bc3)
		a[8] = bc3 ^ (bc0 &^ bc4)
		a[19] = bc4 ^ (bc1 &^ bc0)

		t = a[15] ^ d0
		bc2 = t<<3 | t>>(64-3)
		t = a[1] ^ d1
		bc3 = t<<45 | t>>(64-45)
		t = a[12] ^ d2
		bc4 = t<<61 | t>>(64-61)
		t = a[23] ^ d3
		bc0 = t<<28 | t>>(64-28)
		t = a[9] ^ d4
		bc1 = t<<20 | t>>(64-20)
		a[15] = bc0 ^ (bc2 &^ bc1)
		a[1] = bc1 ^ (bc3 &^ bc2)
		a[12] = bc2 ^ (bc4 &^ bc3)
		a[23] = bc3 ^ (bc0 &^ bc4)
		a[9] = bc4 ^ (bc1 &^ bc0)

		t = a[5] ^ d0
		bc4 = t<<18 | t>>(64-18)
		t = a[16] ^ d1
		bc0 = t<<1 | t>>(64-1)
		t = a[2] ^ d2
		bc1 = t<<6 | t>>(64-6)
		t = a[13] ^ d3
		bc2 = t<<25 | t>>(64-25)
		t = a[24] ^ d4
		bc3 = t<<8 | t>>(64-8)
		a[5] = bc0 ^ (bc2 &^ bc1)
		a[16] = bc1 ^ (bc3 &^ bc2)
		a[2] = bc2 ^ (bc4 &^ bc3)
		a[13] = bc3 ^ (bc0 &^ bc4)
		a[24] = bc4 ^ (bc1 &^ bc0)

		t = a[20] ^ d0
		bc1 = t<<36 | t>>(64-36)
		t = a[6] ^ d1
		bc2 = t<<10 | t>>(64-10)
		t = a[17] ^ d2
		bc3 = t<<15 | t>>(64-15)
		t = a[3] ^ d3
		bc4 = t<<56 | t>>(64-56)
		t = a[14] ^ d4
		bc0 = t<<27 | t>>(64-27)
		a[20] = bc0 ^ (bc2 &^ bc1)
		a[6] = bc1 ^ (bc3 &^ bc2)
		a[17] = bc2 ^ (bc4 &^ bc3)
		a[3] = bc3 ^ (bc0 &^ bc4)
		a[14] = bc4 ^ (bc1 &^ bc0)

		t = a[10] ^ d0
		bc3 = t<<41 | t>>(64-41)
		t = a[21] ^ d1
		bc4 = t<<2 | t>>(64-2)
		t = a[7] ^ d2
		bc0 = t<<62 | t>>(64-62)
		t = a[18] ^ d3
		bc1 = t<<55 | t>>(64-55)
		t = a[4] ^ d4
		bc2 = t<<39 | t>>(64-39)
		a[10] = bc0 ^ (bc2 &^ bc1)
		a[21] = bc1 ^ (bc3 &^ bc2)
		a[7] = bc2 ^ (bc4 &^ bc3)
		a[18] = bc3 ^ (bc0 &^ bc4)
		a[4] = bc4 ^ (bc1 &^ bc0)

		// Round 4
		bc0 = a[0] ^ a[5] ^ a[10] ^ a[15] ^ a[20]
		bc1 = a[1] ^ a[6] ^ a[11] ^ a[16] ^ a[21]
		bc2 = a[2] ^ a[7] ^ a[12] ^ a[17] ^ a[22]
		bc3 = a[3] ^ a[8] ^ a[13] ^ a[18] ^ a[23]
		bc4 = a[4] ^ a[9] ^ a[14] ^ a[19] ^ a[24]
		d0 = bc4 ^ (bc1<<1 | bc1>>63)
		d1 = bc0 ^ (bc2<<1 | bc2>>63)
		d2 = bc1 ^ (bc3<<1 | bc3>>63)
		d3 = bc2 ^ (bc4<<1 | bc4>>63)
		d4 = bc3 ^ (bc0<<1 | bc0>>63)

		bc0 = a[0] ^ d0
		t = a[1] ^ d1
		bc1 = t<<44 | t>>(64-44)
		t = a[2] ^ d2
		bc2 = t<<43 | t>>(64-43)
		t = a[3] ^ d3
		bc3 = t<<21 | t>>(64-21)
		t = a[4] ^ d4
		bc4 = t<<14 | t>>(64-14)
		a[0] = bc0 ^ (bc2 &^ bc1) ^ rc[i+3]
		a[1] = bc1 ^ (bc3 &^ bc2)
		a[2] = bc2 ^ (bc4 &^ bc3)
		a[3] = bc3 ^ (bc0 &^ bc4)
		a[4] = bc4 ^ (bc1 &^ bc0)

		t = a[5] ^ d0
		bc2 = t<<3 | t>>(64-3)
		t = a[6] ^ d1
		bc3 = t<<45 | t>>(64-45)
		t = a[7] ^ d2
		bc4 = t<<61 | t>>(64-61)
		t = a[8] ^ d3
		bc0 = t<<28 | t>>(64-28)
		t = a[9] ^ d4
		bc1 = t<<20 | t>>(64-20)
		a[5] = bc0 ^ (bc2 &^ bc1)
		a[6] = bc1 ^ (bc3 &^ bc2)
		a[7] = bc2 ^ (bc4 &^ bc3)
		a[8] = bc3 ^ (bc0 &^ bc4)
		a[9] = bc4 ^ (bc1 &^ bc0)

		t = a[10] ^ d0
		bc4 = t<<18 | t>>(64-18)
		t = a[11] ^ d1
		bc0 = t<<1 | t>>(64-1)
		t = a[12] ^ d2
		bc1 = t<<6 | t>>(64-6)
		t = a[13] ^ d3
		bc2 = t<<25 | t>>(64-25)
		t = a[14] ^ d4
		bc3 = t<<8 | t>>(64-8)
		a[10] = bc0 ^ (bc2 &^ bc1)
		a[11] = bc1 ^ (bc3 &^ bc2)
		a[12] = bc2 ^ (bc4 &^ bc3)
		a[13] = bc3 ^ (bc0 &^ bc4)
		a[14] = bc4 ^ (bc1 &^ bc0)

		t = a[15] ^ d0
		bc1 = t<<36 | t>>(64-36)
		t = a[16] ^ d1
		bc2 = t<<10 | t>>(64-10)
		t = a[17] ^ d2
		bc3 = t<<15 | t>>(64-15)
		t = a[18] ^ d3
		bc4 = t<<56 | t>>(64-56)
		t = a[19] ^ d4
		bc0 = t<<27 | t>>(64-27)
		a[15] = bc0 ^ (bc2 &^ bc1)
		a[16] = bc1 ^ (bc3 &^ bc2)
		a[17] = bc2 ^ (bc4 &^ bc3)
		a[18] = bc3 ^ (bc0 &^ bc4)
		a[19] = bc4 ^ (bc1 &^ bc0)

		t = a[20] ^ d0
		bc3 = t<<41 | t>>(64-41)
		t = a[21] ^ d1
		bc4 = t<<2 | t>>(64-2)
		t = a[22] ^ d2
		bc0 = t<<62 | t>>(64-62)
		t = a[23] ^ d3
		bc1 = t<<55 | t>>(64-55)
		t = a[24] ^ d4
		bc2 = t<<39 | t>>(64-39)
		a[20] = bc0 ^ (bc2 &^ bc1)
		a[21] = bc1 ^ (bc3 &^ bc2)
		a[22] = bc2 ^ (bc4 &^ bc3)
		a[23] = bc3 ^ (bc0 &^ bc4)
		a[24] = bc4 ^ (bc1 &^ bc0)
	}
}

func finalKeccakF1600(a *[25]uint64) {
	// Implementation translated from Keccak-inplace.c
	// in the keccak reference code.
	var t, bc0, bc1, bc2, bc3, bc4, d0, d1, d2, d3, d4 uint64

	var i int
	for i = 0; i < 20; i += 4 {
		// Combines the 5 steps in each round into 2 steps.
		// Unrolls 4 rounds per loop and spreads some steps across rounds.

		// Round 1
		bc0 = a[0] ^ a[5] ^ a[10] ^ a[15] ^ a[20]
		bc1 = a[1] ^ a[6] ^ a[11] ^ a[16] ^ a[21]
		bc2 = a[2] ^ a[7] ^ a[12] ^ a[17] ^ a[22]
		bc3 = a[3] ^ a[8] ^ a[13] ^ a[18] ^ a[23]
		bc4 = a[4] ^ a[9] ^ a[14] ^ a[19] ^ a[24]
		d0 = bc4 ^ (bc1<<1 | bc1>>63)
		d1 = bc0 ^ (bc2<<1 | bc2>>63)
		d2 = bc1 ^ (bc3<<1 | bc3>>63)
		d3 = bc2 ^ (bc4<<1 | bc4>>63)
		d4 = bc3 ^ (bc0<<1 | bc0>>63)

		bc0 = a[0] ^ d0
		t = a[6] ^ d1
		bc1 = t<<44 | t>>(64-44)
		t = a[12] ^ d2
		bc2 = t<<43 | t>>(64-43)
		t = a[18] ^ d3
		bc3 = t<<21 | t>>(64-21)
		t = a[24] ^ d4
		bc4 = t<<14 | t>>(64-14)
		a[0] = bc0 ^ (bc2 &^ bc1) ^ rc[i]
		a[6] = bc1 ^ (bc3 &^ bc2)
		a[12] = bc2 ^ (bc4 &^ bc3)
		a[18] = bc3 ^ (bc0 &^ bc4)
		a[24] = bc4 ^ (bc1 &^ bc0)

		t = a[10] ^ d0
		bc2 = t<<3 | t>>(64-3)
		t = a[16] ^ d1
		bc3 = t<<45 | t>>(64-45)
		t = a[22] ^ d2
		bc4 = t<<61 | t>>(64-61)
		t = a[3] ^ d3
		bc0 = t<<28 | t>>(64-28)
		t = a[9] ^ d4
		bc1 = t<<20 | t>>(64-20)
		a[10] = bc0 ^ (bc2 &^ bc1)
		a[16] = bc1 ^ (bc3 &^ bc2)
		a[22] = bc2 ^ (bc4 &^ bc3)
		a[3] = bc3 ^ (bc0 &^ bc4)
		a[9] = bc4 ^ (bc1 &^ bc0)

		t = a[20] ^ d0
		bc4 = t<<18 | t>>(64-18)
		t = a[1] ^ d1
		bc0 = t<<1 | t>>(64-1)
		t = a[7] ^ d2
		bc1 = t<<6 | t>>(64-6)
		t = a[13] ^ d3
		bc2 = t<<25 | t>>(64-25)
		t = a[19] ^ d4
		bc3 = t<<8 | t>>(64-8)
		a[20] = bc0 ^ (bc2 &^ bc1)
		a[1] = bc1 ^ (bc3 &^ bc2)
		a[7] = bc2 ^ (bc4 &^ bc3)
		a[13] = bc3 ^ (bc0 &^ bc4)
		a[19] = bc4 ^ (bc1 &^ bc0)

		t = a[5] ^ d0
		bc1 = t<<36 | t>>(64-36)
		t = a[11] ^ d1
		bc2 = t<<10 | t>>(64-10)
		t = a[17] ^ d2
		bc3 = t<<15 | t>>(64-15)
		t = a[23] ^ d3
		bc4 = t<<56 | t>>(64-56)
		t = a[4] ^ d4
		bc0 = t<<27 | t>>(64-27)
		a[5] = bc0 ^ (bc2 &^ bc1)
		a[11] = bc1 ^ (bc3 &^ bc2)
		a[17] = bc2 ^ (bc4 &^ bc3)
		a[23] = bc3 ^ (bc0 &^ bc4)
		a[4] = bc4 ^ (bc1 &^ bc0)

		t = a[15] ^ d0
		bc3 = t<<41 | t>>(64-41)
		t = a[21] ^ d1
		bc4 = t<<2 | t>>(64-2)
		t = a[2] ^ d2
		bc0 = t<<62 | t>>(64-62)
		t = a[8] ^ d3
		bc1 = t<<55 | t>>(64-55)
		t = a[14] ^ d4
		bc2 = t<<39 | t>>(64-39)
		a[15] = bc0 ^ (bc2 &^ bc1)
		a[21] = bc1 ^ (bc3 &^ bc2)
		a[2] = bc2 ^ (bc4 &^ bc3)
		a[8] = bc3 ^ (bc0 &^ bc4)
		a[14] = bc4 ^ (bc1 &^ bc0)

		// Round 2
		bc0 = a[0] ^ a[5] ^ a[10] ^ a[15] ^ a[20]
		bc1 = a[1] ^ a[6] ^ a[11] ^ a[16] ^ a[21]
		bc2 = a[2] ^ a[7] ^ a[12] ^ a[17] ^ a[22]
		bc3 = a[3] ^ a[8] ^ a[13] ^ a[18] ^ a[23]
		bc4 = a[4] ^ a[9] ^ a[14] ^ a[19] ^ a[24]
		d0 = bc4 ^ (bc1<<1 | bc1>>63)
		d1 = bc0 ^ (bc2<<1 | bc2>>63)
		d2 = bc1 ^ (bc3<<1 | bc3>>63)
		d3 = bc2 ^ (bc4<<1 | bc4>>63)
		d4 = bc3 ^ (bc0<<1 | bc0>>63)

		bc0 = a[0] ^ d0
		t = a[16] ^ d1
		bc1 = t<<44 | t>>(64-44)
		t = a[7] ^ d2
		bc2 = t<<43 | t>>(64-43)
		t = a[23] ^ d3
		bc3 = t<<21 | t>>(64-21)
		t = a[14] ^ d4
		bc4 = t<<14 | t>>(64-14)
		a[0] = bc0 ^ (bc2 &^ bc1) ^ rc[i+1]
		a[16] = bc1 ^ (bc3 &^ bc2)
		a[7] = bc2 ^ (bc4 &^ bc3)
		a[23] = bc3 ^ (bc0 &^ bc4)
		a[14] = bc4 ^ (bc1 &^ bc0)

		t = a[20] ^ d0
		bc2 = t<<3 | t>>(64-3)
		t = a[11] ^ d1
		bc3 = t<<45 | t>>(64-45)
		t = a[2] ^ d2
		bc4 = t<<61 | t>>(64-61)
		t = a[18] ^ d3
		bc0 = t<<28 | t>>(64-28)
		t = a[9] ^ d4
		bc1 = t<<20 | t>>(64-20)
		a[20] = bc0 ^ (bc2 &^ bc1)
		a[11] = bc1 ^ (bc3 &^ bc2)
		a[2] = bc2 ^ (bc4 &^ bc3)
		a[18] = bc3 ^ (bc0 &^ bc4)
		a[9] = bc4 ^ (bc1 &^ bc0)

		t = a[15] ^ d0
		bc4 = t<<18 | t>>(64-18)
		t = a[6] ^ d1
		bc0 = t<<1 | t>>(64-1)
		t = a[22] ^ d2
		bc1 = t<<6 | t>>(64-6)
		t = a[13] ^ d3
		bc2 = t<<25 | t>>(64-25)
		t = a[4] ^ d4
		bc3 = t<<8 | t>>(64-8)
		a[15] = bc0 ^ (bc2 &^ bc1)
		a[6] = bc1 ^ (bc3 &^ bc2)
		a[22] = bc2 ^ (bc4 &^ bc3)
		a[13] = bc3 ^ (bc0 &^ bc4)
		a[4] = bc4 ^ (bc1 &^ bc0)

		t = a[10] ^ d0
		bc1 = t<<36 | t>>(64-36)
		t = a[1] ^ d1
		bc2 = t<<10 | t>>(64-10)
		t = a[17] ^ d2
		bc3 = t<<15 | t>>(64-15)
		t = a[8] ^ d3
		bc4 = t<<56 | t>>(64-56)
		t = a[24] ^ d4
		bc0 = t<<27 | t>>(64-27)
		a[10] = bc0 ^ (bc2 &^ bc1)
		a[1] = bc1 ^ (bc3 &^ bc2)
		a[17] = bc2 ^ (bc4 &^ bc3)
		a[8] = bc3 ^ (bc0 &^ bc4)
		a[24] = bc4 ^ (bc1 &^ bc0)

		t = a[5] ^ d0
		bc3 = t<<41 | t>>(64-41)
		t = a[21] ^ d1
		bc4 = t<<2 | t>>(64-2)
		t = a[12] ^ d2
		bc0 = t<<62 | t>>(64-62)
		t = a[3] ^ d3
		bc1 = t<<55 | t>>(64-55)
		t = a[19] ^ d4
		bc2 = t<<39 | t>>(64-39)
		a[5] = bc0 ^ (bc2 &^ bc1)
		a[21] = bc1 ^ (bc3 &^ bc2)
		a[12] = bc2 ^ (bc4 &^ bc3)
		a[3] = bc3 ^ (bc0 &^ bc4)
		a[19] = bc4 ^ (bc1 &^ bc0)

		// Round 3
		bc0 = a[0] ^ a[5] ^ a[10] ^ a[15] ^ a[20]
		bc1 = a[1] ^ a[6] ^ a[11] ^ a[16] ^ a[21]
		bc2 = a[2] ^ a[7] ^ a[12] ^ a[17] ^ a[22]
		bc3 = a[3] ^ a[8] ^ a[13] ^ a[18] ^ a[23]
		bc4 = a[4] ^ a[9] ^ a[14] ^ a[19] ^ a[24]
		d0 = bc4 ^ (bc1<<1 | bc1>>63)
		d1 = bc0 ^ (bc2<<1 | bc2>>63)
		d2 = bc1 ^ (bc3<<1 | bc3>>63)
		d3 = bc2 ^ (bc4<<1 | bc4>>63)
		d4 = bc3 ^ (bc0<<1 | bc0>>63)

		bc0 = a[0] ^ d0
		t = a[11] ^ d1
		bc1 = t<<44 | t>>(64-44)
		t = a[22] ^ d2
		bc2 = t<<43 | t>>(64-43)
		t = a[8] ^ d3
		bc3 = t<<21 | t>>(64-21)
		t = a[19] ^ d4
		bc4 = t<<14 | t>>(64-14)
		a[0] = bc0 ^ (bc2 &^ bc1) ^ rc[i+2]
		a[11] = bc1 ^ (bc3 &^ bc2)
		a[22] = bc2 ^ (bc4 &^ bc3)
		a[8] = bc3 ^ (bc0 &^ bc4)
		a[19] = bc4 ^ (bc1 &^ bc0)

		t = a[15] ^ d0
		bc2 = t<<3 | t>>(64-3)
		t = a[1] ^ d1
		bc3 = t<<45 | t>>(64-45)
		t = a[12] ^ d2
		bc4 = t<<61 | t>>(64-61)
		t = a[23] ^ d3
		bc0 = t<<28 | t>>(64-28)
		t = a[9] ^ d4
		bc1 = t<<20 | t>>(64-20)
		a[15] = bc0 ^ (bc2 &^ bc1)
		a[1] = bc1 ^ (bc3 &^ bc2)
		a[12] = bc2 ^ (bc4 &^ bc3)
		a[23] = bc3 ^ (bc0 &^ bc4)
		a[9] = bc4 ^ (bc1 &^ bc0)

		t = a[5] ^ d0
		bc4 = t<<18 | t>>(64-18)
		t = a[16] ^ d1
		bc0 = t<<1 | t>>(64-1)
		t = a[2] ^ d2
		bc1 = t<<6 | t>>(64-6)
		t = a[13] ^ d3
		bc2 = t<<25 | t>>(64-25)
		t = a[24] ^ d4
		bc3 = t<<8 | t>>(64-8)
		a[5] = bc0 ^ (bc2 &^ bc1)
		a[16] = bc1 ^ (bc3 &^ bc2)
		a[2] = bc2 ^ (bc4 &^ bc3)
		a[13] = bc3 ^ (bc0 &^ bc4)
		a[24] = bc4 ^ (bc1 &^ bc0)

		t = a[20] ^ d0
		bc1 = t<<36 | t>>(64-36)
		t = a[6] ^ d1
		bc2 = t<<10 | t>>(64-10)
		t = a[17] ^ d2
		bc3 = t<<15 | t>>(64-15)
		t = a[3] ^ d3
		bc4 = t<<56 | t>>(64-56)
		t = a[14] ^ d4
		bc0 = t<<27 | t>>(64-27)
		a[20] = bc0 ^ (bc2 &^ bc1)
		a[6] = bc1 ^ (bc3 &^ bc2)
		a[17] = bc2 ^ (bc4 &^ bc3)
		a[3] = bc3 ^ (bc0 &^ bc4)
		a[14] = bc4 ^ (bc1 &^ bc0)

		t = a[10] ^ d0
		bc3 = t<<41 | t>>(64-41)
		t = a[21] ^ d1
		bc4 = t<<2 | t>>(64-2)
		t = a[7] ^ d2
		bc0 = t<<62 | t>>(64-62)
		t = a[18] ^ d3
		bc1 = t<<55 | t>>(64-55)
		t = a[4] ^ d4
		bc2 = t<<39 | t>>(64-39)
		a[10] = bc0 ^ (bc2 &^ bc1)
		a[21] = bc1 ^ (bc3 &^ bc2)
		a[7] = bc2 ^ (bc4 &^ bc3)
		a[18] = bc3 ^ (bc0 &^ bc4)
		a[4] = bc4 ^ (bc1 &^ bc0)

		// Round 4
		bc0 = a[0] ^ a[5] ^ a[10] ^ a[15] ^ a[20]
		bc1 = a[1] ^ a[6] ^ a[11] ^ a[16] ^ a[21]
		bc2 = a[2] ^ a[7] ^ a[12] ^ a[17] ^ a[22]
		bc3 = a[3] ^ a[8] ^ a[13] ^ a[18] ^ a[23]
		bc4 = a[4] ^ a[9] ^ a[14] ^ a[19] ^ a[24]
		d0 = bc4 ^ (bc1<<1 | bc1>>63)
		d1 = bc0 ^ (bc2<<1 | bc2>>63)
		d2 = bc1 ^ (bc3<<1 | bc3>>63)
		d3 = bc2 ^ (bc4<<1 | bc4>>63)
		d4 = bc3 ^ (bc0<<1 | bc0>>63)

		bc0 = a[0] ^ d0
		t = a[1] ^ d1
		bc1 = t<<44 | t>>(64-44)
		t = a[2] ^ d2
		bc2 = t<<43 | t>>(64-43)
		t = a[3] ^ d3
		bc3 = t<<21 | t>>(64-21)
		t = a[4] ^ d4
		bc4 = t<<14 | t>>(64-14)
		a[0] = bc0 ^ (bc2 &^ bc1) ^ rc[i+3]
		a[1] = bc1 ^ (bc3 &^ bc2)
		a[2] = bc2 ^ (bc4 &^ bc3)
		a[3] = bc3 ^ (bc0 &^ bc4)
		a[4] = bc4 ^ (bc1 &^ bc0)

		t = a[5] ^ d0
		bc2 = t<<3 | t>>(64-3)
		t = a[6] ^ d1
		bc3 = t<<45 | t>>(64-45)
		t = a[7] ^ d2
		bc4 = t<<61 | t>>(64-61)
		t = a[8] ^ d3
		bc0 = t<<28 | t>>(64-28)
		t = a[9] ^ d4
		bc1 = t<<20 | t>>(64-20)
		a[5] = bc0 ^ (bc2 &^ bc1)
		a[6] = bc1 ^ (bc3 &^ bc2)
		a[7] = bc2 ^ (bc4 &^ bc3)
		a[8] = bc3 ^ (bc0 &^ bc4)
		a[9] = bc4 ^ (bc1 &^ bc0)

		t = a[10] ^ d0
		bc4 = t<<18 | t>>(64-18)
		t = a[11] ^ d1
		bc0 = t<<1 | t>>(64-1)
		t = a[12] ^ d2
		bc1 = t<<6 | t>>(64-6)
		t = a[13] ^ d3
		bc2 = t<<25 | t>>(64-25)
		t = a[14] ^ d4
		bc3 = t<<8 | t>>(64-8)
		a[10] = bc0 ^ (bc2 &^ bc1)
		a[11] = bc1 ^ (bc3 &^ bc2)
		a[12] = bc2 ^ (bc4 &^ bc3)
		a[13] = bc3 ^ (bc0 &^ bc4)
		a[14] = bc4 ^ (bc1 &^ bc0)

		t = a[15] ^ d0
		bc1 = t<<36 | t>>(64-36)
		t = a[16] ^ d1
		bc2 = t<<10 | t>>(64-10)
		t = a[17] ^ d2
		bc3 = t<<15 | t>>(64-15)
		t = a[18] ^ d3
		bc4 = t<<56 | t>>(64-56)
		t = a[19] ^ d4
		bc0 = t<<27 | t>>(64-27)
		a[15] = bc0 ^ (bc2 &^ bc1)
		a[16] = bc1 ^ (bc3 &^ bc2)
		a[17] = bc2 ^ (bc4 &^ bc3)
		a[18] = bc3 ^ (bc0 &^ bc4)
		a[19] = bc4 ^ (bc1 &^ bc0)

		t = a[20] ^ d0
		bc3 = t<<41 | t>>(64-41)
		t = a[21] ^ d1
		bc4 = t<<2 | t>>(64-2)
		t = a[22] ^ d2
		bc0 = t<<62 | t>>(64-62)
		t = a[23] ^ d3
		bc1 = t<<55 | t>>(64-55)
		t = a[24] ^ d4
		bc2 = t<<39 | t>>(64-39)
		a[20] = bc0 ^ (bc2 &^ bc1)
		a[21] = bc1 ^ (bc3 &^ bc2)
		a[22] = bc2 ^ (bc4 &^ bc3)
		a[23] = bc3 ^ (bc0 &^ bc4)
		a[24] = bc4 ^ (bc1 &^ bc0)
	}

	// Round 1
	bc0 = a[0] ^ a[5] ^ a[10] ^ a[15] ^ a[20]
	bc1 = a[1] ^ a[6] ^ a[11] ^ a[16] ^ a[21]
	bc2 = a[2] ^ a[7] ^ a[12] ^ a[17] ^ a[22]
	bc3 = a[3] ^ a[8] ^ a[13] ^ a[18] ^ a[23]
	bc4 = a[4] ^ a[9] ^ a[14] ^ a[19] ^ a[24]
	d0 = bc4 ^ (bc1<<1 | bc1>>63)
	d1 = bc0 ^ (bc2<<1 | bc2>>63)
	d2 = bc1 ^ (bc3<<1 | bc3>>63)
	d3 = bc2 ^ (bc4<<1 | bc4>>63)
	d4 = bc3 ^ (bc0<<1 | bc0>>63)

	bc0 = a[0] ^ d0
	t = a[6] ^ d1
	bc1 = t<<44 | t>>(64-44)
	t = a[12] ^ d2
	bc2 = t<<43 | t>>(64-43)
	t = a[18] ^ d3
	bc3 = t<<21 | t>>(64-21)
	t = a[24] ^ d4
	bc4 = t<<14 | t>>(64-14)
	a[0] = bc0 ^ (bc2 &^ bc1) ^ rc[i]
	a[6] = bc1 ^ (bc3 &^ bc2)
	a[12] = bc2 ^ (bc4 &^ bc3)
	a[18] = bc3 ^ (bc0 &^ bc4)
	a[24] = bc4 ^ (bc1 &^ bc0)

	t = a[10] ^ d0
	bc2 = t<<3 | t>>(64-3)
	t = a[16] ^ d1
	bc3 = t<<45 | t>>(64-45)
	t = a[22] ^ d2
	bc4 = t<<61 | t>>(64-61)
	t = a[3] ^ d3
	bc0 = t<<28 | t>>(64-28)
	t = a[9] ^ d4
	bc1 = t<<20 | t>>(64-20)
	a[10] = bc0 ^ (bc2 &^ bc1)
	a[16] = bc1 ^ (bc3 &^ bc2)
	a[22] = bc2 ^ (bc4 &^ bc3)
	a[3] = bc3 ^ (bc0 &^ bc4)
	a[9] = bc4 ^ (bc1 &^ bc0)

	t = a[20] ^ d0
	bc4 = t<<18 | t>>(64-18)
	t = a[1] ^ d1
	bc0 = t<<1 | t>>(64-1)
	t = a[7] ^ d2
	bc1 = t<<6 | t>>(64-6)
	t = a[13] ^ d3
	bc2 = t<<25 | t>>(64-25)
	t = a[19] ^ d4
	bc3 = t<<8 | t>>(64-8)
	a[20] = bc0 ^ (bc2 &^ bc1)
	a[1] = bc1 ^ (bc3 &^ bc2)
	a[7] = bc2 ^ (bc4 &^ bc3)
	a[13] = bc3 ^ (bc0 &^ bc4)
	a[19] = bc4 ^ (bc1 &^ bc0)

	t = a[5] ^ d0
	bc1 = t<<36 | t>>(64-36)
	t = a[11] ^ d1
	bc2 = t<<10 | t>>(64-10)
	t = a[17] ^ d2
	bc3 = t<<15 | t>>(64-15)
	t = a[23] ^ d3
	bc4 = t<<56 | t>>(64-56)
	t = a[4] ^ d4
	bc0 = t<<27 | t>>(64-27)
	a[5] = bc0 ^ (bc2 &^ bc1)
	a[11] = bc1 ^ (bc3 &^ bc2)
	a[17] = bc2 ^ (bc4 &^ bc3)
	a[23] = bc3 ^ (bc0 &^ bc4)
	a[4] = bc4 ^ (bc1 &^ bc0)

	t = a[15] ^ d0
	bc3 = t<<41 | t>>(64-41)
	t = a[21] ^ d1
	bc4 = t<<2 | t>>(64-2)
	t = a[2] ^ d2
	bc0 = t<<62 | t>>(64-62)
	t = a[8] ^ d3
	bc1 = t<<55 | t>>(64-55)
	t = a[14] ^ d4
	bc2 = t<<39 | t>>(64-39)
	a[15] = bc0 ^ (bc2 &^ bc1)
	a[21] = bc1 ^ (bc3 &^ bc2)
	a[2] = bc2 ^ (bc4 &^ bc3)
	a[8] = bc3 ^ (bc0 &^ bc4)
	a[14] = bc4 ^ (bc1 &^ bc0)

	// Round 2
	bc0 = a[0] ^ a[5] ^ a[10] ^ a[15] ^ a[20]
	bc1 = a[1] ^ a[6] ^ a[11] ^ a[16] ^ a[21]
	bc2 = a[2] ^ a[7] ^ a[12] ^ a[17] ^ a[22]
	bc3 = a[3] ^ a[8] ^ a[13] ^ a[18] ^ a[23]
	bc4 = a[4] ^ a[9] ^ a[14] ^ a[19] ^ a[24]
	d0 = bc4 ^ (bc1<<1 | bc1>>63)
	d1 = bc0 ^ (bc2<<1 | bc2>>63)
	d2 = bc1 ^ (bc3<<1 | bc3>>63)
	d3 = bc2 ^ (bc4<<1 | bc4>>63)
	d4 = bc3 ^ (bc0<<1 | bc0>>63)

	bc0 = a[0] ^ d0
	t = a[16] ^ d1
	bc1 = t<<44 | t>>(64-44)
	t = a[7] ^ d2
	bc2 = t<<43 | t>>(64-43)
	t = a[23] ^ d3
	bc3 = t<<21 | t>>(64-21)
	t = a[14] ^ d4
	bc4 = t<<14 | t>>(64-14)
	a[0] = bc0 ^ (bc2 &^ bc1) ^ rc[i+1]
	a[16] = bc1 ^ (bc3 &^ bc2)
	a[7] = bc2 ^ (bc4 &^ bc3)
	a[23] = bc3 ^ (bc0 &^ bc4)
	a[14] = bc4 ^ (bc1 &^ bc0)

	t = a[20] ^ d0
	bc2 = t<<3 | t>>(64-3)
	t = a[11] ^ d1
	bc3 = t<<45 | t>>(64-45)
	t = a[2] ^ d2
	bc4 = t<<61 | t>>(64-61)
	t = a[18] ^ d3
	bc0 = t<<28 | t>>(64-28)
	t = a[9] ^ d4
	bc1 = t<<20 | t>>(64-20)
	a[20] = bc0 ^ (bc2 &^ bc1)
	a[11] = bc1 ^ (bc3 &^ bc2)
	a[2] = bc2 ^ (bc4 &^ bc3)
	a[18] = bc3 ^ (bc0 &^ bc4)
	a[9] = bc4 ^ (bc1 &^ bc0)

	t = a[15] ^ d0
	bc4 = t<<18 | t>>(64-18)
	t = a[6] ^ d1
	bc0 = t<<1 | t>>(64-1)
	t = a[22] ^ d2
	bc1 = t<<6 | t>>(64-6)
	t = a[13] ^ d3
	bc2 = t<<25 | t>>(64-25)
	t = a[4] ^ d4
	bc3 = t<<8 | t>>(64-8)
	a[15] = bc0 ^ (bc2 &^ bc1)
	a[6] = bc1 ^ (bc3 &^ bc2)
	a[22] = bc2 ^ (bc4 &^ bc3)
	a[13] = bc3 ^ (bc0 &^ bc4)
	a[4] = bc4 ^ (bc1 &^ bc0)

	t = a[10] ^ d0
	bc1 = t<<36 | t>>(64-36)
	t = a[1] ^ d1
	bc2 = t<<10 | t>>(64-10)
	t = a[17] ^ d2
	bc3 = t<<15 | t>>(64-15)
	t = a[8] ^ d3
	bc4 = t<<56 | t>>(64-56)
	t = a[24] ^ d4
	bc0 = t<<27 | t>>(64-27)
	a[10] = bc0 ^ (bc2 &^ bc1)
	a[1] = bc1 ^ (bc3 &^ bc2)
	a[17] = bc2 ^ (bc4 &^ bc3)
	a[8] = bc3 ^ (bc0 &^ bc4)
	a[24] = bc4 ^ (bc1 &^ bc0)

	t = a[5] ^ d0
	bc3 = t<<41 | t>>(64-41)
	t = a[21] ^ d1
	bc4 = t<<2 | t>>(64-2)
	t = a[12] ^ d2
	bc0 = t<<62 | t>>(64-62)
	t = a[3] ^ d3
	bc1 = t<<55 | t>>(64-55)
	t = a[19] ^ d4
	bc2 = t<<39 | t>>(64-39)
	a[5] = bc0 ^ (bc2 &^ bc1)
	a[21] = bc1 ^ (bc3 &^ bc2)
	a[12] = bc2 ^ (bc4 &^ bc3)
	a[3] = bc3 ^ (bc0 &^ bc4)
	a[19] = bc4 ^ (bc1 &^ bc0)

	// Round 3
	bc0 = a[0] ^ a[5] ^ a[10] ^ a[15] ^ a[20]
	bc1 = a[1] ^ a[6] ^ a[11] ^ a[16] ^ a[21]
	bc2 = a[2] ^ a[7] ^ a[12] ^ a[17] ^ a[22]
	bc3 = a[3] ^ a[8] ^ a[13] ^ a[18] ^ a[23]
	bc4 = a[4] ^ a[9] ^ a[14] ^ a[19] ^ a[24]
	d0 = bc4 ^ (bc1<<1 | bc1>>63)
	d1 = bc0 ^ (bc2<<1 | bc2>>63)
	d2 = bc1 ^ (bc3<<1 | bc3>>63)
	d3 = bc2 ^ (bc4<<1 | bc4>>63)
	d4 = bc3 ^ (bc0<<1 | bc0>>63)

	bc0 = a[0] ^ d0
	t = a[11] ^ d1
	bc1 = t<<44 | t>>(64-44)
	t = a[22] ^ d2
	bc2 = t<<43 | t>>(64-43)
	t = a[8] ^ d3
	bc3 = t<<21 | t>>(64-21)
	t = a[19] ^ d4
	bc4 = t<<14 | t>>(64-14)
	a[0] = bc0 ^ (bc2 &^ bc1) ^ rc[i+2]
	a[11] = bc1 ^ (bc3 &^ bc2)
	a[22] = bc2 ^ (bc4 &^ bc3)
	a[8] = bc3 ^ (bc0 &^ bc4)
	a[19] = bc4 ^ (bc1 &^ bc0)

	t = a[15] ^ d0
	bc2 = t<<3 | t>>(64-3)
	t = a[1] ^ d1
	bc3 = t<<45 | t>>(64-45)
	t = a[12] ^ d2
	bc4 = t<<61 | t>>(64-61)
	t = a[23] ^ d3
	bc0 = t<<28 | t>>(64-28)
	t = a[9] ^ d4
	bc1 = t<<20 | t>>(64-20)
	a[15] = bc0 ^ (bc2 &^ bc1)
	a[1] = bc1 ^ (bc3 &^ bc2)
	a[12] = bc2 ^ (bc4 &^ bc3)
	a[23] = bc3 ^ (bc0 &^ bc4)
	a[9] = bc4 ^ (bc1 &^ bc0)

	t = a[5] ^ d0
	bc4 = t<<18 | t>>(64-18)
	t = a[16] ^ d1
	bc0 = t<<1 | t>>(64-1)
	t = a[2] ^ d2
	bc1 = t<<6 | t>>(64-6)
	t = a[13] ^ d3
	bc2 = t<<25 | t>>(64-25)
	t = a[24] ^ d4
	bc3 = t<<8 | t>>(64-8)
	a[5] = bc0 ^ (bc2 &^ bc1)
	a[16] = bc1 ^ (bc3 &^ bc2)
	a[2] = bc2 ^ (bc4 &^ bc3)
	a[13] = bc3 ^ (bc0 &^ bc4)
	a[24] = bc4 ^ (bc1 &^ bc0)

	t = a[20] ^ d0
	bc1 = t<<36 | t>>(64-36)
	t = a[6] ^ d1
	bc2 = t<<10 | t>>(64-10)
	t = a[17] ^ d2
	bc3 = t<<15 | t>>(64-15)
	t = a[3] ^ d3
	bc4 = t<<56 | t>>(64-56)
	t = a[14] ^ d4
	bc0 = t<<27 | t>>(64-27)
	a[20] = bc0 ^ (bc2 &^ bc1)
	a[6] = bc1 ^ (bc3 &^ bc2)
	a[17] = bc2 ^ (bc4 &^ bc3)
	a[3] = bc3 ^ (bc0 &^ bc4)
	a[14] = bc4 ^ (bc1 &^ bc0)

	t = a[10] ^ d0
	bc3 = t<<41 | t>>(64-41)
	t = a[21] ^ d1
	bc4 = t<<2 | t>>(64-2)
	t = a[7] ^ d2
	bc0 = t<<62 | t>>(64-62)
	t = a[18] ^ d3
	bc1 = t<<55 | t>>(64-55)
	t = a[4] ^ d4
	bc2 = t<<39 | t>>(64-39)
	a[10] = bc0 ^ (bc2 &^ bc1)
	a[21] = bc1 ^ (bc3 &^ bc2)
	a[7] = bc2 ^ (bc4 &^ bc3)
	a[18] = bc3 ^ (bc0 &^ bc4)
	a[4] = bc4 ^ (bc1 &^ bc0)

	// Round 4
	bc0 = a[0] ^ a[5] ^ a[10] ^ a[15] ^ a[20]
	bc1 = a[1] ^ a[6] ^ a[11] ^ a[16] ^ a[21]
	bc2 = a[2] ^ a[7] ^ a[12] ^ a[17] ^ a[22]
	bc3 = a[3] ^ a[8] ^ a[13] ^ a[18] ^ a[23]
	bc4 = a[4] ^ a[9] ^ a[14] ^ a[19] ^ a[24]
	d0 = bc4 ^ (bc1<<1 | bc1>>63)
	d1 = bc0 ^ (bc2<<1 | bc2>>63)
	d2 = bc1 ^ (bc3<<1 | bc3>>63)
	d3 = bc2 ^ (bc4<<1 | bc4>>63)
	d4 = bc3 ^ (bc0<<1 | bc0>>63)

	bc0 = a[0] ^ d0
	t = a[1] ^ d1
	bc1 = t<<44 | t>>(64-44)
	t = a[2] ^ d2
	bc2 = t<<43 | t>>(64-43)
	t = a[3] ^ d3
	bc3 = t<<21 | t>>(64-21)
	t = a[4] ^ d4
	bc4 = t<<14 | t>>(64-14)
	a[0] = bc0 ^ (bc2 &^ bc1) ^ rc[i+3]
	a[1] = bc1 ^ (bc3 &^ bc2)
	a[2] = bc2 ^ (bc4 &^ bc3)
	a[3] = bc3 ^ (bc0 &^ bc4)
	a[4] = bc4 ^ (bc1 &^ bc0)
}
