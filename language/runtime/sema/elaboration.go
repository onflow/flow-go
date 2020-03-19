package sema

import "github.com/dapperlabs/flow-go/language/runtime/ast"

type MemberInfo struct {
	Member     *Member
	IsOptional bool
}

type Elaboration struct {
	FunctionDeclarationFunctionTypes    map[*ast.FunctionDeclaration]*FunctionType
	VariableDeclarationValueTypes       map[*ast.VariableDeclaration]Type
	VariableDeclarationSecondValueTypes map[*ast.VariableDeclaration]Type
	VariableDeclarationTargetTypes      map[*ast.VariableDeclaration]Type
	AssignmentStatementValueTypes       map[*ast.AssignmentStatement]Type
	AssignmentStatementTargetTypes      map[*ast.AssignmentStatement]Type
	CompositeDeclarationTypes           map[*ast.CompositeDeclaration]*CompositeType
	SpecialFunctionTypes                map[*ast.SpecialFunctionDeclaration]*SpecialFunctionType
	FunctionExpressionFunctionType      map[*ast.FunctionExpression]*FunctionType
	InvocationExpressionArgumentTypes   map[*ast.InvocationExpression][]Type
	InvocationExpressionParameterTypes  map[*ast.InvocationExpression][]Type
	InvocationExpressionReturnTypes     map[*ast.InvocationExpression]Type
	InterfaceDeclarationTypes           map[*ast.InterfaceDeclaration]*InterfaceType
	CastingStaticValueTypes             map[*ast.CastingExpression]Type
	CastingTargetTypes                  map[*ast.CastingExpression]Type
	ReturnStatementValueTypes           map[*ast.ReturnStatement]Type
	ReturnStatementReturnTypes          map[*ast.ReturnStatement]Type
	BinaryExpressionResultTypes         map[*ast.BinaryExpression]Type
	BinaryExpressionRightTypes          map[*ast.BinaryExpression]Type
	MemberExpressionMemberInfos         map[*ast.MemberExpression]MemberInfo
	ArrayExpressionArgumentTypes        map[*ast.ArrayExpression][]Type
	ArrayExpressionElementType          map[*ast.ArrayExpression]Type
	DictionaryExpressionType            map[*ast.DictionaryExpression]*DictionaryType
	DictionaryExpressionEntryTypes      map[*ast.DictionaryExpression][]DictionaryEntryType
	TransactionDeclarationTypes         map[*ast.TransactionDeclaration]*TransactionType
	// NOTE: not indexed by `ast.Type`, as IndexExpression might index
	//   with "type" which is an expression, i.e., an IdentifierExpression.
	//   See `Checker.visitTypeIndexingExpression`
	IndexExpressionIndexingTypes           map[*ast.IndexExpression]Type
	SwapStatementLeftTypes                 map[*ast.SwapStatement]Type
	SwapStatementRightTypes                map[*ast.SwapStatement]Type
	IsTypeIndexExpression                  map[*ast.IndexExpression]bool
	IsResourceMovingStorageIndexExpression map[*ast.IndexExpression]bool
	CompositeNestedDeclarations            map[*ast.CompositeDeclaration]map[string]ast.Declaration
	InterfaceNestedDeclarations            map[*ast.InterfaceDeclaration]map[string]ast.Declaration
	PostConditionsRewrite                  map[*ast.Conditions]PostConditionsRewrite
	EmitStatementEventTypes                map[*ast.EmitStatement]*CompositeType
	IsReferenceIntoStorage                 map[*ast.ReferenceExpression]bool
	// Keyed by qualified identifier
	CompositeTypes                         map[string]*CompositeType
	InvocationExpressionTypeParameterTypes map[*ast.InvocationExpression]map[*TypeParameter]Type
}

func NewElaboration() *Elaboration {
	return &Elaboration{
		FunctionDeclarationFunctionTypes:       map[*ast.FunctionDeclaration]*FunctionType{},
		VariableDeclarationValueTypes:          map[*ast.VariableDeclaration]Type{},
		VariableDeclarationSecondValueTypes:    map[*ast.VariableDeclaration]Type{},
		VariableDeclarationTargetTypes:         map[*ast.VariableDeclaration]Type{},
		AssignmentStatementValueTypes:          map[*ast.AssignmentStatement]Type{},
		AssignmentStatementTargetTypes:         map[*ast.AssignmentStatement]Type{},
		CompositeDeclarationTypes:              map[*ast.CompositeDeclaration]*CompositeType{},
		SpecialFunctionTypes:                   map[*ast.SpecialFunctionDeclaration]*SpecialFunctionType{},
		FunctionExpressionFunctionType:         map[*ast.FunctionExpression]*FunctionType{},
		InvocationExpressionArgumentTypes:      map[*ast.InvocationExpression][]Type{},
		InvocationExpressionParameterTypes:     map[*ast.InvocationExpression][]Type{},
		InvocationExpressionReturnTypes:        map[*ast.InvocationExpression]Type{},
		InterfaceDeclarationTypes:              map[*ast.InterfaceDeclaration]*InterfaceType{},
		CastingStaticValueTypes:                map[*ast.CastingExpression]Type{},
		CastingTargetTypes:                     map[*ast.CastingExpression]Type{},
		ReturnStatementValueTypes:              map[*ast.ReturnStatement]Type{},
		ReturnStatementReturnTypes:             map[*ast.ReturnStatement]Type{},
		BinaryExpressionResultTypes:            map[*ast.BinaryExpression]Type{},
		BinaryExpressionRightTypes:             map[*ast.BinaryExpression]Type{},
		MemberExpressionMemberInfos:            map[*ast.MemberExpression]MemberInfo{},
		ArrayExpressionArgumentTypes:           map[*ast.ArrayExpression][]Type{},
		ArrayExpressionElementType:             map[*ast.ArrayExpression]Type{},
		DictionaryExpressionType:               map[*ast.DictionaryExpression]*DictionaryType{},
		DictionaryExpressionEntryTypes:         map[*ast.DictionaryExpression][]DictionaryEntryType{},
		TransactionDeclarationTypes:            map[*ast.TransactionDeclaration]*TransactionType{},
		IndexExpressionIndexingTypes:           map[*ast.IndexExpression]Type{},
		SwapStatementLeftTypes:                 map[*ast.SwapStatement]Type{},
		SwapStatementRightTypes:                map[*ast.SwapStatement]Type{},
		IsTypeIndexExpression:                  map[*ast.IndexExpression]bool{},
		IsResourceMovingStorageIndexExpression: map[*ast.IndexExpression]bool{},
		CompositeNestedDeclarations:            map[*ast.CompositeDeclaration]map[string]ast.Declaration{},
		InterfaceNestedDeclarations:            map[*ast.InterfaceDeclaration]map[string]ast.Declaration{},
		PostConditionsRewrite:                  map[*ast.Conditions]PostConditionsRewrite{},
		EmitStatementEventTypes:                map[*ast.EmitStatement]*CompositeType{},
		IsReferenceIntoStorage:                 map[*ast.ReferenceExpression]bool{},
		CompositeTypes:                         map[string]*CompositeType{},
		InvocationExpressionTypeParameterTypes: map[*ast.InvocationExpression]map[*TypeParameter]Type{},
	}
}
