package checker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckInvalidContractAccountField(t *testing.T) {

	_, err := ParseAndCheck(t, `
      contract Test {
          let account: Account

          init(account: Account) {
              self.account = account
          }
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidDeclarationError{}, errs[0])
}

func TestCheckInvalidContractAccountFunction(t *testing.T) {

	_, err := ParseAndCheck(t, `
      contract Test {
          fun account() {}
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidDeclarationError{}, errs[0])
}

func TestCheckContractAccountFieldUse(t *testing.T) {

	_, err := ParseAndCheck(t, `
      contract Test {

          init() {
              self.account.address
          }
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidContractAccountFieldInitialization(t *testing.T) {

	_, err := ParseAndCheck(t, `
      contract Test {

          init(account: Account) {
              self.account = account
          }
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.AssignmentToConstantMemberError{}, errs[0])
}

func TestCheckInvalidContractAccountFieldAccess(t *testing.T) {

	_, err := ParseAndCheck(t, `
      contract Test {}

      let test = Test.account
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidAccessError{}, errs[0])
}

func TestCheckContractAccountFieldUseInitialized(t *testing.T) {

	code := `
      contract Test {
          let address: Address

          init() {
              // field 'account' can be used, as it is considered initialized
              self.address = self.account.address
          }

          fun test(): Address {
              return self.account.address
          }
      }

      let address1 = Test.address
      let address2 = Test.test()
    `
	_, err := ParseAndCheck(t, code)

	require.NoError(t, err)
}

func TestCheckInvalidContractMoveToFunction(t *testing.T) {

	for _, name := range []string{"self", "C"} {

		t.Run(name, func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      contract C {

                          fun test() {
                              use(%s)
                          }
                      }

                      fun use(_ c: C) {}
                    `,
					name,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidMoveError{}, errs[0])
		})
	}
}

func TestCheckInvalidContractMoveInVariableDeclaration(t *testing.T) {

	for _, name := range []string{"self", "C"} {

		t.Run(name, func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      contract C {

                          fun test() {
                              let x = %s
                          }
                      }
                    `,
					name,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidMoveError{}, errs[0])
		})
	}
}

func TestCheckInvalidContractMoveReturnFromFunction(t *testing.T) {

	for _, name := range []string{"self", "C"} {

		t.Run(name, func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      contract C {

                          fun test(): C {
                              return %s
                          }
                      }
                    `,
					name,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidMoveError{}, errs[0])
		})
	}
}

func TestCheckInvalidContractMoveIntoArrayLiteral(t *testing.T) {

	for _, name := range []string{"self", "C"} {

		t.Run(name, func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      contract C {

                          fun test() {
                              let txs = [%s]
                          }
                      }
                    `,
					name,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidMoveError{}, errs[0])
		})
	}
}

func TestCheckInvalidContractMoveIntoDictionaryLiteral(t *testing.T) {

	for _, name := range []string{"self", "C"} {

		t.Run(name, func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      contract C {

                          fun test() {
                              let txs = {"C": %s}
                          }
                      }
                    `,
					name,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidMoveError{}, errs[0])
		})
	}
}

