package runtime

import (
	"math/big"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/interpreter"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/parser"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/sema"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/trampoline"
)

func parseCheckAndInterpret(code string) *interpreter.Interpreter {
	program, errors := parser.ParseProgram(code)

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
        let s = "123"
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

	Expect(inter.Globals["s"].Value).
		To(Equal(interpreter.StringValue("123")))
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

func TestInterpretFunctionSideEffects(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
       var value = 0

       fun test(_ newValue: Int) {
           value = newValue
       }
	`)

	newValue := big.NewInt(42)

	Expect(inter.Invoke("test", newValue)).
		To(Equal(interpreter.VoidValue{}))

	Expect(inter.Globals["value"].Value).
		To(Equal(interpreter.IntValue{Int: newValue}))
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

func TestInterpretWhileStatementWithContinue(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
       fun test(): Int {
           var i = 0
           var x = 0
           while i < 10 {
               i = i + 1
               if i < 5 {
                   continue
               }
               x = x + 1
           }
           return x
       }
	`)

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(6)}))
}

func TestInterpretWhileStatementWithBreak(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
       fun test(): Int {
           var x = 0
           while x < 10 {
               x = x + 1
               if x == 5 {
                   break
               }
           }
           return x
       }
	`)

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(5)}))
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

func TestInterpretFunctionBindingInFunction(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      fun foo(): Any {
          return foo
      }
  `)

	_, err := inter.Invoke("foo")
	Expect(err).
		To(Not(HaveOccurred()))
}

func TestInterpretRecursionFib(t *testing.T) {
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

func TestInterpretRecursionFactorial(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
        fun factorial(_ n: Int): Int {
            if n < 1 {
               return 1
            }

            return n * factorial(n - 1)
        }
   `)

	Expect(inter.Invoke("factorial", big.NewInt(5))).
		To(Equal(interpreter.IntValue{Int: big.NewInt(120)}))
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

	program, errors := parser.ParseProgram(`
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
		func(inter *interpreter.Interpreter, arguments []interpreter.Value) trampoline.Trampoline {
			a := arguments[0].(interpreter.IntValue).Int
			b := arguments[1].(interpreter.IntValue).Int
			value := big.NewInt(0).Add(a, b)
			result := interpreter.IntValue{Int: value}
			return trampoline.Done{Result: result}
		},
	)

	inter.ImportFunction("test", testFunction)
	err := inter.Interpret()
	Expect(err).
		To(Not(HaveOccurred()))

	Expect(inter.Globals["a"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}

func TestInterpretStructureDeclaration(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
       struct Test {}

       fun test(): Test {
           return Test()
       }
	`)

	Expect(inter.Invoke("test")).
		To(BeAssignableToTypeOf(interpreter.StructureValue{}))
}

func TestInterpretStructureDeclarationWithInitializer(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
       var value = 0

       struct Test {
           init(_ newValue: Int) {
               value = newValue
           }
       }

       fun test(newValue: Int): Test {
           return Test(newValue)
       }
	`)

	newValue := big.NewInt(42)

	Expect(inter.Invoke("test", newValue)).
		To(BeAssignableToTypeOf(interpreter.StructureValue{}))

	Expect(inter.Globals["value"].Value).
		To(Equal(interpreter.IntValue{Int: newValue}))
}

func TestInterpretStructureSelfReferenceInInitializer(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`

      struct Test {

          init() {
              self
          }
      }

      fun test() {
          Test()
      }
	`)

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.VoidValue{}))
}

func TestInterpretStructureConstructorReferenceInInitializerAndFunction(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`

      struct Test {

          init() {
              Test
          }

          fun test(): Test {
              return Test()
          }
      }

      fun test(): Test {
          return Test()
      }

      fun test2(): Test {
          return Test().test()
      }
	`)

	Expect(inter.Invoke("test")).
		To(BeAssignableToTypeOf(interpreter.StructureValue{}))

	Expect(inter.Invoke("test2")).
		To(BeAssignableToTypeOf(interpreter.StructureValue{}))
}

func TestInterpretStructureSelfReferenceInFunction(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`

    struct Test {

        fun test() {
            self
        }
    }

    fun test() {
        Test().test()
    }
	`)

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.VoidValue{}))
}

