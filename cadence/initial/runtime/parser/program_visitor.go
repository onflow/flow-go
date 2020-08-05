/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package parser

import (
	"encoding/hex"
	"math/big"
	"strings"

	"github.com/antlr/antlr4/runtime/Go/antlr"

	"github.com/onflow/cadence-initial/runtime/ast"
	"github.com/onflow/cadence-initial/runtime/common"
	"github.com/onflow/cadence-initial/runtime/errors"
)

type ProgramVisitor struct {
	*BaseCadenceVisitor
	parseErrors []error
}

func (v *ProgramVisitor) report(errs ...error) {
	v.parseErrors = append(v.parseErrors, errs...)
}

func (v *ProgramVisitor) VisitProgram(ctx *ProgramContext) interface{} {
	var allDeclarations []ast.Declaration

	for _, declarationContext := range ctx.AllDeclaration() {
		declarationResult := declarationContext.Accept(v)
		if declarationResult == nil {
			return nil
		}
		declaration := declarationResult.(ast.Declaration)
		allDeclarations = append(allDeclarations, declaration)
	}

	return &ast.Program{
		Declarations: allDeclarations,
	}
}

func (v *ProgramVisitor) VisitReplInput(ctx *ReplInputContext) interface{} {
	var elements []interface{}
	for _, elementCtx := range ctx.AllReplElement() {
		elements = append(elements, elementCtx.Accept(v))
	}
	return elements
}

