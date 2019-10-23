package ast

import (
	"fmt"

	"github.com/dapperlabs/flow-go/language/runtime/errors"
)

type BoolExtractor interface {
	ExtractBool(extractor *ExpressionExtractor, expression *BoolExpression) ExpressionExtraction
}

type NilExtractor interface {
	ExtractNil(extractor *ExpressionExtractor, expression *NilExpression) ExpressionExtraction
}

type IntExtractor interface {
	ExtractInt(extractor *ExpressionExtractor, expression *IntExpression) ExpressionExtraction
}

type StringExtractor interface {
	ExtractString(extractor *ExpressionExtractor, expression *StringExpression) ExpressionExtraction
}

type ArrayExtractor interface {
	ExtractArray(extractor *ExpressionExtractor, expression *ArrayExpression) ExpressionExtraction
}

type DictionaryExtractor interface {
	ExtractDictionary(extractor *ExpressionExtractor, expression *DictionaryExpression) ExpressionExtraction
}

type IdentifierExtractor interface {
	ExtractIdentifier(extractor *ExpressionExtractor, expression *IdentifierExpression) ExpressionExtraction
}

type InvocationExtractor interface {
	ExtractInvocation(extractor *ExpressionExtractor, expression *InvocationExpression) ExpressionExtraction
}

type MemberExtractor interface {
	ExtractMember(extractor *ExpressionExtractor, expression *MemberExpression) ExpressionExtraction
}

type IndexExtractor interface {
	ExtractIndex(extractor *ExpressionExtractor, expression *IndexExpression) ExpressionExtraction
}

type ConditionalExtractor interface {
	ExtractConditional(extractor *ExpressionExtractor, expression *ConditionalExpression) ExpressionExtraction
}

type UnaryExtractor interface {
	ExtractUnary(extractor *ExpressionExtractor, expression *UnaryExpression) ExpressionExtraction
}

type BinaryExtractor interface {
	ExtractBinary(extractor *ExpressionExtractor, expression *BinaryExpression) ExpressionExtraction
}

type FunctionExtractor interface {
	ExtractFunction(extractor *ExpressionExtractor, expression *FunctionExpression) ExpressionExtraction
}

type FailableDowncastExtractor interface {
	ExtractFailableDowncast(extractor *ExpressionExtractor, expression *FailableDowncastExpression) ExpressionExtraction
}

type CreateExtractor interface {
	ExtractCreate(extractor *ExpressionExtractor, expression *CreateExpression) ExpressionExtraction
}

type DestroyExtractor interface {
	ExtractDestroy(extractor *ExpressionExtractor, expression *DestroyExpression) ExpressionExtraction
}

type ReferenceExtractor interface {
	ExtractReference(extractor *ExpressionExtractor, expression *ReferenceExpression) ExpressionExtraction
}

type ExpressionExtractor struct {
	nextIdentifier            int
	BoolExtractor             BoolExtractor
	NilExtractor              NilExtractor
	IntExtractor              IntExtractor
	StringExtractor           StringExtractor
	ArrayExtractor            ArrayExtractor
	DictionaryExtractor       DictionaryExtractor
	IdentifierExtractor       IdentifierExtractor
	InvocationExtractor       InvocationExtractor
	MemberExtractor           MemberExtractor
	IndexExtractor            IndexExtractor
	ConditionalExtractor      ConditionalExtractor
	UnaryExtractor            UnaryExtractor
	BinaryExtractor           BinaryExtractor
	FunctionExtractor         FunctionExtractor
	FailableDowncastExtractor FailableDowncastExtractor
	CreateExtractor           CreateExtractor
	DestroyExtractor          DestroyExtractor
	ReferenceExtractor        ReferenceExtractor
}

func (extractor *ExpressionExtractor) Extract(expression Expression) ExpressionExtraction {
	return expression.AcceptExp(extractor).(ExpressionExtraction)
}

func (extractor *ExpressionExtractor) FreshIdentifier() string {
	defer func() {
		extractor.nextIdentifier += 1
	}()
	// TODO: improve
	// NOTE: to avoid naming clashes with identifiers in the program,
	// include characters that can't be represented in source:
	//   - \x00 = Null character
	//   - \x1F = Information Separator One
	return extractor.FormatIdentifier(extractor.nextIdentifier)
}

