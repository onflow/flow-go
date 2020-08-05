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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/cadence-initial/runtime/sema"
)

func TestDivModUInt8(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  UInt8Value
		valid bool
	}{
		{0, 0, false},
		{1, 0, false},
		{2, 0, false},
		{0x7f, 0, false},
		{0x80, 0, false},
		{0xff, 0, false},

		{0, 1, true},
		{1, 1, true},
		{2, 1, true},
		{0x7f, 1, true},
		{0x80, 1, true},
		{0xff, 1, true},

		{0, 2, true},
		{1, 2, true},
		{2, 2, true},
		{0x7f, 2, true},
		{0x80, 2, true},
		{0xff, 2, true},

		{0, 0x7f, true},
		{1, 0x7f, true},
		{2, 0x7f, true},
		{0x7f, 0x7f, true},
		{0x80, 0x7f, true},
		{0xff, 0x7f, true},

		{0, 0x80, true},
		{1, 0x80, true},
		{2, 0x80, true},
		{0x7f, 0x80, true},
		{0x80, 0x80, true},
		{0xff, 0x80, true},

		{0, 0xff, true},
		{1, 0xff, true},
		{2, 0xff, true},
		{0x7f, 0xff, true},
		{0x80, 0xff, true},
		{0xff, 0xff, true},
	}

	for _, test := range tests {
		for _, f := range []func(a, b UInt8Value){
			func(a, b UInt8Value) {
				a.Div(b)
			},
			func(a, b UInt8Value) {
				a.Mod(b)
			},
		} {
			f := func() {
				f(test.a, test.b)
			}
			if test.valid {
				assert.NotPanics(t, f)
			} else {
				assert.Panics(t, f)
			}
		}
	}
}

func TestDivModUInt16(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  UInt16Value
		valid bool
	}{
		{0, 0, false},
		{1, 0, false},
		{2, 0, false},
		{0x7fff, 0, false},
		{0x8000, 0, false},
		{0xffff, 0, false},

		{0, 1, true},
		{1, 1, true},
		{2, 1, true},
		{0x7fff, 1, true},
		{0x8000, 1, true},
		{0xffff, 1, true},

		{0, 2, true},
		{1, 2, true},
		{2, 2, true},
		{0x7fff, 2, true},
		{0x8000, 2, true},
		{0xffff, 2, true},

		{0, 0x7fff, true},
		{1, 0x7fff, true},
		{2, 0x7fff, true},
		{0x7fff, 0x7fff, true},
		{0x8000, 0x7fff, true},
		{0xffff, 0x7fff, true},

		{0, 0x8000, true},
		{1, 0x8000, true},
		{2, 0x8000, true},
		{0x7fff, 0x8000, true},
		{0x8000, 0x8000, true},
		{0xffff, 0x8000, true},

		{0, 0xffff, true},
		{1, 0xffff, true},
		{2, 0xffff, true},
		{0x7fff, 0xffff, true},
		{0x8000, 0xffff, true},
		{0xfff, 0xffff, true},
	}

	for _, test := range tests {
		for _, f := range []func(a, b UInt16Value){
			func(a, b UInt16Value) {
				a.Div(b)
			},
			func(a, b UInt16Value) {
				a.Mod(b)
			},
		} {
			f := func() {
				f(test.a, test.b)
			}
			if test.valid {
				assert.NotPanics(t, f)
			} else {
				assert.Panics(t, f)
			}
		}
	}
}

func TestDivModUInt32(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  UInt32Value
		valid bool
	}{
		{0, 0, false},
		{1, 0, false},
		{2, 0, false},
		{0x7fffffff, 0, false},
		{0x80000000, 0, false},
		{0xffffffff, 0, false},

		{0, 1, true},
		{1, 1, true},
		{2, 1, true},
		{0x7fffffff, 1, true},
		{0x80000000, 1, true},
		{0xffffffff, 1, true},

		{0, 2, true},
		{1, 2, true},
		{2, 2, true},
		{0x7fffffff, 2, true},
		{0x80000000, 2, true},
		{0xffffffff, 2, true},

		{0, 0x7fffffff, true},
		{1, 0x7fffffff, true},
		{2, 0x7fffffff, true},
		{0x7fffffff, 0x7fffffff, true},
		{0x80000000, 0x7fffffff, true},
		{0xffffffff, 0x7fffffff, true},

		{0, 0x80000000, true},
		{1, 0x80000000, true},
		{2, 0x80000000, true},
		{0x7fffffff, 0x80000000, true},
		{0x80000000, 0x80000000, true},
		{0xffffffff, 0x80000000, true},

		{0, 0xffffffff, true},
		{1, 0xffffffff, true},
		{2, 0xffffffff, true},
		{0x7fffffff, 0xffffffff, true},
		{0x80000000, 0xffffffff, true},
		{0xffffffff, 0xffffffff, true},
	}

	for _, test := range tests {
		for _, f := range []func(a, b UInt32Value){
			func(a, b UInt32Value) {
				a.Div(b)
			},
			func(a, b UInt32Value) {
				a.Mod(b)
			},
		} {
			f := func() {
				f(test.a, test.b)
			}
			if test.valid {
				assert.NotPanics(t, f)
			} else {
				assert.Panics(t, f)
			}
		}
	}
}

