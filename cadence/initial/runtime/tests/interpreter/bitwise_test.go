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

package interpreter_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
)

var bitwiseTestValueFunctions = map[string]func(int) interpreter.NumberValue{
	// Int*
	"Int": func(v int) interpreter.NumberValue {
		return interpreter.NewIntValueFromInt64(int64(v))
	},
	"Int8": func(v int) interpreter.NumberValue {
		return interpreter.Int8Value(v)
	},
	"Int16": func(v int) interpreter.NumberValue {
		return interpreter.Int16Value(v)
	},
	"Int32": func(v int) interpreter.NumberValue {
		return interpreter.Int32Value(v)
	},
	"Int64": func(v int) interpreter.NumberValue {
		return interpreter.Int64Value(v)
	},
	"Int128": func(v int) interpreter.NumberValue {
		return interpreter.NewInt128ValueFromInt64(int64(v))
	},
	"Int256": func(v int) interpreter.NumberValue {
		return interpreter.NewInt256ValueFromInt64(int64(v))
	},
	// UInt*
	"UInt": func(v int) interpreter.NumberValue {
		return interpreter.NewUIntValueFromUint64(uint64(v))
	},
	"UInt8": func(v int) interpreter.NumberValue {
		return interpreter.UInt8Value(v)
	},
	"UInt16": func(v int) interpreter.NumberValue {
		return interpreter.UInt16Value(v)
	},
	"UInt32": func(v int) interpreter.NumberValue {
		return interpreter.UInt32Value(v)
	},
	"UInt64": func(v int) interpreter.NumberValue {
		return interpreter.UInt64Value(v)
	},
	"UInt128": func(v int) interpreter.NumberValue {
		return interpreter.NewUInt128ValueFromUint64(uint64(v))
	},
	"UInt256": func(v int) interpreter.NumberValue {
		return interpreter.NewUInt256ValueFromUint64(uint64(v))
	},
	// Word*
	"Word8": func(v int) interpreter.NumberValue {
		return interpreter.Word8Value(v)
	},
	"Word16": func(v int) interpreter.NumberValue {
		return interpreter.Word16Value(v)
	},
	"Word32": func(v int) interpreter.NumberValue {
		return interpreter.Word32Value(v)
	},
	"Word64": func(v int) interpreter.NumberValue {
		return interpreter.Word64Value(v)
	},
}

func init() {

	for _, integerType := range sema.AllIntegerTypes {
		if _, ok := bitwiseTestValueFunctions[integerType.String()]; !ok {
			panic(fmt.Sprintf("broken test: missing %s", integerType))
		}
	}
}

func TestInterpretBitwiseOr(t *testing.T) {

	t.Parallel()

	for ty, valueFunc := range bitwiseTestValueFunctions {

		t.Run(ty, func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
				fmt.Sprintf(
					`
                      let a: %[1]s = 0b00010001
                      let b: %[1]s = 0b00000100
                      let c = a | b
                    `,
					ty,
				),
			)

			assert.Equal(t,
				valueFunc(0b00010101),
				inter.Globals["c"].Value,
			)
		})
	}
}

func TestInterpretBitwiseXor(t *testing.T) {

	t.Parallel()

	for ty, valueFunc := range bitwiseTestValueFunctions {

		t.Run(ty, func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
				fmt.Sprintf(
					`
                      let a: %[1]s = 0b00010001
                      let b: %[1]s = 0b00010100
                      let c = a ^ b
                    `,
					ty,
				),
			)

			assert.Equal(t,
				valueFunc(0b00000101),
				inter.Globals["c"].Value,
			)
		})
	}
}

func TestInterpretBitwiseAnd(t *testing.T) {

	t.Parallel()

	for ty, valueFunc := range bitwiseTestValueFunctions {

		t.Run(ty, func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
				fmt.Sprintf(
					`
                      let a: %[1]s = 0b00010001
                      let b: %[1]s = 0b00010100
                      let c = a & b
                    `,
					ty,
				),
			)

			assert.Equal(t,
				valueFunc(0b00010000),
				inter.Globals["c"].Value,
			)
		})
	}
}

func TestInterpretBitwiseLeftShift(t *testing.T) {

	t.Parallel()

	for ty, valueFunc := range bitwiseTestValueFunctions {

		t.Run(ty, func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
				fmt.Sprintf(
					`
                      let a: %[1]s = 0b00001100
                      let b: %[1]s = 3
                      let c = a << b
                    `,
					ty,
				),
			)

			assert.Equal(t,
				valueFunc(0b01100000),
				inter.Globals["c"].Value,
			)
		})
	}
}

func TestInterpretBitwiseRightShift(t *testing.T) {

	t.Parallel()

	for ty, valueFunc := range bitwiseTestValueFunctions {

		t.Run(ty, func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
				fmt.Sprintf(
					`
                      let a: %[1]s = 0b01100000
                      let b: %[1]s = 3
                      let c = a >> b
                    `,
					ty,
				),
			)

			assert.Equal(t,
				valueFunc(0b00001100),
				inter.Globals["c"].Value,
			)
		})
	}
}
