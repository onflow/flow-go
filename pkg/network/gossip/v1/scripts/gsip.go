package main

// gsip is gRPC Server Interface Parse that handles the conversion of gRPC
// services to gnode registries
import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
	"strings"
)

//Error types
type readError error
type parseError error
type missingServerInterfacesError error

// parseCode generates registry info corresponding to the content of the given
// code
func parseCode(code io.Reader) (*fileInfo, error) {

	var (
		packageName string
		registries  = make([]registryInfo, 0)
	)

	codeBytes, err := ioutil.ReadAll(code)
	if err != nil {
		return nil, readError(fmt.Errorf("could not read code: %v", err))
	}

	codeText := string(codeBytes)

	// generate the root ast node
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", codeText, parser.ParseComments)
	if err != nil {
		return nil, parseError(fmt.Errorf("could not parse code: %v", err))
	}

	// This function will be applied to every node. In case an interface ending in "Server"
	// is spotted, then its registryInfo are extracted and appended to registries being collected
	inspectFunc := func(n ast.Node) bool {
		switch t := n.(type) {
		// extract the package name from the root node
		case *ast.File:
			packageName = t.Name.Name
		// check all type definitions in the file
		case *ast.TypeSpec:
			// Only consider types that are available to other packages
			if t.Name.IsExported() {
				switch t.Type.(type) {
				case *ast.InterfaceType:
					if strings.HasSuffix(t.Name.Name, "Server") {
						// If an interface ending with "server" is found, extract method information
						interfaceLong := t.Name.Name
						methods := make([]method, 0)
						// cast to InterfaceType in order to access the methods list of the interface
						serv := t.Type.(*ast.InterfaceType)
						for _, field := range serv.Methods.List {
							methods = append(methods, extractMethod(codeText, field))
						}
						// in case the interface short hand was an empty string, a default value is used (r)
						interfaceShort := upperSubstringToLower(interfaceLong)
						registries = append(registries, registryInfo{InterfaceLong: interfaceLong, InterfaceShort: interfaceShort, Methods: methods})
					}
				}
			}
		}

		return true
	}

	ast.Inspect(node, inspectFunc)

	if len(registries) == 0 {
		return nil, missingServerInterfacesError(fmt.Errorf("could not find grpc server interfaces in given code"))
	}
	return &fileInfo{Package: packageName, Registries: registries}, nil
}
