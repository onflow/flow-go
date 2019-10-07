package sema

import "github.com/dapperlabs/flow-go/pkg/language/runtime/ast"

type Elaboration struct {
	FunctionDeclarationFunctionTypes   map[*ast.FunctionDeclaration]*FunctionType
	VariableDeclarationValueTypes      map[*ast.VariableDeclaration]Type
	VariableDeclarationTargetTypes     map[*ast.VariableDeclaration]Type
	AssignmentStatementValueTypes      map[*ast.AssignmentStatement]Type
	AssignmentStatementTargetTypes     map[*ast.AssignmentStatement]Type
	CompositeDeclarationTypes          map[*ast.CompositeDeclaration]*CompositeType
	InitializerFunctionTypes           map[*ast.InitializerDeclaration]*ConstructorFunctionType
	FunctionExpressionFunctionType     map[*ast.FunctionExpression]*FunctionType
	InvocationExpressionArgumentTypes  map[*ast.InvocationExpression][]Type
	InvocationExpressionParameterTypes map[*ast.InvocationExpression][]Type
	InterfaceDeclarationTypes          map[*ast.InterfaceDeclaration]*InterfaceType
	FailableDowncastingTypes           map[*ast.FailableDowncastExpression]Type
	ReturnStatementValueTypes          map[*ast.ReturnStatement]Type
	ReturnStatementReturnTypes         map[*ast.ReturnStatement]Type
	BinaryExpressionResultTypes        map[*ast.BinaryExpression]Type
	BinaryExpressionRightTypes         map[*ast.BinaryExpression]Type
	MemberExpressionMembers            map[*ast.MemberExpression]*Member
	ArrayExpressionArgumentTypes       map[*ast.ArrayExpression][]Type
	ArrayExpressionElementType         map[*ast.ArrayExpression]Type
	DictionaryExpressionType           map[*ast.DictionaryExpression]*DictionaryType
	DictionaryExpressionEntryTypes     map[*ast.DictionaryExpression][]DictionaryEntryType
	// NOTE: not indexed by `ast.Type`, as IndexExpression might index
	//   with "type" which is an expression, i.e., an IdentifierExpression.
	//   See `Checker.visitStorageIndexingExpression`
	IndexExpressionIndexingTypes map[*ast.IndexExpression]Type
}

func NewElaboration() *Elaboration {
	return &Elaboration{
		FunctionDeclarationFunctionTypes:   map[*ast.FunctionDeclaration]*FunctionType{},
		VariableDeclarationValueTypes:      map[*ast.VariableDeclaration]Type{},
		VariableDeclarationTargetTypes:     map[*ast.VariableDeclaration]Type{},
		AssignmentStatementValueTypes:      map[*ast.AssignmentStatement]Type{},
		AssignmentStatementTargetTypes:     map[*ast.AssignmentStatement]Type{},
		CompositeDeclarationTypes:          map[*ast.CompositeDeclaration]*CompositeType{},
		InitializerFunctionTypes:           map[*ast.InitializerDeclaration]*ConstructorFunctionType{},
		FunctionExpressionFunctionType:     map[*ast.FunctionExpression]*FunctionType{},
		InvocationExpressionArgumentTypes:  map[*ast.InvocationExpression][]Type{},
		InvocationExpressionParameterTypes: map[*ast.InvocationExpression][]Type{},
		InterfaceDeclarationTypes:          map[*ast.InterfaceDeclaration]*InterfaceType{},
		FailableDowncastingTypes:           map[*ast.FailableDowncastExpression]Type{},
		ReturnStatementValueTypes:          map[*ast.ReturnStatement]Type{},
		ReturnStatementReturnTypes:         map[*ast.ReturnStatement]Type{},
		BinaryExpressionResultTypes:        map[*ast.BinaryExpression]Type{},
		BinaryExpressionRightTypes:         map[*ast.BinaryExpression]Type{},
		MemberExpressionMembers:            map[*ast.MemberExpression]*Member{},
		ArrayExpressionArgumentTypes:       map[*ast.ArrayExpression][]Type{},
		ArrayExpressionElementType:         map[*ast.ArrayExpression]Type{},
		DictionaryExpressionType:           map[*ast.DictionaryExpression]*DictionaryType{},
		DictionaryExpressionEntryTypes:     map[*ast.DictionaryExpression][]DictionaryEntryType{},
		IndexExpressionIndexingTypes:       map[*ast.IndexExpression]Type{},
	}
}