func TestDivModUInt64(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  UInt64Value
		valid bool
	}{
		{0, 0, false},
		{1, 0, false},
		{2, 0, false},
		{0x7fffffff, 0, false},
		{0x80000000, 0, false},
		{0xffffffff, 0, false},
		{0x100000000, 0, false},
		{0x200000000, 0, false},
		{0x7fffffffffffffff, 0, false},
		{0x8000000000000000, 0, false},
		{0xffffffffffffffff, 0, false},

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
		{0x8000000000000000, 2, true},
		{0xffffffffffffffff, 2, true},

		{0, 0x7fffffff, true},
		{1, 0x7fffffff, true},
		{2, 0x7fffffff, true},
		{0x7fffffff, 0x7fffffff, true},
		{0x80000000, 0x7fffffff, true},
		{0xffffffff, 0x7fffffff, true},
		{0x100000000, 0x7fffffff, true},
		{0x200000000, 0x7fffffff, true},
		{0x7fffffffffffffff, 0x7fffffff, true},
		{0x8000000000000000, 0x7fffffff, true},
		{0xffffffffffffffff, 0x7fffffff, true},

		{0, 0x80000000, true},
		{1, 0x80000000, true},
		{2, 0x80000000, true},
		{0x7fffffff, 0x80000000, true},
		{0x80000000, 0x80000000, true},
		{0xffffffff, 0x80000000, true},
		{0x100000000, 0x80000000, true},
		{0x200000000, 0x80000000, true},
		{0x7fffffffffffffff, 0x80000000, true},
		{0x8000000000000000, 0x80000000, true},
		{0xffffffffffffffff, 0x80000000, true},

		{0, 0xffffffff, true},
		{1, 0xffffffff, true},
		{2, 0xffffffff, true},
		{0x7fffffff, 0xffffffff, true},
		{0x80000000, 0xffffffff, true},
		{0xffffffff, 0xffffffff, true},
		{0x100000000, 0xffffffff, true},
		{0x200000000, 0xffffffff, true},
		{0x7fffffffffffffff, 0xffffffff, true},
		{0x8000000000000000, 0xffffffff, true},
		{0xffffffffffffffff, 0xffffffff, true},

		{0, 0x100000000, true},
		{1, 0x100000000, true},
		{2, 0x100000000, true},
		{0x7fffffff, 0x100000000, true},
		{0x80000000, 0x100000000, true},
		{0xffffffff, 0x100000000, true},
		{0x100000000, 0x100000000, true},
		{0x200000000, 0x100000000, true},
		{0x7fffffffffffffff, 0x100000000, true},
		{0x8000000000000000, 0x100000000, true},
		{0xffffffffffffffff, 0x100000000, true},

		{0, 0x200000000, true},
		{1, 0x200000000, true},
		{2, 0x200000000, true},
		{0x7fffffff, 0x200000000, true},
		{0x80000000, 0x200000000, true},
		{0xffffffff, 0x200000000, true},
		{0x100000000, 0x200000000, true},
		{0x200000000, 0x200000000, true},
		{0x7fffffffffffffff, 0x200000000, true},
		{0x8000000000000000, 0x200000000, true},
		{0xffffffffffffffff, 0x200000000, true},

		{0, 0x7fffffffffffffff, true},
		{1, 0x7fffffffffffffff, true},
		{2, 0x7fffffffffffffff, true},
		{0x7fffffff, 0x7fffffffffffffff, true},
		{0x80000000, 0x7fffffffffffffff, true},
		{0xffffffff, 0x7fffffffffffffff, true},
		{0x100000000, 0x7fffffffffffffff, true},
		{0x200000000, 0x7fffffffffffffff, true},
		{0x7fffffffffffffff, 0x7fffffffffffffff, true},
		{0x8000000000000000, 0x7fffffffffffffff, true},
		{0xffffffffffffffff, 0x7fffffffffffffff, true},

		{0, 0x8000000000000000, true},
		{1, 0x8000000000000000, true},
		{2, 0x8000000000000000, true},
		{0x7fffffff, 0x8000000000000000, true},
		{0x80000000, 0x8000000000000000, true},
		{0xffffffff, 0x8000000000000000, true},
		{0x100000000, 0x8000000000000000, true},
		{0x200000000, 0x8000000000000000, true},
		{0x7fffffffffffffff, 0x8000000000000000, true},
		{0x8000000000000000, 0x8000000000000000, true},
		{0xffffffffffffffff, 0x8000000000000000, true},

		{0, 0xffffffffffffffff, true},
		{1, 0xffffffffffffffff, true},
		{2, 0xffffffffffffffff, true},
		{0x7fffffff, 0xffffffffffffffff, true},
		{0x80000000, 0xffffffffffffffff, true},
		{0xffffffff, 0xffffffffffffffff, true},
		{0x100000000, 0xffffffffffffffff, true},
		{0x200000000, 0xffffffffffffffff, true},
		{0x7fffffffffffffff, 0xffffffffffffffff, true},
		{0x8000000000000000, 0xffffffffffffffff, true},
		{0xffffffffffffffff, 0xffffffffffffffff, true},
	}

	for _, test := range tests {
		for _, f := range []func(a, b UInt64Value){
			func(a, b UInt64Value) {
				a.Div(b)
			},
			func(a, b UInt64Value) {
				a.Mod(b)
			},
		} {
			f := func() {
				f(test.a, test.b)
			}
			if test.valid {
				assert.NotPanics(t, f)
			} else {
				assert.Panics(t, f)
			}
		}
	}
}

func TestDivModUInt128(t *testing.T) {

	t.Parallel()

	// NOTE: hex values are integer values, not bit patterns!

	tests := []struct {
		a, b  UInt128Value
		valid bool
	}{
		{uint128("0x0"), uint128("0x0"), false},
		{uint128("0x1"), uint128("0x0"), false},
		{uint128("0x2"), uint128("0x0"), false},
		{uint128("0x7fffffffffffffff"), uint128("0x0"), false},
		{uint128("0x8000000000000000"), uint128("0x0"), false},
		{uint128("0xffffffffffffffff"), uint128("0x0"), false},
		{uint128("0x10000000000000000"), uint128("0x0"), false},
		{uint128("0x20000000000000000"), uint128("0x0"), false},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0x0"), false},
		{uint128("0x80000000000000000000000000000000"), uint128("0x0"), false},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0x0"), false},

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
		{uint128("0x80000000000000000000000000000000"), uint128("0x2"), true},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0x2"), true},

		{uint128("0x0"), uint128("0x7fffffffffffffff"), true},
		{uint128("0x1"), uint128("0x7fffffffffffffff"), true},
		{uint128("0x2"), uint128("0x7fffffffffffffff"), true},
		{uint128("0x7fffffffffffffff"), uint128("0x7fffffffffffffff"), true},
		{uint128("0x8000000000000000"), uint128("0x7fffffffffffffff"), true},
		{uint128("0xffffffffffffffff"), uint128("0x7fffffffffffffff"), true},
		{uint128("0x10000000000000000"), uint128("0x7fffffffffffffff"), true},
		{uint128("0x20000000000000000"), uint128("0x7fffffffffffffff"), true},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0x7fffffffffffffff"), true},
		{uint128("0x80000000000000000000000000000000"), uint128("0x7fffffffffffffff"), true},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0x7fffffffffffffff"), true},

		{uint128("0x0"), uint128("0x8000000000000000"), true},
		{uint128("0x1"), uint128("0x8000000000000000"), true},
		{uint128("0x2"), uint128("0x8000000000000000"), true},
		{uint128("0x7fffffff"), uint128("0x8000000000000000"), true},
		{uint128("0x8000000000000000"), uint128("0x8000000000000000"), true},
		{uint128("0xffffffff"), uint128("0x8000000000000000"), true},
		{uint128("0x10000000000000000"), uint128("0x8000000000000000"), true},
		{uint128("0x20000000000000000"), uint128("0x8000000000000000"), true},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0x8000000000000000"), true},
		{uint128("0x80000000000000000000000000000000"), uint128("0x8000000000000000"), true},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0x8000000000000000"), true},

		{uint128("0x0"), uint128("0xffffffffffffffff"), true},
		{uint128("0x1"), uint128("0xffffffffffffffff"), true},
		{uint128("0x2"), uint128("0xffffffffffffffff"), true},
		{uint128("0x7fffffffffffffff"), uint128("0xffffffffffffffff"), true},
		{uint128("0x8000000000000000"), uint128("0xffffffffffffffff"), true},
		{uint128("0xffffffffffffffff"), uint128("0xffffffffffffffff"), true},
		{uint128("0x10000000000000000"), uint128("0xffffffffffffffff"), true},
		{uint128("0x20000000000000000"), uint128("0xffffffffffffffff"), true},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0xffffffffffffffff"), true},
		{uint128("0x80000000000000000000000000000000"), uint128("0xffffffffffffffff"), true},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0xffffffffffffffff"), true},

		{uint128("0x0"), uint128("0x10000000000000000"), true},
		{uint128("0x1"), uint128("0x10000000000000000"), true},
		{uint128("0x2"), uint128("0x10000000000000000"), true},
		{uint128("0x7fffffffffffffff"), uint128("0x1000000000000000"), true},
		{uint128("0x8000000000000000"), uint128("0x10000000000000000"), true},
		{uint128("0xffffffffffffffff"), uint128("0x10000000000000000"), true},
		{uint128("0x10000000000000000"), uint128("0x10000000000000000"), true},
		{uint128("0x20000000000000000"), uint128("0x10000000000000000"), true},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0x10000000000000000"), true},
		{uint128("0x80000000000000000000000000000000"), uint128("0x10000000000000000"), true},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0x10000000000000000"), true},

		{uint128("0x0"), uint128("0x20000000000000000"), true},
		{uint128("0x1"), uint128("0x20000000000000000"), true},
		{uint128("0x2"), uint128("0x20000000000000000"), true},
		{uint128("0x7fffffffffffffff"), uint128("0x20000000000000000"), true},
		{uint128("0x8000000000000000"), uint128("0x20000000000000000"), true},
		{uint128("0xffffffffffffffff"), uint128("0x20000000000000000"), true},
		{uint128("0x10000000000000000"), uint128("0x20000000000000000"), true},
		{uint128("0x20000000000000000"), uint128("0x20000000000000000"), true},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0x20000000000000000"), true},
		{uint128("0x80000000000000000000000000000000"), uint128("0x20000000000000000"), true},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0x20000000000000000"), true},

		{uint128("0x0"), uint128("0x7fffffffffffffffffffffffffffffff"), true},
		{uint128("0x1"), uint128("0x7fffffffffffffffffffffffffffffff"), true},
		{uint128("0x2"), uint128("0x7fffffffffffffffffffffffffffffff"), true},
		{uint128("0x7fffffffffffffff"), uint128("0x7fffffffffffffffffffffffffffffff"), true},
		{uint128("0x8000000000000000"), uint128("0x7fffffffffffffffffffffffffffffff"), true},
		{uint128("0xffffffffffffffff"), uint128("0x7fffffffffffffffffffffffffffffff"), true},
		{uint128("0x10000000000000000"), uint128("0x7fffffffffffffffffffffffffffffff"), true},
		{uint128("0x20000000000000000"), uint128("0x7fffffffffffffffffffffffffffffff"), true},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0x7fffffffffffffffffffffffffffffff"), true},
		{uint128("0x80000000000000000000000000000000"), uint128("0x7fffffffffffffffffffffffffffffff"), true},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0x7fffffffffffffffffffffffffffffff"), true},

		{uint128("0x0"), uint128("0x80000000000000000000000000000000"), true},
		{uint128("0x1"), uint128("0x80000000000000000000000000000000"), true},
		{uint128("0x2"), uint128("0x80000000000000000000000000000000"), true},
		{uint128("0x7fffffffffffffff"), uint128("0x80000000000000000000000000000000"), true},
		{uint128("0x8000000000000000"), uint128("0x80000000000000000000000000000000"), true},
		{uint128("0xffffffffffffffff"), uint128("0x80000000000000000000000000000000"), true},
		{uint128("0x10000000000000000"), uint128("0x80000000000000000000000000000000"), true},
		{uint128("0x20000000000000000"), uint128("0x80000000000000000000000000000000"), true},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0x80000000000000000000000000000000"), true},
		{uint128("0x80000000000000000000000000000000"), uint128("0x80000000000000000000000000000000"), true},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0x80000000000000000000000000000000"), true},

		{uint128("0x0"), uint128("0xffffffffffffffffffffffffffffffff"), true},
		{uint128("0x1"), uint128("0xffffffffffffffffffffffffffffffff"), true},
		{uint128("0x2"), uint128("0xffffffffffffffffffffffffffffffff"), true},
		{uint128("0x7fffffffffffffff"), uint128("0xffffffffffffffffffffffffffffffff"), true},
		{uint128("0x8000000000000000"), uint128("0xffffffffffffffffffffffffffffffff"), true},
		{uint128("0xffffffffffffffff"), uint128("0xffffffffffffffffffffffffffffffff"), true},
		{uint128("0x10000000000000000"), uint128("0xffffffffffffffffffffffffffffffff"), true},
		{uint128("0x20000000000000000"), uint128("0xffffffffffffffffffffffffffffffff"), true},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0xffffffffffffffffffffffffffffffff"), true},
		{uint128("0x80000000000000000000000000000000"), uint128("0xffffffffffffffffffffffffffffffff"), true},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0xffffffffffffffffffffffffffffffff"), true},
	}

	for _, test := range tests {
		for _, f := range []func(a, b UInt128Value){
			func(a, b UInt128Value) {
				a.Div(b)
			},
			func(a, b UInt128Value) {
				a.Mod(b)
			},
		} {
			f := func() {
				f(test.a, test.b)
			}
			if test.valid {
				assert.NotPanics(t, f)
			} else {
				assert.Panics(t, f)
			}
		}
	}
}

