// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build (!amd64 && !386 && !ppc64le) || purego
// +build !amd64,!386,!ppc64le purego

package hash

import "encoding/binary"

// A storageBuf is an aligned array of maxRate bytes.
type storageBuf [maxRate]byte

func (b *storageBuf) asBytes() *[maxRate]byte {
	return (*[maxRate]byte)(b)
}

var (
	xorIn   = xorInGeneric
	copyOut = copyOutGeneric
)

// xorInGeneric xors the bytes in buf into the state; it
// makes no non-portable assumptions about memory layout
// or alignment.
func xorInGeneric(d *state, buf []byte) {
	n := len(buf) / 8

	for i := 0; i < n; i++ {
		a := binary.LittleEndian.Uint64(buf)
		d.a[i] ^= a
		buf = buf[8:]
	}
}

// copyOutGeneric copies ulint64s to a byte buffer.
func copyOutGeneric(d *state, b []byte) {
	for i := 0; len(b) >= 8; i++ {
		binary.LittleEndian.PutUint64(b, d.a[i])
		b = b[8:]
	}
}
