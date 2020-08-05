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

package checker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/sema"
)

func TestCheckInvalidEventTypeRequirementConformance(t *testing.T) {

	t.Parallel()

	_, err := ParseAndCheck(t, `
      pub contract interface CI {

          pub event E(a: Int)
      }

      pub contract C: CI {

          pub event E(b: String)
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	require.IsType(t, &sema.ConformanceError{}, errs[0])
}

func TestCheckTypeRequirementConformance(t *testing.T) {

	t.Parallel()

	type test struct {
		name            string
		preparationCode string
		interfaceCode   string
		conformanceCode string
		valid           bool
	}

	tests := []test{
		{
			"Both empty",
			``,
			`pub struct S {}`,
			`pub struct S {}`,
			true,
		},
		{
			"Conformance with additional function",
			``,
			`
              pub struct S {}
            `,
			`
              pub struct S {
                  fun foo() {}
              }
            `,
			true,
		},
		{
			"Conformance with missing function",
			``,
			`
              pub struct S {
                  fun foo()
              }
            `,
			`
              pub struct S {}
            `,
			false,
		},
		{
			"Conformance with same name, same parameter type, but different argument label",
			``,
			`
              pub struct S {
                  fun foo(x: Int)
              }
            `,
			`
              pub struct S {
                  fun foo(y: Int) {}
              }
            `,
			false,
		},
		{
			"Conformance with same name, same argument label, but different parameter type",
			``,
			`
              pub struct S {
                  fun foo(x: Int)
              }
            `,
			`
              pub struct S {
                  fun foo(x: String) {}
              }
            `,
			false,
		},
		{
			"Conformance with same name, same argument label, same parameter type, different parameter name",
			``,
			`
              pub struct S {
                  fun foo(x y: String)
              }
            `,
			`
              pub struct S {
                  fun foo(x z: String) {}
              }
            `,
			true,
		},
		{
			"Conformance with more specific parameter type",
			`
                pub struct interface I {}
                pub struct T: I {}
            `,
			`
              pub struct S {
                  fun foo(bar: {I})
              }
            `,
			`
              pub struct S {
                  fun foo(bar: T) {}
              }
            `,
			false,
		},
		{
			"Conformance with same nested parameter type",
			`
                pub contract X {
                    struct Bar {}
                }
            `,
			`
              pub struct S {
                  fun foo(bar: X.Bar)
              }
            `,
			`
              pub struct S {
                  fun foo(bar: X.Bar) {}
              }
            `,
			true,
		},
		{
			"Conformance with different nested parameter type",
			`
                pub contract X {
                    struct Bar {}
                }

                pub contract Y {
                    struct Bar {}
                }
            `,
			`
              pub struct S {
                  fun foo(bar: X.Bar)
              }
            `,
			`
              pub struct S {
                  fun foo(bar: Y.Bar) {}
              }
            `,
			false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      %s

                      pub contract interface CI {
                          %s
                      }

                      pub contract C: CI {
                          %s
                      }
                    `,
					test.preparationCode,
					test.interfaceCode,
					test.conformanceCode,
				),
			)

			if test.valid {
				require.NoError(t, err)
			} else {
				errs := ExpectCheckerErrors(t, err, 1)

				require.IsType(t, &sema.ConformanceError{}, errs[0])
			}
		})
	}
}
