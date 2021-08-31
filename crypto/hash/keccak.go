// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build amd64 && !purego && gc
// +build amd64,!purego,gc

package hash

/*
#include <stdint.h>
#include <stdlib.h>
void KeccakP1600_Permute_Nrounds(void *state, unsigned int nrounds);
void KeccakP1600_Permute_24rounds(void *state);

void callKeccakF1600(ulong* state) {
	KeccakP1600_Permute_24rounds(state);
}

#cgo LDFLAGS: -L./lib -lXKCP -Wl,-rpath=./lib

*/
import "C"

import (
	_ "unsafe"
)

func keccakF1600(a *[25]uint64) {
	C.callKeccakF1600((*C.ulong)(&a[0]))
}
