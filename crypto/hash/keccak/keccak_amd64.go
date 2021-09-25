// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build amd64 && !purego && gc
// +build amd64,!purego,gc

package keccak

import (
	"github.com/onflow/flow-go/crypto/hash/internal/keccakAMD"
	"github.com/onflow/flow-go/crypto/hash/internal/keccakAVX512"
	"golang.org/x/sys/cpu"
)

func KeccakF1600(a *[25]uint64) {
	if cpu.X86.HasAVX512 {
		keccakAVX512.KeccakF1600AMDAVX512(a)
	} else {
		keccakAMD.KeccakF1600GenericAMD(a)
	}
}