func TestInterpretStructureConstructorReferenceInFunction(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`

    struct Test {

        fun test() {
            Test
        }
    }

    fun test() {
        Test().test()
    }
	`)

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.VoidValue{}))
}

func TestInterpretStructureDeclarationWithField(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`

      struct Test {
          var test: Int

          init(_ test: Int) {
              self.test = test
          }
      }

      fun test(test: Int): Int {
          let test = Test(test)
          return test.test
      }
	`)

	newValue := big.NewInt(42)

	Expect(inter.Invoke("test", newValue)).
		To(Equal(interpreter.IntValue{Int: newValue}))
}

func TestInterpretStructureDeclarationWithFunction(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      var value = 0

      struct Test {
          fun test(_ newValue: Int) {
              value = newValue
          }
      }

      fun test(newValue: Int) {
          let test = Test()
          test.test(newValue)
      }
	`)

	newValue := big.NewInt(42)

	Expect(inter.Invoke("test", newValue)).
		To(Equal(interpreter.VoidValue{}))

	Expect(inter.Globals["value"].Value).
		To(Equal(interpreter.IntValue{Int: newValue}))
}

func TestInterpretStructureFunctionCall(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      struct Test {
          fun foo(): Int {
              return 42
          }

          fun bar(): Int {
              return self.foo()
          }
      }

      let value = Test().bar()
	`)

	Expect(inter.Globals["value"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(42)}))
}

func TestInterpretStructureFieldAssignment(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      struct Test {
          var foo: Int

          init() {
              self.foo = 1
              let alsoSelf = self
              alsoSelf.foo = 2
          }

          fun test() {
              self.foo = 3
              let alsoSelf = self
              alsoSelf.foo = 4
          }
      }

	  let test = Test()

      fun callTest() {
          test.test()
      }
	`)

	Expect(inter.Globals["test"].Value.(interpreter.StructureValue).Get("foo")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(1)}))

	Expect(inter.Invoke("callTest")).
		To(Equal(interpreter.VoidValue{}))

	Expect(inter.Globals["test"].Value.(interpreter.StructureValue).Get("foo")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}

func TestInterpretStructureInitializesConstant(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      struct Test {
          let foo: Int

          init() {
              self.foo = 42
          }
      }

	  let test = Test()
	`)

	Expect(inter.Globals["test"].Value.(interpreter.StructureValue).Get("foo")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(42)}))
}

func TestInterpretStructureFunctionMutatesSelf(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      struct Test {
          var foo: Int

          init() {
              self.foo = 0
          }

          fun inc() {
              self.foo = self.foo + 1
          }
      }

      fun test(): Int {
          let test = Test()
          test.inc()
          test.inc()
          return test.foo
      }
	`)

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
}

func TestInterpretFunctionPreCondition(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      fun test(x: Int): Int {
          pre {
              x == 0
          }
          return x
      }
	`)

	_, err := inter.Invoke("test", big.NewInt(42))
	Expect(err).
		To(BeAssignableToTypeOf(&interpreter.ConditionError{}))

	zero := big.NewInt(0)
	Expect(inter.Invoke("test", zero)).
		To(Equal(interpreter.IntValue{Int: zero}))
}

func TestInterpretFunctionPostCondition(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      fun test(x: Int): Int {
          post {
              y == 0
          }
          let y = x
          return y
      }
	`)

	_, err := inter.Invoke("test", big.NewInt(42))
	Expect(err).
		To(BeAssignableToTypeOf(&interpreter.ConditionError{}))

	zero := big.NewInt(0)
	Expect(inter.Invoke("test", zero)).
		To(Equal(interpreter.IntValue{Int: zero}))
}

func TestInterpretFunctionWithResultAndPostConditionWithResult(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      fun test(x: Int): Int {
          post {
              result == 0
          }
          return x
      }
	`)

	_, err := inter.Invoke("test", big.NewInt(42))
	Expect(err).
		To(BeAssignableToTypeOf(&interpreter.ConditionError{}))

	zero := big.NewInt(0)
	Expect(inter.Invoke("test", zero)).
		To(Equal(interpreter.IntValue{Int: zero}))
}

func TestInterpretFunctionWithoutResultAndPostConditionWithResult(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      fun test() {
          post {
              result == 0
          }
          let result = 0
      }
	`)

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.VoidValue{}))
}

func TestInterpretFunctionPostConditionWithBefore(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      var x = 0

      fun test() {
          pre {
              x == 0
          }
          post {
              x == before(x) + 1
          }
          x = x + 1
      }
	`)

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.VoidValue{}))
}

