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

	emptyTx := test{
		"Empty",
		`
		  transaction {}
		`,
		nil,
	}

	noopTx := test{
		"No-op",
		`
		  transaction {}
		`,
		nil,
	}

	simpleTx := test{
		"Simple",
		`
		  transaction {

		    execute {
 			  let x = 1 + 2
			}
		  }
		`,
		nil,
	}

	invalidPrepareIdentifier := test{
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
	}

	invalidExecuteIdentifier := test{
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
	}

	validPrepareParameters := test{
		"ValidPrepareParameters",
		`
		  transaction {
		    prepare(x: Account, y: Account) {}
		  }
		`,
		nil,
	}

	invalidPrepareParameters := test{
		"InvalidPrepareParameters",
		`
		  transaction {
		    prepare(x: Int, y: Int) {}
		  }
		`,
		[]error{
			&sema.InvalidTransactionPrepareParameterType{},
			&sema.InvalidTransactionPrepareParameterType{},
		},
	}

	fieldAccessSpecified := test{
		"FieldAccessSpecified",
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
	}

	fieldUninitialized := test{
		"FieldUninitialized",
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
	}

	fieldInitialized := test{
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
	}

	preConditions := test{
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
	}

	preConditionsWithUndeclaredFields := test{
		"PreConditionsWithUndeclaredFields",
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
	}

	postConditions := test{
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
	}

	postConditionsAccessExecuteScope := test{
		"PostConditionsAccessExecuteScope",
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
	}

	// TODO: prevent self from being used in function
	// illegalSelfUsage := test{
	// 	"IllegalSelfUsage",
	// 	`
	//  	  fun foo(x: Any) {}
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
	// }

	resourceField := test{
		"ResourceField",
		`
		  resource R {}

		  transaction {

	   		var x: <-R

			prepare() {
			  self.x <- create R()
			}

		    execute {
			  destroy self.x
			}
		  }
		`,
		nil,
	}

	resourceFieldLoss := test{
		"ResourceFieldLoss",
		`
		  resource R {}

		  transaction {

	   		var x: <-R

			prepare() {
			  self.x <- create R()
			}

		    execute {}
		  }
		`,
		[]error{
			&sema.ResourceFieldNotInvalidatedError{},
		},
	}

	tests := []test{
		emptyTx,
		noopTx,
		simpleTx,
		invalidPrepareIdentifier,
		invalidExecuteIdentifier,
		validPrepareParameters,
		invalidPrepareParameters,
		fieldAccessSpecified,
		fieldUninitialized,
		fieldInitialized,
		preConditions,
		preConditionsWithUndeclaredFields,
		postConditions,
		postConditionsAccessExecuteScope,
		// illegalSelfUsage,
		resourceField,
		resourceFieldLoss,
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
	code := `
	  transaction {
		execute {
		  let code: Int = 1
		}
	  }
	`

	_, err := ParseAndCheckWithOptions(t, code, ParseAndCheckOptions{
		Options: []sema.Option{
			sema.WithAccessCheckMode(sema.AccessCheckModeStrict),
		},
	})
	assert.NoError(t, err)
}
