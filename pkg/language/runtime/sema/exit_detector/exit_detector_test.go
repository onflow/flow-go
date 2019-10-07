package exit_detector

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/parser"
)

func parseFunctionBlock(code string) (*ast.FunctionBlock, error) {
	body := fmt.Sprintf("fun test(): Any {%s}", code)

	program, _, err := parser.ParseProgram(body)
	if err != nil {
		return nil, err
	}

	functionDeclaration := program.FunctionDeclarations()[0]
	functionBlock := functionDeclaration.FunctionBlock

	return functionBlock, nil
}

func assertExits(t *testing.T, code string, expected bool) {

	functionBlock, err := parseFunctionBlock(code)
	if err != nil {
		t.Fatalf("Unable to parse function block: %s", err.Error())
	}

	exits := FunctionBlockExits(functionBlock)

	assert.Equal(t, exits, expected)
}

type exitTest struct {
	code     string
	expected bool
}

func testExits(t *testing.T, tests []exitTest) {
	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			assertExits(t, test.code, test.expected)
		})
	}
}

func TestReturnStatementExits(t *testing.T) {
	testExits(t, []exitTest{
		{"return 1", true},
		{"return", true},
	})
}

func TestIfStatementExits(t *testing.T) {
	testExits(t, []exitTest{
		{
			`
			if true {
				return 1
			}
			`,
			true,
		},
		{
			`
			if true {
				x = 1
			} else {
				return 2
			}
			`,
			false,
		},
		{
			`
			if false {
				x = 1
			} else {
				return 2
			}
			`,
			true,
		},
		{
			`
			if true {
				if true {
					return 1
				}
			}
			`,
			true,
		},
		{
			`
			if 2 > 1 {
				return 1
			}
			`,
			false,
		},
		{
			`
			if 2 > 1 {
				return 1
			} else {
				return 2
			}
			`,
			true,
		},
		{
			`
			if 2 > 1 {
				return 1
			}				
			return 2
			`,
			true,
		},
	})
}

func TestWhileStatementExits(t *testing.T) {
	testExits(t, []exitTest{
		{
			`
			while true {
				x = y
			}
			`,
			true,
		},
		{
			`
			while true {
				x = y
				break
			}
			`,
			false,
		},
		{
			`
			while 1 > 2 {
				x = y
			}
			`,
			false,
		},
		{
			`
			while 1 > 2 {
				x = y
				break
			}
			`,
			false,
		},
		{
			`
			while 2 > 1 {
				return
			}
			`,
			false,
		},
	})
}

func TestFunctionInvocationExits(t *testing.T) {
	testExits(t, []exitTest{
		{
			`
			x = foo()
			return 1
			`,
			true,
		},
	})
}

func TestFunctionIfWhileExits(t *testing.T) {
	testExits(t, []exitTest{
		{
			`
			var x = 0
			while x < 10 {
				x = x + 1
				if x == 5 {
					break
				}
			}
			return x
			`,
			true,
		},
		{
			`
			if true {
				while true {
					break
				}
			}
			`,
			false,
		},
		{
			`
			if true {
				while true {
					break
				}
			}
			`,
			false,
		},
	})
}