func TestDivModUInt256(t *testing.T) {

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
			false,
		},
		{
			uint256("0x1"),
			uint256("0x0"),
			false,
		},
		{
			uint256("0x2"),
			uint256("0x0"),
			false,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffff"),
			uint256("0x0"),
			false,
		},
		{
			uint256("0x80000000000000000000000000000000"),
			uint256("0x0"),
			false,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffff"),
			uint256("0x0"),
			false,
		},
		{
			uint256("0x100000000000000000000000000000000"),
			uint256("0x0"),
			false,
		},
		{
			uint256("0x200000000000000000000000000000000"),
			uint256("0x0"),
			false,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x0"),
			false,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0x0"),
			false,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x0"),
			false,
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
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x2"),
			true,
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
			true,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0x7fffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x7fffffffffffffffffffffffffffffff"),
			true,
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
			uint256("0x8000000000000000000000000000000"),
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffff"),
			uint256("0x8000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x100000000000000000000000000000000"),
			uint256("0x8000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x200000000000000000000000000000000"),
			uint256("0x8000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x8000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0x8000000000000000000000000000000"),
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x8000000000000000000000000000000"),
			true,
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
			true,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0xffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0xffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0xffffffffffffffffffffffffffffffff"),
			true,
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
			uint256("0x10000000000000000000000000000000"),
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
			true,
		},
		{
			uint256("0x200000000000000000000000000000000"),
			uint256("0x100000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x100000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0x100000000000000000000000000000000"),
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x100000000000000000000000000000000"),
			true,
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
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffff"),
			uint256("0x200000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x100000000000000000000000000000000"),
			uint256("0x200000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x200000000000000000000000000000000"),
			uint256("0x200000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x200000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0x200000000000000000000000000000000"),
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x200000000000000000000000000000000"),
			true,
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
			uint256("0x7fffffffffffffffffffffffffffffff"),
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x80000000000000000000000000000000"),
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffff"),
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x100000000000000000000000000000000"),
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x200000000000000000000000000000000"),
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
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
			true,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffff"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x80000000000000000000000000000000"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffff"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x100000000000000000000000000000000"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x200000000000000000000000000000000"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			true,
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
			true,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffff"),
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x80000000000000000000000000000000"),
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffff"),
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x100000000000000000000000000000000"),
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x200000000000000000000000000000000"),
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
	}

	for _, test := range tests {
		for _, f := range []func(a, b UInt256Value){
			func(a, b UInt256Value) {
				a.Div(b)
			},
			func(a, b UInt256Value) {
				a.Mod(b)
			},
		} {
			f := func() {
				f(test.a, test.b)
			}
			if test.valid {
				assert.NotPanics(t, f)
			} else {
				assert.Panics(t, f)
			}
		}
	}
}