func (extractor *ExpressionExtractor) FormatIdentifier(identifier int) string {
	return fmt.Sprintf("\x00exp\x1F%d", identifier)
}

type ExtractedExpression struct {
	Identifier Identifier
	Expression Expression
}

type ExpressionExtraction struct {
	RewrittenExpression  Expression
	ExtractedExpressions []ExtractedExpression
}

func (extractor *ExpressionExtractor) VisitBoolExpression(expression *BoolExpression) Repr {

	// delegate to child extractor, if any,
	// or call default implementation

	if extractor.BoolExtractor != nil {
		return extractor.BoolExtractor.ExtractBool(extractor, expression)
	} else {
		return extractor.ExtractBool(expression)
	}
}

func (extractor *ExpressionExtractor) ExtractBool(expression *BoolExpression) ExpressionExtraction {

	// nothing to rewrite, return as-is

	return ExpressionExtraction{
		RewrittenExpression:  expression,
		ExtractedExpressions: nil,
	}
}

func (extractor *ExpressionExtractor) VisitNilExpression(expression *NilExpression) Repr {

	// delegate to child extractor, if any,
	// or call default implementation

	if extractor.NilExtractor != nil {
		return extractor.NilExtractor.ExtractNil(extractor, expression)
	} else {
		return extractor.ExtractNil(expression)
	}
}

func (extractor *ExpressionExtractor) ExtractNil(expression *NilExpression) ExpressionExtraction {

	// nothing to rewrite, return as-is

	return ExpressionExtraction{
		RewrittenExpression:  expression,
		ExtractedExpressions: nil,
	}
}

func (extractor *ExpressionExtractor) VisitIntExpression(expression *IntExpression) Repr {

	// delegate to child extractor, if any,
	// or call default implementation

	if extractor.IntExtractor != nil {
		return extractor.IntExtractor.ExtractInt(extractor, expression)
	} else {
		return extractor.ExtractInt(expression)
	}
}

func (extractor *ExpressionExtractor) ExtractInt(expression *IntExpression) ExpressionExtraction {

	// nothing to rewrite, return as-is

	return ExpressionExtraction{
		RewrittenExpression:  expression,
		ExtractedExpressions: nil,
	}
}

func (extractor *ExpressionExtractor) VisitStringExpression(expression *StringExpression) Repr {

	// delegate to child extractor, if any,
	// or call default implementation

	if extractor.StringExtractor != nil {
		return extractor.StringExtractor.ExtractString(extractor, expression)
	} else {
		return extractor.ExtractString(expression)
	}
}

func (extractor *ExpressionExtractor) ExtractString(expression *StringExpression) ExpressionExtraction {

	// nothing to rewrite, return as-is

	return ExpressionExtraction{
		RewrittenExpression:  expression,
		ExtractedExpressions: nil,
	}
}

func (extractor *ExpressionExtractor) VisitArrayExpression(expression *ArrayExpression) Repr {

	// delegate to child extractor, if any,
	// or call default implementation

	if extractor.ArrayExtractor != nil {
		return extractor.ArrayExtractor.ExtractArray(extractor, expression)
	} else {
		return extractor.ExtractArray(expression)
	}
}

func (extractor *ExpressionExtractor) ExtractArray(expression *ArrayExpression) ExpressionExtraction {

	// copy the expression
	newExpression := *expression

	// rewrite all value expressions

	rewrittenExpressions, extractedExpressions :=
		extractor.VisitExpressions(expression.Values)

	newExpression.Values = rewrittenExpressions

	return ExpressionExtraction{
		RewrittenExpression:  &newExpression,
		ExtractedExpressions: extractedExpressions,
	}
}

func (extractor *ExpressionExtractor) VisitExpressions(
	expressions []Expression,
) (
	[]Expression, []ExtractedExpression,
) {
	var rewrittenExpressions []Expression
	var extractedExpressions []ExtractedExpression

	for _, expression := range expressions {
		result := extractor.Extract(expression)

		rewrittenExpressions = append(
			rewrittenExpressions,
			result.RewrittenExpression,
		)

		extractedExpressions = append(
			extractedExpressions,
			result.ExtractedExpressions...,
		)
	}

	return rewrittenExpressions, extractedExpressions
}

