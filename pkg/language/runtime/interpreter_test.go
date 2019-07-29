package runtime

import (
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/interpreter"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/parser"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/sema"
	. "github.com/onsi/gomega"
	"math/big"
	"testing"
)

func TestInterpretConstantAndVariableDeclarations(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
        let x = 1
        let y = true
        let z = 1 + 2
        var a = 3 == 3
        var b = [1, 2]
    `)

	Expect(errors).To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

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

	program, errors := parser.Parse(`
        fun test(): Int {
            return 42
        }
    `)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(42)}))
}

func TestInterpretInvalidUnknownDeclarationInvocation(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(``)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretInvalidNonFunctionDeclarationInvocation(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       let test = 1
   `)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretInvalidUnknownDeclaration(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun test() {
           return x
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretInvalidUnknownDeclarationAssignment(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun test() {
           x = 2
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretInvalidUnknownDeclarationIndexing(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun test() {
           x[0]
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretInvalidUnknownDeclarationIndexingAssignment(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun test() {
           x[0] = 2
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretLexicalScope(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       let x = 10

       fun f(): Int32 {
          // check resolution
          return x
       }

       fun g(): Int32 {
          // check scope is lexical, not dynamic
          let x = 20
          return f()
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(10)}))

	Expect(inter.Invoke("f")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(10)}))

	Expect(inter.Invoke("g")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(10)}))
}

func TestInterpretNoHoisting(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       let x = 2

       fun test(): Int64 {
          if x == 0 {
              let x = 3
              return x
          }
          return x
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
}

func TestInterpretFunctionExpressionsAndScope(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       let x = 10

       // check first-class functions and scope inside them
       let y = (fun (x: Int32): Int32 { return x })(42)
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(10)}))

	Expect(inter.Globals["y"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(42)}))
}

