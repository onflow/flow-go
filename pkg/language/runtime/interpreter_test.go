package runtime

import (
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/interpreter"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/parser"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/sema"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"math/big"
	"testing"
)

func init() {
	format.UseStringerRepresentation = true
}

func parseCheckAndInterpret(code string) *interpreter.Interpreter {
	program, errors := parser.Parse(code)

	Expect(errors).
		To(BeEmpty())

	checker := sema.NewChecker(program)
	err := checker.Check()

	Expect(err).
		To(Not(HaveOccurred()))

	inter := interpreter.NewInterpreter(program)
	err = inter.Interpret()

	Expect(err).
		To(Not(HaveOccurred()))

	return inter
}

func TestInterpretConstantAndVariableDeclarations(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
        let x = 1
        let y = true
        let z = 1 + 2
        var a = 3 == 3
        var b = [1, 2]
    `)

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(1)}))

	Expect(inter.Globals["y"].Value).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Globals["z"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))

	Expect(inter.Globals["a"].Value).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Globals["b"].Value).
		To(Equal(interpreter.ArrayValue([]interpreter.Value{
			interpreter.IntValue{Int: big.NewInt(1)},
			interpreter.IntValue{Int: big.NewInt(2)},
		})))
}

func TestInterpretDeclarations(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
        fun test(): Int {
            return 42
        }
    `)

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(42)}))
}

func TestInterpretInvalidUnknownDeclarationInvocation(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(``)

	_, err := inter.Invoke("test")
	Expect(err).
		To(BeAssignableToTypeOf(&interpreter.NotDeclaredError{}))
}

func TestInterpretInvalidNonFunctionDeclarationInvocation(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
       let test = 1
   `)

	_, err := inter.Invoke("test")
	Expect(err).
		To(BeAssignableToTypeOf(&interpreter.NotCallableError{}))
}

func TestInterpretLexicalScope(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
       let x = 10

       fun f(): Int {
          // check resolution
          return x
       }

       fun g(): Int {
          // check scope is lexical, not dynamic
          let x = 20
          return f()
       }
	`)

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(10)}))

	Expect(inter.Invoke("f")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(10)}))

	Expect(inter.Invoke("g")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(10)}))
}

func TestInterpretNoHoisting(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
       let x = 2

       fun test(): Int {
          if x == 0 {
              let x = 3
              return x
          }
          return x
       }
	`)

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
}

func TestInterpretFunctionExpressionsAndScope(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
       let x = 10

       // check first-class functions and scope inside them
       let y = (fun (x: Int): Int { return x })(42)
	`)

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(10)}))

	Expect(inter.Globals["y"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(42)}))
}

func TestInterpretVariableAssignment(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
       fun test(): Int {
           var x = 2
           x = 3
           return x
       }
	`)

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}

func TestInterpretGlobalVariableAssignment(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
       var x = 2

       fun test(): Int {
           x = 3
           return x
       }
	`)

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}

func TestInterpretConstantRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
       let x = 2

       fun test(): Int {
           let x = 3
           return x
       }
	`)

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}

func TestInterpretParameters(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
       fun returnA(a: Int, b: Int): Int {
           return a
       }

       fun returnB(a: Int, b: Int): Int {
           return b
       }
	`)

	Expect(inter.Invoke("returnA", big.NewInt(24), big.NewInt(42))).
		To(Equal(interpreter.IntValue{Int: big.NewInt(24)}))

	Expect(inter.Invoke("returnB", big.NewInt(24), big.NewInt(42))).
		To(Equal(interpreter.IntValue{Int: big.NewInt(42)}))
}

func TestInterpretArrayIndexing(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
       fun test(): Int {
           let z = [0, 3]
           return z[1]
       }
	`)

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}

func TestInterpretArrayIndexingAssignment(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
       fun test(): Int {
           let z = [0, 3]
           z[1] = 2
           return z[1]
       }
	`)

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
}

func TestInterpretReturnWithoutExpression(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
       fun returnNothing() {
           return
       }
	`)

	Expect(inter.Invoke("returnNothing")).
		To(Equal(interpreter.VoidValue{}))
}

func TestInterpretReturns(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
       fun returnEarly(): Int {
           return 2
           return 1
       }
	`)

	Expect(inter.Invoke("returnEarly")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
}

// TODO: perform each operator test for each integer type

func TestInterpretPlusOperator(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
       let x = 2 + 4
	`)

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(6)}))
}

func TestInterpretMinusOperator(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
       let x = 2 - 4
	`)

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(-2)}))
}

func TestInterpretMulOperator(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
       let x = 2 * 4
	`)

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(8)}))
}

func TestInterpretDivOperator(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
       let x = 7 / 3
	`)

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
}

