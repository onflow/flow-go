package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckTransactions(t *testing.T) {

	type test struct {
		name   string
		code   string
		errors []error
	}

	tests := []test{
		{
			"Empty",
			`
              transaction {}
            `,
			nil,
		},
		{
			"No-op",
			`
              transaction {}
            `,
			nil,
		},
		{
			"Simple",
			`
              transaction {

                execute {
                   let x = 1 + 2
                }
              }
            `,
			nil,
		},
		{
			"InvalidPrepareIdentifier",
			`
              transaction {

                notPrepare() {}

                execute {}
              }
            `,
			[]error{
				&sema.InvalidTransactionBlockError{},
			},
		},
		{
			"InvalidExecuteIdentifier",
			`
              transaction {

                prepare() {}

                notExecute {}
              }
            `,
			[]error{
				&sema.InvalidTransactionBlockError{},
			},
		},
		{
			"ValidPrepareParameters",
			`
              transaction {

                  prepare(x: Account, y: Account) {}
              }
            `,
			nil,
		},
		{
			"InvalidPrepareParameters",
			`
              transaction {

                prepare(x: Int, y: Int) {}
              }
            `,
			[]error{
				&sema.InvalidTransactionPrepareParameterTypeError{},
				&sema.InvalidTransactionPrepareParameterTypeError{},
			},
		},
		{
			"InvalidFieldAccessSpecified",
			`
              transaction {

                  pub(set) var x: Int

                  prepare() {
                      self.x = 1
                  }

                  execute {}
              }
            `,
			[]error{
				&sema.InvalidTransactionFieldAccessModifierError{},
			},
		},
		{
			"InvalidFieldUninitialized",
			`
              transaction {

                var x: Int

                execute {
                    let y = self.x + 1
                }
              }
            `,
			[]error{
				&sema.TransactionMissingPrepareError{},
			},
		},
		{
			"FieldInitialized",
			`
              transaction {

                  var x: Int

                  prepare() {
                      self.x = 5
                  }

                  execute {
                      let y = self.x + 1
                  }
              }
            `,
			nil,
		},
		{
			"PreConditions",
			`
              transaction {

                  var x: Int
                  var y: Int

                  prepare() {
                      self.x = 5
                      self.y = 10
                  }

                  pre {
                      self.x > 2
                      self.y < 20
                  }

                  execute {
                      let z = self.x + self.y
                  }
              }
            `,
			nil,
		},
		{
			"InvalidPreConditionsWithUndeclaredFields",
			`
              transaction {

                  pre {
                      self.x > 2
                  }

                  execute {
                      let y = 1 + 1
                  }
                }
            `,
			[]error{
				&sema.NotDeclaredMemberError{},
			},
		},
		{
			"PostConditions",
			`
              transaction {

                  var x: Int

                  prepare() {
                      self.x = 5
                  }

                  execute {
                      self.x = 10
                  }

                  post {
                      self.x == 10
                  }
              }
            `,
			nil,
		},
		{
			"InvalidPostConditionsAccessExecuteScope",
			`
              transaction {

                  execute {
                      var x = 5
                  }

                  post {
                      x == 5
                  }
              }
            `,
			[]error{
				&sema.NotDeclaredError{},
			},
		},

		// TODO: prevent self from being used in function
		// {
		// 	"IllegalSelfUsage",
		// 	`
		//  	  fun foo(x: AnyStruct) {}
		//
		// 	  transaction {
		// 	    execute {
		// 		  foo(x: self)
		// 		}
		// 	  }
		// 	`,
		// 	[]error{
		// 		&sema.CheckerError{},
		// 	},
		// },

		{
			"ResourceField",
			`
              resource R {}

              transaction {

                var x: @R

                prepare() {
                    self.x <- create R()
                }

                execute {
                    destroy self.x
                }
              }
            `,
			nil,
		},
		{
			"InvalidResourceFieldLoss",
			`
              resource R {}

              transaction {

                  var x: @R

                  prepare() {
                      self.x <- create R()
                  }

                  execute {}
              }
            `,
			[]error{
				&sema.ResourceFieldNotInvalidatedError{},
			},
		},
		{
			"ParameterUse",
			`
              transaction(x: Bool) {

                  prepare() {
                      x
                  }

                  pre {
                      x
                  }

                  execute {
                      x
                  }

                  post {
                      x
                  }
              }
            `,
			nil,
		},
		{
			"InvalidParameterUseAfterDeclaration",
			`
		      transaction(x: Bool) {}
		
		      let y = x
		    `,
			[]error{
				&sema.NotDeclaredError{},
			},
		},
		{
			"InvalidResourceParameter",
			`
		      resource R {}

		      transaction(rs: @[R]) {}	
		    `,
			[]error{
				&sema.InvalidResourceTransactionParameterError{},
				&sema.ResourceLossError{},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseAndCheck(t, test.code)

			errs := ExpectCheckerErrors(t, err, len(test.errors))

			for i, err := range errs {
				if !assert.IsType(t, test.errors[i], err) {
					t.Log(err)
				}
			}
		})
	}
}

func TestCheckTransactionExecuteScope(t *testing.T) {
	// non-global variable declarations do not require access modifiers
	// execute block should be treated like function block

	_, err := ParseAndCheckWithOptions(
		t,
		`
          transaction {

              execute {
                  let code: Int = 1
              }
          }
        `,
		ParseAndCheckOptions{
			Options: []sema.Option{
				sema.WithAccessCheckMode(sema.AccessCheckModeStrict),
			},
		},
	)

	assert.NoError(t, err)
}

func TestCheckInvalidTransactionSelfMoveToFunction(t *testing.T) {

	_, err := ParseAndCheck(t, `

      transaction {

          execute {
              use(self)
          }
      }

      fun use(_ any: AnyStruct) {}
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidMoveError{}, errs[0])
}

func TestCheckInvalidTransactionSelfMoveInVariableDeclaration(t *testing.T) {

	_, err := ParseAndCheck(t, `

     transaction {

         execute {
             let x = self
         }
     }
   `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidMoveError{}, errs[0])
}

func TestCheckInvalidTransactionSelfMoveReturnFromFunction(t *testing.T) {

	_, err := ParseAndCheck(t, `

     transaction {

         execute {
             return self
         }
     }
   `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidReturnValueError{}, errs[0])
}

func TestCheckInvalidTransactionSelfMoveIntoArrayLiteral(t *testing.T) {

	_, err := ParseAndCheck(t, `

     transaction {

         execute {
             let txs = [self]
         }
     }
   `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidMoveError{}, errs[0])
}

func TestCheckInvalidTransactionSelfMoveIntoDictionaryLiteral(t *testing.T) {

	_, err := ParseAndCheck(t, `

     transaction {

         execute {
             let txs = {"self": self}
         }
     }
   `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidMoveError{}, errs[0])
}
