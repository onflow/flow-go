// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build amd64 && !purego && gc
// +build amd64,!purego,gc

package keccakAVX512

// check if the amd64 machine supports AVX512 instructions at build time and call
// an assembly function using AVX512 if so.

/*
#cgo CFLAGS: -march=native

#include <stdio.h>
#if (defined __AVX512F__)
	void keccakF1600_AVX512(unsigned long* state);
#else
	void keccakF1600_AVX512(unsigned long* state) {
		printf("ERROR: the package is misconfigured\n");
	};
#endif
*/
import "C"

func KeccakF1600AMDAVX512(a *[25]uint64) {
	C.keccakF1600_AVX512((*C.ulong)(&a[0]))
}