func TestInterpretModOperator(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
       let x = 5 % 3
	`)

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
}

func TestInterpretEqualOperator(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      fun testIntegersUnequal(): Bool {
          return 5 == 3
      }

      fun testIntegersEqual(): Bool {
          return 3 == 3
      }

      fun testTrueAndTrue(): Bool {
          return true == true
      }

      fun testTrueAndFalse(): Bool {
          return true == false
      }

      fun testFalseAndTrue(): Bool {
          return false == true
      }

      fun testFalseAndFalse(): Bool {
          return false == false
      }
	`)

	Expect(inter.Invoke("testIntegersUnequal")).
		To(Equal(interpreter.BoolValue(false)))

	Expect(inter.Invoke("testIntegersEqual")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testTrueAndTrue")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testTrueAndFalse")).
		To(Equal(interpreter.BoolValue(false)))

	Expect(inter.Invoke("testFalseAndTrue")).
		To(Equal(interpreter.BoolValue(false)))

	Expect(inter.Invoke("testFalseAndFalse")).
		To(Equal(interpreter.BoolValue(true)))
}

func TestInterpretUnequalOperator(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      fun testIntegersUnequal(): Bool {
          return 5 != 3
      }

      fun testIntegersEqual(): Bool {
          return 3 != 3
      }

      fun testTrueAndTrue(): Bool {
          return true != true
      }

      fun testTrueAndFalse(): Bool {
          return true != false
      }

      fun testFalseAndTrue(): Bool {
          return false != true
      }

      fun testFalseAndFalse(): Bool {
          return false != false
      }
	`)

	Expect(inter.Invoke("testIntegersUnequal")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testIntegersEqual")).
		To(Equal(interpreter.BoolValue(false)))

	Expect(inter.Invoke("testTrueAndTrue")).
		To(Equal(interpreter.BoolValue(false)))

	Expect(inter.Invoke("testTrueAndFalse")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testFalseAndTrue")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testFalseAndFalse")).
		To(Equal(interpreter.BoolValue(false)))
}

func TestInterpretLessOperator(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      fun testIntegersGreater(): Bool {
          return 5 < 3
      }

      fun testIntegersEqual(): Bool {
          return 3 < 3
      }

      fun testIntegersLess(): Bool {
          return 3 < 5
      }
    `)

	Expect(inter.Invoke("testIntegersGreater")).
		To(Equal(interpreter.BoolValue(false)))

	Expect(inter.Invoke("testIntegersEqual")).
		To(Equal(interpreter.BoolValue(false)))

	Expect(inter.Invoke("testIntegersLess")).
		To(Equal(interpreter.BoolValue(true)))
}

func TestInterpretLessEqualOperator(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      fun testIntegersGreater(): Bool {
          return 5 <= 3
      }

      fun testIntegersEqual(): Bool {
          return 3 <= 3
      }

      fun testIntegersLess(): Bool {
          return 3 <= 5
      }
	`)

	Expect(inter.Invoke("testIntegersGreater")).
		To(Equal(interpreter.BoolValue(false)))

	Expect(inter.Invoke("testIntegersEqual")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testIntegersLess")).
		To(Equal(interpreter.BoolValue(true)))
}

func TestInterpretGreaterOperator(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      fun testIntegersGreater(): Bool {
          return 5 > 3
      }

      fun testIntegersEqual(): Bool {
          return 3 > 3
      }

      fun testIntegersLess(): Bool {
          return 3 > 5
      }
	`)

	Expect(inter.Invoke("testIntegersGreater")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testIntegersEqual")).
		To(Equal(interpreter.BoolValue(false)))

	Expect(inter.Invoke("testIntegersLess")).
		To(Equal(interpreter.BoolValue(false)))
}

func TestInterpretGreaterEqualOperator(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      fun testIntegersGreater(): Bool {
          return 5 >= 3
      }

      fun testIntegersEqual(): Bool {
          return 3 >= 3
      }

      fun testIntegersLess(): Bool {
          return 3 >= 5
      }
	`)

	Expect(inter.Invoke("testIntegersGreater")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testIntegersEqual")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testIntegersLess")).
		To(Equal(interpreter.BoolValue(false)))
}

func TestInterpretOrOperator(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      fun testTrueTrue(): Bool {
          return true || true
      }

      fun testTrueFalse(): Bool {
          return true || false
      }

      fun testFalseTrue(): Bool {
          return false || true
      }

      fun testFalseFalse(): Bool {
          return false || false
      }
	`)

	Expect(inter.Invoke("testTrueTrue")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testTrueFalse")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testFalseTrue")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testFalseFalse")).
		To(Equal(interpreter.BoolValue(false)))
}

