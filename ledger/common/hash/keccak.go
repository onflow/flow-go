// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build amd64 && !purego && gc
// +build amd64,!purego,gc

package hash

import (
	chk "github.com/onflow/flow-go/crypto/hash"
)

func keccakF1600(a *[25]uint64) {
	chk.AVXKeccakF1600(a)
}
