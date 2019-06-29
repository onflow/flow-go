package strictus

import (
	"bamboo-runtime/execution/strictus/interpreter"
	. "github.com/onsi/gomega"
	"math/big"
	"testing"
)

func TestInterpretInvalidUnknownDeclarationInvocation(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(``)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretInvalidNonFunctionDeclarationInvocation(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       const test = 1
   `)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretInvalidUnknownDeclaration(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       fun test() {
           return x
       }
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretInvalidUnknownDeclarationAssignment(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       fun test() {
           x = 2
       }
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretInvalidUnknownDeclarationIndexing(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       fun test() {
           x[0]
       }
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretInvalidUnknownDeclarationIndexingAssignment(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       fun test() {
           x[0] = 2
       }
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretLexicalScope(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       const x = 10

       fun f(): Int32 {
          // check resolution
          return x
       }

       fun g(): Int32 {
          // check scope is lexical, not dynamic
          const x = 20
          return f()
       }
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(10)}))

	Expect(inter.Invoke("f")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(10)}))

	Expect(inter.Invoke("g")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(10)}))
}

func TestInterpretNoHoisting(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       const x = 2

       fun test(): Int64 {
          if x == 0 {
              const x = 3
              return x
          }
          return x
       }
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
}

func TestInterpretFunctionExpressionsAndScope(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       const x = 10

       // check first-class functions and scope inside them
       const y = (fun (x: Int32): Int32 { return x })(42)
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(10)}))

	Expect(inter.Globals["y"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(42)}))
}

func TestInterpretInvalidFunctionCallWithTooFewArguments(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       fun f(x: Int32): Int32 {
           return x
       }

       fun test(): Int32 {
           return f()
       }
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretInvalidFunctionCallWithTooManyArguments(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       fun f(x: Int32): Int32 {
           return x
       }

       fun test(): Int32 {
           return f(2, 3)
       }
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretInvalidFunctionCallOfBool(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       fun test(): Int32 {
           return true()
       }
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretInvalidFunctionCallOfInteger(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       fun test(): Int32 {
           return 2()
       }
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretInvalidConstantAssignment(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       fun test() {
           const x = 2
           x = 3
       }
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretVariableAssignment(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       fun test(): Int64 {
           var x = 2
           x = 3
           return x
       }
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}

func TestInterpretInvalidGlobalConstantAssignment(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       const x = 2

       fun test() {
           x = 3
       }
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretGlobalVariableAssignment(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       var x = 2

       fun test(): Int64 {
           x = 3
           return x
       }
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}

func TestInterpretInvalidConstantRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       fun test() {
           const x = 2
           const x = 3
       }
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretInvalidGlobalConstantRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
		const x = 2
		const x = 3
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()

	Expect(err).Should(HaveOccurred())
}

func TestInterpretConstantRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
	    const x = 2

       fun test(): Int64 {
           const x = 3
           return x
       }
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Globals["x"].Value).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}

func TestInterpretParameters(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       fun returnA(a: Int32, b: Int32): Int64 {
           return a
       }

       fun returnB(a: Int32, b: Int32): Int64 {
           return b
       }
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Invoke("returnA", int64(24), int64(42))).
		To(Equal(interpreter.Int64Value(24)))

	Expect(inter.Invoke("returnB", int64(24), int64(42))).
		To(Equal(interpreter.Int64Value(42)))
}

func TestInterpretArrayIndexing(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       fun test(): Int64 {
           const z = [0, 3]
           return z[1]
       }
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}

func TestInterpretInvalidArrayIndexingWithBool(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       fun test(): Int64 {
           const z = [0, 3]
           return z[true]
       }
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretInvalidArrayIndexingIntoBool(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       fun test(): Int64 {
           return true[0]
       }
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretInvalidArrayIndexingIntoInteger(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       fun test(): Int64 {
           return 2[0]
       }
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretArrayIndexingAssignment(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       fun test(): Int64 {
           const z = [0, 3]
           z[1] = 2
           return z[1]
       }
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Invoke("test")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
}

func TestInterpretInvalidArrayIndexingAssignmentWithBool(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       fun test(): Int64 {
           const z = [0, 3]
           z[true] = 2
           return z[1]
       }
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretReturnWithoutExpression(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       fun returnEarly() {
           return
           return 1
       }
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Invoke("returnEarly")).
		To(Equal(interpreter.VoidValue{}))
}

// TODO: perform each operator test for each integer type

func TestInterpretPlusOperator(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
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

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Invoke("testIntegers")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(6)}))

	_, err = inter.Invoke("testIntegerAndBool")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testBoolAndInteger")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testBools")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretMinusOperator(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
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

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Invoke("testIntegers")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(-2)}))

	_, err = inter.Invoke("testIntegerAndBool")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testBoolAndInteger")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testBools")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretMulOperator(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
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

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Invoke("testIntegers")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(8)}))

	_, err = inter.Invoke("testIntegerAndBool")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testBoolAndInteger")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testBools")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretDivOperator(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
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

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Invoke("testIntegers")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))

	_, err = inter.Invoke("testIntegerAndBool")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testBoolAndInteger")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testBools")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretModOperator(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
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

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Invoke("testIntegers")).
		To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))

	_, err = inter.Invoke("testIntegerAndBool")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testBoolAndInteger")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testBools")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretEqualOperator(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
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

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Invoke("testIntegersUnequal")).
		To(Equal(interpreter.BoolValue(false)))

	Expect(inter.Invoke("testIntegersEqual")).
		To(Equal(interpreter.BoolValue(true)))

	_, err = inter.Invoke("testIntegerAndBool")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testBoolAndInteger")
	Expect(err).Should(HaveOccurred())

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

	program, errors := Parse(`
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

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Invoke("testIntegersUnequal")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testIntegersEqual")).
		To(Equal(interpreter.BoolValue(false)))

	_, err = inter.Invoke("testIntegerAndBool")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testBoolAndInteger")
	Expect(err).Should(HaveOccurred())

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

	program, errors := Parse(`
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

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Invoke("testIntegersGreater")).
		To(Equal(interpreter.BoolValue(false)))

	Expect(inter.Invoke("testIntegersEqual")).
		To(Equal(interpreter.BoolValue(false)))

	Expect(inter.Invoke("testIntegersLess")).
		To(Equal(interpreter.BoolValue(true)))

	_, err = inter.Invoke("testIntegerAndBool")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testBoolAndInteger")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testTrueAndTrue")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testTrueAndFalse")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testFalseAndTrue")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testFalseAndFalse")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretLessEqualOperator(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
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

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Invoke("testIntegersGreater")).
		To(Equal(interpreter.BoolValue(false)))

	Expect(inter.Invoke("testIntegersEqual")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testIntegersLess")).
		To(Equal(interpreter.BoolValue(true)))

	_, err = inter.Invoke("testIntegerAndBool")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testBoolAndInteger")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testTrueAndTrue")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testTrueAndFalse")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testFalseAndTrue")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testFalseAndFalse")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretGreaterOperator(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
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

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Invoke("testIntegersGreater")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testIntegersEqual")).
		To(Equal(interpreter.BoolValue(false)))

	Expect(inter.Invoke("testIntegersLess")).
		To(Equal(interpreter.BoolValue(false)))

	_, err = inter.Invoke("testIntegerAndBool")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testBoolAndInteger")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testTrueAndTrue")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testTrueAndFalse")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testFalseAndTrue")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testFalseAndFalse")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretGreaterEqualOperator(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
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

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Invoke("testIntegersGreater")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testIntegersEqual")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testIntegersLess")).
		To(Equal(interpreter.BoolValue(false)))

	_, err = inter.Invoke("testIntegerAndBool")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testBoolAndInteger")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testTrueAndTrue")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testTrueAndFalse")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testFalseAndTrue")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testFalseAndFalse")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretOrOperator(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
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

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Invoke("testTrueTrue")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testTrueFalse")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testFalseTrue")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testFalseFalse")).
		To(Equal(interpreter.BoolValue(false)))

	_, err = inter.Invoke("testBoolAndInteger")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testIntegerAndBool")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testIntegers")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretAndOperator(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
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

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Invoke("testTrueTrue")).
		To(Equal(interpreter.BoolValue(true)))

	Expect(inter.Invoke("testTrueFalse")).
		To(Equal(interpreter.BoolValue(false)))

	Expect(inter.Invoke("testFalseTrue")).
		To(Equal(interpreter.BoolValue(false)))

	Expect(inter.Invoke("testFalseFalse")).
		To(Equal(interpreter.BoolValue(false)))

	_, err = inter.Invoke("testBoolAndInteger")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testIntegerAndBool")
	Expect(err).Should(HaveOccurred())

	_, err = inter.Invoke("testIntegers")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretIfStatement(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
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

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Invoke("testTrue")).To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
	Expect(inter.Invoke("testFalse")).To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
	Expect(inter.Invoke("testNoElse")).To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
	Expect(inter.Invoke("testElseIf")).To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}

func TestInterpretWhileStatement(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       fun test(): Int64 {
           var x = 0
           while x < 5 {
               x = x + 2
           }
           return x
       }

	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Invoke("test")).To(Equal(interpreter.IntValue{Int: big.NewInt(6)}))
}

func TestInterpretWhileStatementWithReturn(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
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

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Invoke("test")).To(Equal(interpreter.IntValue{Int: big.NewInt(6)}))
}

func TestInterpretExpressionStatement(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       var x = 0

       fun incX() {
           x = x + 2
       }

       fun test(): Int64 {
           incX()
           return x
       }
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Globals["x"].Value).To(Equal(interpreter.IntValue{Int: big.NewInt(0)}))
	Expect(inter.Invoke("test")).To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
	Expect(inter.Globals["x"].Value).To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
}

func TestInterpretConditionalOperator(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       fun testTrue(): Int64 {
           return true ? 2 : 3
       }

       fun testFalse(): Int64 {
			return false ? 2 : 3
       }
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Invoke("testTrue")).To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
	Expect(inter.Invoke("testFalse")).To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}

func TestInterpretInvalidAssignmentToParameter(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       fun test(x: Int8) {
            x = 2
       }
   `)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	_, err = inter.Invoke("test")
	Expect(err).Should(HaveOccurred())
}

func TestInterpretFunctionBindingInFunction(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
       fun foo() {
           return foo
       }
   `)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	_, err = inter.Invoke("foo")
	Expect(err).ShouldNot(HaveOccurred())
}

func TestInterpretRecursion(t *testing.T) {
	// mainly tests that the function declaration identifier is bound
	// to the function inside the function and that the arguments
	// of the function calls are evaluated in the call-site scope

	RegisterTestingT(t)

	program, errors := Parse(`
       fun fib(n: Int): Int {
           if n < 2 {
              return n
           }
           return fib(n - 1) + fib(n - 2)
       }
   `)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Invoke("fib", big.NewInt(23))).
		To(Equal(interpreter.IntValue{Int: big.NewInt(28657)}))
}

func TestInterpretUnaryIntegerNegation(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
      const x = -2
      const y = -(-2)
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Globals["x"].Value).To(Equal(interpreter.IntValue{Int: big.NewInt(-2)}))
	Expect(inter.Globals["y"].Value).To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
}

func TestInterpretUnaryBooleanNegation(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
      const a = !true
      const b = !(!true)
      const c = !false
      const d = !(!false)
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Globals["a"].Value).To(Equal(interpreter.BoolValue(false)))
	Expect(inter.Globals["b"].Value).To(Equal(interpreter.BoolValue(true)))
	Expect(inter.Globals["c"].Value).To(Equal(interpreter.BoolValue(true)))
	Expect(inter.Globals["d"].Value).To(Equal(interpreter.BoolValue(false)))
}

func TestInterpretInvalidUnaryIntegerNegation(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
      const a = !1
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).Should(HaveOccurred())
}

func TestInterpretInvalidUnaryBooleanNegation(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
      const a = -true
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)
	err := inter.Interpret()
	Expect(err).Should(HaveOccurred())
}

func TestInterpretHostFunction(t *testing.T) {
	RegisterTestingT(t)

	program, errors := Parse(`
      const a = test(1, 2)
	`)

	Expect(errors).Should(BeEmpty())

	inter := interpreter.NewInterpreter(program)

	testFunction := interpreter.NewHostFunction(
		&interpreter.FunctionType{
			ParameterTypes: []interpreter.Type{
				&interpreter.IntType{},
				&interpreter.IntType{},
			},
			ReturnType: &interpreter.IntType{},
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
	Expect(err).ShouldNot(HaveOccurred())

	Expect(inter.Globals["a"].Value).To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}
