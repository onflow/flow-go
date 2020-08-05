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

var integerTestValues = map[string]interpreter.NumberValue{
	// Int*
	"Int":    interpreter.NewIntValueFromInt64(60),
	"Int8":   interpreter.Int8Value(60),
	"Int16":  interpreter.Int16Value(60),
	"Int32":  interpreter.Int32Value(60),
	"Int64":  interpreter.Int64Value(60),
	"Int128": interpreter.NewInt128ValueFromInt64(60),
	"Int256": interpreter.NewInt256ValueFromInt64(60),
	// UInt*
	"UInt":    interpreter.NewUIntValueFromUint64(60),
	"UInt8":   interpreter.UInt8Value(60),
	"UInt16":  interpreter.UInt16Value(60),
	"UInt32":  interpreter.UInt32Value(60),
	"UInt64":  interpreter.UInt64Value(60),
	"UInt128": interpreter.NewUInt128ValueFromUint64(60),
	"UInt256": interpreter.NewUInt256ValueFromUint64(60),
	// Word*
	"Word8":  interpreter.Word8Value(60),
	"Word16": interpreter.Word16Value(60),
	"Word32": interpreter.Word32Value(60),
	"Word64": interpreter.Word64Value(60),
}

func init() {

	for _, integerType := range sema.AllIntegerTypes {
		if _, ok := integerTestValues[integerType.String()]; !ok {
			panic(fmt.Sprintf("broken test: missing %s", integerType))
		}
	}
}

func TestInterpretPlusOperator(t *testing.T) {

	t.Parallel()

	for ty, value := range integerTestValues {

		t.Run(ty, func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
				fmt.Sprintf(
					`
                      let a: %[1]s = 20
                      let b: %[1]s = 40
                      let c = a + b
                    `,
					ty,
				),
			)

			assert.Equal(t,
				value,
				inter.Globals["c"].Value,
			)
		})
	}
}

func TestInterpretMinusOperator(t *testing.T) {

	t.Parallel()

	for ty, value := range integerTestValues {

		t.Run(ty, func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
				fmt.Sprintf(
					`
                      let a: %[1]s = 80
                      let b: %[1]s = 20
                      let c = a - b
                    `,
					ty,
				),
			)

			assert.Equal(t,
				value,
				inter.Globals["c"].Value,
			)
		})
	}
}

func TestInterpretMulOperator(t *testing.T) {

	t.Parallel()

	for ty, value := range integerTestValues {

		t.Run(ty, func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
				fmt.Sprintf(
					`
                      let a: %[1]s = 20
                      let b: %[1]s = 3
                      let c = a * b
                    `,
					ty,
				),
			)

			assert.Equal(t,
				value,
				inter.Globals["c"].Value,
			)
		})
	}
}

func TestInterpretDivOperator(t *testing.T) {

	t.Parallel()

	for ty, value := range integerTestValues {

		t.Run(ty, func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
				fmt.Sprintf(
					`
                      let a: %[1]s = 120
                      let b: %[1]s = 2
                      let c = a / b
                    `,
					ty,
				),
			)

			assert.Equal(t,
				value,
				inter.Globals["c"].Value,
			)
		})
	}
}

func TestInterpretModOperator(t *testing.T) {

	t.Parallel()

	for ty, value := range integerTestValues {

		t.Run(ty, func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
				fmt.Sprintf(
					`
                      let a: %[1]s = 126
                      let b: %[1]s = 66
                      let c = a %% b
                    `,
					ty,
				),
			)

			assert.Equal(t,
				value,
				inter.Globals["c"].Value,
			)
		})
	}
}