func (extractor *ExpressionExtractor) VisitDictionaryExpression(expression *DictionaryExpression) Repr {

	// delegate to child extractor, if any,
	// or call default implementation

	if extractor.DictionaryExtractor != nil {
		return extractor.DictionaryExtractor.ExtractDictionary(extractor, expression)
	} else {
		return extractor.ExtractDictionary(expression)
	}
}

func (extractor *ExpressionExtractor) ExtractDictionary(expression *DictionaryExpression) ExpressionExtraction {

	var extractedExpressions []ExtractedExpression

	// copy the expression
	newExpression := *expression

	// rewrite all value expressions

	rewrittenEntries := make([]Entry, len(expression.Entries))

	for i, entry := range expression.Entries {
		keyResult := extractor.Extract(entry.Key)
		extractedExpressions = append(extractedExpressions, keyResult.ExtractedExpressions...)

		valueResult := extractor.Extract(entry.Value)
		extractedExpressions = append(extractedExpressions, valueResult.ExtractedExpressions...)

		rewrittenEntries[i] = Entry{
			Key:   keyResult.RewrittenExpression,
			Value: valueResult.RewrittenExpression,
		}
	}

	newExpression.Entries = rewrittenEntries

	return ExpressionExtraction{
		RewrittenExpression:  &newExpression,
		ExtractedExpressions: extractedExpressions,
	}
}

func (extractor *ExpressionExtractor) VisitIdentifierExpression(expression *IdentifierExpression) Repr {

	// delegate to child extractor, if any,
	// or call default implementation

	if extractor.IdentifierExtractor != nil {
		return extractor.IdentifierExtractor.ExtractIdentifier(extractor, expression)
	} else {
		return extractor.ExtractIdentifier(expression)
	}
}

func (extractor *ExpressionExtractor) ExtractIdentifier(expression *IdentifierExpression) ExpressionExtraction {

	// nothing to rewrite, return as-is

	return ExpressionExtraction{
		RewrittenExpression: expression,
	}
}

func (extractor *ExpressionExtractor) VisitInvocationExpression(expression *InvocationExpression) Repr {

	// delegate to child extractor, if any,
	// or call default implementation

	if extractor.InvocationExtractor != nil {
		return extractor.InvocationExtractor.ExtractInvocation(extractor, expression)
	} else {
		return extractor.ExtractInvocation(expression)
	}
}

func (extractor *ExpressionExtractor) ExtractInvocation(expression *InvocationExpression) ExpressionExtraction {
	var extractedExpressions []ExtractedExpression

	invokedExpression := expression.InvokedExpression

	// copy the expression
	newExpression := *expression

	// rewrite invoked expression

	invokedExpressionResult := extractor.Extract(invokedExpression)
	newExpression.InvokedExpression = invokedExpressionResult.RewrittenExpression
	extractedExpressions = append(
		extractedExpressions,
		invokedExpressionResult.ExtractedExpressions...,
	)

	// rewrite all arguments

	newArguments, argumentExtractedExpressions := extractor.extractArguments(expression.Arguments)
	extractedExpressions = append(
		extractedExpressions,
		argumentExtractedExpressions...,
	)

	newExpression.Arguments = newArguments

	return ExpressionExtraction{
		RewrittenExpression:  &newExpression,
		ExtractedExpressions: extractedExpressions,
	}
}

func (extractor *ExpressionExtractor) extractArguments(
	arguments []*Argument,
) (
	newArguments []*Argument,
	extractedExpressions []ExtractedExpression,
) {
	for _, argument := range arguments {

		// copy the argument
		newArgument := *argument

		argumentResult := extractor.Extract(argument.Expression)

		newArgument.Expression = argumentResult.RewrittenExpression

		extractedExpressions = append(
			extractedExpressions,
			argumentResult.ExtractedExpressions...,
		)

		newArguments = append(newArguments, &newArgument)
	}
	return newArguments, extractedExpressions
}