func TestInterpretFunctionPostConditionWithBeforeFailingPreCondition(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      var x = 0

      fun test() {
          pre {
              x == 1
          }
          post {
              x == before(x) + 1
          }
          x = x + 1
      }
	`)

	_, err := inter.Invoke("test")

	Expect(err).
		To(BeAssignableToTypeOf(&interpreter.ConditionError{}))

	Expect(err.(*interpreter.ConditionError).ConditionKind).
		To(Equal(ast.ConditionKindPre))
}

func TestInterpretFunctionPostConditionWithBeforeFailingPostCondition(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      var x = 0

      fun test() {
          pre {
              x == 0
          }
          post {
              x == before(x) + 2
          }
          x = x + 1
      }
	`)

	_, err := inter.Invoke("test")

	Expect(err).
		To(BeAssignableToTypeOf(&interpreter.ConditionError{}))

	Expect(err.(*interpreter.ConditionError).ConditionKind).
		To(Equal(ast.ConditionKindPost))
}

func TestInterpretStructCopyOnDeclaration(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      struct Cat {
          var wasFed: Bool

          init() {
              self.wasFed = false
          }
      }

      fun test(): Bool[] {
          let cat = Cat()
          let kitty = cat
          kitty.wasFed = true
          return [cat.wasFed, kitty.wasFed]
      }
    `)

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.ArrayValue{
			interpreter.BoolValue(false),
			interpreter.BoolValue(true),
		}))
}

func TestInterpretStructCopyOnDeclarationModifiedWithStructFunction(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      struct Cat {
          var wasFed: Bool

          init() {
              self.wasFed = false
          }

          fun feed() {
              self.wasFed = true
          }
      }

      fun test(): Bool[] {
          let cat = Cat()
          let kitty = cat
          kitty.feed()
          return [cat.wasFed, kitty.wasFed]
      }
    `)

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.ArrayValue{
			interpreter.BoolValue(false),
			interpreter.BoolValue(true),
		}))
}

func TestInterpretStructCopyOnIdentifierAssignment(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      struct Cat {
          var wasFed: Bool

          init() {
              self.wasFed = false
          }
      }

      fun test(): Bool[] {
          var cat = Cat()
          let kitty = Cat()
          cat = kitty
          kitty.wasFed = true
          return [cat.wasFed, kitty.wasFed]
      }
    `)

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.ArrayValue{
			interpreter.BoolValue(false),
			interpreter.BoolValue(true),
		}))
}

func TestInterpretStructCopyOnIndexingAssignment(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      struct Cat {
          var wasFed: Bool

          init() {
              self.wasFed = false
          }
      }

      fun test(): Bool[] {
          let cats = [Cat()]
          let kitty = Cat()
          cats[0] = kitty
          kitty.wasFed = true
          return [cats[0].wasFed, kitty.wasFed]
      }
    `)

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.ArrayValue{
			interpreter.BoolValue(false),
			interpreter.BoolValue(true),
		}))
}

func TestInterpretStructCopyOnMemberAssignment(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      struct Cat {
          var wasFed: Bool

          init() {
              self.wasFed = false
          }
      }

      struct Carrier {
          var cat: Cat
          init(cat: Cat) {
              self.cat = cat
          }
      }

      fun test(): Bool[] {
          let carrier = Carrier(cat: Cat())
          let kitty = Cat()
          carrier.cat = kitty
          kitty.wasFed = true
          return [carrier.cat.wasFed, kitty.wasFed]
      }
    `)

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.ArrayValue{
			interpreter.BoolValue(false),
			interpreter.BoolValue(true),
		}))
}

func TestInterpretStructCopyOnPassing(t *testing.T) {
	RegisterTestingT(t)

	inter := parseCheckAndInterpret(`
      struct Cat {
          var wasFed: Bool

          init() {
              self.wasFed = false
          }
      }

      fun feed(cat: Cat) {
          cat.wasFed = true
      }

      fun test(): Bool {
          let kitty = Cat()
          feed(cat: kitty)
          return kitty.wasFed
      }
    `)

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.BoolValue(false)))
}
