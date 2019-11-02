package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

func TestExtractMethod(t *testing.T) {
	tt := []struct {
		inputCode      string
		expectedMethod method
	}{
		{
			inputCode: `package foo

			type foo interface {
				TurnOn(Void) error
			}`,
			expectedMethod: method{Name: "TurnOn", ParamType: "Void"},
		},
		{
			inputCode: `package foo

			type foo interface {
				aMethod(context.Context, aType) error
			}`,
			expectedMethod: method{Name: "aMethod", ParamType: "aType"},
		},
	}

	for _, tc := range tt {
		var m method
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "", tc.inputCode, parser.ParseComments)
		if err != nil {
			t.Errorf("could not test code: %v", err)
		}

		ast.Inspect(node, func(n ast.Node) bool {
			switch typ := n.(type) {
			case *ast.TypeSpec:
				switch typ.Type.(type) {
				case *ast.InterfaceType:
					serv := typ.Type.(*ast.InterfaceType)
					m = extractMethod(tc.inputCode, serv.Methods.List[0])
				}
			}

			return true
		})
		if m != tc.expectedMethod {
			t.Errorf("extraced wrong method. Expected: %v, got: %v", tc.expectedMethod, m)
		}
	}
}

// testing genNewFileName with various filenames
func TestGenerateNewFileName(t *testing.T) {
	tt := []struct {
		filename    string
		newFilename string
	}{
		{filename: "messages.pb.go", newFilename: "messages_registry.gen.go"},
		{filename: "./long/path/to/somewhere/messages.pb.go", newFilename: filepath.FromSlash("long/path/to/somewhere/messages_registry.gen.go")},
		{filename: "/root/messages.pb.go", newFilename: filepath.FromSlash("/root/messages_registry.gen.go")},
		{filename: "interface", newFilename: "registry.gen.go"}, //filepath's base does not end with .pb.go
		{filename: "interface.go", newFilename: "registry.gen.go"}, //filepath's base does not end with .pb.go
		{filename: "interface", newFilename: "registry.gen.go"}, //filepath's base does not end with .pb.go
		//Windows paths
		{filename: "C:\\myfolder\\project\\interface.pb.go", newFilename: "C:\\myfolder\\project\\interface_registry.gen.go"},
		{filename: "C:\\Users\\Me\\project\\invalidInterface.go", newFilename: "registry.gen.go"}, //filepath's base does not end with .pb.go
		{filename: "C:\\Users\\Me\\project\\invalidInterface", newFilename: "registry.gen.go"}, //filepath's base does not end with .pb.go
	}

	for _, tc := range tt {
		generated := genNewFileName(tc.filename)
		if generated != tc.newFilename {
			t.Errorf("generated file name does not match expected value. generated: %v, expected: %v", generated, tc.newFilename)
		}
	}
}

// testing upperSubstringToLower function for different cases
func TestLowerInitials(t *testing.T) {
	testCases := []struct {
		input string
		want  string
	}{
		{input: "", want: ""},                  // empty string test case
		{input: "1", want: ""},                 // single number test case
		{input: "1234567", want: ""},           // many numbers test case
		{input: "a", want: ""},                 // single lower case rune
		{input: "abcdefgh", want: ""},          // many lower case runes
		{input: "A", want: "a"},                // single upper cases rune
		{input: "ABC", want: "abc"},            // many upper cases runes
		{input: "WindowsComputer", want: "wc"}, // hybrid case runes
		{input: "Verify2Server", want: "vs"},   // hybrid case runes
		{input: "1C2o3n4s5e6n7s8u9s1S2e3r4v5i6c7e8S9e1r2v3e4r", want: "css"}, // hybrid case runes
	}

	for _, tc := range testCases {
		got := upperSubstringToLower(tc.input)
		if got != tc.want {
			t.Errorf("upperSubstringToLower test error: got: \"%v\", want: \"%v\"\n.", upperSubstringToLower(tc.input), tc.want)
		}
	}
}