func TestInterpretInvalidFunctionCallWithTooFewArguments(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun f(x: Int32): Int32 {
           return x
       }

       fun test(): Int32 {
           return f()
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretInvalidFunctionCallWithTooManyArguments(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun f(x: Int32): Int32 {
           return x
       }

       fun test(): Int32 {
           return f(2, 3)
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretInvalidFunctionCallOfBool(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun test(): Int32 {
           return true()
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretInvalidFunctionCallOfInteger(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun test(): Int32 {
           return 2()
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretInvalidConstantAssignment(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun test() {
           let x = 2
           x = 3
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretVariableAssignment(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun test(): Int64 {
           var x = 2
           x = 3
           return x
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}

func TestInterpretInvalidGlobalConstantAssignment(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       let x = 2

       fun test() {
           x = 3
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretGlobalVariableAssignment(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       var x = 2

       fun test(): Int64 {
           x = 3
           return x
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}

func TestInterpretConstantRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       let x = 2

       fun test(): Int64 {
           let x = 3
           return x
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}

func TestInterpretParameters(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun returnA(a: Int32, b: Int32): Int64 {
           return a
       }

       fun returnB(a: Int32, b: Int32): Int64 {
           return b
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Invoke("returnA", int64(24), int64(42))).
		To(Equal(interpreter.Int64Value(24)))

	Expect(inter.Invoke("returnB", int64(24), int64(42))).
		To(Equal(interpreter.Int64Value(42)))
}

func TestInterpretArrayIndexing(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun test(): Int64 {
           let z = [0, 3]
           return z[1]
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}

func TestInterpretInvalidArrayIndexingWithBool(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun test(): Int64 {
           let z = [0, 3]
           return z[true]
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretInvalidArrayIndexingIntoBool(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun test(): Int64 {
           return true[0]
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretInvalidArrayIndexingIntoInteger(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun test(): Int64 {
           return 2[0]
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretArrayIndexingAssignment(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun test(): Int64 {
           let z = [0, 3]
           z[1] = 2
           return z[1]
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
}

func TestInterpretInvalidArrayIndexingAssignmentWithBool(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun test(): Int64 {
           let z = [0, 3]
           z[true] = 2
           return z[1]
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretReturnWithoutExpression(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun returnEarly() {
           return
           return 1
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Invoke("returnEarly")).
		To(Equal(interpreter.VoidValue{}))
}

// TODO: perform each operator test for each integer type

func TestInterpretPlusOperator(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun testIntegers(): Int64 {
           return 2 + 4
       }

       fun testIntegerAndBool(): Int64 {
           return 2 + true
       }

       fun testBoolAndInteger(): Int64 {
           return true + 2
       }

       fun testBools(): Int64 {
           return true + true
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Invoke("testIntegers")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(6)}))

	_, err = inter.Invoke("testIntegerAndBool")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testBoolAndInteger")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testBools")
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretMinusOperator(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun testIntegers(): Int64 {
           return 2 - 4
       }

       fun testIntegerAndBool(): Int64 {
           return 2 - true
       }

       fun testBoolAndInteger(): Int64 {
           return true - 2
       }

       fun testBools(): Int64 {
           return true - true
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Invoke("testIntegers")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(-2)}))

	_, err = inter.Invoke("testIntegerAndBool")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testBoolAndInteger")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testBools")
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretMulOperator(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun testIntegers(): Int64 {
           return 2 * 4
       }

       fun testIntegerAndBool(): Int64 {
           return 2 * true
       }

       fun testBoolAndInteger(): Int64 {
           return true * 2
       }

       fun testBools(): Int64 {
           return true * true
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Invoke("testIntegers")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(8)}))

	_, err = inter.Invoke("testIntegerAndBool")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testBoolAndInteger")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testBools")
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretDivOperator(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun testIntegers(): Int64 {
           return 7 / 3
       }

       fun testIntegerAndBool(): Int64 {
           return 7 / true
       }

       fun testBoolAndInteger(): Int64 {
           return true / 2
       }

       fun testBools(): Int64 {
           return true / true
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Invoke("testIntegers")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))

	_, err = inter.Invoke("testIntegerAndBool")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testBoolAndInteger")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testBools")
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretModOperator(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun testIntegers(): Int64 {
           return 5 % 3
       }

       fun testIntegerAndBool(): Int64 {
           return 5 % true
       }

       fun testBoolAndInteger(): Int64 {
           return true % 2
       }

       fun testBools(): Int64 {
           return true % true
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Invoke("testIntegers")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))

	_, err = inter.Invoke("testIntegerAndBool")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testBoolAndInteger")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testBools")
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretEqualOperator(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun testIntegersUnequal(): Bool {
           return 5 == 3
       }

       fun testIntegersEqual(): Bool {
           return 3 == 3
       }

       fun testIntegerAndBool(): Bool {
           return 5 == true
       }

       fun testBoolAndInteger(): Bool {
           return true == 5
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

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Invoke("testIntegersUnequal")).
		To(Equal(interpreter.BoolValue(false)))

	Expect(inter.Invoke("testIntegersEqual")).
		To(Equal(interpreter.BoolValue(true)))

	_, err = inter.Invoke("testIntegerAndBool")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testBoolAndInteger")
	Expect(err).
		To(HaveOccurred())

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

	program, errors := parser.Parse(`
       fun testIntegersUnequal(): Bool {
           return 5 != 3
       }

       fun testIntegersEqual(): Bool {
           return 3 != 3
       }

       fun testIntegerAndBool(): Bool {
           return 5 != true
       }

       fun testBoolAndInteger(): Bool {
           return true != 5
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

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Invoke("testIntegersUnequal")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testIntegersEqual")).
		To(Equal(interpreter.BoolValue(false)))

	_, err = inter.Invoke("testIntegerAndBool")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testBoolAndInteger")
	Expect(err).
		To(HaveOccurred())

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

	program, errors := parser.Parse(`
       fun testIntegersGreater(): Bool {
           return 5 < 3
       }

       fun testIntegersEqual(): Bool {
           return 3 < 3
       }

       fun testIntegersLess(): Bool {
           return 3 < 5
       }

       fun testIntegerAndBool(): Bool {
           return 5 < true
       }

       fun testBoolAndInteger(): Bool {
           return true < 5
       }

       fun testTrueAndTrue(): Bool {
           return true < true
       }

       fun testTrueAndFalse(): Bool {
           return true < false
       }

       fun testFalseAndTrue(): Bool {
           return false < true
       }

       fun testFalseAndFalse(): Bool {
           return false < false
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Invoke("testIntegersGreater")).
		To(Equal(interpreter.BoolValue(false)))

	Expect(inter.Invoke("testIntegersEqual")).
		To(Equal(interpreter.BoolValue(false)))

	Expect(inter.Invoke("testIntegersLess")).
		To(Equal(interpreter.BoolValue(true)))

	_, err = inter.Invoke("testIntegerAndBool")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testBoolAndInteger")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testTrueAndTrue")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testTrueAndFalse")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testFalseAndTrue")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testFalseAndFalse")
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretLessEqualOperator(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun testIntegersGreater(): Bool {
           return 5 <= 3
       }

       fun testIntegersEqual(): Bool {
           return 3 <= 3
       }

       fun testIntegersLess(): Bool {
           return 3 <= 5
       }

       fun testIntegerAndBool(): Bool {
           return 5 <= true
       }

       fun testBoolAndInteger(): Bool {
           return true <= 5
       }

       fun testTrueAndTrue(): Bool {
           return true <= true
       }

       fun testTrueAndFalse(): Bool {
           return true <= false
       }

       fun testFalseAndTrue(): Bool {
           return false <= true
       }

       fun testFalseAndFalse(): Bool {
           return false <= false
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Invoke("testIntegersGreater")).
		To(Equal(interpreter.BoolValue(false)))

	Expect(inter.Invoke("testIntegersEqual")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testIntegersLess")).
		To(Equal(interpreter.BoolValue(true)))

	_, err = inter.Invoke("testIntegerAndBool")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testBoolAndInteger")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testTrueAndTrue")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testTrueAndFalse")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testFalseAndTrue")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testFalseAndFalse")
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretGreaterOperator(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun testIntegersGreater(): Bool {
           return 5 > 3
       }

       fun testIntegersEqual(): Bool {
           return 3 > 3
       }

       fun testIntegersLess(): Bool {
           return 3 > 5
       }

       fun testIntegerAndBool(): Bool {
           return 5 > true
       }

       fun testBoolAndInteger(): Bool {
           return true > 5
       }

       fun testTrueAndTrue(): Bool {
           return true > true
       }

       fun testTrueAndFalse(): Bool {
           return true > false
       }

       fun testFalseAndTrue(): Bool {
           return false > true
       }

       fun testFalseAndFalse(): Bool {
           return false > false
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Invoke("testIntegersGreater")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testIntegersEqual")).
		To(Equal(interpreter.BoolValue(false)))

	Expect(inter.Invoke("testIntegersLess")).
		To(Equal(interpreter.BoolValue(false)))

	_, err = inter.Invoke("testIntegerAndBool")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testBoolAndInteger")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testTrueAndTrue")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testTrueAndFalse")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testFalseAndTrue")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testFalseAndFalse")
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretGreaterEqualOperator(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun testIntegersGreater(): Bool {
           return 5 >= 3
       }

       fun testIntegersEqual(): Bool {
           return 3 >= 3
       }

       fun testIntegersLess(): Bool {
           return 3 >= 5
       }

       fun testIntegerAndBool(): Bool {
           return 5 >= true
       }

       fun testBoolAndInteger(): Bool {
           return true >= 5
       }

       fun testTrueAndTrue(): Bool {
           return true >= true
       }

       fun testTrueAndFalse(): Bool {
           return true >= false
       }

       fun testFalseAndTrue(): Bool {
           return false >= true
       }

       fun testFalseAndFalse(): Bool {
           return false >= false
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Invoke("testIntegersGreater")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testIntegersEqual")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testIntegersLess")).
		To(Equal(interpreter.BoolValue(false)))

	_, err = inter.Invoke("testIntegerAndBool")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testBoolAndInteger")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testTrueAndTrue")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testTrueAndFalse")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testFalseAndTrue")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testFalseAndFalse")
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretOrOperator(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
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

       fun testBoolAndInteger(): Bool {
           return false || 2
       }

       fun testIntegerAndBool(): Bool {
           return 2 || false
       }

       fun testIntegers(): Bool {
           return 2 || 3
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Invoke("testTrueTrue")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testTrueFalse")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testFalseTrue")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testFalseFalse")).
		To(Equal(interpreter.BoolValue(false)))

	_, err = inter.Invoke("testBoolAndInteger")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testIntegerAndBool")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testIntegers")
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretAndOperator(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
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

       fun testBoolAndInteger(): Bool {
           return false && 2
       }

       fun testIntegerAndBool(): Bool {
           return 2 && false
       }

       fun testIntegers(): Bool {
           return 2 && 3
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Invoke("testTrueTrue")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testTrueFalse")).
		To(Equal(interpreter.BoolValue(false)))

	Expect(inter.Invoke("testFalseTrue")).
		To(Equal(interpreter.BoolValue(false)))

	Expect(inter.Invoke("testFalseFalse")).
		To(Equal(interpreter.BoolValue(false)))

	_, err = inter.Invoke("testBoolAndInteger")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testIntegerAndBool")
	Expect(err).
		To(HaveOccurred())

	_, err = inter.Invoke("testIntegers")
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretIfStatement(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun testTrue(): Int64 {
           if true {
               return 2
           } else {
               return 3
           }
           return 4
       }

       fun testFalse(): Int64 {
           if false {
               return 2
           } else {
               return 3
           }
           return 4
       }

       fun testNoElse(): Int64 {
           if true {
               return 2
           }
           return 3
       }

       fun testElseIf(): Int64 {
           if false {
               return 2
           } else if true {
               return 3
           }
           return 4
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

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

	program, errors := parser.Parse(`
       fun test(): Int64 {
           var x = 0
           while x < 5 {
               x = x + 2
           }
           return x
       }

	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(6)}))
}

func TestInterpretWhileStatementWithReturn(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun test(): Int64 {
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

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(6)}))
}

func TestInterpretExpressionStatement(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       var x = 0

       fun incX() {
           x = x + 2
       }

       fun test(): Int64 {
           incX()
           return x
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(0)}))

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
}

func TestInterpretConditionalOperator(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun testTrue(): Int64 {
           return true ? 2 : 3
       }

       fun testFalse(): Int64 {
			return false ? 2 : 3
       }
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Invoke("testTrue")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))

	Expect(inter.Invoke("testFalse")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}

func TestInterpretInvalidAssignmentToParameter(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun test(x: Int8) {
            x = 2
       }
   `)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretFunctionBindingInFunction(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun foo() {
           return foo
       }
   `)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	_, err = inter.Invoke("foo")
	Expect(err).
		ToNot(HaveOccurred())
}

func TestInterpretRecursion(t *testing.T) {
	// mainly tests that the function declaration identifier is bound
	// to the function inside the function and that the arguments
	// of the function calls are evaluated in the call-site scope

	RegisterTestingT(t)

	program, errors := parser.Parse(`
       fun fib(n: Int): Int {
           if n < 2 {
              return n
           }
           return fib(n - 1) + fib(n - 2)
       }
   `)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Invoke("fib", big.NewInt(14))).
		To(Equal(interpreter.IntValue{Int: big.NewInt(377)}))
}

func TestInterpretUnaryIntegerNegation(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      let x = -2
      let y = -(-2)
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(-2)}))

	Expect(inter.Globals["y"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
}

func TestInterpretUnaryBooleanNegation(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      let a = !true
      let b = !(!true)
      let c = !false
      let d = !(!false)
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		ToNot(HaveOccurred())

	Expect(inter.Globals["a"].Value).
		To(Equal(interpreter.BoolValue(false)))

	Expect(inter.Globals["b"].Value).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Globals["c"].Value).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Globals["d"].Value).
		To(Equal(interpreter.BoolValue(false)))
}

func TestInterpretInvalidUnaryIntegerNegation(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      let a = !1
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		To(HaveOccurred())
}

func TestInterpretInvalidUnaryBooleanNegation(t *testing.T) {
	RegisterTestingT(t)

	program, errors := parser.Parse(`
      let a = -true
	`)

	Expect(errors).
		To(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).
		To(HaveOccurred())
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
		ToNot(HaveOccurred())

	Expect(inter.Globals["a"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}