func TestDivInt8(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  Int8Value
		valid bool
	}{
		{0, 0, false},
		{1, 0, false},
		{2, 0, false},
		{0x7f, 0, false},
		{-128, 0, false},
		{-1, 0, false},

		{0, 1, true},
		{1, 1, true},
		{2, 1, true},
		{0x7f, 1, true},
		{-128, 1, true},
		{-1, 1, true},

		{0, 2, true},
		{1, 2, true},
		{2, 2, true},
		{0x7f, 2, true},
		{-128, 2, true},
		{-1, 2, true},

		{0, 0x7f, true},
		{1, 0x7f, true},
		{2, 0x7f, true},
		{0x7f, 0x7f, true},
		{-128, 0x7f, true},
		{-1, 0x7f, true},

		{0, -128, true},
		{1, -128, true},
		{2, -128, true},
		{0x7f, -128, true},
		{-128, -128, true},
		{-1, -128, true},

		{0, -1, true},
		{1, -1, true},
		{2, -1, true},
		{0x7f, -1, true},
		// NOTE:
		{-128, -1, false},
		{-1, -1, true},
	}

	for _, test := range tests {
		f := func() {
			test.a.Div(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestModInt8(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  Int8Value
		valid bool
	}{
		{0, 0, false},
		{1, 0, false},
		{2, 0, false},
		{0x7f, 0, false},
		{-128, 0, false},
		{-1, 0, false},

		{0, 1, true},
		{1, 1, true},
		{2, 1, true},
		{0x7f, 1, true},
		{-128, 1, true},
		{-1, 1, true},

		{0, 2, true},
		{1, 2, true},
		{2, 2, true},
		{0x7f, 2, true},
		{-128, 2, true},
		{-1, 2, true},

		{0, 0x7f, true},
		{1, 0x7f, true},
		{2, 0x7f, true},
		{0x7f, 0x7f, true},
		{-128, 0x7f, true},
		{-1, 0x7f, true},

		{0, -128, true},
		{1, -128, true},
		{2, -128, true},
		{0x7f, -128, true},
		{-128, -128, true},
		{-1, -128, true},

		{0, -1, true},
		{1, -1, true},
		{2, -1, true},
		{0x7f, -1, true},
		{-128, -1, true},
		{-1, -1, true},
	}

	for _, test := range tests {
		f := func() {
			test.a.Mod(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestDivInt16(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  Int16Value
		valid bool
	}{
		{0, 0, false},
		{1, 0, false},
		{2, 0, false},
		{0x7fff, 0, false},
		{-32768, 0, false},
		{-1, 0, false},

		{0, 1, true},
		{1, 1, true},
		{2, 1, true},
		{0x7fff, 1, true},
		{-32768, 1, true},
		{-1, 1, true},

		{0, 2, true},
		{1, 2, true},
		{2, 2, true},
		{0x7fff, 2, true},
		{-32768, 2, true},
		{-1, 2, true},

		{0, 0x7fff, true},
		{1, 0x7fff, true},
		{2, 0x7fff, true},
		{0x7fff, 0x7fff, true},
		{-32768, 0x7fff, true},
		{-1, 0x7fff, true},

		{0, -32768, true},
		{1, -32768, true},
		{2, -32768, true},
		{0x7fff, -32768, true},
		{-32768, -32768, true},
		{-1, -32768, true},

		{0, -1, true},
		{1, -1, true},
		{2, -1, true},
		{0x7fff, -1, true},
		// NOTE:
		{-32768, -1, false},
		{-1, -1, true},
	}

	for _, test := range tests {
		f := func() {
			test.a.Div(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestModInt16(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  Int16Value
		valid bool
	}{
		{0, 0, false},
		{1, 0, false},
		{2, 0, false},
		{0x7fff, 0, false},
		{-32768, 0, false},
		{-1, 0, false},

		{0, 1, true},
		{1, 1, true},
		{2, 1, true},
		{0x7fff, 1, true},
		{-32768, 1, true},
		{-1, 1, true},

		{0, 2, true},
		{1, 2, true},
		{2, 2, true},
		{0x7fff, 2, true},
		{-32768, 2, true},
		{-1, 2, true},

		{0, 0x7fff, true},
		{1, 0x7fff, true},
		{2, 0x7fff, true},
		{0x7fff, 0x7fff, true},
		{-32768, 0x7fff, true},
		{-1, 0x7fff, true},

		{0, -32768, true},
		{1, -32768, true},
		{2, -32768, true},
		{0x7fff, -32768, true},
		{-32768, -32768, true},
		{-1, -32768, true},

		{0, -1, true},
		{1, -1, true},
		{2, -1, true},
		{0x7fff, -1, true},
		{-32768, -1, true},
		{-1, -1, true},
	}

	for _, test := range tests {
		f := func() {
			test.a.Mod(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestDivInt32(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  Int32Value
		valid bool
	}{
		{0, 0, false},
		{1, 0, false},
		{2, 0, false},
		{0x7fffffff, 0, false},
		{-2147483648, 0, false},
		{-1, 0, false},

		{0, 1, true},
		{1, 1, true},
		{2, 1, true},
		{0x7fffffff, 1, true},
		{-2147483648, 1, true},
		{-1, 1, true},

		{0, 2, true},
		{1, 2, true},
		{2, 2, true},
		{0x7fffffff, 2, true},
		{-2147483648, 2, true},
		{-1, 2, true},

		{0, 0x7fffffff, true},
		{1, 0x7fffffff, true},
		{2, 0x7fffffff, true},
		{0x7fffffff, 0x7fffffff, true},
		{-2147483648, 0x7fffffff, true},
		{-1, 0x7fffffff, true},

		{0, -2147483648, true},
		{1, -2147483648, true},
		{2, -2147483648, true},
		{0x7fffffff, -2147483648, true},
		{-2147483648, -2147483648, true},
		{-1, -2147483648, true},

		{0, -1, true},
		{1, -1, true},
		{2, -1, true},
		{0x7fffffff, -1, true},
		// NOTE:
		{-2147483648, -1, false},
		{-1, -1, true},
	}

	for _, test := range tests {
		f := func() {
			test.a.Div(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestModInt32(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  Int32Value
		valid bool
	}{
		{0, 0, false},
		{1, 0, false},
		{2, 0, false},
		{0x7fffffff, 0, false},
		{-2147483648, 0, false},
		{-1, 0, false},

		{0, 1, true},
		{1, 1, true},
		{2, 1, true},
		{0x7fffffff, 1, true},
		{-2147483648, 1, true},
		{-1, 1, true},

		{0, 2, true},
		{1, 2, true},
		{2, 2, true},
		{0x7fffffff, 2, true},
		{-2147483648, 2, true},
		{-1, 2, true},

		{0, 0x7fffffff, true},
		{1, 0x7fffffff, true},
		{2, 0x7fffffff, true},
		{0x7fffffff, 0x7fffffff, true},
		{-2147483648, 0x7fffffff, true},
		{-1, 0x7fffffff, true},

		{0, -2147483648, true},
		{1, -2147483648, true},
		{2, -2147483648, true},
		{0x7fffffff, -2147483648, true},
		{-2147483648, -2147483648, true},
		{-1, -2147483648, true},

		{0, -1, true},
		{1, -1, true},
		{2, -1, true},
		{0x7fffffff, -1, true},
		{-2147483648, -1, true},
		{-1, -1, true},
	}

	for _, test := range tests {
		f := func() {
			test.a.Mod(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestDivInt64(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  Int64Value
		valid bool
	}{
		{0, 0, false},
		{1, 0, false},
		{2, 0, false},
		{0x7fffffff, 0, false},
		{0x80000000, 0, false},
		{0xffffffff, 0, false},
		{0x100000000, 0, false},
		{0x200000000, 0, false},
		{0x7fffffffffffffff, 0, false},
		{-9223372036854775808, 0, false},
		{-1, 0, false},

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
		{0x7fffffffffffffff, 2, true},
		{-9223372036854775808, 2, true},
		{-1, 2, true},

		{0, 0x7fffffff, true},
		{1, 0x7fffffff, true},
		{2, 0x7fffffff, true},
		{0x7fffffff, 0x7fffffff, true},
		{0x80000000, 0x7fffffff, true},
		{0xffffffff, 0x7fffffff, true},
		{0x100000000, 0x7fffffff, true},
		{0x200000000, 0x7fffffff, true},
		{0x7fffffffffffffff, 0x7fffffff, true},
		{-9223372036854775808, 0x7fffffff, true},
		{-1, 0x7fffffff, true},

		{0, 0x80000000, true},
		{1, 0x80000000, true},
		{2, 0x80000000, true},
		{0x7fffffff, 0x80000000, true},
		{0x80000000, 0x80000000, true},
		{0xffffffff, 0x80000000, true},
		{0x100000000, 0x80000000, true},
		{0x200000000, 0x80000000, true},
		{0x7fffffffffffffff, 0x80000000, true},
		{-9223372036854775808, 0x80000000, true},
		{-1, 0x80000000, true},

		{0, 0xffffffff, true},
		{1, 0xffffffff, true},
		{2, 0xffffffff, true},
		{0x7fffffff, 0xffffffff, true},
		{0x80000000, 0xffffffff, true},
		{0xffffffff, 0xffffffff, true},
		{0x100000000, 0xffffffff, true},
		{0x200000000, 0xffffffff, true},
		{0x7fffffffffffffff, 0xffffffff, true},
		{-9223372036854775808, 0xffffffff, true},
		{-1, 0xffffffff, true},

		{0, 0x100000000, true},
		{1, 0x100000000, true},
		{2, 0x100000000, true},
		{0x7fffffff, 0x100000000, true},
		{0x80000000, 0x100000000, true},
		{0xffffffff, 0x100000000, true},
		{0x100000000, 0x100000000, true},
		{0x200000000, 0x100000000, true},
		{0x7fffffffffffffff, 0x100000000, true},
		{-9223372036854775808, 0x100000000, true},
		{-1, 0x100000000, true},

		{0, 0x200000000, true},
		{1, 0x200000000, true},
		{2, 0x200000000, true},
		{0x7fffffff, 0x200000000, true},
		{0x80000000, 0x200000000, true},
		{0xffffffff, 0x200000000, true},
		{0x100000000, 0x200000000, true},
		{0x200000000, 0x200000000, true},
		{0x7fffffffffffffff, 0x200000000, true},
		{-9223372036854775808, 0x200000000, true},
		{-1, 0x200000000, true},

		{0, 0x7fffffffffffffff, true},
		{1, 0x7fffffffffffffff, true},
		{2, 0x7fffffffffffffff, true},
		{0x7fffffff, 0x7fffffffffffffff, true},
		{0x80000000, 0x7fffffffffffffff, true},
		{0xffffffff, 0x7fffffffffffffff, true},
		{0x100000000, 0x7fffffffffffffff, true},
		{0x200000000, 0x7fffffffffffffff, true},
		{0x7fffffffffffffff, 0x7fffffffffffffff, true},
		{-9223372036854775808, 0x7fffffffffffffff, true},
		{-1, 0x7fffffffffffffff, true},

		{0, -9223372036854775808, true},
		{1, -9223372036854775808, true},
		{2, -9223372036854775808, true},
		{0x7fffffff, -9223372036854775808, true},
		{0x80000000, -9223372036854775808, true},
		{0xffffffff, -9223372036854775808, true},
		{0x100000000, -9223372036854775808, true},
		{0x200000000, -9223372036854775808, true},
		{0x7fffffffffffffff, -9223372036854775808, true},
		{-9223372036854775808, -9223372036854775808, true},
		{-1, -9223372036854775808, true},

		{0, -1, true},
		{1, -1, true},
		{2, -1, true},
		{0x7fffffff, -1, true},
		{0x80000000, -1, true},
		{0xffffffff, -1, true},
		{0x100000000, -1, true},
		{0x200000000, -1, true},
		{0x7fffffffffffffff, -1, true},
		// NOTE:
		{-9223372036854775808, -1, false},
		{-1, -1, true},
	}

	for _, test := range tests {
		f := func() {
			test.a.Div(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestModInt64(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  Int64Value
		valid bool
	}{
		{0, 0, false},
		{1, 0, false},
		{2, 0, false},
		{0x7fffffff, 0, false},
		{0x80000000, 0, false},
		{0xffffffff, 0, false},
		{0x100000000, 0, false},
		{0x200000000, 0, false},
		{0x7fffffffffffffff, 0, false},
		{-9223372036854775808, 0, false},
		{-1, 0, false},

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
		{0x7fffffffffffffff, 2, true},
		{-9223372036854775808, 2, true},
		{-1, 2, true},

		{0, 0x7fffffff, true},
		{1, 0x7fffffff, true},
		{2, 0x7fffffff, true},
		{0x7fffffff, 0x7fffffff, true},
		{0x80000000, 0x7fffffff, true},
		{0xffffffff, 0x7fffffff, true},
		{0x100000000, 0x7fffffff, true},
		{0x200000000, 0x7fffffff, true},
		{0x7fffffffffffffff, 0x7fffffff, true},
		{-9223372036854775808, 0x7fffffff, true},
		{-1, 0x7fffffff, true},

		{0, 0x80000000, true},
		{1, 0x80000000, true},
		{2, 0x80000000, true},
		{0x7fffffff, 0x80000000, true},
		{0x80000000, 0x80000000, true},
		{0xffffffff, 0x80000000, true},
		{0x100000000, 0x80000000, true},
		{0x200000000, 0x80000000, true},
		{0x7fffffffffffffff, 0x80000000, true},
		{-9223372036854775808, 0x80000000, true},
		{-1, 0x80000000, true},

		{0, 0xffffffff, true},
		{1, 0xffffffff, true},
		{2, 0xffffffff, true},
		{0x7fffffff, 0xffffffff, true},
		{0x80000000, 0xffffffff, true},
		{0xffffffff, 0xffffffff, true},
		{0x100000000, 0xffffffff, true},
		{0x200000000, 0xffffffff, true},
		{0x7fffffffffffffff, 0xffffffff, true},
		{-9223372036854775808, 0xffffffff, true},
		{-1, 0xffffffff, true},

		{0, 0x100000000, true},
		{1, 0x100000000, true},
		{2, 0x100000000, true},
		{0x7fffffff, 0x100000000, true},
		{0x80000000, 0x100000000, true},
		{0xffffffff, 0x100000000, true},
		{0x100000000, 0x100000000, true},
		{0x200000000, 0x100000000, true},
		{0x7fffffffffffffff, 0x100000000, true},
		{-9223372036854775808, 0x100000000, true},
		{-1, 0x100000000, true},

		{0, 0x200000000, true},
		{1, 0x200000000, true},
		{2, 0x200000000, true},
		{0x7fffffff, 0x200000000, true},
		{0x80000000, 0x200000000, true},
		{0xffffffff, 0x200000000, true},
		{0x100000000, 0x200000000, true},
		{0x200000000, 0x200000000, true},
		{0x7fffffffffffffff, 0x200000000, true},
		{-9223372036854775808, 0x200000000, true},
		{-1, 0x200000000, true},

		{0, 0x7fffffffffffffff, true},
		{1, 0x7fffffffffffffff, true},
		{2, 0x7fffffffffffffff, true},
		{0x7fffffff, 0x7fffffffffffffff, true},
		{0x80000000, 0x7fffffffffffffff, true},
		{0xffffffff, 0x7fffffffffffffff, true},
		{0x100000000, 0x7fffffffffffffff, true},
		{0x200000000, 0x7fffffffffffffff, true},
		{0x7fffffffffffffff, 0x7fffffffffffffff, true},
		{-9223372036854775808, 0x7fffffffffffffff, true},
		{-1, 0x7fffffffffffffff, true},

		{0, -9223372036854775808, true},
		{1, -9223372036854775808, true},
		{2, -9223372036854775808, true},
		{0x7fffffff, -9223372036854775808, true},
		{0x80000000, -9223372036854775808, true},
		{0xffffffff, -9223372036854775808, true},
		{0x100000000, -9223372036854775808, true},
		{0x200000000, -9223372036854775808, true},
		{0x7fffffffffffffff, -9223372036854775808, true},
		{-9223372036854775808, -9223372036854775808, true},
		{-1, -9223372036854775808, true},

		{0, -1, true},
		{1, -1, true},
		{2, -1, true},
		{0x7fffffff, -1, true},
		{0x80000000, -1, true},
		{0xffffffff, -1, true},
		{0x100000000, -1, true},
		{0x200000000, -1, true},
		{0x7fffffffffffffff, -1, true},
		{-9223372036854775808, -1, true},
		{-1, -1, true},
	}

	for _, test := range tests {
		f := func() {
			test.a.Mod(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestDivModInt(t *testing.T) {

	t.Parallel()

	for _, f := range []func(a, b IntValue){
		func(a, b IntValue) {
			a.Div(b)
		},
		func(a, b IntValue) {
			a.Mod(b)
		},
	} {
		assert.Panics(t, func() {
			f(NewIntValueFromInt64(1), NewIntValueFromInt64(0))
		})
	}
}

func TestDivInt128(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  Int128Value
		valid bool
	}{
		{int128("0x00000000000000000000000000000000"), int128("0x00000000000000000000000000000000"), false},
		{int128("0x00000000000000000000000000000001"), int128("0x00000000000000000000000000000000"), false},
		{int128("0x00000000000000000000000000000002"), int128("0x00000000000000000000000000000000"), false},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("0x00000000000000000000000000000000"), false},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("0x00000000000000000000000000000000"), false},
		{int128("-0x80000000000000000000000000000000"), int128("0x00000000000000000000000000000000"), false},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("0x00000000000000000000000000000000"), false},
		{int128("-0x00000000000000000000000000000002"), int128("0x00000000000000000000000000000000"), false},
		{int128("-0x00000000000000000000000000000001"), int128("0x00000000000000000000000000000000"), false},

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
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("0x00000000000000000000000000000002"), true},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("0x00000000000000000000000000000002"), true},
		{int128("-0x80000000000000000000000000000000"), int128("0x00000000000000000000000000000002"), true},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("0x00000000000000000000000000000002"), true},
		{int128("-0x00000000000000000000000000000002"), int128("0x00000000000000000000000000000002"), true},
		{int128("-0x00000000000000000000000000000001"), int128("0x00000000000000000000000000000002"), true},

		{int128("0x00000000000000000000000000000000"), int128("0x7ffffffffffffffffffffffffffffffe"), true},
		{int128("0x00000000000000000000000000000001"), int128("0x7ffffffffffffffffffffffffffffffe"), true},
		{int128("0x00000000000000000000000000000002"), int128("0x7ffffffffffffffffffffffffffffffe"), true},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("0x7ffffffffffffffffffffffffffffffe"), true},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("0x7ffffffffffffffffffffffffffffffe"), true},
		{int128("-0x80000000000000000000000000000000"), int128("0x7ffffffffffffffffffffffffffffffe"), true},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("0x7ffffffffffffffffffffffffffffffe"), true},
		{int128("-0x00000000000000000000000000000002"), int128("0x7ffffffffffffffffffffffffffffffe"), true},
		{int128("-0x00000000000000000000000000000001"), int128("0x7ffffffffffffffffffffffffffffffe"), true},

		{int128("0x00000000000000000000000000000000"), int128("0x7fffffffffffffffffffffffffffffff"), true},
		{int128("0x00000000000000000000000000000001"), int128("0x7fffffffffffffffffffffffffffffff"), true},
		{int128("0x00000000000000000000000000000002"), int128("0x7fffffffffffffffffffffffffffffff"), true},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("0x7fffffffffffffffffffffffffffffff"), true},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("0x7fffffffffffffffffffffffffffffff"), true},
		{int128("-0x80000000000000000000000000000000"), int128("0x7fffffffffffffffffffffffffffffff"), true},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("0x7fffffffffffffffffffffffffffffff"), true},
		{int128("-0x00000000000000000000000000000002"), int128("0x7fffffffffffffffffffffffffffffff"), true},
		{int128("-0x00000000000000000000000000000001"), int128("0x7fffffffffffffffffffffffffffffff"), true},

		{int128("0x00000000000000000000000000000000"), int128("-0x80000000000000000000000000000000"), true},
		{int128("0x00000000000000000000000000000001"), int128("-0x80000000000000000000000000000000"), true},
		{int128("0x00000000000000000000000000000002"), int128("-0x80000000000000000000000000000000"), true},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("-0x80000000000000000000000000000000"), true},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("-0x80000000000000000000000000000000"), true},
		{int128("-0x80000000000000000000000000000000"), int128("-0x80000000000000000000000000000000"), true},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("-0x80000000000000000000000000000000"), true},
		{int128("-0x00000000000000000000000000000002"), int128("-0x80000000000000000000000000000000"), true},
		{int128("-0x00000000000000000000000000000001"), int128("-0x80000000000000000000000000000000"), true},

		{int128("0x00000000000000000000000000000000"), int128("-0x7fffffffffffffffffffffffffffffff"), true},
		{int128("0x00000000000000000000000000000001"), int128("-0x7fffffffffffffffffffffffffffffff"), true},
		{int128("0x00000000000000000000000000000002"), int128("-0x7fffffffffffffffffffffffffffffff"), true},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("-0x7fffffffffffffffffffffffffffffff"), true},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("-0x7fffffffffffffffffffffffffffffff"), true},
		{int128("-0x80000000000000000000000000000000"), int128("-0x7fffffffffffffffffffffffffffffff"), true},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("-0x7fffffffffffffffffffffffffffffff"), true},
		{int128("-0x00000000000000000000000000000002"), int128("-0x7fffffffffffffffffffffffffffffff"), true},
		{int128("-0x00000000000000000000000000000001"), int128("-0x7fffffffffffffffffffffffffffffff"), true},

		{int128("0x00000000000000000000000000000000"), int128("-0x00000000000000000000000000000002"), true},
		{int128("0x00000000000000000000000000000001"), int128("-0x00000000000000000000000000000002"), true},
		{int128("0x00000000000000000000000000000002"), int128("-0x00000000000000000000000000000002"), true},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("-0x00000000000000000000000000000002"), true},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("-0x00000000000000000000000000000002"), true},
		{int128("-0x80000000000000000000000000000000"), int128("-0x00000000000000000000000000000002"), true},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("-0x00000000000000000000000000000002"), true},
		{int128("-0x00000000000000000000000000000002"), int128("-0x00000000000000000000000000000002"), true},
		{int128("-0x00000000000000000000000000000001"), int128("-0x00000000000000000000000000000002"), true},

		{int128("0x00000000000000000000000000000000"), int128("-0x00000000000000000000000000000001"), true},
		{int128("0x00000000000000000000000000000001"), int128("-0x00000000000000000000000000000001"), true},
		{int128("0x00000000000000000000000000000002"), int128("-0x00000000000000000000000000000001"), true},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("-0x00000000000000000000000000000001"), true},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("-0x00000000000000000000000000000001"), true},
		// NOTE:
		{int128("-0x80000000000000000000000000000000"), int128("-0x00000000000000000000000000000001"), false},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("-0x00000000000000000000000000000001"), true},
		{int128("-0x00000000000000000000000000000002"), int128("-0x00000000000000000000000000000001"), true},
		{int128("-0x00000000000000000000000000000001"), int128("-0x00000000000000000000000000000001"), true},
	}

	for _, test := range tests {
		f := func() {
			test.a.Div(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestModInt128(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  Int128Value
		valid bool
	}{
		{int128("0x00000000000000000000000000000000"), int128("0x00000000000000000000000000000000"), false},
		{int128("0x00000000000000000000000000000001"), int128("0x00000000000000000000000000000000"), false},
		{int128("0x00000000000000000000000000000002"), int128("0x00000000000000000000000000000000"), false},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("0x00000000000000000000000000000000"), false},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("0x00000000000000000000000000000000"), false},
		{int128("-0x80000000000000000000000000000000"), int128("0x00000000000000000000000000000000"), false},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("0x00000000000000000000000000000000"), false},
		{int128("-0x00000000000000000000000000000002"), int128("0x00000000000000000000000000000000"), false},
		{int128("-0x00000000000000000000000000000001"), int128("0x00000000000000000000000000000000"), false},

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
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("0x00000000000000000000000000000002"), true},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("0x00000000000000000000000000000002"), true},
		{int128("-0x80000000000000000000000000000000"), int128("0x00000000000000000000000000000002"), true},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("0x00000000000000000000000000000002"), true},
		{int128("-0x00000000000000000000000000000002"), int128("0x00000000000000000000000000000002"), true},
		{int128("-0x00000000000000000000000000000001"), int128("0x00000000000000000000000000000002"), true},

		{int128("0x00000000000000000000000000000000"), int128("0x7ffffffffffffffffffffffffffffffe"), true},
		{int128("0x00000000000000000000000000000001"), int128("0x7ffffffffffffffffffffffffffffffe"), true},
		{int128("0x00000000000000000000000000000002"), int128("0x7ffffffffffffffffffffffffffffffe"), true},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("0x7ffffffffffffffffffffffffffffffe"), true},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("0x7ffffffffffffffffffffffffffffffe"), true},
		{int128("-0x80000000000000000000000000000000"), int128("0x7ffffffffffffffffffffffffffffffe"), true},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("0x7ffffffffffffffffffffffffffffffe"), true},
		{int128("-0x00000000000000000000000000000002"), int128("0x7ffffffffffffffffffffffffffffffe"), true},
		{int128("-0x00000000000000000000000000000001"), int128("0x7ffffffffffffffffffffffffffffffe"), true},

		{int128("0x00000000000000000000000000000000"), int128("0x7fffffffffffffffffffffffffffffff"), true},
		{int128("0x00000000000000000000000000000001"), int128("0x7fffffffffffffffffffffffffffffff"), true},
		{int128("0x00000000000000000000000000000002"), int128("0x7fffffffffffffffffffffffffffffff"), true},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("0x7fffffffffffffffffffffffffffffff"), true},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("0x7fffffffffffffffffffffffffffffff"), true},
		{int128("-0x80000000000000000000000000000000"), int128("0x7fffffffffffffffffffffffffffffff"), true},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("0x7fffffffffffffffffffffffffffffff"), true},
		{int128("-0x00000000000000000000000000000002"), int128("0x7fffffffffffffffffffffffffffffff"), true},
		{int128("-0x00000000000000000000000000000001"), int128("0x7fffffffffffffffffffffffffffffff"), true},

		{int128("0x00000000000000000000000000000000"), int128("-0x80000000000000000000000000000000"), true},
		{int128("0x00000000000000000000000000000001"), int128("-0x80000000000000000000000000000000"), true},
		{int128("0x00000000000000000000000000000002"), int128("-0x80000000000000000000000000000000"), true},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("-0x80000000000000000000000000000000"), true},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("-0x80000000000000000000000000000000"), true},
		{int128("-0x80000000000000000000000000000000"), int128("-0x80000000000000000000000000000000"), true},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("-0x80000000000000000000000000000000"), true},
		{int128("-0x00000000000000000000000000000002"), int128("-0x80000000000000000000000000000000"), true},
		{int128("-0x00000000000000000000000000000001"), int128("-0x80000000000000000000000000000000"), true},

		{int128("0x00000000000000000000000000000000"), int128("-0x7fffffffffffffffffffffffffffffff"), true},
		{int128("0x00000000000000000000000000000001"), int128("-0x7fffffffffffffffffffffffffffffff"), true},
		{int128("0x00000000000000000000000000000002"), int128("-0x7fffffffffffffffffffffffffffffff"), true},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("-0x7fffffffffffffffffffffffffffffff"), true},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("-0x7fffffffffffffffffffffffffffffff"), true},
		{int128("-0x80000000000000000000000000000000"), int128("-0x7fffffffffffffffffffffffffffffff"), true},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("-0x7fffffffffffffffffffffffffffffff"), true},
		{int128("-0x00000000000000000000000000000002"), int128("-0x7fffffffffffffffffffffffffffffff"), true},
		{int128("-0x00000000000000000000000000000001"), int128("-0x7fffffffffffffffffffffffffffffff"), true},

		{int128("0x00000000000000000000000000000000"), int128("-0x00000000000000000000000000000002"), true},
		{int128("0x00000000000000000000000000000001"), int128("-0x00000000000000000000000000000002"), true},
		{int128("0x00000000000000000000000000000002"), int128("-0x00000000000000000000000000000002"), true},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("-0x00000000000000000000000000000002"), true},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("-0x00000000000000000000000000000002"), true},
		{int128("-0x80000000000000000000000000000000"), int128("-0x00000000000000000000000000000002"), true},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("-0x00000000000000000000000000000002"), true},
		{int128("-0x00000000000000000000000000000002"), int128("-0x00000000000000000000000000000002"), true},
		{int128("-0x00000000000000000000000000000001"), int128("-0x00000000000000000000000000000002"), true},

		{int128("0x00000000000000000000000000000000"), int128("-0x00000000000000000000000000000001"), true},
		{int128("0x00000000000000000000000000000001"), int128("-0x00000000000000000000000000000001"), true},
		{int128("0x00000000000000000000000000000002"), int128("-0x00000000000000000000000000000001"), true},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("-0x00000000000000000000000000000001"), true},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("-0x00000000000000000000000000000001"), true},
		{int128("-0x80000000000000000000000000000000"), int128("-0x00000000000000000000000000000001"), true},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("-0x00000000000000000000000000000001"), true},
		{int128("-0x00000000000000000000000000000002"), int128("-0x00000000000000000000000000000001"), true},
		{int128("-0x00000000000000000000000000000001"), int128("-0x00000000000000000000000000000001"), true},
	}

	for _, test := range tests {
		f := func() {
			test.a.Mod(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestDivInt256(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  Int256Value
		valid bool
	}{
		// 0x0000000000000000000000000000000000000000000000000000000000000000

		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			false,
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
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
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

		// 0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe

		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			true,
		},
		{
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			true,
		},
		{
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			true,
		},
		{
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			true,
		},
		{
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
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
			true,
		},
		{
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
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
			true,
		},
		{
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},

		// -0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff

		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
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
			true,
		},
		{
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
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
		// NOTE:
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
			test.a.Div(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestModInt256(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  Int256Value
		valid bool
	}{
		// 0x0000000000000000000000000000000000000000000000000000000000000000

		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			false,
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
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
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

		// 0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe

		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			true,
		},
		{
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			true,
		},
		{
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			true,
		},
		{
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			true,
		},
		{
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
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
			true,
		},
		{
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
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
			true,
		},
		{
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},

		// -0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff

		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			int256("-0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
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
			true,
		},
		{
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("-0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
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
			true,
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
			test.a.Mod(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestReciprocalFix64(t *testing.T) {

	t.Parallel()

	assert.PanicsWithValue(t,
		DivisionByZeroError{},
		func() {
			Fix64Value(0).Reciprocal()
		},
	)

	assert.Equal(t,
		Fix64Value(sema.Fix64Factor),
		Fix64Value(sema.Fix64Factor).Reciprocal(),
	)

	assert.Equal(t,
		Fix64Value(1),
		Fix64Value(Fix64MaxDivisorValue).Reciprocal(),
	)

	assert.Equal(t,
		Fix64Value(0),
		Fix64Value(2*Fix64MaxDivisorValue).Reciprocal(),
	)
}

func TestReciprocalUFix64(t *testing.T) {

	t.Parallel()

	assert.PanicsWithValue(t,
		DivisionByZeroError{},
		func() {
			UFix64Value(0).Reciprocal()
		},
	)

	assert.Equal(t,
		UFix64Value(sema.Fix64Factor),
		UFix64Value(sema.Fix64Factor).Reciprocal(),
	)

	assert.Equal(t,
		UFix64Value(1),
		UFix64Value(Fix64MaxDivisorValue).Reciprocal(),
	)

	assert.Equal(t,
		UFix64Value(0),
		UFix64Value(2*Fix64MaxDivisorValue).Reciprocal(),
	)
}

func TestDivFix64(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  int64
		valid bool
	}{
		{0, 0, false},
		{1, 0, false},
		{2, 0, false},
		{Fix64MaxIntDividend, 0, false},
		{Fix64MaxIntDividend + 1, 0, false},
		{-1, 0, false},

		{0, 1, true},
		{1, 1, true},
		{2, 1, true},
		{Fix64MaxIntDividend, 1, true},
		{Fix64MaxIntDividend + 1, 1, false},
		{-1, 1, true},

		{0, 2, true},
		{1, 2, true},
		{2, 2, true},
		{Fix64MaxIntDividend, 2, true},
		{Fix64MaxIntDividend + 1, 2, false},
		{-1, 2, true},

		{0, -1, true},
		{1, -1, true},
		{2, -1, true},
		{Fix64MaxIntDividend, -1, true},
		{Fix64MaxIntDividend + 1, -1, false},
		{-1, -1, true},
	}

	for _, test := range tests {

		f := func() {
			a := NewFix64ValueWithInteger(test.a)
			b := NewFix64ValueWithInteger(test.b)
			a.Div(b)
		}

		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}

	assert.Equal(t,
		Fix64Value(1),
		NewFix64ValueWithInteger(1).
			Div(Fix64Value(Fix64MaxDivisorValue)),
	)

	assert.Panics(t, func() {
		NewFix64ValueWithInteger(1).
			Div(Fix64Value(Fix64MaxDivisorValue + 1))
	})
}

func TestModFix64(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  Int64Value
		valid bool
	}{
		{0, 0, false},
		{1, 0, false},
		{2, 0, false},
		{-1, 0, false},

		{0, 1, true},
		{1, 1, true},
		{2, 1, true},
		{-1, 1, true},

		{0, 2, true},
		{1, 2, true},
		{2, 2, true},
		{-1, 2, true},

		{0, -1, true},
		{1, -1, true},
		{2, -1, true},
		{-1, -1, true},
	}

	for _, test := range tests {
		f := func() {
			test.a.Mod(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestDivModUFix64(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  uint64
		valid bool
	}{
		{0, 0, false},
		{1, 0, false},
		{2, 0, false},
		{UFix64MaxIntDividend, 0, false},
		{UFix64MaxIntDividend + 1, 0, false},

		{0, 1, true},
		{1, 1, true},
		{2, 1, true},
		{UFix64MaxIntDividend, 1, true},
		{UFix64MaxIntDividend + 1, 1, false},

		{0, 2, true},
		{1, 2, true},
		{2, 2, true},
		{UFix64MaxIntDividend, 2, true},
		{UFix64MaxIntDividend + 1, 2, false},
	}

	for _, test := range tests {

		for _, f := range []func(a, b UFix64Value){
			func(a, b UFix64Value) {
				a.Div(b)
			},
			func(a, b UFix64Value) {
				a.Mod(b)
			},
		} {

			f := func() {
				a := NewUFix64ValueWithInteger(test.a)
				b := NewUFix64ValueWithInteger(test.b)
				f(a, b)
			}

			if test.valid {
				assert.NotPanics(t, f)
			} else {
				assert.Panics(t, f)
			}
		}
	}

	assert.Equal(t,
		UFix64Value(1),
		NewUFix64ValueWithInteger(1).
			Div(UFix64Value(UFix64MaxDivisorValue)),
	)

	assert.Panics(t, func() {
		NewUFix64ValueWithInteger(1).
			Div(UFix64Value(UFix64MaxDivisorValue + 1))
	})
}

// TestNegativeMod ensures that modulo uses the dividend's sign
// when an operand is negative
//
func TestNegativeMod(t *testing.T) {

	t.Parallel()

	t.Run("integer", func(t *testing.T) {

		tests := map[string]struct{ a, b, expected NumberValue }{
			"Int8": {
				Int8Value(-1),
				Int8Value(5),
				Int8Value(-1),
			},
			"Int16": {
				Int16Value(-1),
				Int16Value(5),
				Int16Value(-1),
			},
			"Int32": {
				Int32Value(-1),
				Int32Value(5),
				Int32Value(-1),
			},
			"Int64": {
				Int64Value(-1),
				Int64Value(5),
				Int64Value(-1),
			},
			"Int128": {
				NewInt128ValueFromInt64(-1),
				NewInt128ValueFromInt64(5),
				NewInt128ValueFromInt64(-1),
			},
			"Int256": {
				NewInt256ValueFromInt64(-1),
				NewInt256ValueFromInt64(5),
				NewInt256ValueFromInt64(-1),
			},
			"Int": {
				NewIntValueFromInt64(-1),
				NewIntValueFromInt64(5),
				NewIntValueFromInt64(-1),
			},
		}

		for _, integerType := range sema.AllSignedIntegerTypes {
			if _, ok := tests[integerType.String()]; !ok {
				panic(fmt.Sprintf("broken test: missing %s", integerType))
			}
		}

		for _, test := range tests {
			assert.Equal(t,
				test.expected,
				test.a.Mod(test.b),
			)
		}
	})

	t.Run("fixed-point", func(t *testing.T) {

		tests := map[string]struct{ a, b, expected NumberValue }{
			"Fix64": {
				NewFix64ValueWithInteger(-1),
				NewFix64ValueWithInteger(5),
				NewFix64ValueWithInteger(-1),
			},
		}

		for _, integerType := range sema.AllSignedFixedPointTypes {
			if _, ok := tests[integerType.String()]; !ok {
				panic(fmt.Sprintf("broken test: missing %s", integerType))
			}
		}

		for _, test := range tests {
			assert.Equal(t,
				test.expected,
				test.a.Mod(test.b),
			)
		}
	})
}
