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

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/runtime/stdlib"
)

func ParseAndCheckAccount(t *testing.T, code string) (*sema.Checker, error) {

	constantDeclaration := func(name string, ty sema.Type) stdlib.StandardLibraryValue {
		return stdlib.StandardLibraryValue{
			Name:       name,
			Type:       ty,
			Kind:       common.DeclarationKindConstant,
			IsConstant: true,
		}
	}

	return ParseAndCheckWithOptions(t,
		code,
		ParseAndCheckOptions{
			Options: []sema.Option{
				sema.WithPredeclaredValues(map[string]sema.ValueDeclaration{
					"authAccount":   constantDeclaration("authAccount", &sema.AuthAccountType{}),
					"publicAccount": constantDeclaration("publicAccount", &sema.PublicAccountType{}),
				}),
			},
		},
	)
}

func TestCheckAccount(t *testing.T) {

	t.Parallel()

	for _, domain := range common.AllPathDomainsByIdentifier {

		// NOTE: all domains are statically valid at the moment

		testName := fmt.Sprintf(
			"AuthAccount.save: implicit type argument, %s",
			domain.Name(),
		)
		t.Run(testName, func(t *testing.T) {

			t.Run("resource", func(t *testing.T) {

				_, err := ParseAndCheckAccount(t,
					fmt.Sprintf(
						`
                          resource R {}

                          fun test() {
                              let r <- create R()
                              authAccount.save(<-r, to: /%s/r)
                          }
                        `,
						domain.Identifier(),
					),
				)

				require.NoError(t, err)
			})

			t.Run("struct", func(t *testing.T) {

				_, err := ParseAndCheckAccount(t,
					fmt.Sprintf(
						`
                          struct S {}

                          fun test() {
                              let s = S()
                              authAccount.save(s, to: /%s/s)
                          }
                        `,
						domain.Identifier(),
					),
				)

				require.NoError(t, err)

			})

		})
	}

	for _, domain := range common.AllPathDomainsByIdentifier {

		// NOTE: all domains are statically valid at the moment

		testName := fmt.Sprintf(
			"AuthAccount.save: explicit type argument, %s",
			domain.Name(),
		)

		t.Run(testName, func(t *testing.T) {

			t.Run("resource", func(t *testing.T) {

				_, err := ParseAndCheckAccount(t,
					fmt.Sprintf(
						`
                      resource R {}

                      fun test() {
                          let r <- create R()
                          authAccount.save<@R>(<-r, to: /%s/r)
                      }
                    `,
						domain.Identifier(),
					),
				)

				require.NoError(t, err)
			})

			t.Run("struct", func(t *testing.T) {

				_, err := ParseAndCheckAccount(t,
					fmt.Sprintf(
						`
                          struct S {}

                          fun test() {
                              let s = S()
                              authAccount.save<S>(s, to: /%s/s)
                          }
                        `,
						domain.Identifier(),
					),
				)

				require.NoError(t, err)
			})
		})
	}

	for _, domain := range common.AllPathDomainsByIdentifier {

		// NOTE: all domains are statically valid at the moment

		testName := fmt.Sprintf(
			"AuthAccount.save: explicit type argument, incorrect, %s",
			domain.Name(),
		)
		t.Run(testName, func(t *testing.T) {

			t.Run("resource", func(t *testing.T) {

				_, err := ParseAndCheckAccount(t,
					fmt.Sprintf(
						`
                      resource R {}

                      resource T {}

                      fun test() {
                          let r <- create R()
                          authAccount.save<@T>(<-r, to: /%s/r)
                      }
                    `,
						domain.Identifier(),
					),
				)

				errs := ExpectCheckerErrors(t, err, 2)

				require.IsType(t, &sema.TypeParameterTypeMismatchError{}, errs[0])
				require.IsType(t, &sema.TypeMismatchError{}, errs[1])
			})

			t.Run("struct", func(t *testing.T) {

				_, err := ParseAndCheckAccount(t,
					fmt.Sprintf(
						`
                          struct S {}

                          struct T {}

                          fun test() {
                              let s = S()
                              authAccount.save<T>(s, to: /%s/s)
                          }
                        `,
						domain.Identifier(),
					),
				)

				errs := ExpectCheckerErrors(t, err, 2)

				require.IsType(t, &sema.TypeParameterTypeMismatchError{}, errs[0])
				require.IsType(t, &sema.TypeMismatchError{}, errs[1])
			})
		})
	}

	for _, domain := range common.AllPathDomainsByIdentifier {

		// NOTE: all domains are statically valid at the moment

		testName := fmt.Sprintf(
			"AuthAccount.load: missing type argument, %s",
			domain.Name(),
		)

		t.Run(testName, func(t *testing.T) {

			t.Run("resource", func(t *testing.T) {

				_, err := ParseAndCheckAccount(t,
					fmt.Sprintf(
						`
                      resource R {}

                      let r <- authAccount.load(from: /%s/r)
                    `,
						domain.Identifier(),
					),
				)

				errs := ExpectCheckerErrors(t, err, 1)

				require.IsType(t, &sema.TypeParameterTypeInferenceError{}, errs[0])
			})

			t.Run("struct", func(t *testing.T) {

				_, err := ParseAndCheckAccount(t,
					fmt.Sprintf(
						`
                          struct S {}

                          let s = authAccount.load(from: /%s/s)
                        `,
						domain.Identifier(),
					),
				)

				errs := ExpectCheckerErrors(t, err, 1)

				require.IsType(t, &sema.TypeParameterTypeInferenceError{}, errs[0])
			})
		})
	}

	for _, domain := range common.AllPathDomainsByIdentifier {

		// NOTE: all domains are statically valid at the moment

		testName := fmt.Sprintf(
			"AuthAccount.load: explicit type argument, %s",
			domain.Name(),
		)

		t.Run(testName, func(t *testing.T) {

			t.Run("resource", func(t *testing.T) {

				checker, err := ParseAndCheckAccount(t,
					fmt.Sprintf(
						`
                      resource R {}

                      let r <- authAccount.load<@R>(from: /%s/r)
                    `,
						domain.Identifier(),
					),
				)

				require.NoError(t, err)

				rType := checker.GlobalTypes["R"].Type

				rValueType := checker.GlobalValues["r"].Type

				require.Equal(t,
					&sema.OptionalType{
						Type: rType,
					},
					rValueType,
				)
			})

			t.Run("struct", func(t *testing.T) {

				checker, err := ParseAndCheckAccount(t,
					fmt.Sprintf(
						`
                          struct S {}

                          let s = authAccount.load<S>(from: /%s/s)
                        `,
						domain.Identifier(),
					),
				)

				require.NoError(t, err)

				sType := checker.GlobalTypes["S"].Type

				sValueType := checker.GlobalValues["s"].Type

				require.Equal(t,
					&sema.OptionalType{
						Type: sType,
					},
					sValueType,
				)
			})
		})
	}

	for _, domain := range common.AllPathDomainsByIdentifier {

		// NOTE: all domains are statically valid at the moment

		testName := fmt.Sprintf(
			"AuthAccount.copy: missing type argument, %s",
			domain.Name(),
		)

		t.Run(testName, func(t *testing.T) {

			_, err := ParseAndCheckAccount(t,
				fmt.Sprintf(
					`
                      struct S {}

                      let s = authAccount.copy(from: /%s/s)
                    `,
					domain.Identifier(),
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			require.IsType(t, &sema.TypeParameterTypeInferenceError{}, errs[0])
		})
	}

	for _, domain := range common.AllPathDomainsByIdentifier {

		// NOTE: all domains are statically valid at the moment

		testName := fmt.Sprintf(
			"AuthAccount.copy: explicit type argument, %s",
			domain.Name(),
		)

		t.Run(testName, func(t *testing.T) {

			t.Run("struct", func(t *testing.T) {

				checker, err := ParseAndCheckAccount(t,
					fmt.Sprintf(
						`
                          struct S {}

                          let s = authAccount.copy<S>(from: /%s/s)
                        `,
						domain.Identifier(),
					),
				)

				require.NoError(t, err)

				sType := checker.GlobalTypes["S"].Type

				sValueType := checker.GlobalValues["s"].Type

				require.Equal(t,
					&sema.OptionalType{
						Type: sType,
					},
					sValueType,
				)
			})

			t.Run("resource", func(t *testing.T) {

				_, err := ParseAndCheckAccount(t,
					fmt.Sprintf(
						`
                          resource R {}

                          let r <- authAccount.copy<@R>(from: /%s/r)
                        `,
						domain.Identifier(),
					),
				)

				errs := ExpectCheckerErrors(t, err, 1)

				require.IsType(t, &sema.TypeMismatchError{}, errs[0])
			})
		})
	}

	for _, domain := range common.AllPathDomainsByIdentifier {

		// NOTE: all domains are statically valid at the moment

		testName := fmt.Sprintf(
			"AuthAccount.borrow: missing type argument, %s",
			domain.Name(),
		)

		t.Run(testName, func(t *testing.T) {

			t.Run("resource", func(t *testing.T) {

				_, err := ParseAndCheckAccount(t,
					fmt.Sprintf(
						`
                          let r = authAccount.borrow(from: /%s/r)
                        `,
						domain.Identifier(),
					),
				)

				errs := ExpectCheckerErrors(t, err, 1)

				require.IsType(t, &sema.TypeParameterTypeInferenceError{}, errs[0])
			})

			t.Run("struct", func(t *testing.T) {

				_, err := ParseAndCheckAccount(t,
					fmt.Sprintf(
						`
                          let s = authAccount.borrow(from: /%s/s)
                        `,
						domain.Identifier(),
					),
				)

				errs := ExpectCheckerErrors(t, err, 1)

				require.IsType(t, &sema.TypeParameterTypeInferenceError{}, errs[0])
			})
		})
	}

	for _, domain := range common.AllPathDomainsByIdentifier {

		// NOTE: all domains are statically valid at the moment

		for _, auth := range []bool{false, true} {

			authKeyword := ""
			if auth {
				authKeyword = "auth"
			}

			testName := fmt.Sprintf(
				"AuthAccount.borrow: explicit type argument, %s reference, %s",
				authKeyword,
				domain.Name(),
			)

			t.Run(testName, func(t *testing.T) {

				t.Run("resource", func(t *testing.T) {

					checker, err := ParseAndCheckAccount(t,
						fmt.Sprintf(
							`
                              resource R {}

                              let r = authAccount.borrow<%s &R>(from: /%s/r)
                            `,
							authKeyword,
							domain.Identifier(),
						),
					)

					require.NoError(t, err)

					rType := checker.GlobalTypes["R"].Type

					rValueType := checker.GlobalValues["r"].Type

					require.Equal(t,
						&sema.OptionalType{
							Type: &sema.ReferenceType{
								Authorized: auth,
								Type:       rType,
							},
						},
						rValueType,
					)
				})

				t.Run("struct", func(t *testing.T) {

					checker, err := ParseAndCheckAccount(t,
						fmt.Sprintf(
							`
                              struct S {}

                              let s = authAccount.borrow<%s &S>(from: /%s/s)
                            `,
							authKeyword,
							domain.Identifier(),
						),
					)

					require.NoError(t, err)

					sType := checker.GlobalTypes["S"].Type

					sValueType := checker.GlobalValues["s"].Type

					require.Equal(t,
						&sema.OptionalType{
							Type: &sema.ReferenceType{
								Authorized: auth,
								Type:       sType,
							},
						},
						sValueType,
					)
				})

			})
		}
	}

	for _, domain := range common.AllPathDomainsByIdentifier {

		// NOTE: all domains are statically valid at the moment

		testName := fmt.Sprintf(
			"AuthAccount.borrow: explicit type argument, non-reference type, %s",
			domain.Name(),
		)

		t.Run(testName, func(t *testing.T) {

			t.Run("resource", func(t *testing.T) {

				_, err := ParseAndCheckAccount(t,
					fmt.Sprintf(
						`
                          resource R {}

                          let r <- authAccount.borrow<@R>(from: /%s/r)
                        `,
						domain.Identifier(),
					),
				)

				errs := ExpectCheckerErrors(t, err, 1)

				require.IsType(t, &sema.TypeMismatchError{}, errs[0])
			})

			t.Run("struct", func(t *testing.T) {

				_, err := ParseAndCheckAccount(t,
					fmt.Sprintf(
						`
                          struct S {}

                          let s = authAccount.borrow<S>(from: /%s/s)
                        `,
						domain.Identifier(),
					),
				)

				errs := ExpectCheckerErrors(t, err, 1)

				require.IsType(t, &sema.TypeMismatchError{}, errs[0])
			})
		})
	}

	for _, domain := range common.AllPathDomainsByIdentifier {

		testName := fmt.Sprintf(
			"AuthAccount.link: missing type argument, %s",
			domain.Name(),
		)

		t.Run(testName, func(t *testing.T) {

			t.Run("resource", func(t *testing.T) {

				_, err := ParseAndCheckAccount(t,
					fmt.Sprintf(
						`
                          resource R {}

                          fun test(): Capability? {
                              return authAccount.link(/%s/r, target: /storage/r)
                          }
                        `,
						domain.Identifier(),
					),
				)

				errs := ExpectCheckerErrors(t, err, 1)

				require.IsType(t, &sema.TypeParameterTypeInferenceError{}, errs[0])
			})

			t.Run("struct", func(t *testing.T) {

				_, err := ParseAndCheckAccount(t,
					fmt.Sprintf(
						`
                          struct S {}

                          fun test(): Capability? {
                              return authAccount.link(/%s/s, target: /storage/s)
                          }
                        `,
						domain.Identifier(),
					),
				)

				errs := ExpectCheckerErrors(t, err, 1)

				require.IsType(t, &sema.TypeParameterTypeInferenceError{}, errs[0])
			})
		})
	}

	for _, domain := range common.AllPathDomainsByIdentifier {

		for _, auth := range []bool{false, true} {

			authKeyword := ""
			if auth {
				authKeyword = "auth"
			}

			testName := fmt.Sprintf(
				"AuthAccount.link: explicit type argument, %s reference, %s",
				authKeyword,
				domain.Name(),
			)

			t.Run(testName, func(t *testing.T) {

				t.Run("resource", func(t *testing.T) {

					_, err := ParseAndCheckAccount(t,
						fmt.Sprintf(
							`
                              resource R {}

                              fun test(): Capability? {
                                  return authAccount.link<%s &R>(/%s/r, target: /storage/r)
                              }
                            `,
							authKeyword,
							domain.Identifier(),
						),
					)

					require.NoError(t, err)
				})

				t.Run("struct", func(t *testing.T) {

					_, err := ParseAndCheckAccount(t,
						fmt.Sprintf(
							`
                              struct S {}

                              fun test(): Capability? {
                                  return authAccount.link<%s &S>(/%s/s, target: /storage/s)
                              }
                            `,
							authKeyword,
							domain.Identifier(),
						),
					)

					require.NoError(t, err)
				})
			})
		}
	}

	for _, domain := range common.AllPathDomainsByIdentifier {

		// NOTE: storage domain is statically valid at the moment

		for _, targetDomain := range common.AllPathDomainsByIdentifier {

			// NOTE: all target domains are statically valid at the moment

			testName := fmt.Sprintf(
				"AuthAccount.link: explicit type argument, non-reference type, %s -> %s",
				domain.Name(),
				targetDomain.Name(),
			)

			t.Run(testName, func(t *testing.T) {

				t.Run("resource", func(t *testing.T) {

					_, err := ParseAndCheckAccount(t,
						fmt.Sprintf(
							`
                              resource R {}

                              fun test(): Capability? {
                                  return authAccount.link<@R>(/%s/r, target: /%s/r)
                              }
                            `,
							domain.Identifier(),
							targetDomain.Identifier(),
						),
					)

					errs := ExpectCheckerErrors(t, err, 1)

					require.IsType(t, &sema.TypeMismatchError{}, errs[0])
				})

				t.Run("struct", func(t *testing.T) {

					_, err := ParseAndCheckAccount(t,
						fmt.Sprintf(
							`
                              struct S {}

                              fun test(): Capability? {
                                  return authAccount.link<S>(/%s/s, target: /%s/s)
                              }
                            `,
							domain.Identifier(),
							targetDomain.Identifier(),
						),
					)

					errs := ExpectCheckerErrors(t, err, 1)

					require.IsType(t, &sema.TypeMismatchError{}, errs[0])
				})
			})
		}
	}

	for _, domain := range common.AllPathDomainsByIdentifier {

		// NOTE: storage domain is statically valid at the moment

		testName := fmt.Sprintf(
			"AuthAccount.unlink: %s",
			domain.Name(),
		)
		t.Run(testName, func(t *testing.T) {

			_, err := ParseAndCheckAccount(t,
				fmt.Sprintf(
					`
                      fun test() {
                          authAccount.unlink(/%s/r)
                      }
                    `,
					domain.Identifier(),
				),
			)

			require.NoError(t, err)
		})
	}

	for _, domain := range common.AllPathDomainsByIdentifier {

		// NOTE: storage domain is statically valid at the moment

		for accountType, accountVariable := range map[string]string{
			"AuthAccount":   "authAccount",
			"PublicAccount": "publicAccount",
		} {

			testName := fmt.Sprintf(
				"%s.getLinkTarget: %s",
				accountType,
				domain.Name(),
			)
			t.Run(testName, func(t *testing.T) {

				_, err := ParseAndCheckAccount(t,
					fmt.Sprintf(
						`
                          let path: Path? = %s.getLinkTarget(/%s/r)
                        `,
						accountVariable,
						domain.Identifier(),
					),
				)

				require.NoError(t, err)
			})
		}
	}

	for _, domain := range common.AllPathDomainsByIdentifier {

		for accountType, accountVariable := range map[string]string{
			"AuthAccount":   "authAccount",
			"PublicAccount": "publicAccount",
		} {

			testName := fmt.Sprintf(
				"%s.getCapability: %s",
				accountType,
				domain,
			)

			t.Run(testName, func(t *testing.T) {

				_, err := ParseAndCheckAccount(
					t,
					fmt.Sprintf(
						`
	                      fun test(): Capability? {
	                          return %[1]s.getCapability(/%[2]s/r)
	                      }
	                    `,
						accountVariable,
						domain.Identifier(),
					),
				)

				require.NoError(t, err)
			})
		}
	}
}