func (extractor *ExpressionExtractor) VisitMemberExpression(expression *MemberExpression) Repr {

	// delegate to child extractor, if any,
	// or call default implementation

	if extractor.MemberExtractor != nil {
		return extractor.MemberExtractor.ExtractMember(extractor, expression)
	} else {
		return extractor.ExtractMember(expression)
	}
}

func (extractor *ExpressionExtractor) ExtractMember(expression *MemberExpression) ExpressionExtraction {

	// copy the expression
	newExpression := *expression

	// rewrite the sub-expression

	result := extractor.Extract(newExpression.Expression)

	newExpression.Expression = result.RewrittenExpression

	return ExpressionExtraction{
		RewrittenExpression:  &newExpression,
		ExtractedExpressions: result.ExtractedExpressions,
	}
}

func (extractor *ExpressionExtractor) VisitIndexExpression(expression *IndexExpression) Repr {

	// delegate to child extractor, if any,
	// or call default implementation

	if extractor.IndexExtractor != nil {
		return extractor.IndexExtractor.ExtractIndex(extractor, expression)
	} else {
		return extractor.ExtractIndex(expression)
	}
}

func (extractor *ExpressionExtractor) ExtractIndex(expression *IndexExpression) ExpressionExtraction {

	// copy the expression
	newExpression := *expression

	// rewrite the sub-expression

	result := extractor.Extract(newExpression.TargetExpression)

	newExpression.TargetExpression = result.RewrittenExpression

	return ExpressionExtraction{
		RewrittenExpression:  &newExpression,
		ExtractedExpressions: result.ExtractedExpressions,
	}
}

func (extractor *ExpressionExtractor) VisitConditionalExpression(expression *ConditionalExpression) Repr {

	// delegate to child extractor, if any,
	// or call default implementation

	if extractor.ConditionalExtractor != nil {
		return extractor.ConditionalExtractor.ExtractConditional(extractor, expression)
	} else {
		return extractor.ExtractConditional(expression)
	}
}

func (extractor *ExpressionExtractor) ExtractConditional(expression *ConditionalExpression) ExpressionExtraction {

	// copy the expression
	newExpression := *expression

	// rewrite all sub-expressions

	rewrittenExpressions, extractedExpressions :=
		extractor.VisitExpressions([]Expression{
			newExpression.Test,
			newExpression.Then,
			newExpression.Else,
		})

	newExpression.Test = rewrittenExpressions[0]
	newExpression.Then = rewrittenExpressions[1]
	newExpression.Else = rewrittenExpressions[2]

	return ExpressionExtraction{
		RewrittenExpression:  &newExpression,
		ExtractedExpressions: extractedExpressions,
	}
}

func (extractor *ExpressionExtractor) VisitUnaryExpression(expression *UnaryExpression) Repr {

	// delegate to child extractor, if any,
	// or call default implementation

	if extractor.UnaryExtractor != nil {
		return extractor.UnaryExtractor.ExtractUnary(extractor, expression)
	} else {
		return extractor.ExtractUnary(expression)

	}
}

func (extractor *ExpressionExtractor) ExtractUnary(expression *UnaryExpression) ExpressionExtraction {

	// copy the expression
	newExpression := *expression

	// rewrite the sub-expression

	result := extractor.Extract(newExpression.Expression)

	newExpression.Expression = result.RewrittenExpression

	return ExpressionExtraction{
		RewrittenExpression:  &newExpression,
		ExtractedExpressions: result.ExtractedExpressions,
	}
}

func (extractor *ExpressionExtractor) VisitBinaryExpression(expression *BinaryExpression) Repr {

	// delegate to child extractor, if any,
	// or call default implementation

	if extractor.BinaryExtractor != nil {
		return extractor.BinaryExtractor.ExtractBinary(extractor, expression)
	} else {
		return extractor.ExtractBinary(expression)
	}
}

func (extractor *ExpressionExtractor) ExtractBinary(expression *BinaryExpression) ExpressionExtraction {

	// copy the expression
	newExpression := *expression

	// rewrite left and right sub-expression

	rewrittenExpressions, extractedExpressions :=
		extractor.VisitExpressions([]Expression{
			newExpression.Left,
			newExpression.Right,
		})

	newExpression.Left = rewrittenExpressions[0]
	newExpression.Right = rewrittenExpressions[1]

	return ExpressionExtraction{
		RewrittenExpression:  &newExpression,
		ExtractedExpressions: extractedExpressions,
	}
}

