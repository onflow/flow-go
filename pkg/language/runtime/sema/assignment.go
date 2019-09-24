package sema

import (
	"hash/fnv"

	"github.com/raviqqe/hamt"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
)

type AssignmentSet struct {
	set hamt.Set
}

func NewAssignmentSet() AssignmentSet {
	return AssignmentSet{hamt.NewSet()}
}

func (a AssignmentSet) Insert(identifier ast.Identifier) AssignmentSet {
	return AssignmentSet{a.set.Insert(Field(identifier))}
}

func (a AssignmentSet) Contains(identifier ast.Identifier) bool {
	return a.set.Include(Field(identifier))
}

func (a AssignmentSet) Size() int {
	return a.set.Size()
}

func (a AssignmentSet) Intersection(b AssignmentSet) AssignmentSet {
	c := hamt.NewSet()

	set := a.set

	for set.Size() != 0 {
		var e hamt.Entry
		e, set = set.FirstRest()

		if b.set.Include(e) {
			c = c.Insert(e)
		}
	}

	return AssignmentSet{c}
}

func (a AssignmentSet) Union(b AssignmentSet) AssignmentSet {
	return AssignmentSet{a.set.Merge(b.set)}
}

type Field ast.Identifier

func (f Field) Hash() uint32 {
	h := fnv.New32a()
	h.Write([]byte(f.Identifier))
	return h.Sum32()
}

func (f Field) Equal(other hamt.Entry) bool {
	return f.Identifier == other.(Field).Identifier
}

func CheckFieldAssignments(fields []*ast.FieldDeclaration, block *ast.FunctionBlock) []error {
	assignments := NewAssignmentSet()
	errors := make([]error, 0)

	a := &AssignmentAnalyzer{assignments, &errors}

	assigned := block.Accept(a).(AssignmentSet)

	for _, field := range fields {
		if !assigned.Contains(field.Identifier) {
			errors = append(errors, &UnassignedFieldError{
				Identifier: field.Identifier,
				StartPos:   field.StartPosition(),
				EndPos:     field.EndPosition(),
			})
		}
	}

	return errors
}

type AssignmentAnalyzer struct {
	assignments AssignmentSet
	errors      *[]error
}

func (detector *AssignmentAnalyzer) branch(assignments AssignmentSet) *AssignmentAnalyzer {
	return &AssignmentAnalyzer{assignments, detector.errors}
}

func (detector *AssignmentAnalyzer) isSelfValue(expr ast.Expression) bool {
	if identifier, ok := expr.(*ast.IdentifierExpression); ok {
		return identifier.Identifier.Identifier == SelfIdentifier
	}

	return false
}

func (detector *AssignmentAnalyzer) visitStatements(statements []ast.Statement) AssignmentSet {
	assignments := detector.assignments

	for _, statement := range statements {
		newDetector := detector.branch(assignments)
		newAssignments := newDetector.visitStatement(statement)
		assignments = assignments.Union(newAssignments)
	}

	return assignments
}

func (detector *AssignmentAnalyzer) visitStatement(statement ast.Statement) AssignmentSet {
	return statement.Accept(detector).(AssignmentSet)
}

func (detector *AssignmentAnalyzer) visitNode(node ast.Element) AssignmentSet {
	if node == nil {
		return NewAssignmentSet()
	}

	return node.Accept(detector).(AssignmentSet)
}

func (detector *AssignmentAnalyzer) VisitBlock(node *ast.Block) ast.Repr {
	if node == nil {
		return NewAssignmentSet()
	}

	return detector.visitStatements(node.Statements)
}

func (detector *AssignmentAnalyzer) VisitFunctionBlock(node *ast.FunctionBlock) ast.Repr {
	return detector.visitNode(node.Block)
}

func (detector *AssignmentAnalyzer) VisitIfStatement(node *ast.IfStatement) ast.Repr {
	test := node.Test.(ast.Element)
	detector.visitNode(test)

	thenAssignments := detector.visitNode(node.Then)
	elseAssignments := detector.visitNode(node.Else)

	return thenAssignments.Intersection(elseAssignments)
}