func (v *ProgramVisitor) VisitReplElement(ctx *ReplElementContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func (v *ProgramVisitor) VisitReplDeclaration(ctx *ReplDeclarationContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func (v *ProgramVisitor) VisitReplStatement(ctx *ReplStatementContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func (v *ProgramVisitor) VisitDeclaration(ctx *DeclarationContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func (v *ProgramVisitor) VisitFunctionDeclaration(ctx *FunctionDeclarationContext) interface{} {
	access := ctx.Access().Accept(v).(ast.Access)

	identifier := ctx.Identifier().Accept(v).(ast.Identifier)

	parameterListEnd := ctx.ParameterList().GetStop()
	returnTypeAnnotation := v.visitReturnTypeAnnotation(ctx.returnType, parameterListEnd)

	var parameterList *ast.ParameterList
	parameterListContext := ctx.ParameterList()
	if parameterListContext != nil {
		parameterList = parameterListContext.Accept(v).(*ast.ParameterList)
	}

	// NOTE: in e.g interface declarations, function blocks are optional

	var functionBlock *ast.FunctionBlock
	functionBlockContext := ctx.FunctionBlock()
	if functionBlockContext != nil {
		functionBlock = functionBlockContext.Accept(v).(*ast.FunctionBlock)
	}

	startPosition := PositionFromToken(ctx.GetStart())

	return &ast.FunctionDeclaration{
		Access:               access,
		Identifier:           identifier,
		ParameterList:        parameterList,
		ReturnTypeAnnotation: returnTypeAnnotation,
		FunctionBlock:        functionBlock,
		StartPos:             startPosition,
	}
}

// visitReturnTypeAnnotation returns the return type annotation.
// if none was given in the program, return a type annotation with an empty type, with the position of tokenBefore
func (v *ProgramVisitor) visitReturnTypeAnnotation(ctx ITypeAnnotationContext, tokenBefore antlr.Token) *ast.TypeAnnotation {
	if ctx == nil {
		positionBeforeMissingReturnType :=
			PositionFromToken(tokenBefore)
		returnType := &ast.NominalType{
			Identifier: ast.Identifier{
				Pos: positionBeforeMissingReturnType,
			},
		}
		return &ast.TypeAnnotation{
			IsResource: false,
			Type:       returnType,
			StartPos:   positionBeforeMissingReturnType,
		}
	}
	result := ctx.Accept(v)
	if result == nil {
		return nil
	}
	return result.(*ast.TypeAnnotation)
}

func (v *ProgramVisitor) VisitAccess(ctx *AccessContext) interface{} {
	switch {
	case ctx.Priv() != nil:
		return ast.AccessPrivate

	case ctx.Pub() != nil:
		if ctx.Set() != nil {
			return ast.AccessPublicSettable
		}
		return ast.AccessPublic

	case ctx.Self() != nil:
		return ast.AccessPrivate

	case ctx.All() != nil:
		return ast.AccessPublic

	case ctx.Contract() != nil:
		return ast.AccessContract

	case ctx.Account() != nil:
		return ast.AccessAccount

	default:
		return ast.AccessNotSpecified
	}
}

func (v *ProgramVisitor) VisitImportDeclaration(ctx *ImportDeclarationContext) interface{} {
	startPosition := PositionFromToken(ctx.GetStart())

	var location ast.Location
	var locationPos ast.Position
	var endPos ast.Position

	// string literal?
	stringLiteralContext := ctx.StringLiteral()
	if stringLiteralContext != nil {
		stringExpression := stringLiteralContext.Accept(v).(*ast.StringExpression)
		location = ast.StringLocation(stringExpression.Value)
		locationPos = stringExpression.StartPos
		endPos = stringExpression.EndPos
	} else {
		// hexadecimal literal (address)

		hexadecimalLiteralNode := ctx.HexadecimalLiteral()
		text := hexadecimalLiteralNode.GetText()[2:]
		bytes := []byte(strings.Replace(text, "_", "", -1))

		length := len(bytes)
		if length%2 == 1 {
			bytes = append([]byte{'0'}, bytes...)
			length++
		}

		address := make([]byte, hex.DecodedLen(length))
		_, err := hex.Decode(address, bytes)
		if err != nil {
			// unreachable, hex literal should always be valid
			panic(err)
		}
		location = ast.AddressLocation(address)
		symbol := hexadecimalLiteralNode.GetSymbol()
		locationPos = PositionFromToken(symbol)
		endPos = ast.EndPosition(locationPos, symbol.GetStop())
	}

	allIdentifierNodes := ctx.AllIdentifier()
	var identifiers []ast.Identifier
	if len(allIdentifierNodes) > 0 {
		identifiers = make([]ast.Identifier, len(allIdentifierNodes))
		for i, identifierNode := range allIdentifierNodes {
			identifiers[i] = identifierNode.Accept(v).(ast.Identifier)
		}
	}

	return &ast.ImportDeclaration{
		Identifiers: identifiers,
		Location:    location,
		Range: ast.Range{
			StartPos: startPosition,
			EndPos:   endPos,
		},
		LocationPos: locationPos,
	}
}

func (v *ProgramVisitor) VisitTransactionDeclaration(ctx *TransactionDeclarationContext) interface{} {
	var parameterList *ast.ParameterList
	parameterListContext := ctx.ParameterList()
	if parameterListContext != nil {
		parameterList = parameterListContext.Accept(v).(*ast.ParameterList)
	}

	var fields []*ast.FieldDeclaration
	fieldsCtx := ctx.Fields()
	if fieldsCtx != nil {
		fields = fieldsCtx.Accept(v).([]*ast.FieldDeclaration)
	}

	var prepareFunction *ast.SpecialFunctionDeclaration
	prepareCtx := ctx.Prepare()
	if prepareCtx != nil {
		prepareFunction = prepareCtx.Accept(v).(*ast.SpecialFunctionDeclaration)
		prepareFunction.Kind = common.DeclarationKindPrepare
	}

	var executeFunction *ast.SpecialFunctionDeclaration
	executeCtx := ctx.Execute()
	if executeCtx != nil {
		executeFunction = executeCtx.Accept(v).(*ast.SpecialFunctionDeclaration)
	}

	var preConditions *ast.Conditions
	preConditionsCtx := ctx.PreConditions()
	if preConditionsCtx != nil {
		var conditions ast.Conditions = preConditionsCtx.Accept(v).([]*ast.Condition)
		preConditions = &conditions
	}

	var postConditions *ast.Conditions
	postConditionsCtx := ctx.PostConditions()
	if postConditionsCtx != nil {
		var conditions ast.Conditions = postConditionsCtx.Accept(v).([]*ast.Condition)
		postConditions = &conditions
	}

	startPosition, endPosition := PositionRangeFromContext(ctx)

	return &ast.TransactionDeclaration{
		ParameterList:  parameterList,
		Fields:         fields,
		Prepare:        prepareFunction,
		PreConditions:  preConditions,
		PostConditions: postConditions,
		Execute:        executeFunction,
		Range: ast.Range{
			StartPos: startPosition,
			EndPos:   endPosition,
		},
	}
}

func (v *ProgramVisitor) VisitPrepare(ctx *PrepareContext) interface{} {
	return ctx.SpecialFunctionDeclaration().Accept(v)
}

func (v *ProgramVisitor) VisitExecute(ctx *ExecuteContext) interface{} {
	identifier := ctx.Identifier().Accept(v).(ast.Identifier)
	block := ctx.Block().Accept(v).(*ast.Block)

	startPosition := PositionFromToken(ctx.GetStart())

	return &ast.SpecialFunctionDeclaration{
		Kind: common.DeclarationKindExecute,
		FunctionDeclaration: &ast.FunctionDeclaration{
			Access:        ast.AccessNotSpecified,
			Identifier:    identifier,
			ParameterList: &ast.ParameterList{},
			FunctionBlock: &ast.FunctionBlock{
				Block: block,
			},
			StartPos: startPosition,
		},
	}
}

func (v *ProgramVisitor) VisitEventDeclaration(ctx *EventDeclarationContext) interface{} {
	access := ctx.Access().Accept(v).(ast.Access)
	identifier := ctx.Identifier().Accept(v).(ast.Identifier)

	var specialFunctions []*ast.SpecialFunctionDeclaration

	parameterListContext := ctx.ParameterList()
	if parameterListContext != nil {
		parameterList := parameterListContext.Accept(v).(*ast.ParameterList)

		specialFunctions = append(specialFunctions,
			&ast.SpecialFunctionDeclaration{
				Kind: common.DeclarationKindInitializer,
				FunctionDeclaration: &ast.FunctionDeclaration{
					ParameterList: parameterList,
					StartPos:      parameterList.StartPos,
				},
			},
		)
	}

	startPosition, endPosition := PositionRangeFromContext(ctx)

	return &ast.CompositeDeclaration{
		Access:        access,
		CompositeKind: common.CompositeKindEvent,
		Identifier:    identifier,
		Members: &ast.Members{
			SpecialFunctions: specialFunctions,
		},
		Range: ast.Range{
			StartPos: startPosition,
			EndPos:   endPosition,
		},
	}
}

func (v *ProgramVisitor) VisitEmitStatement(ctx *EmitStatementContext) interface{} {
	identifier := ctx.Identifier().Accept(v).(ast.Identifier)
	invocation := ctx.Invocation().Accept(v).(*ast.InvocationExpression)
	invocation.InvokedExpression =
		&ast.IdentifierExpression{
			Identifier: identifier,
		}

	startPosition := PositionFromToken(ctx.GetStart())

	return &ast.EmitStatement{
		InvocationExpression: invocation,
		StartPos:             startPosition,
	}
}

func (v *ProgramVisitor) VisitCompositeDeclaration(ctx *CompositeDeclarationContext) interface{} {
	access := ctx.Access().Accept(v).(ast.Access)
	kind := ctx.CompositeKind().Accept(v).(common.CompositeKind)
	identifier := ctx.Identifier().Accept(v).(ast.Identifier)
	conformances := ctx.Conformances().Accept(v).([]*ast.NominalType)
	membersAndNestedDeclarations := ctx.MembersAndNestedDeclarations().Accept(v).(membersAndNestedDeclarations)

	startPosition, endPosition := PositionRangeFromContext(ctx)

	return &ast.CompositeDeclaration{
		Access:                access,
		CompositeKind:         kind,
		Identifier:            identifier,
		Conformances:          conformances,
		Members:               membersAndNestedDeclarations.Members,
		InterfaceDeclarations: membersAndNestedDeclarations.InterfaceDeclarations,
		CompositeDeclarations: membersAndNestedDeclarations.CompositeDeclarations,
		Range: ast.Range{
			StartPos: startPosition,
			EndPos:   endPosition,
		},
	}
}

func (v *ProgramVisitor) VisitConformances(ctx *ConformancesContext) interface{} {
	typeContexts := ctx.AllNominalType()
	var conformances []*ast.NominalType
	if len(typeContexts) == 0 {
		return conformances
	}

	conformances = make([]*ast.NominalType, len(typeContexts))
	for i, typeContext := range typeContexts {
		conformances[i] = typeContext.Accept(v).(*ast.NominalType)
	}
	return conformances
}

func (v *ProgramVisitor) VisitMemberOrNestedDeclaration(ctx *MemberOrNestedDeclarationContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func (v *ProgramVisitor) VisitMembersAndNestedDeclarations(ctx *MembersAndNestedDeclarationsContext) interface{} {

	var fields []*ast.FieldDeclaration
	var specialFunctions []*ast.SpecialFunctionDeclaration
	var functions []*ast.FunctionDeclaration
	var compositeDeclarations []*ast.CompositeDeclaration
	var interfaceDeclarations []*ast.InterfaceDeclaration

	for _, memberOrNestedDeclarationContext := range ctx.AllMemberOrNestedDeclaration() {
		memberOrNestedDeclaration := memberOrNestedDeclarationContext.Accept(v)

		switch memberOrNestedDeclaration := memberOrNestedDeclaration.(type) {
		case *ast.FieldDeclaration:
			fields = append(fields, memberOrNestedDeclaration)

		case *ast.SpecialFunctionDeclaration:
			specialFunctions = append(specialFunctions, memberOrNestedDeclaration)

		case *ast.FunctionDeclaration:
			functions = append(functions, memberOrNestedDeclaration)

		case *ast.CompositeDeclaration:
			compositeDeclarations = append(compositeDeclarations, memberOrNestedDeclaration)

		case *ast.InterfaceDeclaration:
			interfaceDeclarations = append(interfaceDeclarations, memberOrNestedDeclaration)
		}
	}

	members := &ast.Members{
		Fields:           fields,
		SpecialFunctions: specialFunctions,
		Functions:        functions,
	}

	return membersAndNestedDeclarations{
		CompositeDeclarations: compositeDeclarations,
		InterfaceDeclarations: interfaceDeclarations,
		Members:               members,
	}
}

type membersAndNestedDeclarations struct {
	CompositeDeclarations []*ast.CompositeDeclaration
	InterfaceDeclarations []*ast.InterfaceDeclaration
	Members               *ast.Members
}

func (v *ProgramVisitor) VisitFields(ctx *FieldsContext) interface{} {
	fieldsCtx := ctx.AllField()

	fieldCount := len(fieldsCtx)

	var fields []*ast.FieldDeclaration

	if fieldCount == 0 {
		return fields
	}

	fields = make([]*ast.FieldDeclaration, fieldCount)

	for i, fieldCtx := range ctx.AllField() {
		fields[i] = fieldCtx.Accept(v).(*ast.FieldDeclaration)
	}

	return fields
}

func (v *ProgramVisitor) VisitField(ctx *FieldContext) interface{} {
	access := ctx.Access().Accept(v).(ast.Access)

	variableKindContext := ctx.VariableKind()
	variableKind := ast.VariableKindNotSpecified
	if variableKindContext != nil {
		variableKind = variableKindContext.Accept(v).(ast.VariableKind)
	}

	identifier := ctx.Identifier().Accept(v).(ast.Identifier)

	typeAnnotationContext := ctx.TypeAnnotation()
	typeAnnotation := typeAnnotationContext.Accept(v).(*ast.TypeAnnotation)

	startPosition := PositionFromToken(ctx.GetStart())
	endPosition := ast.EndPosition(startPosition, typeAnnotationContext.GetStop().GetStop())

	return &ast.FieldDeclaration{
		Access:         access,
		VariableKind:   variableKind,
		Identifier:     identifier,
		TypeAnnotation: typeAnnotation,
		Range: ast.Range{
			StartPos: startPosition,
			EndPos:   endPosition,
		},
	}
}

func (v *ProgramVisitor) VisitSpecialFunctionDeclaration(ctx *SpecialFunctionDeclarationContext) interface{} {
	identifier := ctx.Identifier().Accept(v).(ast.Identifier)

	var parameterList *ast.ParameterList
	parameterListContext := ctx.ParameterList()
	if parameterListContext != nil {
		parameterList = parameterListContext.Accept(v).(*ast.ParameterList)
	}

	// NOTE: in e.g interface declarations, function blocks are optional

	var functionBlock *ast.FunctionBlock
	functionBlockContext := ctx.FunctionBlock()
	if functionBlockContext != nil {
		functionBlock = functionBlockContext.Accept(v).(*ast.FunctionBlock)
	}

	startPosition := PositionFromToken(ctx.GetStart())

	declarationKind := common.DeclarationKindUnknown
	switch identifier.Identifier {
	case common.DeclarationKindInitializer.Keywords():
		declarationKind = common.DeclarationKindInitializer
	case common.DeclarationKindDestructor.Keywords():
		declarationKind = common.DeclarationKindDestructor
	}

	return &ast.SpecialFunctionDeclaration{
		Kind: declarationKind,
		FunctionDeclaration: &ast.FunctionDeclaration{
			Identifier:    identifier,
			ParameterList: parameterList,
			FunctionBlock: functionBlock,
			StartPos:      startPosition,
		},
	}
}

func (v *ProgramVisitor) VisitInterfaceDeclaration(ctx *InterfaceDeclarationContext) interface{} {
	access := ctx.Access().Accept(v).(ast.Access)
	kind := ctx.CompositeKind().Accept(v).(common.CompositeKind)
	identifier := ctx.Identifier().Accept(v).(ast.Identifier)
	membersAndNestedDeclarations := ctx.MembersAndNestedDeclarations().Accept(v).(membersAndNestedDeclarations)
	startPosition, endPosition := PositionRangeFromContext(ctx)

	return &ast.InterfaceDeclaration{
		Access:                access,
		CompositeKind:         kind,
		Identifier:            identifier,
		Members:               membersAndNestedDeclarations.Members,
		InterfaceDeclarations: membersAndNestedDeclarations.InterfaceDeclarations,
		CompositeDeclarations: membersAndNestedDeclarations.CompositeDeclarations,
		Range: ast.Range{
			StartPos: startPosition,
			EndPos:   endPosition,
		},
	}
}

func (v *ProgramVisitor) VisitCompositeKind(ctx *CompositeKindContext) interface{} {
	switch {
	case ctx.Struct() != nil:
		return common.CompositeKindStructure

	case ctx.Resource() != nil:
		return common.CompositeKindResource

	case ctx.Contract() != nil:
		return common.CompositeKindContract

	default:
		panic(errors.NewUnreachableError())
	}
}

func (v *ProgramVisitor) VisitFunctionExpression(ctx *FunctionExpressionContext) interface{} {
	parameterListEnd := ctx.ParameterList().GetStop()
	returnTypeAnnotation := v.visitReturnTypeAnnotation(ctx.returnType, parameterListEnd)

	var parameterList *ast.ParameterList
	parameterListContext := ctx.ParameterList()
	if parameterListContext != nil {
		parameterList = parameterListContext.Accept(v).(*ast.ParameterList)
	}

	functionBlock := ctx.FunctionBlock().Accept(v).(*ast.FunctionBlock)

	startPosition := PositionFromToken(ctx.GetStart())

	return &ast.FunctionExpression{
		ParameterList:        parameterList,
		ReturnTypeAnnotation: returnTypeAnnotation,
		FunctionBlock:        functionBlock,
		StartPos:             startPosition,
	}
}

func (v *ProgramVisitor) VisitParameterList(ctx *ParameterListContext) interface{} {
	var parameters []*ast.Parameter

	for _, parameter := range ctx.AllParameter() {
		parameters = append(
			parameters,
			parameter.Accept(v).(*ast.Parameter),
		)
	}

	startPosition, endPosition := PositionRangeFromContext(ctx)

	return &ast.ParameterList{
		Parameters: parameters,
		Range: ast.Range{
			StartPos: startPosition,
			EndPos:   endPosition,
		},
	}
}

func (v *ProgramVisitor) VisitParameter(ctx *ParameterContext) interface{} {
	// label
	label := ""
	if ctx.argumentLabel != nil {
		label = ctx.argumentLabel.GetText()
	}

	identifier := ctx.parameterName.Accept(v).(ast.Identifier)

	typeAnnotation := ctx.TypeAnnotation().Accept(v).(*ast.TypeAnnotation)

	startPosition := PositionFromToken(ctx.GetStart())
	endPosition := typeAnnotation.EndPosition()

	return &ast.Parameter{
		Label:          label,
		Identifier:     identifier,
		TypeAnnotation: typeAnnotation,
		Range: ast.Range{
			StartPos: startPosition,
			EndPos:   endPosition,
		},
	}
}

func (v *ProgramVisitor) VisitBaseType(ctx *BaseTypeContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func (v *ProgramVisitor) VisitNominalType(ctx *NominalTypeContext) interface{} {
	var identifiers []ast.Identifier
	for _, identifierContext := range ctx.AllIdentifier() {
		identifier := identifierContext.Accept(v).(ast.Identifier)
		identifiers = append(identifiers, identifier)
	}

	if identifiers == nil {
		panic(errors.NewUnreachableError())
	}

	var nestedIdentifiers []ast.Identifier
	if len(identifiers) > 1 {
		nestedIdentifiers = identifiers[1:]
	}

	return &ast.NominalType{
		Identifier:        identifiers[0],
		NestedIdentifiers: nestedIdentifiers,
	}
}

func (v *ProgramVisitor) VisitFunctionType(ctx *FunctionTypeContext) interface{} {

	var parameterTypeAnnotations []*ast.TypeAnnotation
	for _, typeAnnotationContext := range ctx.parameterTypes {
		parameterTypeAnnotations = append(
			parameterTypeAnnotations,
			typeAnnotationContext.Accept(v).(*ast.TypeAnnotation),
		)
	}

	if ctx.returnType == nil {
		return nil
	}
	returnTypeAnnotation := ctx.returnType.Accept(v).(*ast.TypeAnnotation)

	startPosition, endPosition := PositionRangeFromContext(ctx)

	return &ast.FunctionType{
		ParameterTypeAnnotations: parameterTypeAnnotations,
		ReturnTypeAnnotation:     returnTypeAnnotation,
		Range: ast.Range{
			StartPos: startPosition,
			EndPos:   endPosition,
		},
	}
}

func (v *ProgramVisitor) VisitVariableSizedType(ctx *VariableSizedTypeContext) interface{} {
	elementType := ctx.FullType().Accept(v).(ast.Type)

	startPosition, endPosition := PositionRangeFromContext(ctx)

	return &ast.VariableSizedType{
		Type: elementType,
		Range: ast.Range{
			StartPos: startPosition,
			EndPos:   endPosition,
		},
	}
}

func (v *ProgramVisitor) VisitConstantSizedType(ctx *ConstantSizedTypeContext) interface{} {
	elementType := ctx.FullType().Accept(v).(ast.Type)

	size := ctx.size.Accept(v).(*ast.IntegerExpression)

	startPosition, endPosition := PositionRangeFromContext(ctx)

	return &ast.ConstantSizedType{
		Type: elementType,
		Size: size,
		Range: ast.Range{
			StartPos: startPosition,
			EndPos:   endPosition,
		},
	}
}

func (v *ProgramVisitor) VisitDictionaryType(ctx *DictionaryTypeContext) interface{} {
	keyType := ctx.keyType.Accept(v).(ast.Type)
	valueType := ctx.valueType.Accept(v).(ast.Type)

	startPosition, endPosition := PositionRangeFromContext(ctx)

	return &ast.DictionaryType{
		KeyType:   keyType,
		ValueType: valueType,
		Range: ast.Range{
			StartPos: startPosition,
			EndPos:   endPosition,
		},
	}
}

func (v *ProgramVisitor) VisitTypeAnnotation(ctx *TypeAnnotationContext) interface{} {
	isResource := ctx.ResourceAnnotation() != nil
	fullType := ctx.FullType().Accept(v).(ast.Type)
	startPosition := PositionFromToken(ctx.GetStart())

	return &ast.TypeAnnotation{
		IsResource: isResource,
		Type:       fullType,
		StartPos:   startPosition,
	}
}

func (v *ProgramVisitor) VisitFullType(ctx *FullTypeContext) interface{} {

	// First, parse the inner type.
	// Second, parse as a reference.
	// Finally, parse as optional.

	// For example:
	//  - `T` = `NominalType{}`
	//  - `R{I}` = `RestrictedType{NominalType{}}`
	//  - `R{I}?` = `OptionalType{RestrictedType{NominalType{}}}`
	//  - `{I}` = `RestrictedType{NominalType{}}`
	//  - `&R` = `ReferenceType{NominalType{}}`
	//  - `&R{I}` = `ReferenceType{RestrictedType{NominalType{}}}`
	//  - `&{I}` = `ReferenceType{RestrictedType{}}`
	//  - `T?` = `OptionalType{NominalType{}}`
	//  - `&T?` = `OptionalType{ReferenceType{NominalType{}}}`

	result := ctx.InnerType().Accept(v).(ast.Type)

	startPos := PositionFromToken(ctx.GetStart())

	// Reference?

	if ctx.Ampersand() != nil {
		authorized := ctx.Auth() != nil

		result = &ast.ReferenceType{
			Authorized: authorized,
			Type:       result,
			StartPos:   startPos,
		}
	}

	// Optionals

	for _, optional := range ctx.optionals {
		endPos := PositionFromToken(optional)
		result = &ast.OptionalType{
			Type:   result,
			EndPos: endPos,
		}
	}

	return result
}

func (v *ProgramVisitor) VisitInnerType(ctx *InnerTypeContext) interface{} {
	// base type
	baseTypeContext := ctx.BaseType()
	var result ast.Type
	if baseTypeContext != nil {
		result = baseTypeContext.Accept(v).(ast.Type)
	}

	// restrictions
	typeRestrictionsCtx := ctx.TypeRestrictions()
	if typeRestrictionsCtx != nil {
		restrictions := typeRestrictionsCtx.Accept(v).([]*ast.NominalType)

		var startPos ast.Position
		if result != nil {
			startPos = result.StartPosition()
		} else {
			startPos = PositionFromToken(typeRestrictionsCtx.GetStart())
		}

		endPos := PositionFromToken(typeRestrictionsCtx.GetStop())

		result = &ast.RestrictedType{
			Type:         result,
			Restrictions: restrictions,
			Range: ast.Range{
				StartPos: startPos,
				EndPos:   endPos,
			},
		}
	}

	if result == nil {
		panic(errors.NewUnreachableError())
	}

	return result
}

func (v *ProgramVisitor) VisitTypeRestrictions(ctx *TypeRestrictionsContext) interface{} {
	nominalTypeContexts := ctx.AllNominalType()

	var nominalTypes []*ast.NominalType
	if len(nominalTypeContexts) == 0 {
		return nominalTypes
	}

	nominalTypes = make([]*ast.NominalType, len(nominalTypeContexts))

	for i, context := range nominalTypeContexts {
		nominalTypes[i] = context.Accept(v).(*ast.NominalType)
	}

	return nominalTypes
}

func (v *ProgramVisitor) VisitBlock(ctx *BlockContext) interface{} {
	return v.visitBlock(ctx.BaseParserRuleContext, ctx.Statements())
}

func (v *ProgramVisitor) VisitFunctionBlock(ctx *FunctionBlockContext) interface{} {
	block := v.visitBlock(ctx.BaseParserRuleContext, ctx.Statements())

	var preConditions *ast.Conditions
	preConditionsCtx := ctx.PreConditions()
	if preConditionsCtx != nil {
		var conditions ast.Conditions = preConditionsCtx.Accept(v).([]*ast.Condition)
		preConditions = &conditions
	}

	var postConditions *ast.Conditions
	postConditionsCtx := ctx.PostConditions()
	if postConditionsCtx != nil {
		var conditions ast.Conditions = postConditionsCtx.Accept(v).([]*ast.Condition)
		postConditions = &conditions
	}

	return &ast.FunctionBlock{
		Block:          block,
		PreConditions:  preConditions,
		PostConditions: postConditions,
	}
}

func (v *ProgramVisitor) visitBlock(ctx antlr.ParserRuleContext, statementsCtx IStatementsContext) *ast.Block {
	statements := statementsCtx.Accept(v).([]ast.Statement)
	startPosition, endPosition := PositionRangeFromContext(ctx)
	return &ast.Block{
		Statements: statements,
		Range: ast.Range{
			StartPos: startPosition,
			EndPos:   endPosition,
		},
	}
}

func (v *ProgramVisitor) VisitPreConditions(ctx *PreConditionsContext) interface{} {
	return ctx.Conditions().Accept(v)
}

func (v *ProgramVisitor) VisitPostConditions(ctx *PostConditionsContext) interface{} {
	return ctx.Conditions().Accept(v)
}

func (v *ProgramVisitor) VisitConditions(ctx *ConditionsContext) interface{} {
	var conditions []*ast.Condition
	for _, statement := range ctx.AllCondition() {
		conditions = append(
			conditions,
			statement.Accept(v).(*ast.Condition),
		)
	}
	return conditions
}

func (v *ProgramVisitor) VisitCondition(ctx *ConditionContext) interface{} {
	parentParent := ctx.GetParent().GetParent()

	_, isPreCondition := parentParent.(*PreConditionsContext)
	_, isPostCondition := parentParent.(*PostConditionsContext)

	var kind ast.ConditionKind
	if isPreCondition {
		kind = ast.ConditionKindPre
	} else if isPostCondition {
		kind = ast.ConditionKindPost
	} else {
		panic(errors.NewUnreachableError())
	}

	test := ctx.test.Accept(v).(ast.Expression)

	var message ast.Expression
	if ctx.message != nil {
		message = ctx.message.Accept(v).(ast.Expression)
	}

	return &ast.Condition{
		Kind:    kind,
		Test:    test,
		Message: message,
	}
}

func (v *ProgramVisitor) VisitStatements(ctx *StatementsContext) interface{} {
	var statements []ast.Statement
	for _, statement := range ctx.AllStatement() {
		statements = append(
			statements,
			statement.Accept(v).(ast.Statement),
		)
	}
	return statements
}

func (v *ProgramVisitor) VisitChildren(node antlr.RuleNode) interface{} {
	for _, child := range node.GetChildren() {
		ruleChild, ok := child.(antlr.RuleNode)
		if !ok {
			continue
		}

		result := ruleChild.Accept(v)
		if result != nil {
			return result
		}
	}

	return nil
}

func (v *ProgramVisitor) VisitStatement(ctx *StatementContext) interface{} {
	result := v.VisitChildren(ctx.BaseParserRuleContext)
	if expression, ok := result.(ast.Expression); ok {
		return &ast.ExpressionStatement{
			Expression: expression,
		}
	}

	return result
}

func (v *ProgramVisitor) VisitReturnStatement(ctx *ReturnStatementContext) interface{} {
	expressionNode := ctx.Expression()
	var expression ast.Expression
	if expressionNode != nil {
		expression = expressionNode.Accept(v).(ast.Expression)
	}

	startPosition := PositionFromToken(ctx.GetStart())

	var endPosition ast.Position
	if expression != nil {
		endPosition = expression.EndPosition()
	} else {
		returnEnd := ctx.Return().GetSymbol().GetStop()
		endPosition = ast.EndPosition(startPosition, returnEnd)
	}

	return &ast.ReturnStatement{
		Expression: expression,
		Range: ast.Range{
			StartPos: startPosition,
			EndPos:   endPosition,
		},
	}
}

func (v *ProgramVisitor) VisitBreakStatement(ctx *BreakStatementContext) interface{} {
	startPosition := PositionFromToken(ctx.GetStart())
	endPosition := ast.EndPosition(startPosition, ctx.Break().GetSymbol().GetStop())

	return &ast.BreakStatement{
		Range: ast.Range{
			StartPos: startPosition,
			EndPos:   endPosition,
		},
	}
}

func (v *ProgramVisitor) VisitContinueStatement(ctx *ContinueStatementContext) interface{} {
	startPosition := PositionFromToken(ctx.GetStart())
	endPosition := ast.EndPosition(startPosition, ctx.Continue().GetSymbol().GetStop())

	return &ast.ContinueStatement{
		Range: ast.Range{
			StartPos: startPosition,
			EndPos:   endPosition,
		},
	}
}

func (v *ProgramVisitor) VisitVariableDeclaration(ctx *VariableDeclarationContext) interface{} {
	access := ctx.Access().Accept(v).(ast.Access)

	variableKind := ctx.VariableKind().Accept(v).(ast.VariableKind)
	isConstant := variableKind == ast.VariableKindConstant

	identifier := ctx.Identifier().Accept(v).(ast.Identifier)

	// Parse the left expression and the left transfer (required)

	leftExpressionResult := ctx.leftExpression.Accept(v)
	if leftExpressionResult == nil {
		return nil
	}
	leftExpression := leftExpressionResult.(ast.Expression)

	castingExpression, leftIsCasting := leftExpression.(*ast.CastingExpression)

	var typeAnnotation *ast.TypeAnnotation
	typeAnnotationContext := ctx.TypeAnnotation()
	if typeAnnotationContext != nil {
		typeAnnotation, _ = typeAnnotationContext.Accept(v).(*ast.TypeAnnotation)
	}

	leftTransfer := ctx.leftTransfer.Accept(v).(*ast.Transfer)

	// Parse the right transfer and the right expression (optional)

	var rightTransfer *ast.Transfer
	var rightExpression ast.Expression

	if ctx.rightExpression != nil && ctx.rightTransfer != nil {
		rightTransfer = ctx.rightTransfer.Accept(v).(*ast.Transfer)

		rightExpressionResult := ctx.rightExpression.Accept(v)
		rightExpression = rightExpressionResult.(ast.Expression)
	}

	startPosition := PositionFromToken(ctx.GetStart())

	variableDeclaration := &ast.VariableDeclaration{
		Access:         access,
		IsConstant:     isConstant,
		Identifier:     identifier,
		Value:          leftExpression,
		TypeAnnotation: typeAnnotation,
		Transfer:       leftTransfer,
		StartPos:       startPosition,
		SecondTransfer: rightTransfer,
		SecondValue:    rightExpression,
	}

	if leftIsCasting {
		castingExpression.ParentVariableDeclaration = variableDeclaration
	}

	return variableDeclaration
}

func (v *ProgramVisitor) VisitVariableKind(ctx *VariableKindContext) interface{} {
	switch {
	case ctx.Let() != nil:
		return ast.VariableKindConstant

	case ctx.Var() != nil:
		return ast.VariableKindVariable

	default:
		return ast.VariableKindNotSpecified
	}
}

func (v *ProgramVisitor) VisitIfStatement(ctx *IfStatementContext) interface{} {
	var variableDeclaration *ast.VariableDeclaration

	var test ast.IfStatementTest
	if ctx.testExpression != nil {
		test = ctx.testExpression.Accept(v).(ast.Expression)
	} else if ctx.testDeclaration != nil {
		variableDeclaration = ctx.testDeclaration.Accept(v).(*ast.VariableDeclaration)
		test = variableDeclaration
	} else {
		panic(errors.NewUnreachableError())
	}

	then := ctx.then.Accept(v).(*ast.Block)

	var elseBlock *ast.Block
	if ctx.alt != nil {
		elseBlock = ctx.alt.Accept(v).(*ast.Block)
	} else {
		ifStatementContext := ctx.IfStatement()
		if ifStatementContext != nil {
			if ifStatement, ok := ifStatementContext.Accept(v).(*ast.IfStatement); ok {
				elseBlock = &ast.Block{
					Statements: []ast.Statement{ifStatement},
					Range:      ast.NewRangeFromPositioned(ifStatement),
				}
			}
		}
	}

	startPosition := PositionFromToken(ctx.GetStart())

	ifStatement := &ast.IfStatement{
		Test:     test,
		Then:     then,
		Else:     elseBlock,
		StartPos: startPosition,
	}

	if variableDeclaration != nil {
		variableDeclaration.ParentIfStatement = ifStatement
	}

	return ifStatement
}

func (v *ProgramVisitor) VisitWhileStatement(ctx *WhileStatementContext) interface{} {
	test := ctx.Expression().Accept(v).(ast.Expression)
	block := ctx.Block().Accept(v).(*ast.Block)

	startPosition := PositionFromToken(ctx.GetStart())

	return &ast.WhileStatement{
		Test:     test,
		Block:    block,
		StartPos: startPosition,
	}
}

func (v *ProgramVisitor) VisitForStatement(ctx *ForStatementContext) interface{} {

	identifier := ctx.Identifier().Accept(v).(ast.Identifier)

	value := ctx.Expression().Accept(v).(ast.Expression)

	block := ctx.Block().Accept(v).(*ast.Block)

	startPosition := PositionFromToken(ctx.GetStart())

	return &ast.ForStatement{
		Identifier: identifier,
		Value:      value,
		Block:      block,
		StartPos:   startPosition,
	}
}

func (v *ProgramVisitor) VisitAssignment(ctx *AssignmentContext) interface{} {
	target := ctx.target.Accept(v).(ast.Expression)
	transfer := ctx.Transfer().Accept(v).(*ast.Transfer)
	value := ctx.value.Accept(v).(ast.Expression)

	return &ast.AssignmentStatement{
		Target:   target,
		Transfer: transfer,
		Value:    value,
	}
}

func (v *ProgramVisitor) VisitTransfer(ctx *TransferContext) interface{} {
	operation := ast.TransferOperationCopy
	switch {
	case ctx.Move() != nil:
		operation = ast.TransferOperationMove
	case ctx.MoveForced() != nil:
		operation = ast.TransferOperationMoveForced
	}

	position := PositionFromToken(ctx.GetStart())

	return &ast.Transfer{
		Operation: operation,
		Pos:       position,
	}
}

func (v *ProgramVisitor) VisitSwap(ctx *SwapContext) interface{} {
	left := ctx.left.Accept(v).(ast.Expression)
	right := ctx.right.Accept(v).(ast.Expression)

	return &ast.SwapStatement{
		Left:  left,
		Right: right,
	}
}

// NOTE: manually go over all child rules and find a match
func (v *ProgramVisitor) VisitExpression(ctx *ExpressionContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func (v *ProgramVisitor) VisitConditionalExpression(ctx *ConditionalExpressionContext) interface{} {
	element := ctx.OrExpression().Accept(v)
	if element == nil {
		return nil
	}
	expression := element.(ast.Expression)

	if ctx.then != nil && ctx.alt != nil {
		then := ctx.then.Accept(v).(ast.Expression)
		alt := ctx.alt.Accept(v).(ast.Expression)

		return &ast.ConditionalExpression{
			Test: expression,
			Then: then,
			Else: alt,
		}
	}

	return expression
}

func (v *ProgramVisitor) VisitOrExpression(ctx *OrExpressionContext) interface{} {
	right := ctx.AndExpression().Accept(v)
	if right == nil {
		return nil
	}
	rightExpression := right.(ast.Expression)

	leftContext := ctx.OrExpression()
	if leftContext == nil {
		return rightExpression
	}

	leftExpression := leftContext.Accept(v).(ast.Expression)

	return &ast.BinaryExpression{
		Operation: ast.OperationOr,
		Left:      leftExpression,
		Right:     rightExpression,
	}
}

func (v *ProgramVisitor) VisitAndExpression(ctx *AndExpressionContext) interface{} {
	right := ctx.EqualityExpression().Accept(v)
	if right == nil {
		return nil
	}
	rightExpression := right.(ast.Expression)

	leftContext := ctx.AndExpression()
	if leftContext == nil {
		return rightExpression
	}

	leftExpression := leftContext.Accept(v).(ast.Expression)

	return &ast.BinaryExpression{
		Operation: ast.OperationAnd,
		Left:      leftExpression,
		Right:     rightExpression,
	}
}

func (v *ProgramVisitor) VisitEqualityExpression(ctx *EqualityExpressionContext) interface{} {
	right := ctx.RelationalExpression().Accept(v)
	if right == nil {
		return nil
	}
	rightExpression := right.(ast.Expression)

	leftContext := ctx.EqualityExpression()
	if leftContext == nil {
		return rightExpression
	}

	leftExpression := leftContext.Accept(v).(ast.Expression)
	operation := ctx.EqualityOp().Accept(v).(ast.Operation)

	return &ast.BinaryExpression{
		Operation: operation,
		Left:      leftExpression,
		Right:     rightExpression,
	}
}

func (v *ProgramVisitor) VisitRelationalExpression(ctx *RelationalExpressionContext) interface{} {
	right := ctx.NilCoalescingExpression().Accept(v)
	if right == nil {
		return nil
	}
	rightExpression := right.(ast.Expression)

	leftContext := ctx.RelationalExpression()
	if leftContext == nil {
		return rightExpression
	}

	leftExpression := leftContext.Accept(v).(ast.Expression)
	operation := ctx.RelationalOp().Accept(v).(ast.Operation)

	return &ast.BinaryExpression{
		Operation: operation,
		Left:      leftExpression,
		Right:     rightExpression,
	}
}

func (v *ProgramVisitor) VisitNilCoalescingExpression(ctx *NilCoalescingExpressionContext) interface{} {
	// NOTE: right associative

	left := ctx.BitwiseOrExpression().Accept(v)
	if left == nil {
		return nil
	}

	leftExpression := left.(ast.Expression)

	rightContext := ctx.NilCoalescingExpression()
	if rightContext == nil {
		return leftExpression
	}

	rightExpression := rightContext.Accept(v).(ast.Expression)
	return &ast.BinaryExpression{
		Operation: ast.OperationNilCoalesce,
		Left:      leftExpression,
		Right:     rightExpression,
	}
}

func (v *ProgramVisitor) VisitBitwiseOrExpression(ctx *BitwiseOrExpressionContext) interface{} {
	right := ctx.BitwiseXorExpression().Accept(v)
	if right == nil {
		return nil
	}
	rightExpression := right.(ast.Expression)

	leftContext := ctx.BitwiseOrExpression()
	if leftContext == nil {
		return rightExpression
	}

	leftExpression := leftContext.Accept(v).(ast.Expression)

	return &ast.BinaryExpression{
		Operation: ast.OperationBitwiseOr,
		Left:      leftExpression,
		Right:     rightExpression,
	}
}

func (v *ProgramVisitor) VisitBitwiseXorExpression(ctx *BitwiseXorExpressionContext) interface{} {
	right := ctx.BitwiseAndExpression().Accept(v)
	if right == nil {
		return nil
	}
	rightExpression := right.(ast.Expression)

	leftContext := ctx.BitwiseXorExpression()
	if leftContext == nil {
		return rightExpression
	}

	leftExpression := leftContext.Accept(v).(ast.Expression)

	return &ast.BinaryExpression{
		Operation: ast.OperationBitwiseXor,
		Left:      leftExpression,
		Right:     rightExpression,
	}
}

func (v *ProgramVisitor) VisitBitwiseAndExpression(ctx *BitwiseAndExpressionContext) interface{} {
	right := ctx.BitwiseShiftExpression().Accept(v)
	if right == nil {
		return nil
	}
	rightExpression := right.(ast.Expression)

	leftContext := ctx.BitwiseAndExpression()
	if leftContext == nil {
		return rightExpression
	}

	leftExpression := leftContext.Accept(v).(ast.Expression)

	return &ast.BinaryExpression{
		Operation: ast.OperationBitwiseAnd,
		Left:      leftExpression,
		Right:     rightExpression,
	}
}

func (v *ProgramVisitor) VisitBitwiseShiftExpression(ctx *BitwiseShiftExpressionContext) interface{} {
	right := ctx.AdditiveExpression().Accept(v)
	if right == nil {
		return nil
	}
	rightExpression := right.(ast.Expression)

	leftContext := ctx.BitwiseShiftExpression()
	if leftContext == nil {
		return rightExpression
	}

	leftExpression := leftContext.Accept(v).(ast.Expression)
	operation := ctx.BitwiseShiftOp().Accept(v).(ast.Operation)

	return &ast.BinaryExpression{
		Operation: operation,
		Left:      leftExpression,
		Right:     rightExpression,
	}
}

func (v *ProgramVisitor) VisitAdditiveExpression(ctx *AdditiveExpressionContext) interface{} {
	right := ctx.MultiplicativeExpression().Accept(v)
	if right == nil {
		return nil
	}
	rightExpression := right.(ast.Expression)

	leftContext := ctx.AdditiveExpression()
	if leftContext == nil {
		return rightExpression
	}

	leftExpression := leftContext.Accept(v).(ast.Expression)
	operation := ctx.AdditiveOp().Accept(v).(ast.Operation)

	return &ast.BinaryExpression{
		Operation: operation,
		Left:      leftExpression,
		Right:     rightExpression,
	}
}

func (v *ProgramVisitor) VisitMultiplicativeExpression(ctx *MultiplicativeExpressionContext) interface{} {
	right := ctx.CastingExpression().Accept(v)
	if right == nil {
		return nil
	}
	rightExpression := right.(ast.Expression)

	leftContext := ctx.MultiplicativeExpression()
	if leftContext == nil {
		return rightExpression
	}

	leftExpression := leftContext.Accept(v).(ast.Expression)
	operation := ctx.MultiplicativeOp().Accept(v).(ast.Operation)

	return &ast.BinaryExpression{
		Operation: operation,
		Left:      leftExpression,
		Right:     rightExpression,
	}
}

func (v *ProgramVisitor) VisitCastingExpression(ctx *CastingExpressionContext) interface{} {
	typeAnnotationContext := ctx.TypeAnnotation()
	if typeAnnotationContext == nil {
		return ctx.UnaryExpression().Accept(v)
	}

	expression := ctx.CastingExpression().Accept(v).(ast.Expression)
	typeAnnotation := typeAnnotationContext.Accept(v).(*ast.TypeAnnotation)
	operation := ctx.CastingOp().Accept(v).(ast.Operation)

	return &ast.CastingExpression{
		Expression:     expression,
		Operation:      operation,
		TypeAnnotation: typeAnnotation,
	}
}

func (v *ProgramVisitor) VisitUnaryExpression(ctx *UnaryExpressionContext) interface{} {
	unaryContext := ctx.UnaryExpression()
	if unaryContext == nil {
		primaryContext := ctx.PrimaryExpression()
		if primaryContext == nil {
			return nil
		}
		return primaryContext.Accept(v)
	}

	// ensure unary operators are not juxtaposed
	if ctx.GetChildCount() > 2 {
		position := PositionFromToken(ctx.UnaryOp(0).GetStart())
		v.report(
			&JuxtaposedUnaryOperatorsError{
				Pos: position,
			},
		)
	}

	expression := unaryContext.Accept(v).(ast.Expression)
	operation := ctx.UnaryOp(0).Accept(v).(ast.Operation)

	startPosition := PositionFromToken(ctx.GetStart())

	return &ast.UnaryExpression{
		Operation:  operation,
		Expression: expression,
		StartPos:   startPosition,
	}
}

func (v *ProgramVisitor) VisitUnaryOp(ctx *UnaryOpContext) interface{} {
	switch {
	case ctx.Negate() != nil:
		return ast.OperationNegate

	case ctx.Minus() != nil:
		return ast.OperationMinus

	case ctx.Move() != nil:
		return ast.OperationMove

	default:
		panic(errors.NewUnreachableError())
	}
}

func (v *ProgramVisitor) VisitPrimaryExpression(ctx *PrimaryExpressionContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func (v *ProgramVisitor) wrapPartialAccessExpression(
	wrapped ast.Expression,
	partialAccessExpression ast.AccessExpression,
) ast.Expression {

	switch partialAccessExpression := partialAccessExpression.(type) {
	case *ast.IndexExpression:
		return &ast.IndexExpression{
			TargetExpression:   wrapped,
			IndexingExpression: partialAccessExpression.IndexingExpression,
			Range:              ast.NewRangeFromPositioned(partialAccessExpression),
		}

	case *ast.MemberExpression:
		return &ast.MemberExpression{
			Expression: wrapped,
			Optional:   partialAccessExpression.Optional,
			Identifier: partialAccessExpression.Identifier,
		}
	}

	panic(errors.NewUnreachableError())
}

func (v *ProgramVisitor) VisitInvocationExpression(ctx *InvocationExpressionContext) interface{} {
	expression := ctx.Invocation().Accept(v).(*ast.InvocationExpression)
	expression.InvokedExpression = ctx.PostfixExpression().Accept(v).(ast.Expression)
	return expression
}

func (v *ProgramVisitor) VisitAccessExpression(ctx *AccessExpressionContext) interface{} {
	accessExpression := ctx.ExpressionAccess().Accept(v).(ast.AccessExpression)
	accessedExpression := ctx.PostfixExpression().Accept(v).(ast.Expression)
	return v.wrapPartialAccessExpression(accessedExpression, accessExpression)
}

func (v *ProgramVisitor) VisitForceExpression(ctx *ForceExpressionContext) interface{} {
	expression := ctx.PostfixExpression().Accept(v).(ast.Expression)

	endPosition := PositionFromToken(ctx.GetStop())

	return &ast.ForceExpression{
		Expression: expression,
		EndPos:     endPosition,
	}
}

func (v *ProgramVisitor) VisitExpressionAccess(ctx *ExpressionAccessContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func (v *ProgramVisitor) VisitMemberAccess(ctx *MemberAccessContext) interface{} {
	identifier := ctx.Identifier().Accept(v).(ast.Identifier)
	optional := ctx.Optional() != nil

	// NOTE: partial, expression is filled later
	return &ast.MemberExpression{
		Optional:   optional,
		Identifier: identifier,
	}
}

func (v *ProgramVisitor) VisitBracketExpression(ctx *BracketExpressionContext) interface{} {

	indexExpression := ctx.Expression().Accept(v).(ast.Expression)

	startPosition, endPosition := PositionRangeFromContext(ctx)

	// NOTE: partial, expression is filled later
	return &ast.IndexExpression{
		IndexingExpression: indexExpression,
		Range: ast.Range{
			StartPos: startPosition,
			EndPos:   endPosition,
		},
	}
}

func (v *ProgramVisitor) VisitCreateExpression(ctx *CreateExpressionContext) interface{} {
	invocation := ctx.Invocation().Accept(v).(*ast.InvocationExpression)
	ty := ctx.NominalType().Accept(v).(*ast.NominalType)
	invocation.InvokedExpression = &ast.IdentifierExpression{
		Identifier: ty.Identifier,
	}

	for _, nestedIdentifier := range ty.NestedIdentifiers {
		invocation.InvokedExpression = &ast.MemberExpression{
			Expression: invocation.InvokedExpression,
			Identifier: nestedIdentifier,
		}
	}

	startPosition := PositionFromToken(ctx.GetStart())

	return &ast.CreateExpression{
		InvocationExpression: invocation,
		StartPos:             startPosition,
	}
}

func (v *ProgramVisitor) VisitDestroyExpression(ctx *DestroyExpressionContext) interface{} {
	expression := ctx.Expression().Accept(v).(ast.Expression)

	startPosition := PositionFromToken(ctx.GetStart())

	return &ast.DestroyExpression{
		Expression: expression,
		StartPos:   startPosition,
	}
}

func (v *ProgramVisitor) VisitReferenceExpression(ctx *ReferenceExpressionContext) interface{} {
	expression := ctx.Expression().Accept(v).(ast.Expression)
	ty := ctx.FullType().Accept(v).(ast.Type)

	startPosition := PositionFromToken(ctx.GetStart())

	return &ast.ReferenceExpression{
		Expression: expression,
		Type:       ty,
		StartPos:   startPosition,
	}
}

func (v *ProgramVisitor) VisitLiteralExpression(ctx *LiteralExpressionContext) interface{} {
	return ctx.Literal().Accept(v)
}

// NOTE: manually go over all child rules and find a match
func (v *ProgramVisitor) VisitLiteral(ctx *LiteralContext) interface{} {
	return v.VisitChildren(ctx.BaseParserRuleContext)
}

func (v *ProgramVisitor) parseFixedPointPart(part string) (integer *big.Int, scale uint) {
	withoutUnderscores := strings.Replace(part, "_", "", -1)
	integer, _ = new(big.Int).SetString(withoutUnderscores, 10)
	return integer, uint(len(withoutUnderscores))
}

func (v *ProgramVisitor) VisitFixedPointLiteral(ctx *FixedPointLiteralContext) interface{} {
	token := ctx.PositiveFixedPointLiteral().GetSymbol()
	startPosition := PositionFromToken(token)
	endPosition := ast.EndPosition(startPosition, token.GetStop())

	parts := strings.Split(token.GetText(), ".")
	integer, _ := v.parseFixedPointPart(parts[0])
	fractional, scale := v.parseFixedPointPart(parts[1])

	// NOTE: can't just negate integer, might be 0 and fractional part > 0
	negative := ctx.Minus() != nil

	if negative {
		startPosition = PositionFromToken(ctx.Minus().GetSymbol())
	}

	return &ast.FixedPointExpression{
		Negative:        negative,
		UnsignedInteger: integer,
		Fractional:      fractional,
		Scale:           scale,
		Range: ast.Range{
			StartPos: startPosition,
			EndPos:   endPosition,
		},
	}
}

func (v *ProgramVisitor) VisitIntegerLiteral(ctx *IntegerLiteralContext) interface{} {
	intExpression := ctx.PositiveIntegerLiteral().Accept(v).(*ast.IntegerExpression)
	if ctx.Minus() != nil {
		if intExpression.Value != nil {
			intExpression.Value.Neg(intExpression.Value)
		}
		intExpression.StartPos = PositionFromToken(ctx.GetStart())
	}
	return intExpression
}

func (v *ProgramVisitor) parseIntegerExpression(token antlr.Token, text string, kind IntegerLiteralKind) *ast.IntegerExpression {
	startPosition := PositionFromToken(token)
	endPosition := ast.EndPosition(startPosition, token.GetStop())
	tokenRange := ast.Range{
		StartPos: startPosition,
		EndPos:   endPosition,
	}
	literal := token.GetText()

	report := func(invalidKind InvalidNumberLiteralKind) {
		v.report(
			&InvalidIntegerLiteralError{
				IntegerLiteralKind:        kind,
				InvalidIntegerLiteralKind: invalidKind,
				// NOTE: not using text, because it has the base-prefix stripped
				Literal: literal,
				Range:   tokenRange,
			},
		)
	}

	// check literal has no leading underscore

	if strings.HasPrefix(text, "_") {
		report(InvalidNumberLiteralKindLeadingUnderscore)
	}

	// check literal has no trailing underscore
	if strings.HasSuffix(text, "_") {
		report(InvalidNumberLiteralKindTrailingUnderscore)
	}

	withoutUnderscores := strings.Replace(text, "_", "", -1)

	base := kind.Base()
	value, ok := new(big.Int).SetString(withoutUnderscores, base)
	if !ok {
		report(InvalidNumberLiteralKindUnknown)
	}

	return &ast.IntegerExpression{
		Value: value,
		Base:  base,
		Range: tokenRange,
	}
}

func (v *ProgramVisitor) VisitInvalidNumberLiteral(ctx *InvalidNumberLiteralContext) interface{} {
	startPosition := PositionFromToken(ctx.GetStart())
	endPosition := ast.EndPosition(startPosition, ctx.GetStop().GetStop())

	v.report(
		&InvalidIntegerLiteralError{
			IntegerLiteralKind:        IntegerLiteralKindUnknown,
			InvalidIntegerLiteralKind: InvalidNumberLiteralKindUnknownPrefix,
			Literal:                   ctx.GetText(),
			Range: ast.Range{
				StartPos: startPosition,
				EndPos:   endPosition,
			},
		},
	)

	return &ast.IntegerExpression{
		Range: ast.Range{
			StartPos: startPosition,
			EndPos:   endPosition,
		},
	}
}

func (v *ProgramVisitor) VisitDecimalLiteral(ctx *DecimalLiteralContext) interface{} {
	return v.parseIntegerExpression(
		ctx.GetStart(),
		ctx.GetText(),
		IntegerLiteralKindDecimal,
	)
}

func (v *ProgramVisitor) VisitBinaryLiteral(ctx *BinaryLiteralContext) interface{} {
	return v.parseIntegerExpression(
		ctx.GetStart(),
		ctx.GetText()[2:],
		IntegerLiteralKindBinary,
	)
}

func (v *ProgramVisitor) VisitOctalLiteral(ctx *OctalLiteralContext) interface{} {
	return v.parseIntegerExpression(
		ctx.GetStart(),
		ctx.GetText()[2:],
		IntegerLiteralKindOctal,
	)
}

func (v *ProgramVisitor) VisitHexadecimalLiteral(ctx *HexadecimalLiteralContext) interface{} {
	return v.parseIntegerExpression(
		ctx.GetStart(),
		ctx.GetText()[2:],
		IntegerLiteralKindHexadecimal,
	)
}

func (v *ProgramVisitor) VisitNestedExpression(ctx *NestedExpressionContext) interface{} {
	return ctx.Expression().Accept(v)
}

func (v *ProgramVisitor) VisitBooleanLiteral(ctx *BooleanLiteralContext) interface{} {
	startPosition := PositionFromToken(ctx.GetStart())

	trueNode := ctx.True()
	if trueNode != nil {
		endPosition := ast.EndPosition(startPosition, trueNode.GetSymbol().GetStop())

		return &ast.BoolExpression{
			Value: true,
			Range: ast.Range{
				StartPos: startPosition,
				EndPos:   endPosition,
			},
		}
	}

	falseNode := ctx.False()
	if falseNode != nil {
		endPosition := ast.EndPosition(startPosition, falseNode.GetSymbol().GetStop())

		return &ast.BoolExpression{
			Value: false,
			Range: ast.Range{
				StartPos: startPosition,
				EndPos:   endPosition,
			},
		}
	}

	panic(errors.NewUnreachableError())
}

func (v *ProgramVisitor) VisitNilLiteral(ctx *NilLiteralContext) interface{} {
	position := PositionFromToken(ctx.GetStart())
	return &ast.NilExpression{
		Pos: position,
	}
}

func (v *ProgramVisitor) VisitStringLiteral(ctx *StringLiteralContext) interface{} {
	startPosition := PositionFromToken(ctx.GetStart())
	endPosition := ast.EndPosition(startPosition, ctx.StringLiteral().GetSymbol().GetStop())

	stringLiteral := ctx.StringLiteral().GetText()

	// slice off leading and trailing quotes
	// and parse escape characters
	parsedString := parseStringLiteral(
		stringLiteral[1 : len(stringLiteral)-1],
	)

	return &ast.StringExpression{
		Value: parsedString,
		Range: ast.Range{
			StartPos: startPosition,
			EndPos:   endPosition,
		},
	}
}

func parseStringLiteral(s string) string {
	var builder strings.Builder

	var c byte
	for len(s) > 0 {
		c, s = s[0], s[1:]

		if c != '\\' {
			builder.WriteByte(c)
			continue
		}

		c, s = s[0], s[1:]

		switch c {
		case '0':
			builder.WriteByte(0)
		case 'n':
			builder.WriteByte('\n')
		case 'r':
			builder.WriteByte('\r')
		case 't':
			builder.WriteByte('\t')
		case '"':
			builder.WriteByte('"')
		case '\'':
			builder.WriteByte('\'')
		case '\\':
			builder.WriteByte('\\')
		case 'u':
			// skip `{`
			s = s[1:]

			j := 0
			var v rune
			for ; s[j] != '}' && j < 8; j++ {
				x := parseHex(s[j])
				v = v<<4 | x
			}

			builder.WriteRune(v)

			// skip hex characters and `}`

			s = s[j+1:]
		}
	}

	return builder.String()
}

func parseHex(b byte) rune {
	c := rune(b)
	switch {
	case '0' <= c && c <= '9':
		return c - '0'
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10
	}

	panic(errors.NewUnreachableError())
}

func (v *ProgramVisitor) VisitPathLiteral(ctx *PathLiteralContext) interface{} {
	startPos := PositionFromToken(ctx.GetStart())

	domain := ctx.domain.Accept(v).(ast.Identifier)
	identifier := ctx.id.Accept(v).(ast.Identifier)

	return &ast.PathExpression{
		StartPos:   startPos,
		Domain:     domain,
		Identifier: identifier,
	}
}

func (v *ProgramVisitor) VisitArrayLiteral(ctx *ArrayLiteralContext) interface{} {
	var expressions []ast.Expression
	for _, expression := range ctx.AllExpression() {
		expressions = append(
			expressions,
			expression.Accept(v).(ast.Expression),
		)
	}

	startPosition, endPosition := PositionRangeFromContext(ctx)

	return &ast.ArrayExpression{
		Values: expressions,
		Range: ast.Range{
			StartPos: startPosition,
			EndPos:   endPosition,
		},
	}
}

func (v *ProgramVisitor) VisitDictionaryLiteral(ctx *DictionaryLiteralContext) interface{} {
	var entries []ast.Entry
	for _, entry := range ctx.AllDictionaryEntry() {
		entries = append(
			entries,
			entry.Accept(v).(ast.Entry),
		)
	}

	startPosition, endPosition := PositionRangeFromContext(ctx)

	return &ast.DictionaryExpression{
		Entries: entries,
		Range: ast.Range{
			StartPos: startPosition,
			EndPos:   endPosition,
		},
	}
}

func (v *ProgramVisitor) VisitDictionaryEntry(ctx *DictionaryEntryContext) interface{} {
	key := ctx.key.Accept(v).(ast.Expression)
	value := ctx.value.Accept(v).(ast.Expression)

	return ast.Entry{
		Key:   key,
		Value: value,
	}
}

func (v *ProgramVisitor) VisitIdentifierExpression(ctx *IdentifierExpressionContext) interface{} {
	identifier := ctx.Identifier().Accept(v).(ast.Identifier)

	return &ast.IdentifierExpression{
		Identifier: identifier,
	}
}

func (v *ProgramVisitor) VisitIdentifier(ctx *IdentifierContext) interface{} {

	text := ctx.GetText()
	pos := PositionFromToken(ctx.GetStart())

	return ast.Identifier{
		Identifier: text,
		Pos:        pos,
	}
}

func (v *ProgramVisitor) VisitInvocation(ctx *InvocationContext) interface{} {

	// type arguments

	var typeArguments []*ast.TypeAnnotation
	for _, typeArgument := range ctx.AllTypeAnnotation() {
		typeArguments = append(
			typeArguments,
			typeArgument.Accept(v).(*ast.TypeAnnotation),
		)
	}

	// arguments

	var arguments []*ast.Argument
	for _, argument := range ctx.AllArgument() {
		arguments = append(
			arguments,
			argument.Accept(v).(*ast.Argument),
		)
	}

	endPosition := PositionFromToken(ctx.GetStop())

	// NOTE: partial, argument is filled later
	return &ast.InvocationExpression{
		TypeArguments: typeArguments,
		Arguments:     arguments,
		EndPos:        endPosition,
	}
}

func (v *ProgramVisitor) VisitArgument(ctx *ArgumentContext) interface{} {
	identifierNode := ctx.Identifier()
	label := ""
	var labelStartPos, labelEndPos *ast.Position
	if identifierNode != nil {
		label = identifierNode.GetText()
		symbol := identifierNode.GetStart()
		startPos := PositionFromToken(symbol)
		endPos := ast.EndPosition(startPos, symbol.GetStop())
		labelStartPos = &startPos
		labelEndPos = &endPos
	}
	expression := ctx.Expression().Accept(v).(ast.Expression)
	return &ast.Argument{
		Label:         label,
		LabelStartPos: labelStartPos,
		LabelEndPos:   labelEndPos,
		Expression:    expression,
	}
}

func (v *ProgramVisitor) VisitCastingOp(ctx *CastingOpContext) interface{} {
	switch {
	case ctx.Casting() != nil:
		return ast.OperationCast

	case ctx.FailableCasting() != nil:
		return ast.OperationFailableCast

	case ctx.ForceCasting() != nil:
		return ast.OperationForceCast

	default:
		panic(errors.NewUnreachableError())
	}
}

func (v *ProgramVisitor) VisitEqualityOp(ctx *EqualityOpContext) interface{} {
	switch {
	case ctx.Equal() != nil:
		return ast.OperationEqual

	case ctx.Unequal() != nil:
		return ast.OperationNotEqual

	default:
		panic(errors.NewUnreachableError())
	}
}

func (v *ProgramVisitor) VisitRelationalOp(ctx *RelationalOpContext) interface{} {
	switch {
	case ctx.Less() != nil:
		return ast.OperationLess

	case ctx.Greater() != nil:
		return ast.OperationGreater

	case ctx.LessEqual() != nil:
		return ast.OperationLessEqual

	case ctx.GreaterEqual() != nil:
		return ast.OperationGreaterEqual

	default:
		panic(errors.NewUnreachableError())
	}
}

func (v *ProgramVisitor) VisitBitwiseShiftOp(ctx *BitwiseShiftOpContext) interface{} {
	switch {
	case ctx.ShiftLeft() != nil:
		return ast.OperationBitwiseLeftShift

	case ctx.ShiftRight() != nil:
		return ast.OperationBitwiseRightShift

	default:
		panic(errors.NewUnreachableError())
	}
}

func (v *ProgramVisitor) VisitAdditiveOp(ctx *AdditiveOpContext) interface{} {
	switch {
	case ctx.Plus() != nil:
		return ast.OperationPlus

	case ctx.Minus() != nil:
		return ast.OperationMinus

	default:
		panic(errors.NewUnreachableError())
	}
}

func (v *ProgramVisitor) VisitMultiplicativeOp(ctx *MultiplicativeOpContext) interface{} {
	switch {
	case ctx.Mul() != nil:
		return ast.OperationMul

	case ctx.Div() != nil:
		return ast.OperationDiv

	case ctx.Mod() != nil:
		return ast.OperationMod

	default:
		panic(errors.NewUnreachableError())
	}
}
