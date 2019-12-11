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

// BuiltinTypes

var BuiltinTypes = StandardLibraryTypes{}

// built-in event types

var AccountCreatedEventType = sema.EventType{
	Identifier: "AccountCreated",
	Fields: []sema.EventFieldType{
		{
			Identifier: "address",
			Type:       &sema.StringType{},
		},
	},
	ConstructorParameterTypeAnnotations: []*sema.TypeAnnotation{
		{
			IsResource: false,
			Type:       &sema.StringType{},
		},
	},
}

var AccountKeyAddedEventType = sema.EventType{
	Identifier: "AccountKeyAdded",
	Fields: []sema.EventFieldType{
		{
			Identifier: "address",
			Type:       &sema.StringType{},
		},
		{
			Identifier: "publicKey",
			Type: &sema.VariableSizedType{
				Type: &sema.IntType{},
			},
		},
	},
	ConstructorParameterTypeAnnotations: []*sema.TypeAnnotation{
		{
			IsResource: false,
			Type:       &sema.StringType{},
		},
		{
			IsResource: false,
			Type: &sema.VariableSizedType{
				Type: &sema.IntType{},
			},
		},
	},
}

var AccountKeyRemovedEventType = sema.EventType{
	Identifier: "AccountKeyRemoved",
	Fields: []sema.EventFieldType{
		{
			Identifier: "address",
			Type:       &sema.StringType{},
		},
		{
			Identifier: "publicKey",
			Type: &sema.VariableSizedType{
				Type: &sema.IntType{},
			},
		},
	},
	ConstructorParameterTypeAnnotations: []*sema.TypeAnnotation{
		{
			IsResource: false,
			Type:       &sema.StringType{},
		},
		{
			IsResource: false,
			Type: &sema.VariableSizedType{
				Type: &sema.IntType{},
			},
		},
	},
}

var AccountCodeUpdatedEventType = sema.EventType{
	Identifier: "AccountCodeUpdated",
	Fields: []sema.EventFieldType{
		{
			Identifier: "address",
			Type:       &sema.StringType{},
		},
		{
			Identifier: "codeHash",
			Type: &sema.VariableSizedType{
				Type: &sema.IntType{},
			},
		},
	},
	ConstructorParameterTypeAnnotations: []*sema.TypeAnnotation{
		{
			IsResource: false,
			Type:       &sema.StringType{},
		},
		{
			IsResource: false,
			Type: &sema.VariableSizedType{
				Type: &sema.IntType{},
			},
		},
	},
}