func (detector *AssignmentAnalyzer) VisitWhileStatement(node *ast.WhileStatement) ast.Repr {
	detector.visitNode(node.Test)
	detector.visitNode(node.Block)

	// TODO: optimize
	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitVariableDeclaration(*ast.VariableDeclaration) ast.Repr {
	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitReturnStatement(*ast.ReturnStatement) ast.Repr {
	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitBreakStatement(*ast.BreakStatement) ast.Repr {
	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitAssignment(node *ast.AssignmentStatement) ast.Repr {
	assignments := detector.assignments

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
	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitInvocationExpression(node *ast.InvocationExpression) ast.Repr {
	for _, arg := range node.Arguments {
		arg.Expression.Accept(detector)
	}

	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitConditionalExpression(node *ast.ConditionalExpression) ast.Repr {
	detector.visitNode(node.Test)
	detector.visitNode(node.Then)
	detector.visitNode(node.Else)

	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitProgram(node *ast.Program) ast.Repr {
	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitFunctionDeclaration(*ast.FunctionDeclaration) ast.Repr {
	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitCompositeDeclaration(*ast.CompositeDeclaration) ast.Repr {
	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitInterfaceDeclaration(*ast.InterfaceDeclaration) ast.Repr {
	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitFieldDeclaration(*ast.FieldDeclaration) ast.Repr {
	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitInitializerDeclaration(node *ast.InitializerDeclaration) ast.Repr {
	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitCondition(node *ast.Condition) ast.Repr {
	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitImportDeclaration(*ast.ImportDeclaration) ast.Repr {
	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitContinueStatement(*ast.ContinueStatement) ast.Repr {
	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitBoolExpression(*ast.BoolExpression) ast.Repr {
	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitNilExpression(*ast.NilExpression) ast.Repr {
	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitIntExpression(*ast.IntExpression) ast.Repr {
	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitArrayExpression(node *ast.ArrayExpression) ast.Repr {
	for _, value := range node.Values {
		detector.visitNode(value)
	}

	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitDictionaryExpression(node *ast.DictionaryExpression) ast.Repr {
	for _, entry := range node.Entries {
		detector.visitNode(entry.Key)
		detector.visitNode(entry.Value)
	}

	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitIdentifierExpression(*ast.IdentifierExpression) ast.Repr {
	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitMemberExpression(node *ast.MemberExpression) ast.Repr {
	if !detector.isSelfValue(node.Expression) {
		return NewAssignmentSet()
	}

	if !detector.assignments.Contains(node.Identifier) {
		*detector.errors = append(*detector.errors, &UnassignedFieldError{
			Identifier: node.Identifier,
			StartPos:   node.StartPosition(),
			EndPos:     node.EndPosition(),
		})
	}

	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitIndexExpression(node *ast.IndexExpression) ast.Repr {
	detector.visitNode(node.Expression)
	detector.visitNode(node.Index)

	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitUnaryExpression(node *ast.UnaryExpression) ast.Repr {
	detector.visitNode(node.Expression)
	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitBinaryExpression(node *ast.BinaryExpression) ast.Repr {
	detector.visitNode(node.Left)
	detector.visitNode(node.Right)

	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitFunctionExpression(node *ast.FunctionExpression) ast.Repr {
	// TODO: how to handle this?
	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitStringExpression(*ast.StringExpression) ast.Repr {
	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitFailableDowncastExpression(node *ast.FailableDowncastExpression) ast.Repr {
	detector.visitNode(node.Expression)
	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitCreateExpression(node *ast.CreateExpression) ast.Repr {
	return NewAssignmentSet()
}

func (detector *AssignmentAnalyzer) VisitDestroyExpression(expression *ast.DestroyExpression) ast.Repr {
	return NewAssignmentSet()
}
