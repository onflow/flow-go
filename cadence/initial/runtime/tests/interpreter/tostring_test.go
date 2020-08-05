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

func TestInterpretToString(t *testing.T) {

	for _, ty := range sema.AllIntegerTypes {

		t.Run(ty.String(), func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
				fmt.Sprintf(
					`
                      let x: %s = 42
                      let y = x.toString()
                    `,
					ty,
				),
			)

			assert.Equal(t,
				interpreter.NewStringValue("42"),
				inter.Globals["y"].Value,
			)
		})
	}

	t.Run("Address", func(t *testing.T) {

		inter := parseCheckAndInterpret(t,
			`
              let x: Address = 0x42
              let y = x.toString()
            `,
		)

		assert.Equal(t,
			interpreter.NewStringValue("0x42"),
			inter.Globals["y"].Value,
		)
	})

	for _, ty := range sema.AllFixedPointTypes {

		t.Run(ty.String(), func(t *testing.T) {

			inter := parseCheckAndInterpret(t,
				fmt.Sprintf(
					`
                      let x: %s = 12.34
                      let y = x.toString()
                    `,
					ty,
				),
			)

			assert.Equal(t,
				interpreter.NewStringValue("12.34000000"),
				inter.Globals["y"].Value,
			)
		})
	}
}
