package stdlib

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
)

type StandardLibraryType struct {
	Name string
	Type sema.Type
	Kind common.DeclarationKind
}

func (t StandardLibraryType) TypeDeclarationType() sema.Type {
	return t.Type
}

func (t StandardLibraryType) TypeDeclarationKind() common.DeclarationKind {
	return t.Kind
}

func (StandardLibraryType) TypeDeclarationPosition() ast.Position {
	return ast.Position{}
}

// StandardLibraryTypes

type StandardLibraryTypes []StandardLibraryType

func (types StandardLibraryTypes) ToTypeDeclarations() map[string]sema.TypeDeclaration {
	valueDeclarations := make(map[string]sema.TypeDeclaration, len(types))
	for _, ty := range types {
		valueDeclarations[ty.Name] = ty
	}
	return valueDeclarations
}

// AccountType

var AccountType = StandardLibraryType{
	Name: "Account",
	Type: &sema.CompositeType{
		Kind:       common.CompositeKindStructure,
		Identifier: "Account",
		Members: map[string]*sema.Member{
			"address": {
				Type: &sema.StringType{},
			},
			"storage": {
				Type: &sema.StorageType{},
			},
		},
	},
	Kind: common.DeclarationKindStructure,
}

// BuiltinTypes

var BuiltinTypes = StandardLibraryTypes{
	AccountType,
}
