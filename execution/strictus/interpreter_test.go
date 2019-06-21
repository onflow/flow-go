package strictus

import (
	. "bamboo-runtime/execution/strictus/ast"
	"bamboo-runtime/execution/strictus/interpreter"
	. "github.com/onsi/gomega"
	"testing"
)

func TestInterpret(t *testing.T) {
	gomega := NewWithT(t)

	program := parse(`
        const x = 10

        // check first-class functions and scope inside them
        const y = (fun (x: Int32): Int32 { return x })(42)

        fun f(): Int32 {
           // check resolution
           return x
        }

        fun g(): Int32 {
           // check scope is lexical, not dynamic
           const x = 20
           return f()
        }

        pub fun foo(a: Int32, b: Int32): Int64 {
            var c = 2
            c = 3
            const z = [0, 3]
            z[1] = 2
            return ((a + b) * c) / z[1]
        }
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	gomega.Expect(inter.Globals["x"].Value).To(Equal(UInt64Expression(10)))
	gomega.Expect(inter.Globals["y"].Value).To(Equal(UInt64Expression(42)))
	gomega.Expect(inter.Invoke("f")).To(Equal(UInt64Expression(10)))
	gomega.Expect(inter.Invoke("g")).To(Equal(UInt64Expression(10)))
	gomega.Expect(inter.Invoke("foo", uint64(24), uint64(42))).To(Equal(UInt64Expression(99)))
}

func TestReturnWithoutExpression(t *testing.T) {
	gomega := NewWithT(t)

	program := parse(`
        fun returnEarly() {
            return
            return 1
        }
	`)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	gomega.Expect(inter.Invoke("returnEarly")).To(Equal(interpreter.Void{}))
}
