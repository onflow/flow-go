/// The functions below were copied and modified from golang.org/x/crypto/sha3.
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

//go:build (amd64 || 386 || ppc64le) && !purego
// +build amd64 386 ppc64le
// +build !purego

package hash

import "unsafe"

// A storageBuf is an aligned array of maxRate bytes.
type storageBuf [maxRate / 8]uint64

func (b *storageBuf) asBytes() *[maxRate]byte {
	return (*[maxRate]byte)(unsafe.Pointer(b))
}

// xorInUnaligned uses unaligned reads and writes to update d.a to contain d.a
// XOR buf.
func xorInUnaligned(d *sha3State, buf []byte) {
	n := len(buf)
	bw := (*[maxRate / 8]uint64)(unsafe.Pointer(&buf[0]))[: n/8 : n/8]

	d.a[0] ^= bw[0]
	d.a[1] ^= bw[1]
	d.a[2] ^= bw[2]
	d.a[3] ^= bw[3]
	d.a[4] ^= bw[4]
	d.a[5] ^= bw[5]
	d.a[6] ^= bw[6]
	d.a[7] ^= bw[7]
	d.a[8] ^= bw[8]
	d.a[9] ^= bw[9]
	d.a[10] ^= bw[10]
	d.a[11] ^= bw[11]
	d.a[12] ^= bw[12]
	if n >= 136 {
		d.a[13] ^= bw[13]
		d.a[14] ^= bw[14]
		d.a[15] ^= bw[15]
		d.a[16] ^= bw[16]
	}
}

func copyOutUnaligned(buf []byte, d *sha3State) {
	ab := (*[maxRate]uint8)(unsafe.Pointer(&d.a[0]))
	copy(buf, ab[:])
}

var (
	xorIn   = xorInUnaligned
	copyOut = copyOutUnaligned
)
