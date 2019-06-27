package strictus

import (
	"bamboo-runtime/execution/strictus/interpreter"
	. "github.com/onsi/gomega"
	"math/big"
	"testing"
)

func TestInterpretInvalidUnknownDeclarationInvocation(t *testing.T) {
	RegisterTestingT(t)

	program := parse(``)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(func() { inter.Invoke("test") }).Should(Panic())
}

func TestInterpretInvalidNonFunctionDeclarationInvocation(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        const test = 1
    `)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(func() { inter.Invoke("test") }).Should(Panic())
}

func TestInterpretInvalidUnknownDeclaration(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        fun test() {
            return x
        }
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(func() { inter.Invoke("test") }).Should(Panic())
}

func TestInterpretInvalidUnknownDeclarationAssignment(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        fun test() {
            x = 2
        }
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(func() { inter.Invoke("test") }).Should(Panic())
}

func TestInterpretInvalidUnknownDeclarationIndexing(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        fun test() {
            x[0]
        }
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(func() { inter.Invoke("test") }).Should(Panic())
}

func TestInterpretInvalidUnknownDeclarationIndexingAssignment(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        fun test() {
            x[0] = 2
        }
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(func() { inter.Invoke("test") }).Should(Panic())
}

func TestInterpretLexicalScope(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
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

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Globals["x"].Value).To(Equal(interpreter.IntValue{Int: big.NewInt(10)}))
	Expect(inter.Invoke("f")).To(Equal(interpreter.IntValue{Int: big.NewInt(10)}))
	Expect(inter.Invoke("g")).To(Equal(interpreter.IntValue{Int: big.NewInt(10)}))
}

func TestInterpretNoHoisting(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        const x = 2

        fun test(): Int64 {
           if x == 0 {
               const x = 3
               return x
           }
           return x
        }
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Invoke("test")).To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
	Expect(inter.Globals["x"].Value).To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
}

func TestInterpretFunctionExpressionsAndScope(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        const x = 10

        // check first-class functions and scope inside them
        const y = (fun (x: Int32): Int32 { return x })(42)
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Globals["x"].Value).To(Equal(interpreter.IntValue{Int: big.NewInt(10)}))
	Expect(inter.Globals["y"].Value).To(Equal(interpreter.IntValue{Int: big.NewInt(42)}))
}

func TestInterpretInvalidFunctionCallWithTooFewArguments(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        fun f(x: Int32): Int32 {
            return x
        }

        fun test(): Int32 {
            return f()
        }
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(func() { inter.Invoke("test") }).Should(Panic())
}

func TestInterpretInvalidFunctionCallWithTooManyArguments(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        fun f(x: Int32): Int32 {
            return x
        }

        fun test(): Int32 {
            return f(2, 3)
        }
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(func() { inter.Invoke("test") }).Should(Panic())
}

func TestInterpretInvalidFunctionCallOfBool(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        fun test(): Int32 {
            return true()
        }
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(func() { inter.Invoke("test") }).Should(Panic())
}

func TestInterpretInvalidFunctionCallOfInteger(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        fun test(): Int32 {
            return 2()
        }
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(func() { inter.Invoke("test") }).Should(Panic())
}

func TestInterpretInvalidConstantAssignment(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        fun test() {
            const x = 2
            x = 3
        }
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(func() { inter.Invoke("test") }).Should(Panic())
}

func TestInterpretVariableAssignment(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        fun test(): Int64 {
            var x = 2
            x = 3
            return x
        }
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Invoke("test")).To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}

func TestInterpretInvalidGlobalConstantAssignment(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        const x = 2

        fun test() {
            x = 3
        }
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(func() { inter.Invoke("test") }).Should(Panic())
}

func TestInterpretGlobalVariableAssignment(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        var x = 2

        fun test(): Int64 {
            x = 3
            return x
        }
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Globals["x"].Value).To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
	Expect(inter.Invoke("test")).To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
	Expect(inter.Globals["x"].Value).To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}

func TestInterpretInvalidConstantRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        fun test() {
            const x = 2
            const x = 3
        }
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(func() { inter.Invoke("test") }).Should(Panic())
}

func TestInterpretInvalidGlobalConstantRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	Expect(func() {
		parse(`
            const x = 2
            const x = 3
		`)
	}).Should(Panic())
}

func TestInterpretConstantRedeclaration(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
	    const x = 2

        fun test(): Int64 {
            const x = 3
            return x
        }
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Globals["x"].Value).To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
	Expect(inter.Invoke("test")).To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}

func TestInterpretParameters(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        fun returnA(a: Int32, b: Int32): Int64 {
            return a
        }

        fun returnB(a: Int32, b: Int32): Int64 {
            return b
        }
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Invoke("returnA", int64(24), int64(42))).To(Equal(interpreter.Int64Value(24)))
	Expect(inter.Invoke("returnB", int64(24), int64(42))).To(Equal(interpreter.Int64Value(42)))
}

func TestInterpretArrayIndexing(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        fun test(): Int64 {
            const z = [0, 3]
            return z[1]
        }
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Invoke("test")).To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}

func TestInterpretInvalidArrayIndexingWithBool(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        fun test(): Int64 {
            const z = [0, 3]
            return z[true]
        }
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(func() { inter.Invoke("test") }).Should(Panic())
}

func TestInterpretInvalidArrayIndexingIntoBool(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        fun test(): Int64 {
            return true[0]
        }
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(func() { inter.Invoke("test") }).Should(Panic())
}

func TestInterpretInvalidArrayIndexingIntoInteger(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        fun test(): Int64 {
            return 2[0]
        }
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(func() { inter.Invoke("test") }).Should(Panic())
}

func TestInterpretArrayIndexingAssignment(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        fun test(): Int64 {
            const z = [0, 3]
            z[1] = 2
            return z[1]
        }
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Invoke("test")).To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
}

func TestInterpretInvalidArrayIndexingAssignmentWithBool(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        fun test(): Int64 {
            const z = [0, 3]
            z[true] = 2
            return z[1]
        }
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(func() { inter.Invoke("test") }).Should(Panic())
}

func TestInterpretReturnWithoutExpression(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        fun returnEarly() {
            return
            return 1
        }
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Invoke("returnEarly")).To(Equal(interpreter.VoidValue{}))
}

// TODO: perform each operator test for each integer type

func TestInterpretPlusOperator(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
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

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Invoke("testIntegers")).To(Equal(interpreter.IntValue{Int: big.NewInt(6)}))
	Expect(func() { inter.Invoke("testIntegerAndBool") }).Should(Panic())
	Expect(func() { inter.Invoke("testBoolAndInteger") }).Should(Panic())
	Expect(func() { inter.Invoke("testBools") }).Should(Panic())
}

func TestInterpretMinusOperator(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
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

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Invoke("testIntegers")).To(Equal(interpreter.IntValue{Int: big.NewInt(-2)}))
	Expect(func() { inter.Invoke("testIntegerAndBool") }).Should(Panic())
	Expect(func() { inter.Invoke("testBoolAndInteger") }).Should(Panic())
	Expect(func() { inter.Invoke("testBools") }).Should(Panic())
}

func TestInterpretMulOperator(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
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

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Invoke("testIntegers")).To(Equal(interpreter.IntValue{Int: big.NewInt(8)}))
	Expect(func() { inter.Invoke("testIntegerAndBool") }).Should(Panic())
	Expect(func() { inter.Invoke("testBoolAndInteger") }).Should(Panic())
	Expect(func() { inter.Invoke("testBools") }).Should(Panic())
}

func TestInterpretDivOperator(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
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

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Invoke("testIntegers")).To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
	Expect(func() { inter.Invoke("testIntegerAndBool") }).Should(Panic())
	Expect(func() { inter.Invoke("testBoolAndInteger") }).Should(Panic())
	Expect(func() { inter.Invoke("testBools") }).Should(Panic())
}

func TestInterpretModOperator(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
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

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Invoke("testIntegers")).To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
	Expect(func() { inter.Invoke("testIntegerAndBool") }).Should(Panic())
	Expect(func() { inter.Invoke("testBoolAndInteger") }).Should(Panic())
	Expect(func() { inter.Invoke("testBools") }).Should(Panic())
}

func TestInterpretEqualOperator(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
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

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Invoke("testIntegersUnequal")).To(Equal(interpreter.BoolValue(false)))
	Expect(inter.Invoke("testIntegersEqual")).To(Equal(interpreter.BoolValue(true)))
	Expect(func() { inter.Invoke("testIntegerAndBool") }).Should(Panic())
	Expect(func() { inter.Invoke("testBoolAndInteger") }).Should(Panic())
	Expect(inter.Invoke("testTrueAndTrue")).To(Equal(interpreter.BoolValue(true)))
	Expect(inter.Invoke("testTrueAndFalse")).To(Equal(interpreter.BoolValue(false)))
	Expect(inter.Invoke("testFalseAndTrue")).To(Equal(interpreter.BoolValue(false)))
	Expect(inter.Invoke("testFalseAndFalse")).To(Equal(interpreter.BoolValue(true)))
}

func TestInterpretUnequalOperator(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
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

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Invoke("testIntegersUnequal")).To(Equal(interpreter.BoolValue(true)))
	Expect(inter.Invoke("testIntegersEqual")).To(Equal(interpreter.BoolValue(false)))
	Expect(func() { inter.Invoke("testIntegerAndBool") }).Should(Panic())
	Expect(func() { inter.Invoke("testBoolAndInteger") }).Should(Panic())
	Expect(inter.Invoke("testTrueAndTrue")).To(Equal(interpreter.BoolValue(false)))
	Expect(inter.Invoke("testTrueAndFalse")).To(Equal(interpreter.BoolValue(true)))
	Expect(inter.Invoke("testFalseAndTrue")).To(Equal(interpreter.BoolValue(true)))
	Expect(inter.Invoke("testFalseAndFalse")).To(Equal(interpreter.BoolValue(false)))
}

func TestInterpretLessOperator(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
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

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Invoke("testIntegersGreater")).To(Equal(interpreter.BoolValue(false)))
	Expect(inter.Invoke("testIntegersEqual")).To(Equal(interpreter.BoolValue(false)))
	Expect(inter.Invoke("testIntegersLess")).To(Equal(interpreter.BoolValue(true)))
	Expect(func() { inter.Invoke("testIntegerAndBool") }).Should(Panic())
	Expect(func() { inter.Invoke("testBoolAndInteger") }).Should(Panic())
	Expect(func() { inter.Invoke("testTrueAndTrue") }).Should(Panic())
	Expect(func() { inter.Invoke("testTrueAndFalse") }).Should(Panic())
	Expect(func() { inter.Invoke("testFalseAndTrue") }).Should(Panic())
	Expect(func() { inter.Invoke("testFalseAndFalse") }).Should(Panic())
}

func TestInterpretLessEqualOperator(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
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

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Invoke("testIntegersGreater")).To(Equal(interpreter.BoolValue(false)))
	Expect(inter.Invoke("testIntegersEqual")).To(Equal(interpreter.BoolValue(true)))
	Expect(inter.Invoke("testIntegersLess")).To(Equal(interpreter.BoolValue(true)))
	Expect(func() { inter.Invoke("testIntegerAndBool") }).Should(Panic())
	Expect(func() { inter.Invoke("testBoolAndInteger") }).Should(Panic())
	Expect(func() { inter.Invoke("testTrueAndTrue") }).Should(Panic())
	Expect(func() { inter.Invoke("testTrueAndFalse") }).Should(Panic())
	Expect(func() { inter.Invoke("testFalseAndTrue") }).Should(Panic())
	Expect(func() { inter.Invoke("testFalseAndFalse") }).Should(Panic())
}

func TestInterpretGreaterOperator(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
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

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Invoke("testIntegersGreater")).To(Equal(interpreter.BoolValue(true)))
	Expect(inter.Invoke("testIntegersEqual")).To(Equal(interpreter.BoolValue(false)))
	Expect(inter.Invoke("testIntegersLess")).To(Equal(interpreter.BoolValue(false)))
	Expect(func() { inter.Invoke("testIntegerAndBool") }).Should(Panic())
	Expect(func() { inter.Invoke("testBoolAndInteger") }).Should(Panic())
	Expect(func() { inter.Invoke("testTrueAndTrue") }).Should(Panic())
	Expect(func() { inter.Invoke("testTrueAndFalse") }).Should(Panic())
	Expect(func() { inter.Invoke("testFalseAndTrue") }).Should(Panic())
	Expect(func() { inter.Invoke("testFalseAndFalse") }).Should(Panic())
}

func TestInterpretGreaterEqualOperator(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
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

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Invoke("testIntegersGreater")).To(Equal(interpreter.BoolValue(true)))
	Expect(inter.Invoke("testIntegersEqual")).To(Equal(interpreter.BoolValue(true)))
	Expect(inter.Invoke("testIntegersLess")).To(Equal(interpreter.BoolValue(false)))
	Expect(func() { inter.Invoke("testIntegerAndBool") }).Should(Panic())
	Expect(func() { inter.Invoke("testBoolAndInteger") }).Should(Panic())
	Expect(func() { inter.Invoke("testTrueAndTrue") }).Should(Panic())
	Expect(func() { inter.Invoke("testTrueAndFalse") }).Should(Panic())
	Expect(func() { inter.Invoke("testFalseAndTrue") }).Should(Panic())
	Expect(func() { inter.Invoke("testFalseAndFalse") }).Should(Panic())
}

func TestInterpretOrOperator(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
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

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Invoke("testTrueTrue")).To(Equal(interpreter.BoolValue(true)))
	Expect(inter.Invoke("testTrueFalse")).To(Equal(interpreter.BoolValue(true)))
	Expect(inter.Invoke("testFalseTrue")).To(Equal(interpreter.BoolValue(true)))
	Expect(inter.Invoke("testFalseFalse")).To(Equal(interpreter.BoolValue(false)))
	Expect(func() { inter.Invoke("testBoolAndInteger") }).Should(Panic())
	Expect(func() { inter.Invoke("testIntegerAndBool") }).Should(Panic())
	Expect(func() { inter.Invoke("testIntegers") }).Should(Panic())
}

func TestInterpretAndOperator(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
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

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Invoke("testTrueTrue")).To(Equal(interpreter.BoolValue(true)))
	Expect(inter.Invoke("testTrueFalse")).To(Equal(interpreter.BoolValue(false)))
	Expect(inter.Invoke("testFalseTrue")).To(Equal(interpreter.BoolValue(false)))
	Expect(inter.Invoke("testFalseFalse")).To(Equal(interpreter.BoolValue(false)))
	Expect(func() { inter.Invoke("testBoolAndInteger") }).Should(Panic())
	Expect(func() { inter.Invoke("testIntegerAndBool") }).Should(Panic())
	Expect(func() { inter.Invoke("testIntegers") }).Should(Panic())
}

func TestInterpretIfStatement(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
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

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Invoke("testTrue")).To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
	Expect(inter.Invoke("testFalse")).To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
	Expect(inter.Invoke("testNoElse")).To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
	Expect(inter.Invoke("testElseIf")).To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}

func TestInterpretWhileStatement(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        fun test(): Int64 {
            var x = 0
            while x < 5 {
                x = x + 2
            }
            return x
        }

	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Invoke("test")).To(Equal(interpreter.IntValue{Int: big.NewInt(6)}))
}

func TestInterpretWhileStatementWithReturn(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
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

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Invoke("test")).To(Equal(interpreter.IntValue{Int: big.NewInt(6)}))
}

func TestInterpretExpressionStatement(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        var x = 0

        fun incX() {
            x = x + 2
        }

        fun test(): Int64 {
            incX()
            return x
        }
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Globals["x"].Value).To(Equal(interpreter.IntValue{Int: big.NewInt(0)}))
	Expect(inter.Invoke("test")).To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
	Expect(inter.Globals["x"].Value).To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
}

func TestInterpretConditionalOperator(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        fun testTrue(): Int64 {
            return true ? 2 : 3
        }

        fun testFalse(): Int64 {
			return false ? 2 : 3
        }
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Invoke("testTrue")).To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
	Expect(inter.Invoke("testFalse")).To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}

func TestInterpretInvalidAssignmentToParameter(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        fun test(x: Int8) {
             x = 2
        }
    `)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(func() { inter.Invoke("test") }).Should(Panic())
}