func TestCheckContractNestedDeclarationOrderOutsideInside(t *testing.T) {

	for _, isInterface := range []bool{true, false} {

		interfaceKeyword := ""
		if isInterface {
			interfaceKeyword = "interface"
		}

		body := ""
		if !isInterface {
			body = "{}"
		}

		t.Run(interfaceKeyword, func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      contract C {

                          fun callGo(r: @R) {
                              r.go()
                              destroy r
                          }

                          resource %[1]s R {
                              fun go() %[2]s
                          }
                      }
                    `,
					interfaceKeyword,
					body,
				),
			)

			require.NoError(t, err)
		})
	}
}

func TestCheckContractNestedDeclarationOrderInsideOutside(t *testing.T) {

	_, err := ParseAndCheck(t, `
      contract C {

          fun go() {}

          resource R {
              fun callGo() {
                  C.go()
              }
          }
      }
    `)

	require.NoError(t, err)
}

func TestCheckMutualTypeUseInContract(t *testing.T) {

	interfacePossibilities := []bool{true, false}

	compositeKinds := []common.CompositeKind{
		common.CompositeKindStructure,
		common.CompositeKindResource,
	}

	for _, firstKind := range compositeKinds {
		for _, firstIsInterface := range interfacePossibilities {
			for _, secondKind := range compositeKinds {
				for _, secondIsInterface := range interfacePossibilities {

					firstInterfaceKeyword := ""
					if firstIsInterface {
						firstInterfaceKeyword = "interface"
					}

					secondInterfaceKeyword := ""
					if secondIsInterface {
						secondInterfaceKeyword = "interface"
					}

					testName := fmt.Sprintf(
						"%s_%s/%s_%s",
						firstKind.Keyword(),
						firstInterfaceKeyword,
						secondKind.Keyword(),
						secondInterfaceKeyword,
					)

					firstBody := ""
					if !firstIsInterface {
						firstBody = fmt.Sprintf(
							"{ %s b }",
							secondKind.DestructionKeyword(),
						)
					}

					secondBody := ""
					if !secondIsInterface {
						secondBody = fmt.Sprintf(
							"{ %s a }",
							firstKind.DestructionKeyword(),
						)
					}

					t.Run(testName, func(t *testing.T) {

						_, err := ParseAndCheck(t,
							fmt.Sprintf(
								`
                                  contract C {
                                      %[1]s %[2]s A {
                                          fun use(_ b: %[3]sB) %[4]s
                                      }

                                      %[5]s %[6]s B {
                                          fun use(_ a: %[7]sA) %[8]s
                                      }
                                  }
                                `,
								firstKind.Keyword(),
								firstInterfaceKeyword,
								secondKind.Annotation(),
								firstBody,
								secondKind.Keyword(),
								secondInterfaceKeyword,
								firstKind.Annotation(),
								secondBody,
							),
						)

						require.NoError(t, err)
					})
				}
			}
		}
	}
}

func TestCheckContractInterfaceNestedAndMutualTypeUses(t *testing.T) {

	_, err := ParseAndCheck(t, `
      contract interface CI {

          fun qualifiedBeforeA(_ a: @CI.A)
		  fun localBeforeA(_ a: @A)

          fun qualifiedBeforeB(_ b: @CI.B)
		  fun localBeforeB(_ b: @B)

          resource interface A {
              fun qualifiedB(_ b: @CI.B)
              fun localB(_ b: @B)

              fun qualifiedA(_ a: @CI.A)
              fun localA(_ a: @A)
          }

          resource interface B {
              fun qualifiedA(_ a: @CI.A)
              fun localA(_ a: @A)

              fun qualifiedB(_ b: @CI.B)
              fun localB(_ b: @B)
          }

          fun qualifiedAfterA(_ a: @CI.A)
		  fun localAfterA(_ a: @A)

          fun qualifiedAfterB(_ b: @CI.B)
		  fun localAfterB(_ b: @B)
      }
    `)

	require.NoError(t, err)
}

func TestCheckContractNestedAndMutualTypeUses(t *testing.T) {

	_, err := ParseAndCheck(t, `
      contract CI {

          fun qualifiedBeforeA(_ a: @CI.A) { destroy a }
		  fun localBeforeA(_ a: @A) { destroy a }

          fun qualifiedBeforeB(_ b: @CI.B) { destroy b }
		  fun localBeforeB(_ b: @B) { destroy b }

          resource interface A {
              fun qualifiedB(_ b: @CI.B)
              fun localB(_ b: @B)

              fun qualifiedA(_ a: @CI.A)
              fun localA(_ a: @A)
          }

          resource B {
              fun qualifiedA(_ a: @CI.A) { destroy a }
              fun localA(_ a: @A) { destroy a }

              fun qualifiedB(_ b: @CI.B) { destroy b }
              fun localB(_ b: @B) { destroy b }
          }

          fun qualifiedAfterA(_ a: @CI.A) { destroy a }
		  fun localAfterA(_ a: @A) { destroy a }

          fun qualifiedAfterB(_ b: @CI.B) { destroy b }
		  fun localAfterB(_ b: @B) { destroy b }
      }
    `)

	require.NoError(t, err)
}