func TestInterpretAndOperator(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      fun testTrueTrue(): Bool {
          return true && true
      }

      fun testTrueFalse(): Bool {
          return true && false
      }

      fun testFalseTrue(): Bool {
          return false && true
      }

      fun testFalseFalse(): Bool {
          return false && false
      }
	`)

	Expect(inter.Invoke("testTrueTrue")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testTrueFalse")).
		To(Equal(interpreter.BoolValue(false)))

	Expect(inter.Invoke("testFalseTrue")).
		To(Equal(interpreter.BoolValue(false)))

	Expect(inter.Invoke("testFalseFalse")).
		To(Equal(interpreter.BoolValue(false)))
}

func TestInterpretIfStatement(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
       fun testTrue(): Int {
           if true {
               return 2
           } else {
               return 3
           }
           return 4
       }

       fun testFalse(): Int {
           if false {
               return 2
           } else {
               return 3
           }
           return 4
       }

       fun testNoElse(): Int {
           if true {
               return 2
           }
           return 3
       }

       fun testElseIf(): Int {
           if false {
               return 2
           } else if true {
               return 3
           }
           return 4
       }
	`)

	Expect(inter.Invoke("testTrue")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))

	Expect(inter.Invoke("testFalse")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))

	Expect(inter.Invoke("testNoElse")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))

	Expect(inter.Invoke("testElseIf")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}

func TestInterpretWhileStatement(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
       fun test(): Int {
           var x = 0
           while x < 5 {
               x = x + 2
           }
           return x
       }

	`)

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(6)}))
}

func TestInterpretWhileStatementWithReturn(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
       fun test(): Int {
           var x = 0
           while x < 10 {
               x = x + 2
               if x > 5 {
                   return x
               }
           }
           return x
       }

	`)

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(6)}))
}

func TestInterpretExpressionStatement(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
       var x = 0

       fun incX() {
           x = x + 2
       }

       fun test(): Int {
           incX()
           return x
       }
	`)

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(0)}))

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
}

func TestInterpretConditionalOperator(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
       fun testTrue(): Int {
           return true ? 2 : 3
       }

       fun testFalse(): Int {
			return false ? 2 : 3
       }
	`)

	Expect(inter.Invoke("testTrue")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))

	Expect(inter.Invoke("testFalse")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}

// TODO: requires Any type
//
//func TestInterpretFunctionBindingInFunction(t *testing.T) {
//	RegisterTestingT(t)
//
//	inter := parseCheckAndInterpret(`
//       fun foo(): Any {
//           return foo
//       }
//   `)
//
//	_, err := inter.Invoke("foo")
//	Expect(err).
//		To(Not(HaveOccurred()))
//}

func TestInterpretRecursion(t *testing.T) {
	// mainly tests that the function declaration identifier is bound
	// to the function inside the function and that the arguments
	// of the function calls are evaluated in the call-site scope

	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
       fun fib(_ n: Int): Int {
           if n < 2 {
              return n
           }
           return fib(n - 1) + fib(n - 2)
       }
   `)

	Expect(inter.Invoke("fib", big.NewInt(14))).
		To(Equal(interpreter.IntValue{Int: big.NewInt(377)}))
}

func TestInterpretUnaryIntegerNegation(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      let x = -2
      let y = -(-2)
	`)

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(-2)}))

	Expect(inter.Globals["y"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
}

func TestInterpretUnaryBooleanNegation(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      let a = !true
      let b = !(!true)
      let c = !false
      let d = !(!false)
	`)

	Expect(inter.Globals["a"].Value).
		To(Equal(interpreter.BoolValue(false)))

	Expect(inter.Globals["b"].Value).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Globals["c"].Value).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Globals["d"].Value).
		To(Equal(interpreter.BoolValue(false)))
}

func TestInterpretHostFunction(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      let a = test(1, 2)
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)

	testFunction := interpreter.NewHostFunction(
		&sema.FunctionType{
			ParameterTypes: []sema.Type{
				&sema.IntType{},
				&sema.IntType{},
			},
			ReturnType: &sema.IntType{},
		},
		func(inter *interpreter.Interpreter, arguments []interpreter.Value) interpreter.Value {
			a := arguments[0].(interpreter.IntValue).Int
			b := arguments[1].(interpreter.IntValue).Int
			result := big.NewInt(0).Add(a, b)
			return interpreter.IntValue{Int: result}
		},
	)

	inter.ImportFunction("test", testFunction)
	err := inter.Interpret()
	Expect(err).
		To(Not(HaveOccurred()))

	Expect(inter.Globals["a"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}
