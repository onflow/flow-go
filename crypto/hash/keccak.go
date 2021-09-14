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
#include <stdlib.h>

void KeccakP1600_Permute_24rounds(void *state);

void callKeccakF1600(ulong* state) {
	KeccakP1600_Permute_24rounds(state);
}

*/
import "C"

func keccakF1600(a *[25]uint64) {
	if cpu.X86.HasAVX512 {
		C.callKeccakF1600((*C.ulong)(&a[0]))
	} else {
		keccak.KeccakF1600(a)
	}
}
