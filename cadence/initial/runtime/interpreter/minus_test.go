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

func TestMinusUInt8(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  UInt8Value
		valid bool
	}{
		{0x00, 0x00, true},
		{0x01, 0x00, true},
		{0x02, 0x00, true},
		{0x7e, 0x00, true},
		{0x7f, 0x00, true},
		{0x80, 0x00, true},
		{0x81, 0x00, true},
		{0xfe, 0x00, true},
		{0xff, 0x00, true},

		{0x00, 0x01, false},
		{0x01, 0x01, true},
		{0x02, 0x01, true},
		{0x7e, 0x01, true},
		{0x7f, 0x01, true},
		{0x80, 0x01, true},
		{0x81, 0x01, true},
		{0xfe, 0x01, true},
		{0xff, 0x01, true},

		{0x00, 0x02, false},
		{0x01, 0x02, false},
		{0x02, 0x02, true},
		{0x7e, 0x02, true},
		{0x7f, 0x02, true},
		{0x80, 0x02, true},
		{0x81, 0x02, true},
		{0xfe, 0x02, true},
		{0xff, 0x02, true},

		{0x00, 0x7e, false},
		{0x01, 0x7e, false},
		{0x02, 0x7e, false},
		{0x7e, 0x7e, true},
		{0x7f, 0x7e, true},
		{0x80, 0x7e, true},
		{0x81, 0x7e, true},
		{0xfe, 0x7e, true},
		{0xff, 0x7e, true},

		{0x00, 0x7f, false},
		{0x01, 0x7f, false},
		{0x02, 0x7f, false},
		{0x7e, 0x7f, false},
		{0x7f, 0x7f, true},
		{0x80, 0x7f, true},
		{0x81, 0x7f, true},
		{0xfe, 0x7f, true},
		{0xff, 0x7f, true},

		{0x00, 0x80, false},
		{0x01, 0x80, false},
		{0x02, 0x80, false},
		{0x7e, 0x80, false},
		{0x7f, 0x80, false},
		{0x80, 0x80, true},
		{0x81, 0x80, true},
		{0xfe, 0x80, true},
		{0xff, 0x80, true},

		{0x00, 0x81, false},
		{0x01, 0x81, false},
		{0x02, 0x81, false},
		{0x7e, 0x81, false},
		{0x7f, 0x81, false},
		{0x80, 0x81, false},
		{0x81, 0x81, true},
		{0xfe, 0x81, true},
		{0xff, 0x81, true},

		{0x00, 0xfe, false},
		{0x01, 0xfe, false},
		{0x02, 0xfe, false},
		{0x7e, 0xfe, false},
		{0x7f, 0xfe, false},
		{0x80, 0xfe, false},
		{0x81, 0xfe, false},
		{0xfe, 0xfe, true},
		{0xff, 0xfe, true},

		{0x00, 0xff, false},
		{0x01, 0xff, false},
		{0x02, 0xff, false},
		{0x7e, 0xff, false},
		{0x7f, 0xff, false},
		{0x80, 0xff, false},
		{0x81, 0xff, false},
		{0xfe, 0xff, false},
		{0xff, 0xff, true},
	}

	for _, test := range tests {
		f := func() {
			test.a.Minus(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestMinusUInt16(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  UInt16Value
		valid bool
	}{
		{0x0000, 0x0000, true},
		{0x0001, 0x0000, true},
		{0x0002, 0x0000, true},
		{0x7ffe, 0x0000, true},
		{0x7fff, 0x0000, true},
		{0x8000, 0x0000, true},
		{0x8001, 0x0000, true},
		{0xfffe, 0x0000, true},
		{0xffff, 0x0000, true},

		{0x0000, 0x0001, false},
		{0x0001, 0x0001, true},
		{0x0002, 0x0001, true},
		{0x7ffe, 0x0001, true},
		{0x7fff, 0x0001, true},
		{0x8000, 0x0001, true},
		{0x8001, 0x0001, true},
		{0xfffe, 0x0001, true},
		{0xffff, 0x0001, true},

		{0x0000, 0x0002, false},
		{0x0001, 0x0002, false},
		{0x0002, 0x0002, true},
		{0x7ffe, 0x0002, true},
		{0x7fff, 0x0002, true},
		{0x8000, 0x0002, true},
		{0x8001, 0x0002, true},
		{0xfffe, 0x0002, true},
		{0xffff, 0x0002, true},

		{0x0000, 0x7ffe, false},
		{0x0001, 0x7ffe, false},
		{0x0002, 0x7ffe, false},
		{0x7ffe, 0x7ffe, true},
		{0x7fff, 0x7ffe, true},
		{0x8000, 0x7ffe, true},
		{0x8001, 0x7ffe, true},
		{0xfffe, 0x7ffe, true},
		{0xffff, 0x7ffe, true},

		{0x0000, 0x7fff, false},
		{0x0001, 0x7fff, false},
		{0x0002, 0x7fff, false},
		{0x7ffe, 0x7fff, false},
		{0x7fff, 0x7fff, true},
		{0x8000, 0x7fff, true},
		{0x8001, 0x7fff, true},
		{0xfffe, 0x7fff, true},
		{0xffff, 0x7fff, true},

		{0x0000, 0x8000, false},
		{0x0001, 0x8000, false},
		{0x0002, 0x8000, false},
		{0x7ffe, 0x8000, false},
		{0x7fff, 0x8000, false},
		{0x8000, 0x8000, true},
		{0x8001, 0x8000, true},
		{0xfffe, 0x8000, true},
		{0xffff, 0x8000, true},

		{0x0000, 0x8001, false},
		{0x0001, 0x8001, false},
		{0x0002, 0x8001, false},
		{0x7ffe, 0x8001, false},
		{0x7fff, 0x8001, false},
		{0x8000, 0x8001, false},
		{0x8001, 0x8001, true},
		{0xfffe, 0x8001, true},
		{0xffff, 0x8001, true},

		{0x0000, 0xfffe, false},
		{0x0001, 0xfffe, false},
		{0x0002, 0xfffe, false},
		{0x7ffe, 0xfffe, false},
		{0x7fff, 0xfffe, false},
		{0x8000, 0xfffe, false},
		{0x8001, 0xfffe, false},
		{0xfffe, 0xfffe, true},
		{0xffff, 0xfffe, true},

		{0x0000, 0xffff, false},
		{0x0001, 0xffff, false},
		{0x0002, 0xffff, false},
		{0x7ffe, 0xffff, false},
		{0x7fff, 0xffff, false},
		{0x8000, 0xffff, false},
		{0x8001, 0xffff, false},
		{0xfffe, 0xffff, false},
		{0xffff, 0xffff, true},
	}

	for _, test := range tests {
		f := func() {
			test.a.Minus(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestMinusUInt32(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  UInt32Value
		valid bool
	}{
		{0x00000000, 0x00000000, true},
		{0x00000001, 0x00000000, true},
		{0x00000002, 0x00000000, true},
		{0x7ffffffe, 0x00000000, true},
		{0x7fffffff, 0x00000000, true},
		{0x80000000, 0x00000000, true},
		{0x80000001, 0x00000000, true},
		{0xfffffffe, 0x00000000, true},
		{0xffffffff, 0x00000000, true},

		{0x00000000, 0x00000001, false},
		{0x00000001, 0x00000001, true},
		{0x00000002, 0x00000001, true},
		{0x7ffffffe, 0x00000001, true},
		{0x7fffffff, 0x00000001, true},
		{0x80000000, 0x00000001, true},
		{0x80000001, 0x00000001, true},
		{0xfffffffe, 0x00000001, true},
		{0xffffffff, 0x00000001, true},

		{0x00000000, 0x00000002, false},
		{0x00000001, 0x00000002, false},
		{0x00000002, 0x00000002, true},
		{0x7ffffffe, 0x00000002, true},
		{0x7fffffff, 0x00000002, true},
		{0x80000000, 0x00000002, true},
		{0x80000001, 0x00000002, true},
		{0xfffffffe, 0x00000002, true},
		{0xffffffff, 0x00000002, true},

		{0x00000000, 0x7ffffffe, false},
		{0x00000001, 0x7ffffffe, false},
		{0x00000002, 0x7ffffffe, false},
		{0x7ffffffe, 0x7ffffffe, true},
		{0x7fffffff, 0x7ffffffe, true},
		{0x80000000, 0x7ffffffe, true},
		{0x80000001, 0x7ffffffe, true},
		{0xfffffffe, 0x7ffffffe, true},
		{0xffffffff, 0x7ffffffe, true},

		{0x00000000, 0x7fffffff, false},
		{0x00000001, 0x7fffffff, false},
		{0x00000002, 0x7fffffff, false},
		{0x7ffffffe, 0x7fffffff, false},
		{0x7fffffff, 0x7fffffff, true},
		{0x80000000, 0x7fffffff, true},
		{0x80000001, 0x7fffffff, true},
		{0xfffffffe, 0x7fffffff, true},
		{0xffffffff, 0x7fffffff, true},

		{0x00000000, 0x80000000, false},
		{0x00000001, 0x80000000, false},
		{0x00000002, 0x80000000, false},
		{0x7ffffffe, 0x80000000, false},
		{0x7fffffff, 0x80000000, false},
		{0x80000000, 0x80000000, true},
		{0x80000001, 0x80000000, true},
		{0xfffffffe, 0x80000000, true},
		{0xffffffff, 0x80000000, true},

		{0x00000000, 0x80000001, false},
		{0x00000001, 0x80000001, false},
		{0x00000002, 0x80000001, false},
		{0x7ffffffe, 0x80000001, false},
		{0x7fffffff, 0x80000001, false},
		{0x80000000, 0x80000001, false},
		{0x80000001, 0x80000001, true},
		{0xfffffffe, 0x80000001, true},
		{0xffffffff, 0x80000001, true},

		{0x00000000, 0xfffffffe, false},
		{0x00000001, 0xfffffffe, false},
		{0x00000002, 0xfffffffe, false},
		{0x7ffffffe, 0xfffffffe, false},
		{0x7fffffff, 0xfffffffe, false},
		{0x80000000, 0xfffffffe, false},
		{0x80000001, 0xfffffffe, false},
		{0xfffffffe, 0xfffffffe, true},
		{0xffffffff, 0xfffffffe, true},

		{0x00000000, 0xffffffff, false},
		{0x00000001, 0xffffffff, false},
		{0x00000002, 0xffffffff, false},
		{0x7ffffffe, 0xffffffff, false},
		{0x7fffffff, 0xffffffff, false},
		{0x80000000, 0xffffffff, false},
		{0x80000001, 0xffffffff, false},
		{0xfffffffe, 0xffffffff, false},
		{0xffffffff, 0xffffffff, true},
	}

	for _, test := range tests {
		f := func() {
			test.a.Minus(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestMinusUInt64(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  UInt64Value
		valid bool
	}{
		{0x0000000000000000, 0x0000000000000000, true},
		{0x0000000000000001, 0x0000000000000000, true},
		{0x0000000000000002, 0x0000000000000000, true},
		{0x000000007ffffffe, 0x0000000000000000, true},
		{0x000000007fffffff, 0x0000000000000000, true},
		{0x0000000080000000, 0x0000000000000000, true},
		{0x0000000080000001, 0x0000000000000000, true},
		{0x00000000fffffffe, 0x0000000000000000, true},
		{0x00000000ffffffff, 0x0000000000000000, true},
		{0x0000000100000000, 0x0000000000000000, true},
		{0x0000000200000000, 0x0000000000000000, true},
		{0x7ffffffffffffffe, 0x0000000000000000, true},
		{0x7fffffffffffffff, 0x0000000000000000, true},
		{0x8000000000000000, 0x0000000000000000, true},
		{0x8000000000000001, 0x0000000000000000, true},
		{0xfffffffffffffffe, 0x0000000000000000, true},
		{0xffffffffffffffff, 0x0000000000000000, true},

		{0x0000000000000000, 0x0000000000000001, false},
		{0x0000000000000001, 0x0000000000000001, true},
		{0x0000000000000002, 0x0000000000000001, true},
		{0x000000007ffffffe, 0x0000000000000001, true},
		{0x000000007fffffff, 0x0000000000000001, true},
		{0x0000000080000000, 0x0000000000000001, true},
		{0x0000000080000001, 0x0000000000000001, true},
		{0x00000000fffffffe, 0x0000000000000001, true},
		{0x00000000ffffffff, 0x0000000000000001, true},
		{0x0000000100000000, 0x0000000000000001, true},
		{0x0000000200000000, 0x0000000000000001, true},
		{0x7ffffffffffffffe, 0x0000000000000001, true},
		{0x7fffffffffffffff, 0x0000000000000001, true},
		{0x8000000000000000, 0x0000000000000001, true},
		{0x8000000000000001, 0x0000000000000001, true},
		{0xfffffffffffffffe, 0x0000000000000001, true},
		{0xffffffffffffffff, 0x0000000000000001, true},

		{0x0000000000000000, 0x0000000000000002, false},
		{0x0000000000000001, 0x0000000000000002, false},
		{0x0000000000000002, 0x0000000000000002, true},
		{0x000000007ffffffe, 0x0000000000000002, true},
		{0x000000007fffffff, 0x0000000000000002, true},
		{0x0000000080000000, 0x0000000000000002, true},
		{0x0000000080000001, 0x0000000000000002, true},
		{0x00000000fffffffe, 0x0000000000000002, true},
		{0x00000000ffffffff, 0x0000000000000002, true},
		{0x0000000100000000, 0x0000000000000002, true},
		{0x0000000200000000, 0x0000000000000002, true},
		{0x7ffffffffffffffe, 0x0000000000000002, true},
		{0x7fffffffffffffff, 0x0000000000000002, true},
		{0x8000000000000000, 0x0000000000000002, true},
		{0x8000000000000001, 0x0000000000000002, true},
		{0xfffffffffffffffe, 0x0000000000000002, true},
		{0xffffffffffffffff, 0x0000000000000002, true},

		{0x0000000000000000, 0x000000007ffffffe, false},
		{0x0000000000000001, 0x000000007ffffffe, false},
		{0x0000000000000002, 0x000000007ffffffe, false},
		{0x000000007ffffffe, 0x000000007ffffffe, true},
		{0x000000007fffffff, 0x000000007ffffffe, true},
		{0x0000000080000000, 0x000000007ffffffe, true},
		{0x0000000080000001, 0x000000007ffffffe, true},
		{0x00000000fffffffe, 0x000000007ffffffe, true},
		{0x00000000ffffffff, 0x000000007ffffffe, true},
		{0x0000000100000000, 0x000000007ffffffe, true},
		{0x0000000200000000, 0x000000007ffffffe, true},
		{0x7ffffffffffffffe, 0x000000007ffffffe, true},
		{0x7fffffffffffffff, 0x000000007ffffffe, true},
		{0x8000000000000000, 0x000000007ffffffe, true},
		{0x8000000000000001, 0x000000007ffffffe, true},
		{0xfffffffffffffffe, 0x000000007ffffffe, true},
		{0xffffffffffffffff, 0x000000007ffffffe, true},

		{0x0000000000000000, 0x000000007fffffff, false},
		{0x0000000000000001, 0x000000007fffffff, false},
		{0x0000000000000002, 0x000000007fffffff, false},
		{0x000000007ffffffe, 0x000000007fffffff, false},
		{0x000000007fffffff, 0x000000007fffffff, true},
		{0x0000000080000000, 0x000000007fffffff, true},
		{0x0000000080000001, 0x000000007fffffff, true},
		{0x00000000fffffffe, 0x000000007fffffff, true},
		{0x00000000ffffffff, 0x000000007fffffff, true},
		{0x0000000100000000, 0x000000007fffffff, true},
		{0x0000000200000000, 0x000000007fffffff, true},
		{0x7ffffffffffffffe, 0x000000007fffffff, true},
		{0x7fffffffffffffff, 0x000000007fffffff, true},
		{0x8000000000000000, 0x000000007fffffff, true},
		{0x8000000000000001, 0x000000007fffffff, true},
		{0xfffffffffffffffe, 0x000000007fffffff, true},
		{0xffffffffffffffff, 0x000000007fffffff, true},

		{0x0000000000000000, 0x0000000080000000, false},
		{0x0000000000000001, 0x0000000080000000, false},
		{0x0000000000000002, 0x0000000080000000, false},
		{0x000000007ffffffe, 0x0000000080000000, false},
		{0x000000007fffffff, 0x0000000080000000, false},
		{0x0000000080000000, 0x0000000080000000, true},
		{0x0000000080000001, 0x0000000080000000, true},
		{0x00000000fffffffe, 0x0000000080000000, true},
		{0x00000000ffffffff, 0x0000000080000000, true},
		{0x0000000100000000, 0x0000000080000000, true},
		{0x0000000200000000, 0x0000000080000000, true},
		{0x7ffffffffffffffe, 0x0000000080000000, true},
		{0x7fffffffffffffff, 0x0000000080000000, true},
		{0x8000000000000000, 0x0000000080000000, true},
		{0x8000000000000001, 0x0000000080000000, true},
		{0xfffffffffffffffe, 0x0000000080000000, true},
		{0xffffffffffffffff, 0x0000000080000000, true},

		{0x0000000000000000, 0x0000000080000001, false},
		{0x0000000000000001, 0x0000000080000001, false},
		{0x0000000000000002, 0x0000000080000001, false},
		{0x000000007ffffffe, 0x0000000080000001, false},
		{0x000000007fffffff, 0x0000000080000001, false},
		{0x0000000080000000, 0x0000000080000001, false},
		{0x0000000080000001, 0x0000000080000001, true},
		{0x00000000fffffffe, 0x0000000080000001, true},
		{0x00000000ffffffff, 0x0000000080000001, true},
		{0x0000000100000000, 0x0000000080000001, true},
		{0x0000000200000000, 0x0000000080000001, true},
		{0x7ffffffffffffffe, 0x0000000080000001, true},
		{0x7fffffffffffffff, 0x0000000080000001, true},
		{0x8000000000000000, 0x0000000080000001, true},
		{0x8000000000000001, 0x0000000080000001, true},
		{0xfffffffffffffffe, 0x0000000080000001, true},
		{0xffffffffffffffff, 0x0000000080000001, true},

		{0x0000000000000000, 0x00000000fffffffe, false},
		{0x0000000000000001, 0x00000000fffffffe, false},
		{0x0000000000000002, 0x00000000fffffffe, false},
		{0x000000007ffffffe, 0x00000000fffffffe, false},
		{0x000000007fffffff, 0x00000000fffffffe, false},
		{0x0000000080000000, 0x00000000fffffffe, false},
		{0x0000000080000001, 0x00000000fffffffe, false},
		{0x00000000fffffffe, 0x00000000fffffffe, true},
		{0x00000000ffffffff, 0x00000000fffffffe, true},
		{0x0000000100000000, 0x00000000fffffffe, true},
		{0x0000000200000000, 0x00000000fffffffe, true},
		{0x7ffffffffffffffe, 0x00000000fffffffe, true},
		{0x7fffffffffffffff, 0x00000000fffffffe, true},
		{0x8000000000000000, 0x00000000fffffffe, true},
		{0x8000000000000001, 0x00000000fffffffe, true},
		{0xfffffffffffffffe, 0x00000000fffffffe, true},
		{0xffffffffffffffff, 0x00000000fffffffe, true},

		{0x0000000000000000, 0x00000000ffffffff, false},
		{0x0000000000000001, 0x00000000ffffffff, false},
		{0x0000000000000002, 0x00000000ffffffff, false},
		{0x000000007ffffffe, 0x00000000ffffffff, false},
		{0x000000007fffffff, 0x00000000ffffffff, false},
		{0x0000000080000000, 0x00000000ffffffff, false},
		{0x0000000080000001, 0x00000000ffffffff, false},
		{0x00000000fffffffe, 0x00000000ffffffff, false},
		{0x00000000ffffffff, 0x00000000ffffffff, true},
		{0x0000000100000000, 0x00000000ffffffff, true},
		{0x0000000200000000, 0x00000000ffffffff, true},
		{0x7ffffffffffffffe, 0x00000000ffffffff, true},
		{0x7fffffffffffffff, 0x00000000ffffffff, true},
		{0x8000000000000000, 0x00000000ffffffff, true},
		{0x8000000000000001, 0x00000000ffffffff, true},
		{0xfffffffffffffffe, 0x00000000ffffffff, true},
		{0xffffffffffffffff, 0x00000000ffffffff, true},

		{0x0000000000000000, 0x0000000100000000, false},
		{0x0000000000000001, 0x0000000100000000, false},
		{0x0000000000000002, 0x0000000100000000, false},
		{0x000000007ffffffe, 0x0000000100000000, false},
		{0x000000007fffffff, 0x0000000100000000, false},
		{0x0000000080000000, 0x0000000100000000, false},
		{0x0000000080000001, 0x0000000100000000, false},
		{0x00000000fffffffe, 0x0000000100000000, false},
		{0x00000000ffffffff, 0x0000000100000000, false},
		{0x0000000100000000, 0x0000000100000000, true},
		{0x0000000200000000, 0x0000000100000000, true},
		{0x7ffffffffffffffe, 0x0000000100000000, true},
		{0x7fffffffffffffff, 0x0000000100000000, true},
		{0x8000000000000000, 0x0000000100000000, true},
		{0x8000000000000001, 0x0000000100000000, true},
		{0xfffffffffffffffe, 0x0000000100000000, true},
		{0xffffffffffffffff, 0x0000000100000000, true},

		{0x0000000000000000, 0x0000000200000000, false},
		{0x0000000000000001, 0x0000000200000000, false},
		{0x0000000000000002, 0x0000000200000000, false},
		{0x000000007ffffffe, 0x0000000200000000, false},
		{0x000000007fffffff, 0x0000000200000000, false},
		{0x0000000080000000, 0x0000000200000000, false},
		{0x0000000080000001, 0x0000000200000000, false},
		{0x00000000fffffffe, 0x0000000200000000, false},
		{0x00000000ffffffff, 0x0000000200000000, false},
		{0x0000000100000000, 0x0000000200000000, false},
		{0x0000000200000000, 0x0000000200000000, true},
		{0x7ffffffffffffffe, 0x0000000200000000, true},
		{0x7fffffffffffffff, 0x0000000200000000, true},
		{0x8000000000000000, 0x0000000200000000, true},
		{0x8000000000000001, 0x0000000200000000, true},
		{0xfffffffffffffffe, 0x0000000200000000, true},
		{0xffffffffffffffff, 0x0000000200000000, true},

		{0x0000000000000000, 0x7ffffffffffffffe, false},
		{0x0000000000000001, 0x7ffffffffffffffe, false},
		{0x0000000000000002, 0x7ffffffffffffffe, false},
		{0x000000007ffffffe, 0x7ffffffffffffffe, false},
		{0x000000007fffffff, 0x7ffffffffffffffe, false},
		{0x0000000080000000, 0x7ffffffffffffffe, false},
		{0x0000000080000001, 0x7ffffffffffffffe, false},
		{0x00000000fffffffe, 0x7ffffffffffffffe, false},
		{0x00000000ffffffff, 0x7ffffffffffffffe, false},
		{0x0000000100000000, 0x7ffffffffffffffe, false},
		{0x0000000200000000, 0x7ffffffffffffffe, false},
		{0x7ffffffffffffffe, 0x7ffffffffffffffe, true},
		{0x7fffffffffffffff, 0x7ffffffffffffffe, true},
		{0x8000000000000000, 0x7ffffffffffffffe, true},
		{0x8000000000000001, 0x7ffffffffffffffe, true},
		{0xfffffffffffffffe, 0x7ffffffffffffffe, true},
		{0xffffffffffffffff, 0x7ffffffffffffffe, true},

		{0x0000000000000000, 0x7fffffffffffffff, false},
		{0x0000000000000001, 0x7fffffffffffffff, false},
		{0x0000000000000002, 0x7fffffffffffffff, false},
		{0x000000007ffffffe, 0x7fffffffffffffff, false},
		{0x000000007fffffff, 0x7fffffffffffffff, false},
		{0x0000000080000000, 0x7fffffffffffffff, false},
		{0x0000000080000001, 0x7fffffffffffffff, false},
		{0x00000000fffffffe, 0x7fffffffffffffff, false},
		{0x00000000ffffffff, 0x7fffffffffffffff, false},
		{0x0000000100000000, 0x7fffffffffffffff, false},
		{0x0000000200000000, 0x7fffffffffffffff, false},
		{0x7ffffffffffffffe, 0x7fffffffffffffff, false},
		{0x7fffffffffffffff, 0x7fffffffffffffff, true},
		{0x8000000000000000, 0x7fffffffffffffff, true},
		{0x8000000000000001, 0x7fffffffffffffff, true},
		{0xfffffffffffffffe, 0x7fffffffffffffff, true},
		{0xffffffffffffffff, 0x7fffffffffffffff, true},

		{0x0000000000000000, 0x8000000000000000, false},
		{0x0000000000000001, 0x8000000000000000, false},
		{0x0000000000000002, 0x8000000000000000, false},
		{0x000000007ffffffe, 0x8000000000000000, false},
		{0x000000007fffffff, 0x8000000000000000, false},
		{0x0000000080000000, 0x8000000000000000, false},
		{0x0000000080000001, 0x8000000000000000, false},
		{0x00000000fffffffe, 0x8000000000000000, false},
		{0x00000000ffffffff, 0x8000000000000000, false},
		{0x0000000100000000, 0x8000000000000000, false},
		{0x0000000200000000, 0x8000000000000000, false},
		{0x7ffffffffffffffe, 0x8000000000000000, false},
		{0x7fffffffffffffff, 0x8000000000000000, false},
		{0x8000000000000000, 0x8000000000000000, true},
		{0x8000000000000001, 0x8000000000000000, true},
		{0xfffffffffffffffe, 0x8000000000000000, true},
		{0xffffffffffffffff, 0x8000000000000000, true},

		{0x0000000000000000, 0x8000000000000001, false},
		{0x0000000000000001, 0x8000000000000001, false},
		{0x0000000000000002, 0x8000000000000001, false},
		{0x000000007ffffffe, 0x8000000000000001, false},
		{0x000000007fffffff, 0x8000000000000001, false},
		{0x0000000080000000, 0x8000000000000001, false},
		{0x0000000080000001, 0x8000000000000001, false},
		{0x00000000fffffffe, 0x8000000000000001, false},
		{0x00000000ffffffff, 0x8000000000000001, false},
		{0x0000000100000000, 0x8000000000000001, false},
		{0x0000000200000000, 0x8000000000000001, false},
		{0x7ffffffffffffffe, 0x8000000000000001, false},
		{0x7fffffffffffffff, 0x8000000000000001, false},
		{0x8000000000000000, 0x8000000000000001, false},
		{0x8000000000000001, 0x8000000000000001, true},
		{0xfffffffffffffffe, 0x8000000000000001, true},
		{0xffffffffffffffff, 0x8000000000000001, true},

		{0x0000000000000000, 0xfffffffffffffffe, false},
		{0x0000000000000001, 0xfffffffffffffffe, false},
		{0x0000000000000002, 0xfffffffffffffffe, false},
		{0x000000007ffffffe, 0xfffffffffffffffe, false},
		{0x000000007fffffff, 0xfffffffffffffffe, false},
		{0x0000000080000000, 0xfffffffffffffffe, false},
		{0x0000000080000001, 0xfffffffffffffffe, false},
		{0x00000000fffffffe, 0xfffffffffffffffe, false},
		{0x00000000ffffffff, 0xfffffffffffffffe, false},
		{0x0000000100000000, 0xfffffffffffffffe, false},
		{0x0000000200000000, 0xfffffffffffffffe, false},
		{0x7ffffffffffffffe, 0xfffffffffffffffe, false},
		{0x7fffffffffffffff, 0xfffffffffffffffe, false},
		{0x8000000000000000, 0xfffffffffffffffe, false},
		{0x8000000000000001, 0xfffffffffffffffe, false},
		{0xfffffffffffffffe, 0xfffffffffffffffe, true},
		{0xffffffffffffffff, 0xfffffffffffffffe, true},

		{0x0000000000000000, 0xffffffffffffffff, false},
		{0x0000000000000001, 0xffffffffffffffff, false},
		{0x0000000000000002, 0xffffffffffffffff, false},
		{0x000000007ffffffe, 0xffffffffffffffff, false},
		{0x000000007fffffff, 0xffffffffffffffff, false},
		{0x0000000080000000, 0xffffffffffffffff, false},
		{0x0000000080000001, 0xffffffffffffffff, false},
		{0x00000000fffffffe, 0xffffffffffffffff, false},
		{0x00000000ffffffff, 0xffffffffffffffff, false},
		{0x0000000100000000, 0xffffffffffffffff, false},
		{0x0000000200000000, 0xffffffffffffffff, false},
		{0x7ffffffffffffffe, 0xffffffffffffffff, false},
		{0x7fffffffffffffff, 0xffffffffffffffff, false},
		{0x8000000000000000, 0xffffffffffffffff, false},
		{0x8000000000000001, 0xffffffffffffffff, false},
		{0xfffffffffffffffe, 0xffffffffffffffff, false},
		{0xffffffffffffffff, 0xffffffffffffffff, true},
	}

	for _, test := range tests {
		f := func() {
			test.a.Minus(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestMinusUInt128(t *testing.T) {

	t.Parallel()

	// NOTE: hex values are integer values, not bit patterns!

	tests := []struct {
		a, b  UInt128Value
		valid bool
	}{
		{uint128("0x00000000000000000000000000000000"), uint128("0x00000000000000000000000000000000"), true},
		{uint128("0x00000000000000000000000000000001"), uint128("0x00000000000000000000000000000000"), true},
		{uint128("0x00000000000000000000000000000002"), uint128("0x00000000000000000000000000000000"), true},
		{uint128("0x7ffffffffffffffffffffffffffffffe"), uint128("0x00000000000000000000000000000000"), true},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0x00000000000000000000000000000000"), true},
		{uint128("0x80000000000000000000000000000000"), uint128("0x00000000000000000000000000000000"), true},
		{uint128("0x80000000000000000000000000000001"), uint128("0x00000000000000000000000000000000"), true},
		{uint128("0xfffffffffffffffffffffffffffffffe"), uint128("0x00000000000000000000000000000000"), true},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0x00000000000000000000000000000000"), true},

		{uint128("0x00000000000000000000000000000000"), uint128("0x00000000000000000000000000000001"), false},
		{uint128("0x00000000000000000000000000000001"), uint128("0x00000000000000000000000000000001"), true},
		{uint128("0x00000000000000000000000000000002"), uint128("0x00000000000000000000000000000001"), true},
		{uint128("0x7ffffffffffffffffffffffffffffffe"), uint128("0x00000000000000000000000000000001"), true},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0x00000000000000000000000000000001"), true},
		{uint128("0x80000000000000000000000000000000"), uint128("0x00000000000000000000000000000001"), true},
		{uint128("0x80000000000000000000000000000001"), uint128("0x00000000000000000000000000000001"), true},
		{uint128("0xfffffffffffffffffffffffffffffffe"), uint128("0x00000000000000000000000000000001"), true},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0x00000000000000000000000000000001"), true},

		{uint128("0x00000000000000000000000000000000"), uint128("0x00000000000000000000000000000002"), false},
		{uint128("0x00000000000000000000000000000001"), uint128("0x00000000000000000000000000000002"), false},
		{uint128("0x00000000000000000000000000000002"), uint128("0x00000000000000000000000000000002"), true},
		{uint128("0x7ffffffffffffffffffffffffffffffe"), uint128("0x00000000000000000000000000000002"), true},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0x00000000000000000000000000000002"), true},
		{uint128("0x80000000000000000000000000000000"), uint128("0x00000000000000000000000000000002"), true},
		{uint128("0x80000000000000000000000000000001"), uint128("0x00000000000000000000000000000002"), true},
		{uint128("0xfffffffffffffffffffffffffffffffe"), uint128("0x00000000000000000000000000000002"), true},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0x00000000000000000000000000000002"), true},

		{uint128("0x00000000000000000000000000000000"), uint128("0x7ffffffffffffffffffffffffffffffe"), false},
		{uint128("0x00000000000000000000000000000001"), uint128("0x7ffffffffffffffffffffffffffffffe"), false},
		{uint128("0x00000000000000000000000000000002"), uint128("0x7ffffffffffffffffffffffffffffffe"), false},
		{uint128("0x7ffffffffffffffffffffffffffffffe"), uint128("0x7ffffffffffffffffffffffffffffffe"), true},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0x7ffffffffffffffffffffffffffffffe"), true},
		{uint128("0x80000000000000000000000000000000"), uint128("0x7ffffffffffffffffffffffffffffffe"), true},
		{uint128("0x80000000000000000000000000000001"), uint128("0x7ffffffffffffffffffffffffffffffe"), true},
		{uint128("0xfffffffffffffffffffffffffffffffe"), uint128("0x7ffffffffffffffffffffffffffffffe"), true},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0x7ffffffffffffffffffffffffffffffe"), true},

		{uint128("0x00000000000000000000000000000000"), uint128("0x7fffffffffffffffffffffffffffffff"), false},
		{uint128("0x00000000000000000000000000000001"), uint128("0x7fffffffffffffffffffffffffffffff"), false},
		{uint128("0x00000000000000000000000000000002"), uint128("0x7fffffffffffffffffffffffffffffff"), false},
		{uint128("0x7ffffffffffffffffffffffffffffffe"), uint128("0x7fffffffffffffffffffffffffffffff"), false},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0x7fffffffffffffffffffffffffffffff"), true},
		{uint128("0x80000000000000000000000000000000"), uint128("0x7fffffffffffffffffffffffffffffff"), true},
		{uint128("0x80000000000000000000000000000001"), uint128("0x7fffffffffffffffffffffffffffffff"), true},
		{uint128("0xfffffffffffffffffffffffffffffffe"), uint128("0x7fffffffffffffffffffffffffffffff"), true},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0x7fffffffffffffffffffffffffffffff"), true},

		{uint128("0x00000000000000000000000000000000"), uint128("0x80000000000000000000000000000000"), false},
		{uint128("0x00000000000000000000000000000001"), uint128("0x80000000000000000000000000000000"), false},
		{uint128("0x00000000000000000000000000000002"), uint128("0x80000000000000000000000000000000"), false},
		{uint128("0x7ffffffffffffffffffffffffffffffe"), uint128("0x80000000000000000000000000000000"), false},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0x80000000000000000000000000000000"), false},
		{uint128("0x80000000000000000000000000000000"), uint128("0x80000000000000000000000000000000"), true},
		{uint128("0x80000000000000000000000000000001"), uint128("0x80000000000000000000000000000000"), true},
		{uint128("0xfffffffffffffffffffffffffffffffe"), uint128("0x80000000000000000000000000000000"), true},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0x80000000000000000000000000000000"), true},

		{uint128("0x00000000000000000000000000000000"), uint128("0x80000000000000000000000000000001"), false},
		{uint128("0x00000000000000000000000000000001"), uint128("0x80000000000000000000000000000001"), false},
		{uint128("0x00000000000000000000000000000002"), uint128("0x80000000000000000000000000000001"), false},
		{uint128("0x7ffffffffffffffffffffffffffffffe"), uint128("0x80000000000000000000000000000001"), false},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0x80000000000000000000000000000001"), false},
		{uint128("0x80000000000000000000000000000000"), uint128("0x80000000000000000000000000000001"), false},
		{uint128("0x80000000000000000000000000000001"), uint128("0x80000000000000000000000000000001"), true},
		{uint128("0xfffffffffffffffffffffffffffffffe"), uint128("0x80000000000000000000000000000001"), true},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0x80000000000000000000000000000001"), true},

		{uint128("0x00000000000000000000000000000000"), uint128("0xfffffffffffffffffffffffffffffffe"), false},
		{uint128("0x00000000000000000000000000000001"), uint128("0xfffffffffffffffffffffffffffffffe"), false},
		{uint128("0x00000000000000000000000000000002"), uint128("0xfffffffffffffffffffffffffffffffe"), false},
		{uint128("0x7ffffffffffffffffffffffffffffffe"), uint128("0xfffffffffffffffffffffffffffffffe"), false},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0xfffffffffffffffffffffffffffffffe"), false},
		{uint128("0x80000000000000000000000000000000"), uint128("0xfffffffffffffffffffffffffffffffe"), false},
		{uint128("0x80000000000000000000000000000001"), uint128("0xfffffffffffffffffffffffffffffffe"), false},
		{uint128("0xfffffffffffffffffffffffffffffffe"), uint128("0xfffffffffffffffffffffffffffffffe"), true},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0xfffffffffffffffffffffffffffffffe"), true},

		{uint128("0x00000000000000000000000000000000"), uint128("0xffffffffffffffffffffffffffffffff"), false},
		{uint128("0x00000000000000000000000000000001"), uint128("0xffffffffffffffffffffffffffffffff"), false},
		{uint128("0x00000000000000000000000000000002"), uint128("0xffffffffffffffffffffffffffffffff"), false},
		{uint128("0x7ffffffffffffffffffffffffffffffe"), uint128("0xffffffffffffffffffffffffffffffff"), false},
		{uint128("0x7fffffffffffffffffffffffffffffff"), uint128("0xffffffffffffffffffffffffffffffff"), false},
		{uint128("0x80000000000000000000000000000000"), uint128("0xffffffffffffffffffffffffffffffff"), false},
		{uint128("0x80000000000000000000000000000001"), uint128("0xffffffffffffffffffffffffffffffff"), false},
		{uint128("0xfffffffffffffffffffffffffffffffe"), uint128("0xffffffffffffffffffffffffffffffff"), false},
		{uint128("0xffffffffffffffffffffffffffffffff"), uint128("0xffffffffffffffffffffffffffffffff"), true},
	}

	for _, test := range tests {
		f := func() {
			test.a.Minus(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestMinusUInt256(t *testing.T) {

	t.Parallel()

	// NOTE: hex values are integer values, not bit patterns!

	tests := []struct {
		a, b  UInt256Value
		valid bool
	}{
		// 0x0000000000000000000000000000000000000000000000000000000000000000

		{
			uint256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			uint256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			uint256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			uint256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000001"),
			uint256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			uint256("0xfffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			uint256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},

		// 0x0000000000000000000000000000000000000000000000000000000000000001

		{
			uint256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			false,
		},
		{
			uint256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			uint256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			true,
		},
		{
			uint256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			uint256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			true,
		},
		{
			uint256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			uint256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			true,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			true,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			true,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000001"),
			uint256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			true,
		},
		{
			uint256("0xfffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			uint256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			true,
		},

		// 0x0000000000000000000000000000000000000000000000000000000000000002

		{
			uint256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			false,
		},
		{
			uint256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			uint256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			false,
		},
		{
			uint256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			uint256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			uint256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			uint256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000001"),
			uint256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			uint256("0xfffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			uint256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			true,
		},

		// 0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe

		{
			uint256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			false,
		},
		{
			uint256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			uint256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			false,
		},
		{
			uint256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			uint256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			false,
		},
		{
			uint256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			uint256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			true,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			true,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			true,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000001"),
			uint256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			true,
		},
		{
			uint256("0xfffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			uint256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			true,
		},

		// 0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff

		{
			uint256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			uint256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			uint256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			uint256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
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
			uint256("0x8000000000000000000000000000000000000000000000000000000000000001"),
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
		{
			uint256("0xfffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
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
			uint256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			uint256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			uint256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			false,
		},
		{
			uint256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
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
			true,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000001"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			uint256("0xfffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			true,
		},

		// 0x8000000000000000000000000000000000000000000000000000000000000001

		{
			uint256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000001"),
			false,
		},
		{
			uint256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000001"),
			false,
		},
		{
			uint256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000001"),
			false,
		},
		{
			uint256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000001"),
			false,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000001"),
			false,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000001"),
			false,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000001"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000001"),
			true,
		},
		{
			uint256("0xfffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000001"),
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0x8000000000000000000000000000000000000000000000000000000000000001"),
			true,
		},

		// 0xfffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe

		{
			uint256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0xfffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			false,
		},
		{
			uint256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			uint256("0xfffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			false,
		},
		{
			uint256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			uint256("0xfffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			false,
		},
		{
			uint256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			uint256("0xfffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			false,
		},
		{
			uint256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0xfffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			false,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0xfffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			false,
		},
		{
			uint256("0x8000000000000000000000000000000000000000000000000000000000000001"),
			uint256("0xfffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			false,
		},
		{
			uint256("0xfffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			uint256("0xfffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			true,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0xfffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			true,
		},

		// 0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff

		{
			uint256("0x0000000000000000000000000000000000000000000000000000000000000000"),
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			uint256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			uint256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			uint256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
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
			uint256("0x8000000000000000000000000000000000000000000000000000000000000001"),
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			uint256("0xfffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			uint256("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			true,
		},
	}

	for _, test := range tests {
		f := func() {
			test.a.Minus(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestMinusInt8(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  Int8Value
		valid bool
	}{
		{0x00, 0x00, true},
		{0x01, 0x00, true},
		{0x02, 0x00, true},
		{0x7e, 0x00, true},
		{0x7f, 0x00, true},
		{-128, 0x00, true},
		{-127, 0x00, true},
		{-2, 0x00, true},
		{-1, 0x00, true},

		{0x00, 0x01, true},
		{0x01, 0x01, true},
		{0x02, 0x01, true},
		{0x7e, 0x01, true},
		{0x7f, 0x01, true},
		{-128, 0x01, false},
		{-127, 0x01, true},
		{-2, 0x01, true},
		{-1, 0x01, true},

		{0x00, 0x02, true},
		{0x01, 0x02, true},
		{0x02, 0x02, true},
		{0x7e, 0x02, true},
		{0x7f, 0x02, true},
		{-128, 0x02, false},
		{-127, 0x02, false},
		{-2, 0x02, true},
		{-1, 0x02, true},

		{0x00, 0x7e, true},
		{0x01, 0x7e, true},
		{0x02, 0x7e, true},
		{0x7e, 0x7e, true},
		{0x7f, 0x7e, true},
		{-128, 0x7e, false},
		{-127, 0x7e, false},
		{-2, 0x7e, true},
		{-1, 0x7e, true},

		{0x00, 0x7f, true},
		{0x01, 0x7f, true},
		{0x02, 0x7f, true},
		{0x7e, 0x7f, true},
		{0x7f, 0x7f, true},
		{-128, 0x7f, false},
		{-127, 0x7f, false},
		{-2, 0x7f, false},
		{-1, 0x7f, true},

		{0x00, -128, false},
		{0x01, -128, false},
		{0x02, -128, false},
		{0x7e, -128, false},
		{0x7f, -128, false},
		{-128, -128, true},
		{-127, -128, true},
		{-2, -128, true},
		{-1, -128, true},

		{0x00, -127, true},
		{0x01, -127, false},
		{0x02, -127, false},
		{0x7e, -127, false},
		{0x7f, -127, false},
		{-128, -127, true},
		{-127, -127, true},
		{-2, -127, true},
		{-1, -127, true},

		{0x00, -2, true},
		{0x01, -2, true},
		{0x02, -2, true},
		{0x7e, -2, false},
		{0x7f, -2, false},
		{-128, -2, true},
		{-127, -2, true},
		{-2, -2, true},
		{-1, -2, true},

		{0x00, -1, true},
		{0x01, -1, true},
		{0x02, -1, true},
		{0x7e, -1, true},
		{0x7f, -1, false},
		{-128, -1, true},
		{-127, -1, true},
		{-2, -1, true},
		{-1, -1, true},
	}

	for _, test := range tests {
		f := func() {
			test.a.Minus(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}

	}
}

func TestMinusInt16(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  Int16Value
		valid bool
	}{
		{0x0000, 0x0000, true},
		{0x0001, 0x0000, true},
		{0x0002, 0x0000, true},
		{0x7ffe, 0x0000, true},
		{0x7fff, 0x0000, true},
		{-32768, 0x0000, true},
		{-32767, 0x0000, true},
		{-2, 0x0000, true},
		{-1, 0x0000, true},

		{0x0000, 0x0001, true},
		{0x0001, 0x0001, true},
		{0x0002, 0x0001, true},
		{0x7ffe, 0x0001, true},
		{0x7fff, 0x0001, true},
		{-32768, 0x0001, false},
		{-32767, 0x0001, true},
		{-2, 0x0001, true},
		{-1, 0x0001, true},

		{0x0000, 0x0002, true},
		{0x0001, 0x0002, true},
		{0x0002, 0x0002, true},
		{0x7ffe, 0x0002, true},
		{0x7fff, 0x0002, true},
		{-32768, 0x0002, false},
		{-32767, 0x0002, false},
		{-2, 0x0002, true},
		{-1, 0x0002, true},

		{0x0000, 0x7ffe, true},
		{0x0001, 0x7ffe, true},
		{0x0002, 0x7ffe, true},
		{0x7ffe, 0x7ffe, true},
		{0x7fff, 0x7ffe, true},
		{-32768, 0x7ffe, false},
		{-32767, 0x7ffe, false},
		{-2, 0x7ffe, true},
		{-1, 0x7ffe, true},

		{0x0000, 0x7fff, true},
		{0x0001, 0x7fff, true},
		{0x0002, 0x7fff, true},
		{0x7ffe, 0x7fff, true},
		{0x7fff, 0x7fff, true},
		{-32768, 0x7fff, false},
		{-32767, 0x7fff, false},
		{-2, 0x7fff, false},
		{-1, 0x7fff, true},

		{0x0000, -32768, false},
		{0x0001, -32768, false},
		{0x0002, -32768, false},
		{0x7ffe, -32768, false},
		{0x7fff, -32768, false},
		{-32768, -32768, true},
		{-32767, -32768, true},
		{-2, -32768, true},
		{-1, -32768, true},

		{0x0000, -32767, true},
		{0x0001, -32767, false},
		{0x0002, -32767, false},
		{0x7ffe, -32767, false},
		{0x7fff, -32767, false},
		{-32768, -32767, true},
		{-32767, -32767, true},
		{-2, -32767, true},
		{-1, -32767, true},

		{0x0000, -2, true},
		{0x0001, -2, true},
		{0x0002, -2, true},
		{0x7ffe, -2, false},
		{0x7fff, -2, false},
		{-32768, -2, true},
		{-32767, -2, true},
		{-2, -2, true},
		{-1, -2, true},

		{0x0000, -1, true},
		{0x0001, -1, true},
		{0x0002, -1, true},
		{0x7ffe, -1, true},
		{0x7fff, -1, false},
		{-32768, -1, true},
		{-32767, -1, true},
		{-2, -1, true},
		{-1, -1, true},
	}

	for _, test := range tests {
		f := func() {
			test.a.Minus(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestMinusInt32(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  Int32Value
		valid bool
	}{
		{0x00000000, 0x00000000, true},
		{0x00000001, 0x00000000, true},
		{0x00000002, 0x00000000, true},
		{0x7ffffffe, 0x00000000, true},
		{0x7fffffff, 0x00000000, true},
		{-2147483648, 0x00000000, true},
		{-2147483647, 0x00000000, true},
		{-2, 0x00000000, true},
		{-1, 0x00000000, true},

		{0x00000000, 0x00000001, true},
		{0x00000001, 0x00000001, true},
		{0x00000002, 0x00000001, true},
		{0x7ffffffe, 0x00000001, true},
		{0x7fffffff, 0x00000001, true},
		{-2147483648, 0x00000001, false},
		{-2147483647, 0x00000001, true},
		{-2, 0x00000001, true},
		{-1, 0x00000001, true},

		{0x00000000, 0x00000002, true},
		{0x00000001, 0x00000002, true},
		{0x00000002, 0x00000002, true},
		{0x7ffffffe, 0x00000002, true},
		{0x7fffffff, 0x00000002, true},
		{-2147483648, 0x00000002, false},
		{-2147483647, 0x00000002, false},
		{-2, 0x00000002, true},
		{-1, 0x00000002, true},

		{0x00000000, 0x7ffffffe, true},
		{0x00000001, 0x7ffffffe, true},
		{0x00000002, 0x7ffffffe, true},
		{0x7ffffffe, 0x7ffffffe, true},
		{0x7fffffff, 0x7ffffffe, true},
		{-2147483648, 0x7ffffffe, false},
		{-2147483647, 0x7ffffffe, false},
		{-2, 0x7ffffffe, true},
		{-1, 0x7ffffffe, true},

		{0x00000000, 0x7fffffff, true},
		{0x00000001, 0x7fffffff, true},
		{0x00000002, 0x7fffffff, true},
		{0x7ffffffe, 0x7fffffff, true},
		{0x7fffffff, 0x7fffffff, true},
		{-2147483648, 0x7fffffff, false},
		{-2147483647, 0x7fffffff, false},
		{-2, 0x7fffffff, false},
		{-1, 0x7fffffff, true},

		{0x00000000, -2147483648, false},
		{0x00000001, -2147483648, false},
		{0x00000002, -2147483648, false},
		{0x7ffffffe, -2147483648, false},
		{0x7fffffff, -2147483648, false},
		{-2147483648, -2147483648, true},
		{-2147483647, -2147483648, true},
		{-2, -2147483648, true},
		{-1, -2147483648, true},

		{0x00000000, -2147483647, true},
		{0x00000001, -2147483647, false},
		{0x00000002, -2147483647, false},
		{0x7ffffffe, -2147483647, false},
		{0x7fffffff, -2147483647, false},
		{-2147483648, -2147483647, true},
		{-2147483647, -2147483647, true},
		{-2, -2147483647, true},
		{-1, -2147483647, true},

		{0x00000000, -2, true},
		{0x00000001, -2, true},
		{0x00000002, -2, true},
		{0x7ffffffe, -2, false},
		{0x7fffffff, -2, false},
		{-2147483648, -2, true},
		{-2147483647, -2, true},
		{-2, -2, true},
		{-1, -2, true},

		{0x00000000, -1, true},
		{0x00000001, -1, true},
		{0x00000002, -1, true},
		{0x7ffffffe, -1, true},
		{0x7fffffff, -1, false},
		{-2147483648, -1, true},
		{-2147483647, -1, true},
		{-2, -1, true},
		{-1, -1, true},
	}

	for _, test := range tests {
		f := func() {
			test.a.Minus(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestMinusInt64(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  Int64Value
		valid bool
	}{
		{0x0000000000000000, 0x0000000000000000, true},
		{0x0000000000000001, 0x0000000000000000, true},
		{0x0000000000000002, 0x0000000000000000, true},
		{0x000000007ffffffe, 0x0000000000000000, true},
		{0x000000007fffffff, 0x0000000000000000, true},
		{0x0000000080000000, 0x0000000000000000, true},
		{0x0000000080000001, 0x0000000000000000, true},
		{0x00000000fffffffe, 0x0000000000000000, true},
		{0x00000000ffffffff, 0x0000000000000000, true},
		{0x0000000100000000, 0x0000000000000000, true},
		{0x0000000200000000, 0x0000000000000000, true},
		{0x7ffffffffffffffe, 0x0000000000000000, true},
		{0x7fffffffffffffff, 0x0000000000000000, true},
		{-9223372036854775808, 0x0000000000000000, true},
		{-9223372036854775807, 0x0000000000000000, true},
		{-2, 0x0000000000000000, true},
		{-1, 0x0000000000000000, true},

		{0x0000000000000000, 0x0000000000000001, true},
		{0x0000000000000001, 0x0000000000000001, true},
		{0x0000000000000002, 0x0000000000000001, true},
		{0x000000007ffffffe, 0x0000000000000001, true},
		{0x000000007fffffff, 0x0000000000000001, true},
		{0x0000000080000000, 0x0000000000000001, true},
		{0x0000000080000001, 0x0000000000000001, true},
		{0x00000000fffffffe, 0x0000000000000001, true},
		{0x00000000ffffffff, 0x0000000000000001, true},
		{0x0000000100000000, 0x0000000000000001, true},
		{0x0000000200000000, 0x0000000000000001, true},
		{0x7ffffffffffffffe, 0x0000000000000001, true},
		{0x7fffffffffffffff, 0x0000000000000001, true},
		{-9223372036854775808, 0x0000000000000001, false},
		{-9223372036854775807, 0x0000000000000001, true},
		{-2, 0x0000000000000001, true},
		{-1, 0x0000000000000001, true},

		{0x0000000000000000, 0x0000000000000002, true},
		{0x0000000000000001, 0x0000000000000002, true},
		{0x0000000000000002, 0x0000000000000002, true},
		{0x000000007ffffffe, 0x0000000000000002, true},
		{0x000000007fffffff, 0x0000000000000002, true},
		{0x0000000080000000, 0x0000000000000002, true},
		{0x0000000080000001, 0x0000000000000002, true},
		{0x00000000fffffffe, 0x0000000000000002, true},
		{0x00000000ffffffff, 0x0000000000000002, true},
		{0x0000000100000000, 0x0000000000000002, true},
		{0x0000000200000000, 0x0000000000000002, true},
		{0x7ffffffffffffffe, 0x0000000000000002, true},
		{0x7fffffffffffffff, 0x0000000000000002, true},
		{-9223372036854775808, 0x0000000000000002, false},
		{-9223372036854775807, 0x0000000000000002, false},
		{-2, 0x0000000000000002, true},
		{-1, 0x0000000000000002, true},

		{0x0000000000000000, 0x000000007ffffffe, true},
		{0x0000000000000001, 0x000000007ffffffe, true},
		{0x0000000000000002, 0x000000007ffffffe, true},
		{0x000000007ffffffe, 0x000000007ffffffe, true},
		{0x000000007fffffff, 0x000000007ffffffe, true},
		{0x0000000080000000, 0x000000007ffffffe, true},
		{0x0000000080000001, 0x000000007ffffffe, true},
		{0x00000000fffffffe, 0x000000007ffffffe, true},
		{0x00000000ffffffff, 0x000000007ffffffe, true},
		{0x0000000100000000, 0x000000007ffffffe, true},
		{0x0000000200000000, 0x000000007ffffffe, true},
		{0x7ffffffffffffffe, 0x000000007ffffffe, true},
		{0x7fffffffffffffff, 0x000000007ffffffe, true},
		{-9223372036854775808, 0x000000007ffffffe, false},
		{-9223372036854775807, 0x000000007ffffffe, false},
		{-2, 0x000000007ffffffe, true},
		{-1, 0x000000007ffffffe, true},

		{0x0000000000000000, 0x000000007fffffff, true},
		{0x0000000000000001, 0x000000007fffffff, true},
		{0x0000000000000002, 0x000000007fffffff, true},
		{0x000000007ffffffe, 0x000000007fffffff, true},
		{0x000000007fffffff, 0x000000007fffffff, true},
		{0x0000000080000000, 0x000000007fffffff, true},
		{0x0000000080000001, 0x000000007fffffff, true},
		{0x00000000fffffffe, 0x000000007fffffff, true},
		{0x00000000ffffffff, 0x000000007fffffff, true},
		{0x0000000100000000, 0x000000007fffffff, true},
		{0x0000000200000000, 0x000000007fffffff, true},
		{0x7ffffffffffffffe, 0x000000007fffffff, true},
		{0x7fffffffffffffff, 0x000000007fffffff, true},
		{-9223372036854775808, 0x000000007fffffff, false},
		{-9223372036854775807, 0x000000007fffffff, false},
		{-2, 0x000000007fffffff, true},
		{-1, 0x000000007fffffff, true},

		{0x0000000000000000, 0x0000000080000000, true},
		{0x0000000000000001, 0x0000000080000000, true},
		{0x0000000000000002, 0x0000000080000000, true},
		{0x000000007ffffffe, 0x0000000080000000, true},
		{0x000000007fffffff, 0x0000000080000000, true},
		{0x0000000080000000, 0x0000000080000000, true},
		{0x0000000080000001, 0x0000000080000000, true},
		{0x00000000fffffffe, 0x0000000080000000, true},
		{0x00000000ffffffff, 0x0000000080000000, true},
		{0x0000000100000000, 0x0000000080000000, true},
		{0x0000000200000000, 0x0000000080000000, true},
		{0x7ffffffffffffffe, 0x0000000080000000, true},
		{0x7fffffffffffffff, 0x0000000080000000, true},
		{-9223372036854775808, 0x0000000080000000, false},
		{-9223372036854775807, 0x0000000080000000, false},
		{-2, 0x0000000080000000, true},
		{-1, 0x0000000080000000, true},

		{0x0000000000000000, 0x0000000080000001, true},
		{0x0000000000000001, 0x0000000080000001, true},
		{0x0000000000000002, 0x0000000080000001, true},
		{0x000000007ffffffe, 0x0000000080000001, true},
		{0x000000007fffffff, 0x0000000080000001, true},
		{0x0000000080000000, 0x0000000080000001, true},
		{0x0000000080000001, 0x0000000080000001, true},
		{0x00000000fffffffe, 0x0000000080000001, true},
		{0x00000000ffffffff, 0x0000000080000001, true},
		{0x0000000100000000, 0x0000000080000001, true},
		{0x0000000200000000, 0x0000000080000001, true},
		{0x7ffffffffffffffe, 0x0000000080000001, true},
		{0x7fffffffffffffff, 0x0000000080000001, true},
		{-9223372036854775808, 0x0000000080000001, false},
		{-9223372036854775807, 0x0000000080000001, false},
		{-2, 0x0000000080000001, true},
		{-1, 0x0000000080000001, true},

		{0x0000000000000000, 0x00000000fffffffe, true},
		{0x0000000000000001, 0x00000000fffffffe, true},
		{0x0000000000000002, 0x00000000fffffffe, true},
		{0x000000007ffffffe, 0x00000000fffffffe, true},
		{0x000000007fffffff, 0x00000000fffffffe, true},
		{0x0000000080000000, 0x00000000fffffffe, true},
		{0x0000000080000001, 0x00000000fffffffe, true},
		{0x00000000fffffffe, 0x00000000fffffffe, true},
		{0x00000000ffffffff, 0x00000000fffffffe, true},
		{0x0000000100000000, 0x00000000fffffffe, true},
		{0x0000000200000000, 0x00000000fffffffe, true},
		{0x7ffffffffffffffe, 0x00000000fffffffe, true},
		{0x7fffffffffffffff, 0x00000000fffffffe, true},
		{-9223372036854775808, 0x00000000fffffffe, false},
		{-9223372036854775807, 0x00000000fffffffe, false},
		{-2, 0x00000000fffffffe, true},
		{-1, 0x00000000fffffffe, true},

		{0x0000000000000000, 0x00000000ffffffff, true},
		{0x0000000000000001, 0x00000000ffffffff, true},
		{0x0000000000000002, 0x00000000ffffffff, true},
		{0x000000007ffffffe, 0x00000000ffffffff, true},
		{0x000000007fffffff, 0x00000000ffffffff, true},
		{0x0000000080000000, 0x00000000ffffffff, true},
		{0x0000000080000001, 0x00000000ffffffff, true},
		{0x00000000fffffffe, 0x00000000ffffffff, true},
		{0x00000000ffffffff, 0x00000000ffffffff, true},
		{0x0000000100000000, 0x00000000ffffffff, true},
		{0x0000000200000000, 0x00000000ffffffff, true},
		{0x7ffffffffffffffe, 0x00000000ffffffff, true},
		{0x7fffffffffffffff, 0x00000000ffffffff, true},
		{-9223372036854775808, 0x00000000ffffffff, false},
		{-9223372036854775807, 0x00000000ffffffff, false},
		{-2, 0x00000000ffffffff, true},
		{-1, 0x00000000ffffffff, true},

		{0x0000000000000000, 0x0000000100000000, true},
		{0x0000000000000001, 0x0000000100000000, true},
		{0x0000000000000002, 0x0000000100000000, true},
		{0x000000007ffffffe, 0x0000000100000000, true},
		{0x000000007fffffff, 0x0000000100000000, true},
		{0x0000000080000000, 0x0000000100000000, true},
		{0x0000000080000001, 0x0000000100000000, true},
		{0x00000000fffffffe, 0x0000000100000000, true},
		{0x00000000ffffffff, 0x0000000100000000, true},
		{0x0000000100000000, 0x0000000100000000, true},
		{0x0000000200000000, 0x0000000100000000, true},
		{0x7ffffffffffffffe, 0x0000000100000000, true},
		{0x7fffffffffffffff, 0x0000000100000000, true},
		{-9223372036854775808, 0x0000000100000000, false},
		{-9223372036854775807, 0x0000000100000000, false},
		{-2, 0x0000000100000000, true},
		{-1, 0x0000000100000000, true},

		{0x0000000000000000, 0x0000000200000000, true},
		{0x0000000000000001, 0x0000000200000000, true},
		{0x0000000000000002, 0x0000000200000000, true},
		{0x000000007ffffffe, 0x0000000200000000, true},
		{0x000000007fffffff, 0x0000000200000000, true},
		{0x0000000080000000, 0x0000000200000000, true},
		{0x0000000080000001, 0x0000000200000000, true},
		{0x00000000fffffffe, 0x0000000200000000, true},
		{0x00000000ffffffff, 0x0000000200000000, true},
		{0x0000000100000000, 0x0000000200000000, true},
		{0x0000000200000000, 0x0000000200000000, true},
		{0x7ffffffffffffffe, 0x0000000200000000, true},
		{0x7fffffffffffffff, 0x0000000200000000, true},
		{-9223372036854775808, 0x0000000200000000, false},
		{-9223372036854775807, 0x0000000200000000, false},
		{-2, 0x0000000200000000, true},
		{-1, 0x0000000200000000, true},

		{0x0000000000000000, 0x7ffffffffffffffe, true},
		{0x0000000000000001, 0x7ffffffffffffffe, true},
		{0x0000000000000002, 0x7ffffffffffffffe, true},
		{0x000000007ffffffe, 0x7ffffffffffffffe, true},
		{0x000000007fffffff, 0x7ffffffffffffffe, true},
		{0x0000000080000000, 0x7ffffffffffffffe, true},
		{0x0000000080000001, 0x7ffffffffffffffe, true},
		{0x00000000fffffffe, 0x7ffffffffffffffe, true},
		{0x00000000ffffffff, 0x7ffffffffffffffe, true},
		{0x0000000100000000, 0x7ffffffffffffffe, true},
		{0x0000000200000000, 0x7ffffffffffffffe, true},
		{0x7ffffffffffffffe, 0x7ffffffffffffffe, true},
		{0x7fffffffffffffff, 0x7ffffffffffffffe, true},
		{-9223372036854775808, 0x7ffffffffffffffe, false},
		{-9223372036854775807, 0x7ffffffffffffffe, false},
		{-2, 0x7ffffffffffffffe, true},
		{-1, 0x7ffffffffffffffe, true},

		{0x0000000000000000, 0x7fffffffffffffff, true},
		{0x0000000000000001, 0x7fffffffffffffff, true},
		{0x0000000000000002, 0x7fffffffffffffff, true},
		{0x000000007ffffffe, 0x7fffffffffffffff, true},
		{0x000000007fffffff, 0x7fffffffffffffff, true},
		{0x0000000080000000, 0x7fffffffffffffff, true},
		{0x0000000080000001, 0x7fffffffffffffff, true},
		{0x00000000fffffffe, 0x7fffffffffffffff, true},
		{0x00000000ffffffff, 0x7fffffffffffffff, true},
		{0x0000000100000000, 0x7fffffffffffffff, true},
		{0x0000000200000000, 0x7fffffffffffffff, true},
		{0x7ffffffffffffffe, 0x7fffffffffffffff, true},
		{0x7fffffffffffffff, 0x7fffffffffffffff, true},
		{-9223372036854775808, 0x7fffffffffffffff, false},
		{-9223372036854775807, 0x7fffffffffffffff, false},
		{-2, 0x7fffffffffffffff, false},
		{-1, 0x7fffffffffffffff, true},

		{0x0000000000000000, -9223372036854775808, false},
		{0x0000000000000001, -9223372036854775808, false},
		{0x0000000000000002, -9223372036854775808, false},
		{0x000000007ffffffe, -9223372036854775808, false},
		{0x000000007fffffff, -9223372036854775808, false},
		{0x0000000080000000, -9223372036854775808, false},
		{0x0000000080000001, -9223372036854775808, false},
		{0x00000000fffffffe, -9223372036854775808, false},
		{0x00000000ffffffff, -9223372036854775808, false},
		{0x0000000100000000, -9223372036854775808, false},
		{0x0000000200000000, -9223372036854775808, false},
		{0x7ffffffffffffffe, -9223372036854775808, false},
		{0x7fffffffffffffff, -9223372036854775808, false},
		{-9223372036854775808, -9223372036854775808, true},
		{-9223372036854775807, -9223372036854775808, true},
		{-2, -9223372036854775808, true},
		{-1, -9223372036854775808, true},

		{0x0000000000000000, -9223372036854775807, true},
		{0x0000000000000001, -9223372036854775807, false},
		{0x0000000000000002, -9223372036854775807, false},
		{0x000000007ffffffe, -9223372036854775807, false},
		{0x000000007fffffff, -9223372036854775807, false},
		{0x0000000080000000, -9223372036854775807, false},
		{0x0000000080000001, -9223372036854775807, false},
		{0x00000000fffffffe, -9223372036854775807, false},
		{0x00000000ffffffff, -9223372036854775807, false},
		{0x0000000100000000, -9223372036854775807, false},
		{0x0000000200000000, -9223372036854775807, false},
		{0x7ffffffffffffffe, -9223372036854775807, false},
		{0x7fffffffffffffff, -9223372036854775807, false},
		{-9223372036854775808, -9223372036854775807, true},
		{-9223372036854775807, -9223372036854775807, true},
		{-2, -9223372036854775807, true},
		{-1, -9223372036854775807, true},

		{0x0000000000000000, -2, true},
		{0x0000000000000001, -2, true},
		{0x0000000000000002, -2, true},
		{0x000000007ffffffe, -2, true},
		{0x000000007fffffff, -2, true},
		{0x0000000080000000, -2, true},
		{0x0000000080000001, -2, true},
		{0x00000000fffffffe, -2, true},
		{0x00000000ffffffff, -2, true},
		{0x0000000100000000, -2, true},
		{0x0000000200000000, -2, true},
		{0x7ffffffffffffffe, -2, false},
		{0x7fffffffffffffff, -2, false},
		{-9223372036854775808, -2, true},
		{-9223372036854775807, -2, true},
		{-2, -2, true},
		{-1, -2, true},

		{0x0000000000000000, -1, true},
		{0x0000000000000001, -1, true},
		{0x0000000000000002, -1, true},
		{0x000000007ffffffe, -1, true},
		{0x000000007fffffff, -1, true},
		{0x0000000080000000, -1, true},
		{0x0000000080000001, -1, true},
		{0x00000000fffffffe, -1, true},
		{0x00000000ffffffff, -1, true},
		{0x0000000100000000, -1, true},
		{0x0000000200000000, -1, true},
		{0x7ffffffffffffffe, -1, true},
		{0x7fffffffffffffff, -1, false},
		{-9223372036854775808, -1, true},
		{-9223372036854775807, -1, true},
		{-2, -1, true},
		{-1, -1, true},
	}

	for _, test := range tests {
		f := func() {
			test.a.Minus(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestMinusInt128(t *testing.T) {

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
		{int128("-0x80000000000000000000000000000000"), int128("0x00000000000000000000000000000001"), false},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("0x00000000000000000000000000000001"), true},
		{int128("-0x00000000000000000000000000000002"), int128("0x00000000000000000000000000000001"), true},
		{int128("-0x00000000000000000000000000000001"), int128("0x00000000000000000000000000000001"), true},

		{int128("0x00000000000000000000000000000000"), int128("0x00000000000000000000000000000002"), true},
		{int128("0x00000000000000000000000000000001"), int128("0x00000000000000000000000000000002"), true},
		{int128("0x00000000000000000000000000000002"), int128("0x00000000000000000000000000000002"), true},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("0x00000000000000000000000000000002"), true},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("0x00000000000000000000000000000002"), true},
		{int128("-0x80000000000000000000000000000000"), int128("0x00000000000000000000000000000002"), false},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("0x00000000000000000000000000000002"), false},
		{int128("-0x00000000000000000000000000000002"), int128("0x00000000000000000000000000000002"), true},
		{int128("-0x00000000000000000000000000000001"), int128("0x00000000000000000000000000000002"), true},

		{int128("0x00000000000000000000000000000000"), int128("0x7ffffffffffffffffffffffffffffffe"), true},
		{int128("0x00000000000000000000000000000001"), int128("0x7ffffffffffffffffffffffffffffffe"), true},
		{int128("0x00000000000000000000000000000002"), int128("0x7ffffffffffffffffffffffffffffffe"), true},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("0x7ffffffffffffffffffffffffffffffe"), true},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("0x7ffffffffffffffffffffffffffffffe"), true},
		{int128("-0x80000000000000000000000000000000"), int128("0x7ffffffffffffffffffffffffffffffe"), false},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("0x7ffffffffffffffffffffffffffffffe"), false},
		{int128("-0x00000000000000000000000000000002"), int128("0x7ffffffffffffffffffffffffffffffe"), true},
		{int128("-0x00000000000000000000000000000001"), int128("0x7ffffffffffffffffffffffffffffffe"), true},

		{int128("0x00000000000000000000000000000000"), int128("0x7fffffffffffffffffffffffffffffff"), true},
		{int128("0x00000000000000000000000000000001"), int128("0x7fffffffffffffffffffffffffffffff"), true},
		{int128("0x00000000000000000000000000000002"), int128("0x7fffffffffffffffffffffffffffffff"), true},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("0x7fffffffffffffffffffffffffffffff"), true},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("0x7fffffffffffffffffffffffffffffff"), true},
		{int128("-0x80000000000000000000000000000000"), int128("0x7fffffffffffffffffffffffffffffff"), false},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("0x7fffffffffffffffffffffffffffffff"), false},
		{int128("-0x00000000000000000000000000000002"), int128("0x7fffffffffffffffffffffffffffffff"), false},
		{int128("-0x00000000000000000000000000000001"), int128("0x7fffffffffffffffffffffffffffffff"), true},

		{int128("0x00000000000000000000000000000000"), int128("-0x80000000000000000000000000000000"), false},
		{int128("0x00000000000000000000000000000001"), int128("-0x80000000000000000000000000000000"), false},
		{int128("0x00000000000000000000000000000002"), int128("-0x80000000000000000000000000000000"), false},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("-0x80000000000000000000000000000000"), false},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("-0x80000000000000000000000000000000"), false},
		{int128("-0x80000000000000000000000000000000"), int128("-0x80000000000000000000000000000000"), true},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("-0x80000000000000000000000000000000"), true},
		{int128("-0x00000000000000000000000000000002"), int128("-0x80000000000000000000000000000000"), true},
		{int128("-0x00000000000000000000000000000001"), int128("-0x80000000000000000000000000000000"), true},

		{int128("0x00000000000000000000000000000000"), int128("-0x7fffffffffffffffffffffffffffffff"), true},
		{int128("0x00000000000000000000000000000001"), int128("-0x7fffffffffffffffffffffffffffffff"), false},
		{int128("0x00000000000000000000000000000002"), int128("-0x7fffffffffffffffffffffffffffffff"), false},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("-0x7fffffffffffffffffffffffffffffff"), false},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("-0x7fffffffffffffffffffffffffffffff"), false},
		{int128("-0x80000000000000000000000000000000"), int128("-0x7fffffffffffffffffffffffffffffff"), true},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("-0x7fffffffffffffffffffffffffffffff"), true},
		{int128("-0x00000000000000000000000000000002"), int128("-0x7fffffffffffffffffffffffffffffff"), true},
		{int128("-0x00000000000000000000000000000001"), int128("-0x7fffffffffffffffffffffffffffffff"), true},

		{int128("0x00000000000000000000000000000000"), int128("-0x00000000000000000000000000000002"), true},
		{int128("0x00000000000000000000000000000001"), int128("-0x00000000000000000000000000000002"), true},
		{int128("0x00000000000000000000000000000002"), int128("-0x00000000000000000000000000000002"), true},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("-0x00000000000000000000000000000002"), false},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("-0x00000000000000000000000000000002"), false},
		{int128("-0x80000000000000000000000000000000"), int128("-0x00000000000000000000000000000002"), true},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("-0x00000000000000000000000000000002"), true},
		{int128("-0x00000000000000000000000000000002"), int128("-0x00000000000000000000000000000002"), true},
		{int128("-0x00000000000000000000000000000001"), int128("-0x00000000000000000000000000000002"), true},

		{int128("0x00000000000000000000000000000000"), int128("-0x00000000000000000000000000000001"), true},
		{int128("0x00000000000000000000000000000001"), int128("-0x00000000000000000000000000000001"), true},
		{int128("0x00000000000000000000000000000002"), int128("-0x00000000000000000000000000000001"), true},
		{int128("0x7ffffffffffffffffffffffffffffffe"), int128("-0x00000000000000000000000000000001"), true},
		{int128("0x7fffffffffffffffffffffffffffffff"), int128("-0x00000000000000000000000000000001"), false},
		{int128("-0x80000000000000000000000000000000"), int128("-0x00000000000000000000000000000001"), true},
		{int128("-0x7fffffffffffffffffffffffffffffff"), int128("-0x00000000000000000000000000000001"), true},
		{int128("-0x00000000000000000000000000000002"), int128("-0x00000000000000000000000000000001"), true},
		{int128("-0x00000000000000000000000000000001"), int128("-0x00000000000000000000000000000001"), true},
	}

	for _, test := range tests {
		f := func() {
			test.a.Minus(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestMinusInt256(t *testing.T) {

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
			false,
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
			false,
		},
		{
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			false,
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
			false,
		},
		{
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			false,
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
			false,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000001"),
			int256("-0x8000000000000000000000000000000000000000000000000000000000000000"),
			false,
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
			false,
		},
		{
			int256("0x0000000000000000000000000000000000000000000000000000000000000002"),
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			int256("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe"),
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
		},
		{
			int256("0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			int256("-0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			false,
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
			false,
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
			test.a.Minus(test.b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}

func TestMinusUInt(t *testing.T) {

	t.Parallel()

	tests := []struct {
		a, b  uint64
		valid bool
	}{
		{0x00, 0x00, true},
		{0x01, 0x00, true},
		{0x02, 0x00, true},
		{0x7e, 0x00, true},
		{0x7f, 0x00, true},
		{0x80, 0x00, true},
		{0x81, 0x00, true},
		{0xfe, 0x00, true},
		{0xff, 0x00, true},

		{0x00, 0x01, false},
		{0x01, 0x01, true},
		{0x02, 0x01, true},
		{0x7e, 0x01, true},
		{0x7f, 0x01, true},
		{0x80, 0x01, true},
		{0x81, 0x01, true},
		{0xfe, 0x01, true},
		{0xff, 0x01, true},

		{0x00, 0x02, false},
		{0x01, 0x02, false},
		{0x02, 0x02, true},
		{0x7e, 0x02, true},
		{0x7f, 0x02, true},
		{0x80, 0x02, true},
		{0x81, 0x02, true},
		{0xfe, 0x02, true},
		{0xff, 0x02, true},

		{0x00, 0x7e, false},
		{0x01, 0x7e, false},
		{0x02, 0x7e, false},
		{0x7e, 0x7e, true},
		{0x7f, 0x7e, true},
		{0x80, 0x7e, true},
		{0x81, 0x7e, true},
		{0xfe, 0x7e, true},
		{0xff, 0x7e, true},

		{0x00, 0x7f, false},
		{0x01, 0x7f, false},
		{0x02, 0x7f, false},
		{0x7e, 0x7f, false},
		{0x7f, 0x7f, true},
		{0x80, 0x7f, true},
		{0x81, 0x7f, true},
		{0xfe, 0x7f, true},
		{0xff, 0x7f, true},

		{0x00, 0x80, false},
		{0x01, 0x80, false},
		{0x02, 0x80, false},
		{0x7e, 0x80, false},
		{0x7f, 0x80, false},
		{0x80, 0x80, true},
		{0x81, 0x80, true},
		{0xfe, 0x80, true},
		{0xff, 0x80, true},

		{0x00, 0x81, false},
		{0x01, 0x81, false},
		{0x02, 0x81, false},
		{0x7e, 0x81, false},
		{0x7f, 0x81, false},
		{0x80, 0x81, false},
		{0x81, 0x81, true},
		{0xfe, 0x81, true},
		{0xff, 0x81, true},

		{0x00, 0xfe, false},
		{0x01, 0xfe, false},
		{0x02, 0xfe, false},
		{0x7e, 0xfe, false},
		{0x7f, 0xfe, false},
		{0x80, 0xfe, false},
		{0x81, 0xfe, false},
		{0xfe, 0xfe, true},
		{0xff, 0xfe, true},

		{0x00, 0xff, false},
		{0x01, 0xff, false},
		{0x02, 0xff, false},
		{0x7e, 0xff, false},
		{0x7f, 0xff, false},
		{0x80, 0xff, false},
		{0x81, 0xff, false},
		{0xfe, 0xff, false},
		{0xff, 0xff, true},
	}

	for _, test := range tests {
		f := func() {
			a := NewUIntValueFromUint64(test.a)
			b := NewUIntValueFromUint64(test.b)
			a.Minus(b)
		}
		if test.valid {
			assert.NotPanics(t, f)
		} else {
			assert.Panics(t, f)
		}
	}
}
