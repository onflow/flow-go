package main

import (
	"go/ast"
	"path/filepath"
	"strings"
	"unicode"
)

// upperSubstringToLower returns lowercase substring concatenated uppercase runes of input s
// Example: VerifyServer -> vs
func upperSubstringToLower(s string) string {
	//to upper-case initials
	shortHandRunes := strings.Map(filterUpperRune, s)
	//to lower-case initials
	return strings.ToLower(shortHandRunes)
}

// filerUpperRune on receiving rune r, returns -1 if it is Unicode upper-case, otherwise returns r
func filterUpperRune(r rune) rune {
	if !unicode.IsUpper(r) {
		return -1
	}
	return r
}

// extracts method information from an ast field
func extractMethod(codeText string, field *ast.Field) method {
	const offset = 1

	method := newMethod()
	method.Name = field.Names[0].String()

	typ := field.Type.(*ast.FuncType)
	for _, param := range typ.Params.List {
		// set the beginning and ending position in of the parameter being processed in the file
		begin := param.Type.Pos() - offset
		end := param.Type.End() - offset
		// Trim the prefix in case it was a pointer in order to get the actual type name
		method.ParamType = strings.TrimPrefix(codeText[begin:end], "*")
		// in case we reach a parameter that is not a Context type, add it and then terminate the method
		if !strings.HasSuffix(method.ParamType, "Context") {
			break
		}
	}

	return method
}

// genNewFileName generates a new filename for the generated registry code from
// the original filename. If filename is not valid, a default name is used
// instead.
func genNewFileName(filename string) string {
	if !strings.HasSuffix(filename, ".pb.go") {
		return "registry.gen.go"
	}
	return filepath.Join(filepath.Dir(filename), strings.Split(filepath.Base(filename), ".")[0]+"_registry.gen.go")
}