func TestInterpretFunctionBindingInFunction(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
        fun foo() {
            return foo
        }
    `)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	inter.Invoke("foo")
}

func TestInterpretRecursion(t *testing.T) {
	// mainly tests that the function declaration identifier is bound
	// to the function inside the function and that the arguments
	// of the function calls are evaluated in the call-site scope

	RegisterTestingT(t)

	program := parse(`
        fun fib(n: Int): Int {
            if n < 2 {
               return n
            }
            return fib(n - 1) + fib(n - 2)
        }
    `)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Invoke("fib", big.NewInt(23))).
		To(Equal(interpreter.IntValue{Int: big.NewInt(28657)}))
}

func TestInterpretUnaryIntegerNegation(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
       const x = -2
       const y = -(-2)
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Globals["x"].Value).To(Equal(interpreter.IntValue{Int: big.NewInt(-2)}))
	Expect(inter.Globals["y"].Value).To(Equal(interpreter.IntValue{Int: big.NewInt(2)}))
}

func TestInterpretUnaryBooleanNegation(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
       const a = !true
       const b = !(!true)
       const c = !false
       const d = !(!false)
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	Expect(inter.Globals["a"].Value).To(Equal(interpreter.BoolValue(false)))
	Expect(inter.Globals["b"].Value).To(Equal(interpreter.BoolValue(true)))
	Expect(inter.Globals["c"].Value).To(Equal(interpreter.BoolValue(true)))
	Expect(inter.Globals["d"].Value).To(Equal(interpreter.BoolValue(false)))
}

func TestInterpretInvalidUnaryIntegerNegation(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
       const a = !1
	`)

	inter := interpreter.NewInterpreter(program)
	Expect(func() { inter.Interpret() }).Should(Panic())
}

func TestInterpretInvalidUnaryBooleanNegation(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
       const a = -true
	`)

	inter := interpreter.NewInterpreter(program)
	Expect(func() { inter.Interpret() }).Should(Panic())
}

func TestInterpretHostFunction(t *testing.T) {
	RegisterTestingT(t)

	program := parse(`
       const a = test(1, 2)
	`)

	inter := interpreter.NewInterpreter(program)
	inter.ImportFunction(
		"test",
		interpreter.FunctionType{
			ParameterTypes: []interpreter.Type{
				interpreter.IntType{},
				interpreter.IntType{},
			},
			ReturnType: interpreter.IntType{},
		},
		func(inter *interpreter.Interpreter, arguments []interpreter.Value) interpreter.Value {
			a := arguments[0].(interpreter.IntValue).Int
			b := arguments[1].(interpreter.IntValue).Int
			result := big.NewInt(0).Add(a, b)
			return interpreter.IntValue{Int: result}
		},
	)
	inter.Interpret()
	Expect(inter.Globals["a"].Value).To(Equal(interpreter.IntValue{Int: big.NewInt(3)}))
}
