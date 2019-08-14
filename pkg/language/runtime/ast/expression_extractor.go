package ast

import (
	"fmt"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/errors"
)

type BoolExtractor interface {
	ExtractBool(extractor *ExpressionExtractor, expression *BoolExpression) ExpressionExtraction
}

type IntExtractor interface {
	ExtractInt(extractor *ExpressionExtractor, expression *IntExpression) ExpressionExtraction
}

type ArrayExtractor interface {
	ExtractArray(extractor *ExpressionExtractor, expression *ArrayExpression) ExpressionExtraction
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

type ExpressionExtractor struct {
	nextIdentifier       int
	BoolExtractor        BoolExtractor
	IntExtractor         IntExtractor
	ArrayExtractor       ArrayExtractor
	IdentifierExtractor  IdentifierExtractor
	InvocationExtractor  InvocationExtractor
	MemberExtractor      MemberExtractor
	IndexExtractor       IndexExtractor
	ConditionalExtractor ConditionalExtractor
	UnaryExtractor       UnaryExtractor
	BinaryExtractor      BinaryExtractor
	FunctionExtractor    FunctionExtractor
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
	Identifier string
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

	var newArguments []*Argument
	for _, argument := range expression.Arguments {

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

	newExpression.Arguments = newArguments

	return ExpressionExtraction{
		RewrittenExpression:  &newExpression,
		ExtractedExpressions: extractedExpressions,
	}
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

	result := extractor.Extract(newExpression.Expression)

	newExpression.Expression = result.RewrittenExpression

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
