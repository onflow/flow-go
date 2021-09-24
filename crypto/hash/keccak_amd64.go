// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build amd64 && !purego && gc
// +build amd64,!purego,gc

package hash

import (
	"github.com/onflow/flow-go/crypto/hash/keccak"
	"golang.org/x/sys/cpu"
)

/*
// check if the amd64 machine supports AVX512 instructions at build time and call
// an assembly function using AVX512 if so.
#if (__AVX512CD__ |  __AVX512ER__ | __AVX512F__ | __AVX512PF__ == 1)
void KeccakF1600_AVX512(void *state);
#else
void KeccakF1600_AVX512(unsigned long* state) {};
#endif
*/
import "C"

func keccakF1600(a *[25]uint64) {
	if cpu.X86.HasAVX512 {
		C.KeccakF1600_AVX512((*C.ulong)(&a[0]))
	} else {
		keccak.KeccakF1600GenericAMD(a)
	}
}
