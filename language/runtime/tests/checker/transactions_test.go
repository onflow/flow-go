package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestTransactions(t *testing.T) {

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
		[]error{
			&sema.TransactionMissingExecuteError{},
		},
	}

	noopTx := test{
		"No-op",
		`
		  transaction {
		    execute {}
		  }
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

	preConditionsWithNotDeclaredFields := test{
		"PreConditionsWithNotDeclaredFields",
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

	resourceTransaction := test{
		"ResourceTransaction",
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

	resourceLoss := test{
		"ResourceLoss",
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
		fieldUninitialized,
		fieldInitialized,
		preConditions,
		preConditionsWithNotDeclaredFields,
		postConditions,
		postConditionsAccessExecuteScope,
		// illegalSelfUsage,
		resourceTransaction,
		resourceLoss,
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseAndCheck(t, test.code)

			errs := ExpectCheckerErrors(t, err, len(test.errors))

			for i, err := range errs {
				assert.IsType(t, test.errors[i], err)
			}
		})
	}

}
