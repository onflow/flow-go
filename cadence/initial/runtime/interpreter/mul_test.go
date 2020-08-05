/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package interpreter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMulUInt8(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  UInt8Value
		valid bool
	}{
		{0, 0, true},
		{1, 0, true},
		{2, 0, true},
		{4, 0, true},
		{0x10, 0, true},
		{0x20, 0, true},
		{0x7f, 0, true},
		{0x80, 0, true},
		{0xc0, 0, true},
		{0xe0, 0, true},
		{0xff, 0, true},

		{0, 1, true},
		{1, 1, true},
		{2, 1, true},
		{4, 1, true},
		{0x10, 1, true},
		{0x20, 1, true},
		{0x7f, 1, true},
		{0x80, 1, true},
		{0xc0, 1, true},
		{0xe0, 1, true},
		{0xff, 1, true},

		{0, 2, true},
		{1, 2, true},
		{2, 2, true},
		{4, 2, true},
		{0x10, 2, true},
		{0x20, 2, true},
		{0x7f, 2, true},
		{0x80, 2, false},
		{0xc0, 2, false},
		{0xe0, 2, false},
		{0xff, 2, false},

		{0, 4, true},
		{1, 4, true},
		{2, 4, true},
		{4, 4, true},
		{0x10, 4, true},
		{0x20, 4, true},
		{0x3f, 4, true},
		{0x40, 4, false},
		{0x7f, 4, false},
		{0x80, 4, false},
		{0xc0, 4, false},
		{0xe0, 4, false},
		{0xff, 4, false},

		{0, 0x10, true},
		{1, 0x10, true},
		{2, 0x10, true},
		{4, 0x10, true},
		{0x0f, 0x10, true},
		{0x10, 0x10, false},
		{0x20, 0x10, false},
		{0x7f, 0x10, false},
		{0x80, 0x10, false},
		{0xc0, 0x10, false},
		{0xe0, 0x10, false},
		{0xff, 0x10, false},

		{0, 0x20, true},
		{1, 0x20, true},
		{2, 0x20, true},
		{4, 0x20, true},
		{7, 0x20, true},
		{0x10, 0x20, false},
		{0x20, 0x20, false},
		{0x7f, 0x20, false},
		{0x80, 0x20, false},
		{0xc0, 0x20, false},
		{0xe0, 0x20, false},
		{0xff, 0x20, false},

		{0, 0x7f, true},
		{1, 0x7f, true},
		{2, 0x7f, true},
		{4, 0x7f, false},
		{0x10, 0x7f, false},
		{0x20, 0x7f, false},
		{0x7f, 0x7f, false},
		{0x80, 0x7f, false},
		{0xc0, 0x7f, false},
		{0xe0, 0x7f, false},
		{0xff, 0x7f, false},

		{0, 0x40, true},
		{1, 0x40, true},
		{2, 0x40, true},
		{3, 0x40, true},
		{4, 0x40, false},
		{0x10, 0x40, false},
		{0x20, 0x40, false},
		{0x7f, 0x40, false},
		{0x80, 0x40, false},
		{0xc0, 0x40, false},
		{0xe0, 0x40, false},
		{0xff, 0x40, false},

		{0, 0x80, true},
		{1, 0x80, true},
		{2, 0x80, false},
		{4, 0x80, false},
		{0x10, 0x80, false},
		{0x20, 0x80, false},
		{0x7f, 0x80, false},
		{0x80, 0x80, false},
		{0xc0, 0x80, false},
		{0xe0, 0x80, false},
		{0xff, 0x80, false},

		{0, 0xff, true},
		{1, 0xff, true},
		{2, 0xff, false},
		{4, 0xff, false},
		{0x10, 0xff, false},
		{0x20, 0xff, false},
		{0x7f, 0xff, false},
		{0x80, 0xff, false},
		{0xc0, 0xff, false},
		{0xe0, 0xff, false},
		{0xff, 0xff, false},
	}

	for _, test := range tests {
		f := func() {
			test.a.Mul(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestMulUInt16(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  UInt16Value
		valid bool
	}{
		{0, 0, true},
		{1, 0, true},
		{2, 0, true},
		{4, 0, true},
		{0x1000, 0, true},
		{0x2000, 0, true},
		{0x7fff, 0, true},
		{0x8000, 0, true},
		{0xc000, 0, true},
		{0xe000, 0, true},
		{0xffff, 0, true},

		{0, 1, true},
		{1, 1, true},
		{2, 1, true},
		{4, 1, true},
		{0x1000, 1, true},
		{0x2000, 1, true},
		{0x7fff, 1, true},
		{0x8000, 1, true},
		{0xc000, 1, true},
		{0xe000, 1, true},
		{0xffff, 1, true},

		{0, 2, true},
		{1, 2, true},
		{2, 2, true},
		{4, 2, true},
		{0x1000, 2, true},
		{0x2000, 2, true},
		{0x7fff, 2, true},
		{0x8000, 2, false},
		{0xc000, 2, false},
		{0xe000, 2, false},
		{0xffff, 2, false},

		{0, 4, true},
		{1, 4, true},
		{2, 4, true},
		{4, 4, true},
		{0x1000, 4, true},
		{0x2000, 4, true},
		{0x3fff, 4, true},
		{0x4000, 4, false},
		{0x7fff, 4, false},
		{0x8000, 4, false},
		{0xc000, 4, false},
		{0xe000, 4, false},
		{0xffff, 4, false},

		{0, 0x1000, true},
		{1, 0x1000, true},
		{2, 0x1000, true},
		{4, 0x1000, true},
		{0x0fff, 0x1000, false},
		{0x1000, 0x1000, false},
		{0x2000, 0x1000, false},
		{0x7fff, 0x1000, false},
		{0x8000, 0x1000, false},
		{0xc000, 0x1000, false},
		{0xe000, 0x1000, false},
		{0xffff, 0x1000, false},

		{0, 0x2000, true},
		{1, 0x2000, true},
		{2, 0x2000, true},
		{4, 0x2000, true},
		{7, 0x2000, true},
		{0x1000, 0x2000, false},
		{0x2000, 0x2000, false},
		{0x7fff, 0x2000, false},
		{0x8000, 0x2000, false},
		{0xc000, 0x2000, false},
		{0xe000, 0x2000, false},
		{0xffff, 0x2000, false},

		{0, 0x7fff, true},
		{1, 0x7fff, true},
		{2, 0x7fff, true},
		{4, 0x7fff, false},
		{0x1000, 0x7fff, false},
		{0x2000, 0x7fff, false},
		{0x7fff, 0x7fff, false},
		{0x8000, 0x7fff, false},
		{0xc000, 0x7fff, false},
		{0xe000, 0x7fff, false},
		{0xffff, 0x7fff, false},

		{0, 0x4000, true},
		{1, 0x4000, true},
		{2, 0x4000, true},
		{3, 0x4000, true},
		{4, 0x4000, false},
		{0x1000, 0x4000, false},
		{0x2000, 0x4000, false},
		{0x7fff, 0x4000, false},
		{0x8000, 0x4000, false},
		{0xc000, 0x4000, false},
		{0xe000, 0x4000, false},
		{0xffff, 0x4000, false},

		{0, 0x8000, true},
		{1, 0x8000, true},
		{2, 0x8000, false},
		{4, 0x8000, false},
		{0x1000, 0x8000, false},
		{0x2000, 0x8000, false},
		{0x7fff, 0x8000, false},
		{0x8000, 0x8000, false},
		{0xc000, 0x8000, false},
		{0xe000, 0x8000, false},
		{0xffff, 0x8000, false},

		{0, 0xffff, true},
		{1, 0xffff, true},
		{2, 0xffff, false},
		{4, 0xffff, false},
		{0x1000, 0xffff, false},
		{0x2000, 0xffff, false},
		{0x7fff, 0xffff, false},
		{0x8000, 0xffff, false},
		{0xc000, 0xffff, false},
		{0xe000, 0xffff, false},
		{0xffff, 0xffff, false},
	}

	for _, test := range tests {
		f := func() {
			test.a.Mul(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestMulUInt32(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  UInt32Value
		valid bool
	}{
		{0, 0, true},
		{1, 0, true},
		{2, 0, true},
		{4, 0, true},
		{0x10000000, 0, true},
		{0x20000000, 0, true},
		{0x7fffffff, 0, true},
		{0x80000000, 0, true},
		{0xc0000000, 0, true},
		{0xe0000000, 0, true},
		{0xffffffff, 0, true},

		{0, 1, true},
		{1, 1, true},
		{2, 1, true},
		{4, 1, true},
		{0x10000000, 1, true},
		{0x20000000, 1, true},
		{0x7fffffff, 1, true},
		{0x80000000, 1, true},
		{0xc0000000, 1, true},
		{0xe0000000, 1, true},
		{0xffffffff, 1, true},

		{0, 2, true},
		{1, 2, true},
		{2, 2, true},
		{4, 2, true},
		{0x10000000, 2, true},
		{0x20000000, 2, true},
		{0x7fffffff, 2, true},
		{0x80000000, 2, false},
		{0xc0000000, 2, false},
		{0xe0000000, 2, false},
		{0xffffffff, 2, false},

		{0, 4, true},
		{1, 4, true},
		{2, 4, true},
		{4, 4, true},
		{0x10000000, 4, true},
		{0x20000000, 4, true},
		{0x3fffffff, 4, true},
		{0x40000000, 4, false},
		{0x7fffffff, 4, false},
		{0x80000000, 4, false},
		{0xc0000000, 4, false},
		{0xe0000000, 4, false},
		{0xffffffff, 4, false},

		{0, 0x10000000, true},
		{1, 0x10000000, true},
		{2, 0x10000000, true},
		{4, 0x10000000, true},
		{0x0fffffff, 0x10000000, false},
		{0x10000000, 0x10000000, false},
		{0x20000000, 0x10000000, false},
		{0x7fffffff, 0x10000000, false},
		{0x80000000, 0x10000000, false},
		{0xc0000000, 0x10000000, false},
		{0xe0000000, 0x10000000, false},
		{0xffffffff, 0x10000000, false},

		{0, 0x20000000, true},
		{1, 0x20000000, true},
		{2, 0x20000000, true},
		{4, 0x20000000, true},
		{7, 0x20000000, true},
		{0x10000000, 0x20000000, false},
		{0x20000000, 0x20000000, false},
		{0x7fffffff, 0x20000000, false},
		{0x80000000, 0x20000000, false},
		{0xc0000000, 0x20000000, false},
		{0xe0000000, 0x20000000, false},
		{0xffffffff, 0x20000000, false},

		{0, 0x7fffffff, true},
		{1, 0x7fffffff, true},
		{2, 0x7fffffff, true},
		{4, 0x7fffffff, false},
		{0x10000000, 0x7fffffff, false},
		{0x20000000, 0x7fffffff, false},
		{0x7fffffff, 0x7fffffff, false},
		{0x80000000, 0x7fffffff, false},
		{0xc0000000, 0x7fffffff, false},
		{0xe0000000, 0x7fffffff, false},
		{0xffffffff, 0x7fffffff, false},

		{0, 0x40000000, true},
		{1, 0x40000000, true},
		{2, 0x40000000, true},
		{3, 0x40000000, true},
		{4, 0x40000000, false},
		{0x10000000, 0x40000000, false},
		{0x20000000, 0x40000000, false},
		{0x7fffffff, 0x40000000, false},
		{0x80000000, 0x40000000, false},
		{0xc0000000, 0x40000000, false},
		{0xe0000000, 0x40000000, false},
		{0xffffffff, 0x40000000, false},

		{0, 0x80000000, true},
		{1, 0x80000000, true},
		{2, 0x80000000, false},
		{4, 0x80000000, false},
		{0x10000000, 0x80000000, false},
		{0x20000000, 0x80000000, false},
		{0x7fffffff, 0x80000000, false},
		{0x80000000, 0x80000000, false},
		{0xc0000000, 0x80000000, false},
		{0xe0000000, 0x80000000, false},
		{0xffffffff, 0x80000000, false},

		{0, 0xffffffff, true},
		{1, 0xffffffff, true},
		{2, 0xffffffff, false},
		{4, 0xffffffff, false},
		{0x10000000, 0xffffffff, false},
		{0x20000000, 0xffffffff, false},
		{0x7fffffff, 0xffffffff, false},
		{0x80000000, 0xffffffff, false},
		{0xc0000000, 0xffffffff, false},
		{0xe0000000, 0xffffffff, false},
		{0xffffffff, 0xffffffff, false},
	}

	for _, test := range tests {
		f := func() {
			test.a.Mul(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}

	}
}

func TestMulUInt64(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  UInt64Value
		valid bool
	}{
		{0, 0, true},
		{1, 0, true},
		{2, 0, true},
		{0x7fffffff, 0, true},
		{0x80000000, 0, true},
		{0xffffffff, 0, true},
		{0x100000000, 0, true},
		{0x200000000, 0, true},
		{0x7fffffffffffffff, 0, true},
		{0x8000000000000000, 0, true},
		{0xffffffffffffffff, 0, true},

		{0, 1, true},
		{1, 1, true},
		{2, 1, true},
		{0x7fffffff, 1, true},
		{0x80000000, 1, true},
		{0xffffffff, 1, true},
		{0x100000000, 1, true},
		{0x200000000, 1, true},
		{0x7fffffffffffffff, 1, true},
		{0x8000000000000000, 1, true},
		{0xffffffffffffffff, 1, true},

		{0, 2, true},
		{1, 2, true},
		{2, 2, true},
		{0x7fffffff, 2, true},
		{0x80000000, 2, true},
		{0xffffffff, 2, true},
		{0x100000000, 2, true},
		{0x200000000, 2, true},
		{0x7fffffffffffffff, 2, true},
		{0x8000000000000000, 2, false},
		{0xffffffffffffffff, 2, false},

		{0, 0x7fffffff, true},
		{1, 0x7fffffff, true},
		{2, 0x7fffffff, true},
		{0x7fffffff, 0x7fffffff, true},
		{0x80000000, 0x7fffffff, true},
		{0xffffffff, 0x7fffffff, true},
		{0x100000000, 0x7fffffff, true},
		{0x200000000, 0x7fffffff, true},
		{0x7fffffffffffffff, 0x7fffffff, false},
		{0x8000000000000000, 0x7fffffff, false},
		{0xffffffffffffffff, 0x7fffffff, false},

		{0, 0x80000000, true},
		{1, 0x80000000, true},
		{2, 0x80000000, true},
		{0x7fffffff, 0x80000000, true},
		{0x80000000, 0x80000000, true},
		{0xffffffff, 0x80000000, true},
		{0x100000000, 0x80000000, true},
		{0x200000000, 0x80000000, false},
		{0x7fffffffffffffff, 0x80000000, false},
		{0x8000000000000000, 0x80000000, false},
		{0xffffffffffffffff, 0x80000000, false},

		{0, 0xffffffff, true},
		{1, 0xffffffff, true},
		{2, 0xffffffff, true},
		{0x7fffffff, 0xffffffff, true},
		{0x80000000, 0xffffffff, true},
		{0xffffffff, 0xffffffff, true},
		{0x100000000, 0xffffffff, true},
		{0x200000000, 0xffffffff, false},
		{0x7fffffffffffffff, 0xffffffff, false},
		{0x8000000000000000, 0xffffffff, false},
		{0xffffffffffffffff, 0xffffffff, false},

		{0, 0x100000000, true},
		{1, 0x100000000, true},
		{2, 0x100000000, true},
		{0x7fffffff, 0x100000000, true},
		{0x80000000, 0x100000000, true},
		{0xffffffff, 0x100000000, true},
		{0x100000000, 0x100000000, false},
		{0x200000000, 0x100000000, false},
		{0x7fffffffffffffff, 0x100000000, false},
		{0x8000000000000000, 0x100000000, false},
		{0xffffffffffffffff, 0x100000000, false},

		{0, 0x200000000, true},
		{1, 0x200000000, true},
		{2, 0x200000000, true},
		{0x7fffffff, 0x200000000, true},
		{0x80000000, 0x200000000, false},
		{0xffffffff, 0x200000000, false},
		{0x100000000, 0x200000000, false},
		{0x200000000, 0x200000000, false},
		{0x7fffffffffffffff, 0x200000000, false},
		{0x8000000000000000, 0x200000000, false},
		{0xffffffffffffffff, 0x200000000, false},

		{0, 0x7fffffffffffffff, true},
		{1, 0x7fffffffffffffff, true},
		{2, 0x7fffffffffffffff, true},
		{0x7fffffff, 0x7fffffffffffffff, false},
		{0x80000000, 0x7fffffffffffffff, false},
		{0xffffffff, 0x7fffffffffffffff, false},
		{0x100000000, 0x7fffffffffffffff, false},
		{0x200000000, 0x7fffffffffffffff, false},
		{0x7fffffffffffffff, 0x7fffffffffffffff, false},
		{0x8000000000000000, 0x7fffffffffffffff, false},
		{0xffffffffffffffff, 0x7fffffffffffffff, false},

		{0, 0x8000000000000000, true},
		{1, 0x8000000000000000, true},
		{2, 0x8000000000000000, false},
		{0x7fffffff, 0x8000000000000000, false},
		{0x80000000, 0x8000000000000000, false},
		{0xffffffff, 0x8000000000000000, false},
		{0x100000000, 0x8000000000000000, false},
		{0x200000000, 0x8000000000000000, false},
		{0x7fffffffffffffff, 0x8000000000000000, false},
		{0x8000000000000000, 0x8000000000000000, false},
		{0xffffffffffffffff, 0x8000000000000000, false},

		{0, 0xffffffffffffffff, true},
		{1, 0xffffffffffffffff, true},
		{2, 0xffffffffffffffff, false},
		{0x7fffffff, 0xffffffffffffffff, false},
		{0x80000000, 0xffffffffffffffff, false},
		{0xffffffff, 0xffffffffffffffff, false},
		{0x100000000, 0xffffffffffffffff, false},
		{0x200000000, 0xffffffffffffffff, false},
		{0x7fffffffffffffff, 0xffffffffffffffff, false},
		{0x8000000000000000, 0xffffffffffffffff, false},
		{0xffffffffffffffff, 0xffffffffffffffff, false},

		// Special case - force addition overflow case
		{0xffffffff, 0x100000002, false},
	}

	for _, test := range tests {
		f := func() {
			test.a.Mul(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}

	}
}

func TestMulUInt128(t *testing.T) {

	t.Parallel()

	// NOTE: hex values are integer values, not bit patterns!

	tests := []struct {
		a, b  UInt128Value
		valid bool
	}{
		{uint128("0x0"), uint128("0x0"), true},
		{uint128("0x1"), uint128("0x0"), true},
		{uint128("0x2"), uint128("0x0"), true},
		{uint128("0x7fffffffffffffff"), uint128("0x0"), true},
		{uint128("0x8000000000000000"), uint128("0x0"), true},
		{uint128("0xffffffffffffffff"), uint128("0x0"), true},
		{uint128("0x10000000000000000"), uint128("0x0"), true},
		{uint128("0x20000000000000000"), uint128("0x0"), true},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0x0"), true},
		{uint128("0x80000000000000000000000000000000"), uint128("0x0"), true},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0x0"), true},

		{uint128("0x0"), uint128("0x1"), true},
		{uint128("0x1"), uint128("0x1"), true},
		{uint128("0x2"), uint128("0x1"), true},
		{uint128("0x7fffffffffffffff"), uint128("0x1"), true},
		{uint128("0x8000000000000000"), uint128("0x1"), true},
		{uint128("0xffffffffffffffff"), uint128("0x1"), true},
		{uint128("0x10000000000000000"), uint128("0x1"), true},
		{uint128("0x20000000000000000"), uint128("0x1"), true},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0x1"), true},
		{uint128("0x80000000000000000000000000000000"), uint128("0x1"), true},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0x1"), true},

		{uint128("0x0"), uint128("0x2"), true},
		{uint128("0x1"), uint128("0x2"), true},
		{uint128("0x2"), uint128("0x2"), true},
		{uint128("0x7fffffffffffffff"), uint128("0x2"), true},
		{uint128("0x8000000000000000"), uint128("0x2"), true},
		{uint128("0xffffffffffffffff"), uint128("0x2"), true},
		{uint128("0x10000000000000000"), uint128("0x2"), true},
		{uint128("0x20000000000000000"), uint128("0x2"), true},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0x2"), true},
		{uint128("0x80000000000000000000000000000000"), uint128("0x2"), false},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0x2"), false},

		{uint128("0x0"), uint128("0x7fffffffffffffff"), true},
		{uint128("0x1"), uint128("0x7fffffffffffffff"), true},
		{uint128("0x2"), uint128("0x7fffffffffffffff"), true},
		{uint128("0x7fffffffffffffff"), uint128("0x7fffffffffffffff"), true},
		{uint128("0x8000000000000000"), uint128("0x7fffffffffffffff"), true},
		{uint128("0xffffffffffffffff"), uint128("0x7fffffffffffffff"), true},
		{uint128("0x10000000000000000"), uint128("0x7fffffffffffffff"), true},
		{uint128("0x20000000000000000"), uint128("0x7fffffffffffffff"), true},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0x7fffffffffffffff"), false},
		{uint128("0x80000000000000000000000000000000"), uint128("0x7fffffffffffffff"), false},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0x7fffffffffffffff"), false},

		{uint128("0x0"), uint128("0x8000000000000000"), true},
		{uint128("0x1"), uint128("0x8000000000000000"), true},
		{uint128("0x2"), uint128("0x8000000000000000"), true},
		{uint128("0x7fffffffffffffff"), uint128("0x8000000000000000"), true},
		{uint128("0x8000000000000000"), uint128("0x8000000000000000"), true},
		{uint128("0xffffffffffffffff"), uint128("0x8000000000000000"), true},
		{uint128("0x10000000000000000"), uint128("0x8000000000000000"), true},
		{uint128("0x20000000000000000"), uint128("0x8000000000000000"), false},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0x8000000000000000"), false},
		{uint128("0x80000000000000000000000000000000"), uint128("0x8000000000000000"), false},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0x8000000000000000"), false},

		{uint128("0x0"), uint128("0xffffffffffffffff"), true},
		{uint128("0x1"), uint128("0xffffffffffffffff"), true},
		{uint128("0x2"), uint128("0xffffffffffffffff"), true},
		{uint128("0x7fffffffffffffff"), uint128("0xffffffffffffffff"), true},
		{uint128("0x8000000000000000"), uint128("0xffffffffffffffff"), true},
		{uint128("0xffffffffffffffff"), uint128("0xffffffffffffffff"), true},
		{uint128("0x10000000000000000"), uint128("0xffffffffffffffff"), true},
		{uint128("0x20000000000000000"), uint128("0xffffffffffffffff"), false},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0xffffffffffffffff"), false},
		{uint128("0x80000000000000000000000000000000"), uint128("0xffffffffffffffff"), false},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0xffffffffffffffff"), false},

		{uint128("0x0"), uint128("0x10000000000000000"), true},
		{uint128("0x1"), uint128("0x10000000000000000"), true},
		{uint128("0x2"), uint128("0x10000000000000000"), true},
		{uint128("0x7fffffffffffffff"), uint128("0x10000000000000000"), true},
		{uint128("0x8000000000000000"), uint128("0x10000000000000000"), true},
		{uint128("0xffffffffffffffff"), uint128("0x10000000000000000"), true},
		{uint128("0x10000000000000000"), uint128("0x10000000000000000"), false},
		{uint128("0x20000000000000000"), uint128("0x10000000000000000"), false},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0x10000000000000000"), false},
		{uint128("0x80000000000000000000000000000000"), uint128("0x10000000000000000"), false},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0x10000000000000000"), false},

		{uint128("0x0"), uint128("0x20000000000000000"), true},
		{uint128("0x1"), uint128("0x20000000000000000"), true},
		{uint128("0x2"), uint128("0x20000000000000000"), true},
		{uint128("0x7fffffffffffffff"), uint128("0x20000000000000000"), true},
		{uint128("0x8000000000000000"), uint128("0x20000000000000000"), false},
		{uint128("0xffffffffffffffff"), uint128("0x20000000000000000"), false},
		{uint128("0x10000000000000000"), uint128("0x20000000000000000"), false},
		{uint128("0x20000000000000000"), uint128("0x20000000000000000"), false},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0x20000000000000000"), false},
		{uint128("0x80000000000000000000000000000000"), uint128("0x20000000000000000"), false},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0x20000000000000000"), false},

		{uint128("0x0"), uint128("0x7fffffffffffffffffffffffffffffff"), true},
		{uint128("0x1"), uint128("0x7fffffffffffffffffffffffffffffff"), true},
		{uint128("0x2"), uint128("0x7fffffffffffffffffffffffffffffff"), true},
		{uint128("0x7fffffffffffffff"), uint128("0x7fffffffffffffffffffffffffffffff"), false},
		{uint128("0x8000000000000000"), uint128("0x7fffffffffffffffffffffffffffffff"), false},
		{uint128("0xffffffffffffffff"), uint128("0x7fffffffffffffffffffffffffffffff"), false},
		{uint128("0x10000000000000000"), uint128("0x7fffffffffffffffffffffffffffffff"), false},
		{uint128("0x20000000000000000"), uint128("0x7fffffffffffffffffffffffffffffff"), false},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0x7fffffffffffffffffffffffffffffff"), false},
		{uint128("0x80000000000000000000000000000000"), uint128("0x7fffffffffffffffffffffffffffffff"), false},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0x7fffffffffffffffffffffffffffffff"), false},

		{uint128("0x0"), uint128("0x80000000000000000000000000000000"), true},
		{uint128("0x1"), uint128("0x80000000000000000000000000000000"), true},
		{uint128("0x2"), uint128("0x80000000000000000000000000000000"), false},
		{uint128("0x7fffffffffffffff"), uint128("0x80000000000000000000000000000000"), false},
		{uint128("0x8000000000000000"), uint128("0x80000000000000000000000000000000"), false},
		{uint128("0xffffffffffffffff"), uint128("0x80000000000000000000000000000000"), false},
		{uint128("0x10000000000000000"), uint128("0x80000000000000000000000000000000"), false},
		{uint128("0x20000000000000000"), uint128("0x80000000000000000000000000000000"), false},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0x80000000000000000000000000000000"), false},
		{uint128("0x80000000000000000000000000000000"), uint128("0x80000000000000000000000000000000"), false},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0x80000000000000000000000000000000"), false},

		{uint128("0x0"), uint128("0xffffffffffffffffffffffffffffffff"), true},
		{uint128("0x1"), uint128("0xffffffffffffffffffffffffffffffff"), true},
		{uint128("0x2"), uint128("0xffffffffffffffffffffffffffffffff"), false},
		{uint128("0x7fffffffffffffff"), uint128("0xffffffffffffffffffffffffffffffff"), false},
		{uint128("0x8000000000000000"), uint128("0xffffffffffffffffffffffffffffffff"), false},
		{uint128("0xffffffffffffffff"), uint128("0xffffffffffffffffffffffffffffffff"), false},
		{uint128("0x10000000000000000"), uint128("0xffffffffffffffffffffffffffffffff"), false},
		{uint128("0x20000000000000000"), uint128("0xffffffffffffffffffffffffffffffff"), false},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0xffffffffffffffffffffffffffffffff"), false},
		{uint128("0x80000000000000000000000000000000"), uint128("0xffffffffffffffffffffffffffffffff"), false},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0xffffffffffffffffffffffffffffffff"), false},

		// Special case - force addition overflow case
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0x10000000000000000000000000000002"), false},
	}

	for _, test := range tests {
		f := func() {
			test.a.Mul(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestMulUInt256(t *testing.T) {

	t.Parallel()

	// NOTE: hex values are integer values, not bit patterns!

	tests := []struct {
		a, b  UInt256Value
		valid bool
	}{
		// 0x0

		{
			uint256("0x0"),
			uint256("0x0"),
			true,
		},
		{
			uint256("0x1"),
			uint256("0x0"),
			true,
		},
		{
			uint256("0x2"),
			uint256("0x0"),
			true,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffff"),
			uint256("0x0"),
			true,
		},
		{
			uint256("0x80000000000000000000000000000000"),
			uint256("0x0"),
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffff"),
			uint256("0x0"),
			true,
		},
		{
			uint256("0x100000000000000000000000000000000"),
			uint256("0x0"),
			true,
		},
		{
			uint256("0x2000000000000000000000000000000000"),
			uint256("0x0"),
			true,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x0"),
			true,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0x0"),
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x0"),
			true,
		},

		// 0x1

		{
			uint256("0x0"),
			uint256("0x1"),
			true,
		},
		{
			uint256("0x1"),
			uint256("0x1"),
			true,
		},
		{
			uint256("0x2"),
			uint256("0x1"),
			true,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffff"),
			uint256("0x1"),
			true,
		},
		{
			uint256("0x80000000000000000000000000000000"),
			uint256("0x1"),
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffff"),
			uint256("0x1"),
			true,
		},
		{
			uint256("0x100000000000000000000000000000000"),
			uint256("0x1"),
			true,
		},
		{
			uint256("0x200000000000000000000000000000000"),
			uint256("0x1"),
			true,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x1"),
			true,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0x1"),
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x1"),
			true,
		},

		// 0x2

		{
			uint256("0x0"),
			uint256("0x2"),
			true,
		},
		{
			uint256("0x1"),
			uint256("0x2"),
			true,
		},
		{
			uint256("0x2"),
			uint256("0x2"),
			true,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffff"),
			uint256("0x2"),
			true,
		},
		{
			uint256("0x80000000000000000000000000000000"),
			uint256("0x2"),
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffff"),
			uint256("0x2"),
			true,
		},
		{
			uint256("0x100000000000000000000000000000000"),
			uint256("0x2"),
			true,
		},
		{
			uint256("0x200000000000000000000000000000000"),
			uint256("0x2"),
			true,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x2"),
			true,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0x2"),
			false,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x2"),
			false,
		},

		// 0x7fffffffffffffffffffffffffffffff

		{
			uint256("0x0"),
			uint256("0x7fffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x1"),
			uint256("0x7fffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x2"),
			uint256("0x7fffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffff"),
			uint256("0x7fffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x80000000000000000000000000000000"),
			uint256("0x7fffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffff"),
			uint256("0x7fffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x100000000000000000000000000000000"),
			uint256("0x7fffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x200000000000000000000000000000000"),
			uint256("0x7fffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x7fffffffffffffffffffffffffffffff"),
			false,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0x7fffffffffffffffffffffffffffffff"),
			false,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x7fffffffffffffffffffffffffffffff"),
			false,
		},

		// 0x80000000000000000000000000000000

		{
			uint256("0x0"),
			uint256("0x80000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x1"),
			uint256("0x80000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x2"),
			uint256("0x80000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffff"),
			uint256("0x80000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x80000000000000000000000000000000"),
			uint256("0x80000000000000000000000000000000"),
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffff"),
			uint256("0x80000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x100000000000000000000000000000000"),
			uint256("0x80000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x200000000000000000000000000000000"),
			uint256("0x80000000000000000000000000000000"),
			false,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x80000000000000000000000000000000"),
			false,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0x80000000000000000000000000000000"),
			false,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x80000000000000000000000000000000"),
			false,
		},

		// 0xffffffffffffffffffffffffffffffff

		{
			uint256("0x0"),
			uint256("0xffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x1"),
			uint256("0xffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x2"),
			uint256("0xffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffff"),
			uint256("0xffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x80000000000000000000000000000000"),
			uint256("0xffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffff"),
			uint256("0xffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x100000000000000000000000000000000"),
			uint256("0xffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x200000000000000000000000000000000"),
			uint256("0xffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0xffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			uint256("0x8000000000000000000000000000000080000000000000000000000000000000"),
			uint256("0xffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0xffffffffffffffffffffffffffffffff"),
			false,
		},

		// 0x100000000000000000000000000000000

		{
			uint256("0x0"),
			uint256("0x100000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x1"),
			uint256("0x100000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x2"),
			uint256("0x100000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffff"),
			uint256("0x100000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x80000000000000000000000000000000"),
			uint256("0x100000000000000000000000000000000"),
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffff"),
			uint256("0x100000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x100000000000000000000000000000000"),
			uint256("0x100000000000000000000000000000000"),
			false,
		},
		{
			uint256("0x200000000000000000000000000000000"),
			uint256("0x100000000000000000000000000000000"),
			false,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x100000000000000000000000000000000"),
			false,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0x100000000000000000000000000000000"),
			false,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x100000000000000000000000000000000"),
			false,
		},

		// 0x200000000000000000000000000000000

		{
			uint256("0x0"),
			uint256("0x200000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x1"),
			uint256("0x200000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x2"),
			uint256("0x200000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffff"),
			uint256("0x200000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x80000000000000000000000000000000"),
			uint256("0x200000000000000000000000000000000"),
			false,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffff"),
			uint256("0x200000000000000000000000000000000"),
			false,
		},
		{
			uint256("0x100000000000000000000000000000000"),
			uint256("0x200000000000000000000000000000000"),
			false,
		},
		{
			uint256("0x200000000000000000000000000000000"),
			uint256("0x200000000000000000000000000000000"),
			false,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x200000000000000000000000000000000"),
			false,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0x200000000000000000000000000000000"),
			false,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x200000000000000000000000000000000"),
			false,
		},

		// 0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff

		{
			uint256("0x0"),
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x1"),
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x2"),
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x7ffffffffffffffffffffffffffffff"),
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			uint256("0x8000000000000000000000000000000"),
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffff"),
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			uint256("0x100000000000000000000000000000000"),
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			uint256("0x200000000000000000000000000000000"),
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			uint256("0x80000000000000000000000000000000"),
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},

		// 0x8000000000000000000000000000000000000000000000000000000000000000

		{
			uint256("0x0"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x1"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x2"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffff"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			uint256("0x80000000000000000000000000000000"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffff"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			uint256("0x100000000000000000000000000000000"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			uint256("0x200000000000000000000000000000000"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},

		// 0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff

		{
			uint256("0x0"),
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x1"),
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x2"),
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffff"),
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			uint256("0x80000000000000000000000000000000"),
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffff"),
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			uint256("0x100000000000000000000000000000000"),
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			uint256("0x200000000000000000000000000000000"),
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},

		// Special case - force addition overflow case
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x1000000000000000000000000000000000000000000000000000000000000002"),
			false,
		},
	}

	for _, test := range tests {
		f := func() {
			test.a.Mul(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestMulInt8(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  Int8Value
		valid bool
	}{
		{0, 0, true},
		{1, 0, true},
		{2, 0, true},
		{4, 0, true},
		{0x10, 0, true},
		{0x20, 0, true},
		{0x7f, 0, true},
		{-128, 0, true},
		{-64, 0, true},
		{-32, 0, true},
		{-2, 0, true},
		{-1, 0, true},

		{0, 1, true},
		{1, 1, true},
		{2, 1, true},
		{4, 1, true},
		{0x10, 1, true},
		{0x20, 1, true},
		{0x7f, 1, true},
		{-128, 1, true},
		{-64, 1, true},
		{-32, 1, true},
		{-2, 1, true},
		{-1, 1, true},

		{0, 2, true},
		{1, 2, true},
		{2, 2, true},
		{4, 2, true},
		{0x10, 2, true},
		{0x20, 2, true},
		{0x3f, 2, true},
		{0x40, 2, false},
		{0x7f, 2, false},
		{-128, 2, false},
		{-64, 2, true},
		{-32, 2, true},
		{-2, 2, true},
		{-1, 2, true},

		{0, 4, true},
		{1, 4, true},
		{2, 4, true},
		{4, 4, true},
		{0x10, 4, true},
		{0x1f, 4, true},
		{0x20, 4, false},
		{0x7f, 4, false},
		{-128, 4, false},
		{-64, 4, false},
		{-33, 4, false},
		{-32, 4, true},
		{-2, 4, true},
		{-1, 4, true},

		{0, 0x10, true},
		{1, 0x10, true},
		{2, 0x10, true},
		{4, 0x10, true},
		{7, 0x10, true},
		{8, 0x10, false},
		{0x10, 0x10, false},
		{0x20, 0x10, false},
		{0x7f, 0x10, false},
		{-128, 0x10, false},
		{-64, 0x10, false},
		{-32, 0x10, false},
		{-9, 0x10, false},
		{-8, 0x10, true},
		{-2, 0x10, true},
		{-1, 0x10, true},

		{0, 0x20, true},
		{1, 0x20, true},
		{2, 0x20, true},
		{3, 0x20, true},
		{4, 0x20, false},
		{0x10, 0x20, false},
		{0x20, 0x20, false},
		{0x7f, 0x20, false},
		{-128, 0x20, false},
		{-64, 0x20, false},
		{-32, 0x20, false},
		{-5, 0x20, false},
		{-4, 0x20, true},
		{-2, 0x20, true},
		{-1, 0x20, true},

		{0, 0x40, true},
		{1, 0x40, true},
		{2, 0x40, false},
		{3, 0x40, false},
		{4, 0x40, false},
		{0x10, 0x40, false},
		{0x20, 0x40, false},
		{0x7f, 0x40, false},
		{-128, 0x40, false},
		{-64, 0x40, false},
		{-32, 0x40, false},
		{-3, 0x40, false},
		{-2, 0x40, true},
		{-1, 0x40, true},

		{0, 0x7f, true},
		{1, 0x7f, true},
		{2, 0x7f, false},
		{4, 0x7f, false},
		{0x10, 0x7f, false},
		{0x20, 0x7f, false},
		{0x7f, 0x7f, false},
		{-128, 0x7f, false},
		{-64, 0x7f, false},
		{-32, 0x7f, false},
		{-2, 0x7f, false},
		{-1, 0x7f, true},

		{0, -128, true},
		{1, -128, true},
		{2, -128, false},
		{3, -128, false},
		{4, -128, false},
		{0x10, -128, false},
		{0x20, -128, false},
		{0x7f, -128, false},
		{-128, -128, false},
		{-64, -128, false},
		{-32, -128, false},
		{-2, -128, false},
		{-1, -128, false},

		{0, -2, true},
		{1, -2, true},
		{2, -2, true},
		{4, -2, true},
		{0x10, -2, true},
		{0x20, -2, true},
		{0x40, -2, true},
		{0x41, -2, false},
		{0x7f, -2, false},
		{-128, -2, false},
		{-64, -2, false},
		{-33, -2, true},
		{-32, -2, true},
		{-1, -2, true},

		{0, -1, true},
		{1, -1, true},
		{2, -1, true},
		{4, -1, true},
		{0x10, -1, true},
		{0x20, -1, true},
		{0x7f, -1, true},
		{-128, -1, false},
		{-64, -1, true},
		{-32, -1, true},
		{-1, -1, true},
	}

	for _, test := range tests {
		f := func() {
			test.a.Mul(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}

	}
}

func TestMulInt16(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  Int16Value
		valid bool
	}{
		{0, 0, true},
		{1, 0, true},
		{2, 0, true},
		{4, 0, true},
		{0x1000, 0, true},
		{0x2000, 0, true},
		{0x7fff, 0, true},
		{-32768, 0, true},
		{-8192, 0, true},
		{-128, 0, true},
		{-2, 0, true},
		{-1, 0, true},

		{0, 1, true},
		{1, 1, true},
		{2, 1, true},
		{4, 1, true},
		{0x1000, 1, true},
		{0x2000, 1, true},
		{0x7fff, 1, true},
		{-32768, 1, true},
		{-8192, 1, true},
		{-128, 1, true},
		{-2, 1, true},
		{-1, 1, true},

		{0, 2, true},
		{1, 2, true},
		{2, 2, true},
		{4, 2, true},
		{0x1000, 2, true},
		{0x2000, 2, true},
		{0x3fff, 2, true},
		{0x4000, 2, false},
		{0x7fff, 2, false},
		{-32768, 2, false},
		{-16385, 2, false},
		{-16384, 2, true},
		{-8192, 2, true},
		{-128, 2, true},
		{-2, 2, true},
		{-1, 2, true},

		{0, 4, true},
		{1, 4, true},
		{2, 4, true},
		{4, 4, true},
		{0x1000, 4, true},
		{0x1fff, 4, true},
		{0x2000, 4, false},
		{0x7fff, 4, false},
		{-32768, 4, false},
		{-8193, 4, false},
		{-8192, 4, true},
		{-128, 4, true},
		{-2, 4, true},
		{-1, 4, true},

		{0, 0x1000, true},
		{1, 0x1000, true},
		{2, 0x1000, true},
		{4, 0x1000, true},
		{7, 0x1000, true},
		{8, 0x1000, false},
		{0x1000, 0x1000, false},
		{0x2000, 0x1000, false},
		{0x7fff, 0x1000, false},
		{-32768, 0x1000, false},
		{-9, 0x1000, false},
		{-8, 0x1000, true},
		{-2, 0x1000, true},
		{-1, 0x1000, true},

		{0, 0x2000, true},
		{1, 0x2000, true},
		{2, 0x2000, true},
		{3, 0x2000, true},
		{4, 0x2000, false},
		{0x1000, 0x2000, false},
		{0x2000, 0x2000, false},
		{0x7fff, 0x2000, false},
		{-32768, 0x2000, false},
		{-32, 0x2000, false},
		{-5, 0x2000, false},
		{-4, 0x2000, true},
		{-2, 0x2000, true},
		{-1, 0x2000, true},

		{0, 0x4000, true},
		{1, 0x4000, true},
		{2, 0x4000, false},
		{3, 0x4000, false},
		{4, 0x4000, false},
		{0x1000, 0x4000, false},
		{0x2000, 0x4000, false},
		{0x7fff, 0x4000, false},
		{-32768, 0x4000, false},
		{-32, 0x4000, false},
		{-3, 0x4000, false},
		{-2, 0x4000, true},
		{-1, 0x4000, true},

		{0, 0x7fff, true},
		{1, 0x7fff, true},
		{2, 0x7fff, false},
		{4, 0x7fff, false},
		{0x10, 0x7fff, false},
		{0x20, 0x7fff, false},
		{0x7f, 0x7fff, false},
		{-32768, 0x7fff, false},
		{-2, 0x7fff, false},
		{-1, 0x7fff, true},

		{0, -32768, true},
		{1, -32768, true},
		{2, -32768, false},
		{3, -32768, false},
		{4, -32768, false},
		{0x1000, -32768, false},
		{0x2000, -32768, false},
		{0x7fff, -32768, false},
		{-32768, -32768, false},
		{-16384, -32768, false},
		{-2, -32768, false},
		{-1, -32768, false},

		{0, -16384, true},
		{1, -16384, true},
		{2, -16384, true},
		{3, -16384, false},
		{4, -16384, false},
		{0x1000, -16384, false},
		{0x2000, -16384, false},
		{0x7fff, -16384, false},
		{-32768, -16384, false},
		{-16384, -16384, false},
		{-2, -16384, false},
		{-1, -16384, true},

		{0, -2, true},
		{1, -2, true},
		{2, -2, true},
		{4, -2, true},
		{0x1000, -2, true},
		{0x2000, -2, true},
		{0x4000, -2, true},
		{0x4100, -2, false},
		{0x7fff, -2, false},
		{-32768, -2, false},
		{-16384, -2, false},
		{-16383, -2, true},
		{-2, -2, true},
		{-1, -2, true},

		{0, -1, true},
		{1, -1, true},
		{2, -1, true},
		{4, -1, true},
		{0x1000, -1, true},
		{0x2000, -1, true},
		{0x7fff, -1, true},
		{-32768, -1, false},
		{-16384, -1, true},
		{-2, -1, true},
		{-1, -1, true},
	}

	for _, test := range tests {
		f := func() {
			test.a.Mul(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestMulInt32(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  Int32Value
		valid bool
	}{
		{0, 0, true},
		{1, 0, true},
		{2, 0, true},
		{4, 0, true},
		{0x10000000, 0, true},
		{0x20000000, 0, true},
		{0x7fffffff, 0, true},
		{-2147483648, 0, true},
		{-1073741824, 0, true},
		{-2, 0, true},
		{-1, 0, true},

		{0, 1, true},
		{1, 1, true},
		{2, 1, true},
		{4, 1, true},
		{0x10000000, 1, true},
		{0x20000000, 1, true},
		{0x7fffffff, 1, true},
		{-2147483648, 1, true},
		{-1073741824, 1, true},
		{-2, 1, true},
		{-1, 1, true},

		{0, 2, true},
		{1, 2, true},
		{2, 2, true},
		{4, 2, true},
		{0x10000000, 2, true},
		{0x20000000, 2, true},
		{0x3fffffff, 2, true},
		{0x40000000, 2, false},
		{0x7fffffff, 2, false},
		{-2147483648, 2, false},
		{-1073741824, 2, true},
		{-2, 2, true},
		{-1, 2, true},

		{0, 4, true},
		{1, 4, true},
		{2, 4, true},
		{4, 4, true},
		{0x10000000, 4, true},
		{0x1fffffff, 4, true},
		{0x20000000, 4, false},
		{0x7fffffff, 4, false},
		{-2147483648, 4, false},
		{-1073741824, 4, false},
		{-536870913, 4, false},
		{-536870912, 4, true},
		{-2, 4, true},
		{-1, 4, true},

		{0, 0x10000000, true},
		{1, 0x10000000, true},
		{2, 0x10000000, true},
		{4, 0x10000000, true},
		{7, 0x10000000, true},
		{8, 0x10000000, false},
		{0x10000000, 0x10000000, false},
		{0x20000000, 0x10000000, false},
		{0x7fffffff, 0x10000000, false},
		{-2147483648, 0x10000000, false},
		{-1073741824, 0x10000000, false},
		{-9, 0x10000000, false},
		{-8, 0x10000000, true},
		{-2, 0x10000000, true},
		{-1, 0x10000000, true},

		{0, 0x20000000, true},
		{1, 0x20000000, true},
		{2, 0x20000000, true},
		{3, 0x20000000, true},
		{4, 0x20000000, false},
		{0x10000000, 0x20000000, false},
		{0x20000000, 0x20000000, false},
		{0x7fffffff, 0x20000000, false},
		{-2147483648, 0x20000000, false},
		{-1073741824, 0x20000000, false},
		{-5, 0x20000000, false},
		{-4, 0x20000000, true},
		{-2, 0x20000000, true},
		{-1, 0x20000000, true},

		{0, 0x40000000, true},
		{1, 0x40000000, true},
		{2, 0x40000000, false},
		{3, 0x40000000, false},
		{4, 0x40000000, false},
		{0x10000000, 0x40000000, false},
		{0x20000000, 0x40000000, false},
		{0x7fffffff, 0x40000000, false},
		{-2147483648, 0x40000000, false},
		{-1073741824, 0x40000000, false},
		{-3, 0x40000000, false},
		{-2, 0x40000000, true},
		{-1, 0x40000000, true},

		{0, 0x7fffffff, true},
		{1, 0x7fffffff, true},
		{2, 0x7fffffff, false},
		{4, 0x7fffffff, false},
		{0x10000000, 0x7fffffff, false},
		{0x20000000, 0x7fffffff, false},
		{0x7fffffff, 0x7fffffff, false},
		{-2147483648, 0x7fffffff, false},
		{-1073741824, 0x7fffffff, false},
		{-2, 0x7fffffff, false},
		{-1, 0x7fffffff, true},

		{0, -2147483648, true},
		{1, -2147483648, true},
		{2, -2147483648, false},
		{3, -2147483648, false},
		{4, -2147483648, false},
		{0x10000000, -2147483648, false},
		{0x20000000, -2147483648, false},
		{0x7fffffff, -2147483648, false},
		{-2147483648, -2147483648, false},
		{-1073741824, -2147483648, false},
		{-2, -2147483648, false},
		{-1, -2147483648, false},

		{0, -2, true},
		{1, -2, true},
		{2, -2, true},
		{4, -2, true},
		{0x10000000, -2, true},
		{0x20000000, -2, true},
		{0x40000000, -2, true},
		{0x41000000, -2, false},
		{0x7fffffff, -2, false},
		{-2147483648, -2, false},
		{-1073741824, -2, false},
		{-1073741823, -2, true},
		{-2, -2, true},
		{-1, -2, true},

		{0, -1, true},
		{1, -1, true},
		{2, -1, true},
		{4, -1, true},
		{0x10000000, -1, true},
		{0x20000000, -1, true},
		{0x7fffffff, -1, true},
		{-2147483648, -1, false},
		{-1073741824, -1, true},
		{-2, -1, true},
		{-1, -1, true},
	}

	for _, test := range tests {
		f := func() {
			test.a.Mul(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestMulInt64(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  Int64Value
		valid bool
	}{
		{0, 0, true},
		{1, 0, true},
		{2, 0, true},
		{0x7fffffff, 0, true},
		{0x80000000, 0, true},
		{0xffffffff, 0, true},
		{0x100000000, 0, true},
		{0x200000000, 0, true},
		{0x7fffffffffffffff, 0, true},
		{-9223372036854775808, 0, true},
		{-1, 0, true},

		{0, 1, true},
		{1, 1, true},
		{2, 1, true},
		{0x7fffffff, 1, true},
		{0x80000000, 1, true},
		{0xffffffff, 1, true},
		{0x100000000, 1, true},
		{0x200000000, 1, true},
		{0x7fffffffffffffff, 1, true},
		{-9223372036854775808, 1, true},
		{-1, 1, true},

		{0, 2, true},
		{1, 2, true},
		{2, 2, true},
		{0x7fffffff, 2, true},
		{0x80000000, 2, true},
		{0xffffffff, 2, true},
		{0x100000000, 2, true},
		{0x200000000, 2, true},
		{0x7fffffffffffffff, 2, false},
		{-9223372036854775808, 2, false},
		{-1, 2, true},

		{0, 0x7fffffff, true},
		{1, 0x7fffffff, true},
		{2, 0x7fffffff, true},
		{0x7fffffff, 0x7fffffff, true},
		{0x80000000, 0x7fffffff, true},
		{0xffffffff, 0x7fffffff, true},
		{0x100000000, 0x7fffffff, true},
		{0x200000000, 0x7fffffff, false},
		{0x7fffffffffffffff, 0x7fffffff, false},
		{-9223372036854775808, 0x7fffffff, false},
		{-1, 0x7fffffff, true},

		{0, 0x80000000, true},
		{1, 0x80000000, true},
		{2, 0x80000000, true},
		{0x7fffffff, 0x80000000, true},
		{0x80000000, 0x80000000, true},
		{0xffffffff, 0x80000000, true},
		{0x100000000, 0x80000000, false},
		{0x200000000, 0x80000000, false},
		{0x7fffffffffffffff, 0x80000000, false},
		{-9223372036854775808, 0x80000000, false},
		{-1, 0x80000000, true},

		{0, 0xffffffff, true},
		{1, 0xffffffff, true},
		{2, 0xffffffff, true},
		{0x7fffffff, 0xffffffff, true},
		{0x80000000, 0xffffffff, true},
		{0xffffffff, 0xffffffff, false},
		{0x100000000, 0xffffffff, false},
		{0x200000000, 0xffffffff, false},
		{0x7fffffffffffffff, 0xffffffff, false},
		{-9223372036854775808, 0xffffffff, false},
		{-1, 0xffffffff, true},

		{0, 0x100000000, true},
		{1, 0x100000000, true},
		{2, 0x100000000, true},
		{0x7fffffff, 0x100000000, true},
		{0x80000000, 0x100000000, false},
		{0xffffffff, 0x100000000, false},
		{0x100000000, 0x100000000, false},
		{0x200000000, 0x100000000, false},
		{0x7fffffffffffffff, 0x100000000, false},
		{-9223372036854775808, 0x100000000, false},
		{-1, 0x100000000, true},

		{0, 0x200000000, true},
		{1, 0x200000000, true},
		{2, 0x200000000, true},
		{0x7fffffff, 0x200000000, false},
		{0x80000000, 0x200000000, false},
		{0xffffffff, 0x200000000, false},
		{0x100000000, 0x200000000, false},
		{0x200000000, 0x200000000, false},
		{0x7fffffffffffffff, 0x200000000, false},
		{-9223372036854775808, 0x200000000, false},
		{-1, 0x200000000, true},

		{0, 0x7fffffffffffffff, true},
		{1, 0x7fffffffffffffff, true},
		{2, 0x7fffffffffffffff, false},
		{0x7fffffff, 0x7fffffffffffffff, false},
		{0x80000000, 0x7fffffffffffffff, false},
		{0xffffffff, 0x7fffffffffffffff, false},
		{0x100000000, 0x7fffffffffffffff, false},
		{0x200000000, 0x7fffffffffffffff, false},
		{0x7fffffffffffffff, 0x7fffffffffffffff, false},
		{-9223372036854775808, 0x7fffffffffffffff, false},
		{-1, 0x7fffffffffffffff, true},

		{0, -9223372036854775808, true},
		{1, -9223372036854775808, true},
		{2, -9223372036854775808, false},
		{0x7fffffff, -9223372036854775808, false},
		{0x80000000, -9223372036854775808, false},
		{0xffffffff, -9223372036854775808, false},
		{0x100000000, -9223372036854775808, false},
		{0x200000000, -9223372036854775808, false},
		{0x7fffffffffffffff, -9223372036854775808, false},
		{-9223372036854775808, -9223372036854775808, false},
		{-1, -9223372036854775808, false},

		{0, -1, true},
		{1, -1, true},
		{2, -1, true},
		{0x7fffffff, -1, true},
		{0x80000000, -1, true},
		{0xffffffff, -1, true},
		{0x100000000, -1, true},
		{0x200000000, -1, true},
		{0x7fffffffffffffff, -1, true},
		{-9223372036854775808, -1, false},
		{-1, -1, true},

		// Special case - force addition overflow case
		{0xffffffff, 0x100000002, false},
	}

	for _, test := range tests {
		f := func() {
			test.a.Mul(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}

	}
}

func TestMulInt128(t *testing.T) {

	t.Parallel()

	// NOTE: hex values are integer values, not bit patterns!

	tests := []struct {
		a, b  Int128Value
		valid bool
	}{
		{int128("0x00000000000000000000000000000000"), int128("0x00000000000000000000000000000000"), true},
		{int128("0x00000000000000000000000000000001"), int128("0x00000000000000000000000000000000"), true},
		{int128("0x00000000000000000000000000000002"), int128("0x00000000000000000000000000000000"), true},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("0x00000000000000000000000000000000"), true},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("0x00000000000000000000000000000000"), true},
		{int128("-0x80000000000000000000000000000000"), int128("0x00000000000000000000000000000000"), true},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("0x00000000000000000000000000000000"), true},
		{int128("-0x00000000000000000000000000000002"), int128("0x00000000000000000000000000000000"), true},
		{int128("-0x00000000000000000000000000000001"), int128("0x00000000000000000000000000000000"), true},

		{int128("0x00000000000000000000000000000000"), int128("0x00000000000000000000000000000001"), true},
		{int128("0x00000000000000000000000000000001"), int128("0x00000000000000000000000000000001"), true},
		{int128("0x00000000000000000000000000000002"), int128("0x00000000000000000000000000000001"), true},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("0x00000000000000000000000000000001"), true},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("0x00000000000000000000000000000001"), true},
		{int128("-0x80000000000000000000000000000000"), int128("0x00000000000000000000000000000001"), true},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("0x00000000000000000000000000000001"), true},
		{int128("-0x00000000000000000000000000000002"), int128("0x00000000000000000000000000000001"), true},
		{int128("-0x00000000000000000000000000000001"), int128("0x00000000000000000000000000000001"), true},

		{int128("0x00000000000000000000000000000000"), int128("0x00000000000000000000000000000002"), true},
		{int128("0x00000000000000000000000000000001"), int128("0x00000000000000000000000000000002"), true},
		{int128("0x00000000000000000000000000000002"), int128("0x00000000000000000000000000000002"), true},
		{int128("0x00000000000000000000000000000004"), int128("0x00000000000000000000000000000002"), true},
		{int128("0x3fffffffffffffffffffffffffffffff"), int128("0x00000000000000000000000000000002"), true},
		{int128("0x40000000000000000000000000000000"), int128("0x00000000000000000000000000000002"), false},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("0x00000000000000000000000000000002"), false},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("0x00000000000000000000000000000002"), false},
		{int128("-0x80000000000000000000000000000000"), int128("0x00000000000000000000000000000002"), false},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("0x00000000000000000000000000000002"), false},
		{int128("-0x40000000000000000000000000000000"), int128("0x00000000000000000000000000000002"), true},
		{int128("-0x3fffffffffffffffffffffffffffffff"), int128("0x00000000000000000000000000000002"), true},
		{int128("-0x00000000000000000000000000000004"), int128("0x00000000000000000000000000000002"), true},
		{int128("-0x00000000000000000000000000000002"), int128("0x00000000000000000000000000000002"), true},
		{int128("-0x00000000000000000000000000000001"), int128("0x00000000000000000000000000000002"), true},

		{int128("0x00000000000000000000000000000000"), int128("0x00000000000000000000000000000004"), true},
		{int128("0x00000000000000000000000000000001"), int128("0x00000000000000000000000000000004"), true},
		{int128("0x00000000000000000000000000000002"), int128("0x00000000000000000000000000000004"), true},
		{int128("0x00000000000000000000000000000004"), int128("0x00000000000000000000000000000004"), true},
		{int128("0x1fffffffffffffffffffffffffffffff"), int128("0x00000000000000000000000000000004"), true},
		{int128("0x20000000000000000000000000000000"), int128("0x00000000000000000000000000000004"), false},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("0x00000000000000000000000000000004"), false},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("0x00000000000000000000000000000004"), false},
		{int128("-0x80000000000000000000000000000000"), int128("0x00000000000000000000000000000004"), false},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("0x00000000000000000000000000000004"), false},
		{int128("-0x20000000000000000000000000000000"), int128("0x00000000000000000000000000000004"), true},
		{int128("-0x1fffffffffffffffffffffffffffffff"), int128("0x00000000000000000000000000000004"), true},
		{int128("-0x00000000000000000000000000000004"), int128("0x00000000000000000000000000000004"), true},
		{int128("-0x00000000000000000000000000000002"), int128("0x00000000000000000000000000000004"), true},
		{int128("-0x00000000000000000000000000000001"), int128("0x00000000000000000000000000000004"), true},

		{int128("0x00000000000000000000000000000000"), int128("0x10000000000000000000000000000000"), true},
		{int128("0x00000000000000000000000000000001"), int128("0x10000000000000000000000000000000"), true},
		{int128("0x00000000000000000000000000000002"), int128("0x10000000000000000000000000000000"), true},
		{int128("0x00000000000000000000000000000004"), int128("0x10000000000000000000000000000000"), true},
		{int128("0x00000000000000000000000000000007"), int128("0x10000000000000000000000000000000"), true},
		{int128("0x00000000000000000000000000000008"), int128("0x10000000000000000000000000000000"), false},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("0x10000000000000000000000000000000"), false},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("0x10000000000000000000000000000000"), false},
		{int128("-0x80000000000000000000000000000000"), int128("0x10000000000000000000000000000000"), false},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("0x10000000000000000000000000000000"), false},
		{int128("-0x20000000000000000000000000000000"), int128("0x10000000000000000000000000000000"), false},
		{int128("-0x1fffffffffffffffffffffffffffffff"), int128("0x10000000000000000000000000000000"), false},
		{int128("-0x00000000000000000000000000000008"), int128("0x10000000000000000000000000000000"), true},
		{int128("-0x00000000000000000000000000000007"), int128("0x10000000000000000000000000000000"), true},
		{int128("-0x00000000000000000000000000000004"), int128("0x10000000000000000000000000000000"), true},
		{int128("-0x00000000000000000000000000000002"), int128("0x10000000000000000000000000000000"), true},
		{int128("-0x00000000000000000000000000000001"), int128("0x10000000000000000000000000000000"), true},

		{int128("0x00000000000000000000000000000000"), int128("0x20000000000000000000000000000000"), true},
		{int128("0x00000000000000000000000000000001"), int128("0x20000000000000000000000000000000"), true},
		{int128("0x00000000000000000000000000000002"), int128("0x20000000000000000000000000000000"), true},
		{int128("0x00000000000000000000000000000004"), int128("0x20000000000000000000000000000000"), false},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("0x20000000000000000000000000000000"), false},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("0x20000000000000000000000000000000"), false},
		{int128("-0x80000000000000000000000000000000"), int128("0x20000000000000000000000000000000"), false},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("0x20000000000000000000000000000000"), false},
		{int128("-0x00000000000000000000000000000005"), int128("0x20000000000000000000000000000000"), false},
		{int128("-0x00000000000000000000000000000004"), int128("0x20000000000000000000000000000000"), true},
		{int128("-0x00000000000000000000000000000002"), int128("0x20000000000000000000000000000000"), true},
		{int128("-0x00000000000000000000000000000001"), int128("0x20000000000000000000000000000000"), true},

		{int128("0x00000000000000000000000000000000"), int128("0x40000000000000000000000000000000"), true},
		{int128("0x00000000000000000000000000000001"), int128("0x40000000000000000000000000000000"), true},
		{int128("0x00000000000000000000000000000002"), int128("0x40000000000000000000000000000000"), false},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("0x40000000000000000000000000000000"), false},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("0x40000000000000000000000000000000"), false},
		{int128("-0x80000000000000000000000000000000"), int128("0x40000000000000000000000000000000"), false},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("0x40000000000000000000000000000000"), false},
		{int128("-0x00000000000000000000000000000003"), int128("0x40000000000000000000000000000000"), false},
		{int128("-0x00000000000000000000000000000002"), int128("0x40000000000000000000000000000000"), true},
		{int128("-0x00000000000000000000000000000001"), int128("0x40000000000000000000000000000000"), true},

		{int128("0x00000000000000000000000000000000"), int128("0x7fffffffffffffffffffffffffffffff"), true},
		{int128("0x00000000000000000000000000000001"), int128("0x7fffffffffffffffffffffffffffffff"), true},
		{int128("0x00000000000000000000000000000002"), int128("0x7fffffffffffffffffffffffffffffff"), false},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("0x7fffffffffffffffffffffffffffffff"), false},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("0x7fffffffffffffffffffffffffffffff"), false},
		{int128("-0x80000000000000000000000000000000"), int128("0x7fffffffffffffffffffffffffffffff"), false},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("0x7fffffffffffffffffffffffffffffff"), false},
		{int128("-0x00000000000000000000000000000002"), int128("0x7fffffffffffffffffffffffffffffff"), false},
		{int128("-0x00000000000000000000000000000001"), int128("0x7fffffffffffffffffffffffffffffff"), true},

		{int128("0x00000000000000000000000000000000"), int128("-0x80000000000000000000000000000000"), true},
		{int128("0x00000000000000000000000000000001"), int128("-0x80000000000000000000000000000000"), true},
		{int128("0x00000000000000000000000000000002"), int128("-0x80000000000000000000000000000000"), false},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("-0x80000000000000000000000000000000"), false},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("-0x80000000000000000000000000000000"), false},
		{int128("-0x80000000000000000000000000000000"), int128("-0x80000000000000000000000000000000"), false},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("-0x80000000000000000000000000000000"), false},
		{int128("-0x00000000000000000000000000000002"), int128("-0x80000000000000000000000000000000"), false},
		{int128("-0x00000000000000000000000000000001"), int128("-0x80000000000000000000000000000000"), false},

		{int128("0x00000000000000000000000000000000"), int128("-0x00000000000000000000000000000002"), true},
		{int128("0x00000000000000000000000000000001"), int128("-0x00000000000000000000000000000002"), true},
		{int128("0x00000000000000000000000000000002"), int128("-0x00000000000000000000000000000002"), true},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("-0x00000000000000000000000000000002"), false},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("-0x00000000000000000000000000000002"), false},
		{int128("-0x80000000000000000000000000000000"), int128("-0x00000000000000000000000000000002"), false},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("-0x00000000000000000000000000000002"), false},
		{int128("-0x00000000000000000000000000000002"), int128("-0x00000000000000000000000000000002"), true},
		{int128("-0x00000000000000000000000000000001"), int128("-0x00000000000000000000000000000002"), true},

		{int128("0x00000000000000000000000000000000"), int128("-0x00000000000000000000000000000001"), true},
		{int128("0x00000000000000000000000000000001"), int128("-0x00000000000000000000000000000001"), true},
		{int128("0x00000000000000000000000000000002"), int128("-0x00000000000000000000000000000001"), true},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("-0x00000000000000000000000000000001"), true},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("-0x00000000000000000000000000000001"), true},
		{int128("-0x80000000000000000000000000000000"), int128("-0x00000000000000000000000000000001"), false},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("-0x00000000000000000000000000000001"), true},
		{int128("-0x00000000000000000000000000000002"), int128("-0x00000000000000000000000000000001"), true},
		{int128("-0x00000000000000000000000000000001"), int128("-0x00000000000000000000000000000001"), true},
	}

	for _, test := range tests {
		f := func() {
			test.a.Mul(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestMulInt256(t *testing.T) {

	t.Parallel()

	// NOTE: hex values are integer values, not bit patterns!

	tests := []struct {
		a, b  Int256Value
		valid bool
	}{
		// 0x0000000000000000000000000000000000000000000000000000000000000000

		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},

		// 0x0000000000000000000000000000000000000000000000000000000000000001

		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			true,
		},
		{
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			true,
		},
		{
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			true,
		},
		{
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			true,
		},
		{
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			true,
		},

		// 0x0000000000000000000000000000000000000000000000000000000000000002

		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000004"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			int256("0x3fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			int256("0x4000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			false,
		},
		{
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			false,
		},
		{
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			false,
		},
		{
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			false,
		},
		{
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			false,
		},
		{
			int256("-0x4000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			int256("-0x3fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000004"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},

		// 0x0000000000000000000000000000000000000000000000000000000000000004

		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000004"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000004"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000004"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000004"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000004"),
			true,
		},
		{
			int256("0x1fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000004"),
			true,
		},
		{
			int256("0x2000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000004"),
			false,
		},
		{
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000004"),
			false,
		},
		{
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000004"),
			false,
		},
		{
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000004"),
			false,
		},
		{
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000004"),
			false,
		},
		{
			int256("-0x2000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000004"),
			true,
		},
		{
			int256("-0x1fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000004"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000004"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000004"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000004"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000004"),
			true,
		},

		// 0x1000000000000000000000000000000000000000000000000000000000000000

		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x1000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("0x1000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("0x1000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000004"),
			int256("0x1000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000007"),
			int256("0x1000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000008"),
			int256("0x1000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			int256("0x1000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x1000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x1000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x1000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("-0x2000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x1000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("-0x1fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x1000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000008"),
			int256("0x1000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000007"),
			int256("0x1000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000004"),
			int256("0x1000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("0x1000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("0x1000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},

		// 0x2000000000000000000000000000000000000000000000000000000000000000

		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x2000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("0x2000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("0x2000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000004"),
			int256("0x2000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			int256("0x2000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x2000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x2000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x2000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000005"),
			int256("0x2000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000004"),
			int256("0x2000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("0x2000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("0x2000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},

		// 0x4000000000000000000000000000000000000000000000000000000000000000

		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x4000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("0x4000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("0x4000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			int256("0x4000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x4000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x4000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x4000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000003"),
			int256("0x4000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("0x4000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("0x4000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},

		// 0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff

		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},

		// -0x8000000000000000000000000000000000000000000000000000000000000000

		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},

		// -0x0000000000000000000000000000000000000000000000000000000000000002

		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			false,
		},
		{
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			false,
		},
		{
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			false,
		},
		{
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			false,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},

		// -0x0000000000000000000000000000000000000000000000000000000000000001

		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			int256("-0x0000000000000000000000000000000000000000000000000000000000000001"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("-0x0000000000000000000000000000000000000000000000000000000000000001"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("-0x0000000000000000000000000000000000000000000000000000000000000001"),
			true,
		},
		{
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			int256("-0x0000000000000000000000000000000000000000000000000000000000000001"),
			true,
		},
		{
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("-0x0000000000000000000000000000000000000000000000000000000000000001"),
			true,
		},
		{
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			int256("-0x0000000000000000000000000000000000000000000000000000000000000001"),
			false,
		},
		{
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("-0x0000000000000000000000000000000000000000000000000000000000000001"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("-0x0000000000000000000000000000000000000000000000000000000000000001"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("-0x0000000000000000000000000000000000000000000000000000000000000001"),
			true,
		},
	}

	for _, test := range tests {
		f := func() {
			test.a.Mul(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}
