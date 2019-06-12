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
		pub fun foo(a: i32, b: i32): i64 {
            var x = 2
            x = 3
            const z = [0, 2]
            return ((a + b) * x) / z[1]
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
	gomega.Expect(inter.Invoke("foo", 24, 42)).To(Equal(int64(99)))
}
