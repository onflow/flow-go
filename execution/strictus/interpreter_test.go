package strictus

import (
	. "bamboo-runtime/execution/strictus/ast"
	"bamboo-runtime/execution/strictus/interpreter"
	. "github.com/onsi/gomega"
	"testing"
)

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
	Expect(inter.Globals["x"].Value).To(Equal(Int64Expression(10)))
	Expect(inter.Invoke("f")).To(Equal(Int64Expression(10)))
	Expect(inter.Invoke("g")).To(Equal(Int64Expression(10)))
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
	Expect(inter.Globals["x"].Value).To(Equal(Int64Expression(10)))
	Expect(inter.Globals["y"].Value).To(Equal(Int64Expression(42)))
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
	Expect(inter.Invoke("test")).To(Equal(Int64Expression(3)))
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
	Expect(inter.Globals["x"].Value).To(Equal(Int64Expression(2)))
	Expect(inter.Invoke("test")).To(Equal(Int64Expression(3)))
	Expect(inter.Globals["x"].Value).To(Equal(Int64Expression(3)))
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
	Expect(inter.Globals["x"].Value).To(Equal(Int64Expression(2)))
	Expect(inter.Invoke("test")).To(Equal(Int64Expression(3)))
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
	Expect(inter.Invoke("returnA", int64(24), int64(42))).To(Equal(Int64Expression(24)))
	Expect(inter.Invoke("returnB", int64(24), int64(42))).To(Equal(Int64Expression(42)))
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
	Expect(inter.Invoke("test")).To(Equal(Int64Expression(3)))
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
	Expect(inter.Invoke("test")).To(Equal(Int64Expression(2)))
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
	Expect(inter.Invoke("returnEarly")).To(Equal(interpreter.Void{}))
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
	Expect(inter.Invoke("testIntegers")).To(Equal(Int64Expression(6)))
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
	Expect(inter.Invoke("testIntegers")).To(Equal(Int64Expression(-2)))
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
	Expect(inter.Invoke("testIntegers")).To(Equal(Int64Expression(8)))
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
	Expect(inter.Invoke("testIntegers")).To(Equal(Int64Expression(2)))
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
	Expect(inter.Invoke("testIntegers")).To(Equal(Int64Expression(2)))
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
	Expect(inter.Invoke("testIntegersUnequal")).To(Equal(BoolExpression(false)))
	Expect(inter.Invoke("testIntegersEqual")).To(Equal(BoolExpression(true)))
	Expect(func() { inter.Invoke("testIntegerAndBool") }).Should(Panic())
	Expect(func() { inter.Invoke("testBoolAndInteger") }).Should(Panic())
	Expect(inter.Invoke("testTrueAndTrue")).To(Equal(BoolExpression(true)))
	Expect(inter.Invoke("testTrueAndFalse")).To(Equal(BoolExpression(false)))
	Expect(inter.Invoke("testFalseAndTrue")).To(Equal(BoolExpression(false)))
	Expect(inter.Invoke("testFalseAndFalse")).To(Equal(BoolExpression(true)))
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
	Expect(inter.Invoke("testIntegersUnequal")).To(Equal(BoolExpression(true)))
	Expect(inter.Invoke("testIntegersEqual")).To(Equal(BoolExpression(false)))
	Expect(func() { inter.Invoke("testIntegerAndBool") }).Should(Panic())
	Expect(func() { inter.Invoke("testBoolAndInteger") }).Should(Panic())
	Expect(inter.Invoke("testTrueAndTrue")).To(Equal(BoolExpression(false)))
	Expect(inter.Invoke("testTrueAndFalse")).To(Equal(BoolExpression(true)))
	Expect(inter.Invoke("testFalseAndTrue")).To(Equal(BoolExpression(true)))
	Expect(inter.Invoke("testFalseAndFalse")).To(Equal(BoolExpression(false)))
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
	Expect(inter.Invoke("testIntegersGreater")).To(Equal(BoolExpression(false)))
	Expect(inter.Invoke("testIntegersEqual")).To(Equal(BoolExpression(false)))
	Expect(inter.Invoke("testIntegersLess")).To(Equal(BoolExpression(true)))
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
	Expect(inter.Invoke("testIntegersGreater")).To(Equal(BoolExpression(false)))
	Expect(inter.Invoke("testIntegersEqual")).To(Equal(BoolExpression(true)))
	Expect(inter.Invoke("testIntegersLess")).To(Equal(BoolExpression(true)))
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
	Expect(inter.Invoke("testIntegersGreater")).To(Equal(BoolExpression(true)))
	Expect(inter.Invoke("testIntegersEqual")).To(Equal(BoolExpression(false)))
	Expect(inter.Invoke("testIntegersLess")).To(Equal(BoolExpression(false)))
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
	Expect(inter.Invoke("testIntegersGreater")).To(Equal(BoolExpression(true)))
	Expect(inter.Invoke("testIntegersEqual")).To(Equal(BoolExpression(true)))
	Expect(inter.Invoke("testIntegersLess")).To(Equal(BoolExpression(false)))
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
	Expect(inter.Invoke("testTrueTrue")).To(Equal(BoolExpression(true)))
	Expect(inter.Invoke("testTrueFalse")).To(Equal(BoolExpression(true)))
	Expect(inter.Invoke("testFalseTrue")).To(Equal(BoolExpression(true)))
	Expect(inter.Invoke("testFalseFalse")).To(Equal(BoolExpression(false)))
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
	Expect(inter.Invoke("testTrueTrue")).To(Equal(BoolExpression(true)))
	Expect(inter.Invoke("testTrueFalse")).To(Equal(BoolExpression(false)))
	Expect(inter.Invoke("testFalseTrue")).To(Equal(BoolExpression(false)))
	Expect(inter.Invoke("testFalseFalse")).To(Equal(BoolExpression(false)))
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
	Expect(inter.Invoke("testTrue")).To(Equal(Int64Expression(2)))
	Expect(inter.Invoke("testFalse")).To(Equal(Int64Expression(3)))
	Expect(inter.Invoke("testNoElse")).To(Equal(Int64Expression(2)))
	Expect(inter.Invoke("testElseIf")).To(Equal(Int64Expression(3)))
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
	Expect(inter.Invoke("test")).To(Equal(Int64Expression(6)))
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
	Expect(inter.Invoke("test")).To(Equal(Int64Expression(6)))
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
	Expect(inter.Globals["x"].Value).To(Equal(Int64Expression(0)))
	Expect(inter.Invoke("test")).To(Equal(Int64Expression(2)))
	Expect(inter.Globals["x"].Value).To(Equal(Int64Expression(2)))
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
	Expect(inter.Invoke("testTrue")).To(Equal(Int64Expression(2)))
	Expect(inter.Invoke("testFalse")).To(Equal(Int64Expression(3)))
}