func (extractor *ExpressionExtractor) VisitFunctionExpression(expression *FunctionExpression) Repr {

	// delegate to child extractor, if any,
	// or call default implementation

	if extractor.FunctionExtractor != nil {
		return extractor.FunctionExtractor.ExtractFunction(extractor, expression)
	} else {
		return extractor.ExtractFunction(expression)
	}
}

func (extractor *ExpressionExtractor) ExtractFunction(expression *FunctionExpression) ExpressionExtraction {
	// NOTE: not supported
	panic(&errors.UnreachableError{})
}

func (extractor *ExpressionExtractor) VisitFailableDowncastExpression(expression *FailableDowncastExpression) Repr {

	// delegate to child extractor, if any,
	// or call default implementation

	if extractor.FailableDowncastExtractor != nil {
		return extractor.FailableDowncastExtractor.ExtractFailableDowncast(extractor, expression)
	} else {
		return extractor.ExtractFailableDowncast(expression)
	}
}

func (extractor *ExpressionExtractor) ExtractFailableDowncast(expression *FailableDowncastExpression) ExpressionExtraction {

	// copy the expression
	newExpression := *expression

	// rewrite the sub-expression

	result := extractor.Extract(newExpression.Expression)

	newExpression.Expression = result.RewrittenExpression

	return ExpressionExtraction{
		RewrittenExpression:  &newExpression,
		ExtractedExpressions: result.ExtractedExpressions,
	}
}

func (extractor *ExpressionExtractor) VisitCreateExpression(expression *CreateExpression) Repr {
	// delegate to child extractor, if any,
	// or call default implementation

	if extractor.CreateExtractor != nil {
		return extractor.CreateExtractor.ExtractCreate(extractor, expression)
	} else {
		return extractor.ExtractCreate(expression)
	}
}

func (extractor *ExpressionExtractor) ExtractCreate(expression *CreateExpression) ExpressionExtraction {

	// copy the expression
	newExpression := *expression

	// rewrite the sub-expression

	result := extractor.Extract(newExpression.InvocationExpression)

	newExpression.InvocationExpression = result.RewrittenExpression.(*InvocationExpression)

	return ExpressionExtraction{
		RewrittenExpression:  &newExpression,
		ExtractedExpressions: result.ExtractedExpressions,
	}
}

func (extractor *ExpressionExtractor) VisitDestroyExpression(expression *DestroyExpression) Repr {
	// delegate to child extractor, if any,
	// or call default implementation

	if extractor.DestroyExtractor != nil {
		return extractor.DestroyExtractor.ExtractDestroy(extractor, expression)
	} else {
		return extractor.ExtractDestroy(expression)
	}
}

func (extractor *ExpressionExtractor) ExtractDestroy(expression *DestroyExpression) ExpressionExtraction {

	// copy the expression
	newExpression := *expression

	// rewrite the sub-expression

	result := extractor.Extract(newExpression.Expression)

	newExpression.Expression = result.RewrittenExpression

	return ExpressionExtraction{
		RewrittenExpression:  &newExpression,
		ExtractedExpressions: result.ExtractedExpressions,
	}
}

func (extractor *ExpressionExtractor) VisitReferenceExpression(expression *ReferenceExpression) Repr {
	// delegate to child extractor, if any,
	// or call default implementation

	if extractor.ReferenceExtractor != nil {
		return extractor.ReferenceExtractor.ExtractReference(extractor, expression)
	} else {
		return extractor.ExtractReference(expression)
	}
}

func (extractor *ExpressionExtractor) ExtractReference(expression *ReferenceExpression) ExpressionExtraction {

	// copy the expression
	newExpression := *expression

	// rewrite the sub-expression

	result := extractor.Extract(newExpression.Expression)

	newExpression.Expression = result.RewrittenExpression

	return ExpressionExtraction{
		RewrittenExpression:  &newExpression,
		ExtractedExpressions: result.ExtractedExpressions,
	}
}
