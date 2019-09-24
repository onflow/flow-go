package sema

import (
	"fmt"

	"github.com/raviqqe/hamt"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
)

type AssignmentAnalyzer struct {
	assignments hamt.Set
	errors      *[]error
}

func intersection(a, b hamt.Set) hamt.Set {
	c := hamt.NewSet()

	for a.Size() != 0 {
		var e hamt.Entry
		e, a = a.FirstRest()
		if b.Include(e) {
			c = c.Insert(e)
		}
	}

	return c
}

func union(a, b hamt.Set) hamt.Set {
	return a.Merge(b)
}

func CheckFieldAssignments(fields []*ast.FieldDeclaration, block *ast.FunctionBlock) []error {
	assignments := hamt.NewSet()

	errors := make([]error, 0)

	a := &AssignmentAnalyzer{assignments, &errors}

	assigned := block.Accept(a).(hamt.Set)

	fmt.Println("FINAL ASSIGNMENTS:", assigned.Size())

	for _, field := range fields {
		if !assigned.Include(field.Identifier) {
			errors = append(errors, &UnassignedFieldError{
				Identifier: field.Identifier,
				StartPos:   field.StartPosition(),
				EndPos:     field.EndPosition(),
			})
		}
	}

	return errors
}

func (detector *AssignmentAnalyzer) branch(assignments hamt.Set) *AssignmentAnalyzer {
	return &AssignmentAnalyzer{assignments, detector.errors}
}

func (detector *AssignmentAnalyzer) visitStatements(statements []ast.Statement) hamt.Set {
	assignments := detector.assignments

	for _, statement := range statements {
		newDetector := detector.branch(assignments)
		newAssignments := newDetector.visitStatement(statement)
		assignments = assignments.Merge(newAssignments)
	}

	return assignments
}

func (detector *AssignmentAnalyzer) visitStatement(statement ast.Statement) hamt.Set {
	return statement.Accept(detector).(hamt.Set)
}

func (detector *AssignmentAnalyzer) visitNode(node ast.Element) hamt.Set {
	if node == nil {
		return hamt.NewSet()
	}

	return node.Accept(detector).(hamt.Set)
}

func (detector *AssignmentAnalyzer) VisitReturnStatement(*ast.ReturnStatement) ast.Repr {
	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitBlock(node *ast.Block) ast.Repr {
	if node == nil {
		return hamt.NewSet()
	}

	return detector.visitStatements(node.Statements)
}

func (detector *AssignmentAnalyzer) VisitFunctionBlock(node *ast.FunctionBlock) ast.Repr {
	return detector.visitNode(node.Block)
}

func (detector *AssignmentAnalyzer) VisitBreakStatement(*ast.BreakStatement) ast.Repr {
	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitIfStatement(node *ast.IfStatement) ast.Repr {
	test := node.Test.(ast.Element)
	detector.visitNode(test)

	thenAssignments := detector.visitNode(node.Then)
	elseAssignments := detector.visitNode(node.Else)

	return intersection(thenAssignments, elseAssignments)
}

func (detector *AssignmentAnalyzer) VisitWhileStatement(node *ast.WhileStatement) ast.Repr {
	detector.visitNode(node.Test)
	detector.visitNode(node.Block)

	// TODO: optimize
	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitVariableDeclaration(node *ast.VariableDeclaration) ast.Repr {
	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitAssignment(node *ast.AssignmentStatement) ast.Repr {
	assignments := detector.assignments

	fmt.Println("assignments:", assignments.Size())
	node.Value.Accept(detector)

	if memberExpression, ok := node.Target.(*ast.MemberExpression); ok {
		if identifier, ok := memberExpression.Expression.(*ast.IdentifierExpression); ok {
			if identifier.Identifier.Identifier == SelfIdentifier {
				assignments = assignments.Insert(memberExpression.Identifier)
			}
		}
	}

	return assignments
}

func (detector *AssignmentAnalyzer) VisitExpressionStatement(node *ast.ExpressionStatement) ast.Repr {
	detector.visitNode(node.Expression)
	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitInvocationExpression(node *ast.InvocationExpression) ast.Repr {
	for _, arg := range node.Arguments {
		arg.Expression.Accept(detector)
	}

	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitConditionalExpression(node *ast.ConditionalExpression) ast.Repr {
	detector.visitNode(node.Test)
	detector.visitNode(node.Then)
	detector.visitNode(node.Else)

	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitProgram(node *ast.Program) ast.Repr {
	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitFunctionDeclaration(*ast.FunctionDeclaration) ast.Repr {
	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitCompositeDeclaration(*ast.CompositeDeclaration) ast.Repr {
	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitInterfaceDeclaration(*ast.InterfaceDeclaration) ast.Repr {
	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitFieldDeclaration(*ast.FieldDeclaration) ast.Repr {
	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitInitializerDeclaration(node *ast.InitializerDeclaration) ast.Repr {
	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitCondition(node *ast.Condition) ast.Repr {
	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitImportDeclaration(*ast.ImportDeclaration) ast.Repr {
	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitContinueStatement(*ast.ContinueStatement) ast.Repr {
	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitBoolExpression(*ast.BoolExpression) ast.Repr {
	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitNilExpression(*ast.NilExpression) ast.Repr {
	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitIntExpression(*ast.IntExpression) ast.Repr {
	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitArrayExpression(node *ast.ArrayExpression) ast.Repr {
	for _, value := range node.Values {
		detector.visitNode(value)
	}

	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitDictionaryExpression(node *ast.DictionaryExpression) ast.Repr {
	for _, entry := range node.Entries {
		detector.visitNode(entry.Key)
		detector.visitNode(entry.Value)
	}

	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitIdentifierExpression(*ast.IdentifierExpression) ast.Repr {
	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitMemberExpression(node *ast.MemberExpression) ast.Repr {
	if identifier, ok := node.Expression.(*ast.IdentifierExpression); ok {
		if identifier.Identifier.Identifier == SelfIdentifier {
			if !detector.assignments.Include(node.Identifier) {
				*detector.errors = append(*detector.errors, &UnassignedFieldError{
					Identifier: node.Identifier,
					StartPos:   node.StartPosition(),
					EndPos:     node.EndPosition(),
				})
			}
		}
	}

	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitIndexExpression(node *ast.IndexExpression) ast.Repr {
	detector.visitNode(node.Expression)
	detector.visitNode(node.Index)

	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitUnaryExpression(node *ast.UnaryExpression) ast.Repr {
	detector.visitNode(node.Expression)
	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitBinaryExpression(node *ast.BinaryExpression) ast.Repr {
	detector.visitNode(node.Left)
	detector.visitNode(node.Right)

	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitFunctionExpression(node *ast.FunctionExpression) ast.Repr {
	// TODO: how to handle this?
	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitStringExpression(*ast.StringExpression) ast.Repr {
	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitFailableDowncastExpression(node *ast.FailableDowncastExpression) ast.Repr {
	detector.visitNode(node.Expression)
	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitCreateExpression(node *ast.CreateExpression) ast.Repr {
	return hamt.NewSet()
}

func (detector *AssignmentAnalyzer) VisitDestroyExpression(expression *ast.DestroyExpression) ast.Repr {
	return hamt.NewSet()
}
