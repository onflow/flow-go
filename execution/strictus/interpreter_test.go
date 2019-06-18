package strictus

import (
	. "bamboo-emulator/execution/strictus/ast"
	"bamboo-emulator/execution/strictus/interpreter"
	"github.com/antlr/antlr4/runtime/Go/antlr"
	. "github.com/onsi/gomega"
	"testing"
)

func TestInterpret(t *testing.T) {
	gomega := NewWithT(t)

	input := antlr.NewInputStream(`
        const x = 10

        // check first-class functions and scope inside them
        const y = (fun (x: i32): i32 { return x })(42)

        fun f(): i32 {
           // check resolution
           return x
        }

        fun g(): i32 {
           // check scope is lexical, not dynamic
           const x = 20
           return f()
        }

        pub fun foo(a: i32, b: i32): i64 {
            var c = 2
            c = 3
            const z = [0, 3]
            z[1] = 2
            return ((a + b) * c) / z[1]
        }
	`)

	lexer := NewStrictusLexer(input)
	stream := antlr.NewCommonTokenStream(lexer, 0)
	parser := NewStrictusParser(stream)
	// diagnostics, for debugging only:
	// parser.AddErrorListener(antlr.NewDiagnosticErrorListener(true))
	parser.AddErrorListener(antlr.NewConsoleErrorListener())
	program := parser.Program().Accept(&ProgramVisitor{}).(Program)

	inter := interpreter.NewInterpreter(program)
	inter.Interpret()
	gomega.Expect(inter.Globals["x"].Value).To(Equal(UInt64Expression(10)))
	gomega.Expect(inter.Globals["y"].Value).To(Equal(UInt64Expression(42)))
	gomega.Expect(inter.Invoke("f")).To(Equal(UInt64Expression(10)))
	gomega.Expect(inter.Invoke("g")).To(Equal(UInt64Expression(10)))
	gomega.Expect(inter.Invoke("foo", uint64(24), uint64(42))).To(Equal(UInt64Expression(99)))
}
