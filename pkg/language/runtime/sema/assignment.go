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
	return AssignmentSet{a.set.Insert(hashableIdentifier(identifier))}
}

func (a AssignmentSet) Contains(identifier ast.Identifier) bool {
	return a.set.Include(hashableIdentifier(identifier))
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

type hashableIdentifier ast.Identifier

func (f hashableIdentifier) Hash() uint32 {
	h := fnv.New32a()
	h.Write([]byte(f.Identifier))
	return h.Sum32()
}

func (f hashableIdentifier) Equal(other hamt.Entry) bool {
	return f.Identifier == other.(hashableIdentifier).Identifier
}

func CheckFieldAssignments(fields []*ast.FieldDeclaration, block *ast.FunctionBlock) ([]*ast.FieldDeclaration, []error) {
	assignments := NewAssignmentSet()
	errors := make([]error, 0)

	a := &AssignmentAnalyzer{assignments, &errors}

	assigned := block.Accept(a).(AssignmentSet)

	unassigned := make([]*ast.FieldDeclaration, 0)

	for _, field := range fields {
		if !assigned.Contains(field.Identifier) {
			unassigned = append(unassigned, field)
		}
	}

	return unassigned, errors
}

type AssignmentAnalyzer struct {
	assignments AssignmentSet
	errors      *[]error
}

func (analyzer *AssignmentAnalyzer) branch(assignments AssignmentSet) *AssignmentAnalyzer {
	return &AssignmentAnalyzer{assignments, analyzer.errors}
}

func (analyzer *AssignmentAnalyzer) isSelfExpression(expr ast.Expression) bool {
	if identifier, ok := expr.(*ast.IdentifierExpression); ok {
		return identifier.Identifier.Identifier == SelfIdentifier
	}

	return false
}

func (analyzer *AssignmentAnalyzer) visitStatements(statements []ast.Statement) AssignmentSet {
	assignments := analyzer.assignments

	for _, statement := range statements {
		newAnalyzer := analyzer.branch(assignments)
		newAssignments := newAnalyzer.visitStatement(statement)
		assignments = assignments.Union(newAssignments)
	}

	return assignments
}

func (analyzer *AssignmentAnalyzer) visitStatement(statement ast.Statement) AssignmentSet {
	if statement == nil {
		return analyzer.assignments
	}

	return statement.Accept(analyzer).(AssignmentSet)
}

func (analyzer *AssignmentAnalyzer) visitNode(node ast.Element) AssignmentSet {
	if node == nil {
		return analyzer.assignments
	}

	return node.Accept(analyzer).(AssignmentSet)
}

func (analyzer *AssignmentAnalyzer) VisitBlock(node *ast.Block) ast.Repr {
	if node == nil {
		return analyzer.assignments
	}

	return analyzer.visitStatements(node.Statements)
}

func (analyzer *AssignmentAnalyzer) VisitFunctionBlock(node *ast.FunctionBlock) ast.Repr {
	return analyzer.visitNode(node.Block)
}

func (analyzer *AssignmentAnalyzer) VisitIfStatement(node *ast.IfStatement) ast.Repr {
	test := node.Test.(ast.Element)
	analyzer.visitNode(test)

	thenAssignments := analyzer.visitNode(node.Then)
	elseAssignments := analyzer.visitNode(node.Else)

	assignments := thenAssignments.Intersection(elseAssignments)

	return analyzer.assignments.Union(assignments)
}

func (analyzer *AssignmentAnalyzer) VisitWhileStatement(node *ast.WhileStatement) ast.Repr {
	analyzer.visitNode(node.Test)
	analyzer.visitNode(node.Block)

	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitVariableDeclaration(*ast.VariableDeclaration) ast.Repr {
	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitReturnStatement(*ast.ReturnStatement) ast.Repr {
	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitBreakStatement(*ast.BreakStatement) ast.Repr {
	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitAssignment(node *ast.AssignmentStatement) ast.Repr {
	node.Value.Accept(analyzer)

	if memberExpression, ok := node.Target.(*ast.MemberExpression); ok {
		if !analyzer.isSelfExpression(node.Target) {
			return analyzer.assignments.Insert(memberExpression.Identifier)
		}
	}

	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitExpressionStatement(node *ast.ExpressionStatement) ast.Repr {
	analyzer.visitNode(node.Expression)
	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitInvocationExpression(node *ast.InvocationExpression) ast.Repr {
	for _, arg := range node.Arguments {
		arg.Expression.Accept(analyzer)
	}

	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitConditionalExpression(node *ast.ConditionalExpression) ast.Repr {
	analyzer.visitNode(node.Test)
	analyzer.visitNode(node.Then)
	analyzer.visitNode(node.Else)

	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitProgram(node *ast.Program) ast.Repr {
	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitFunctionDeclaration(*ast.FunctionDeclaration) ast.Repr {
	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitCompositeDeclaration(*ast.CompositeDeclaration) ast.Repr {
	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitInterfaceDeclaration(*ast.InterfaceDeclaration) ast.Repr {
	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitFieldDeclaration(*ast.FieldDeclaration) ast.Repr {
	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitInitializerDeclaration(node *ast.InitializerDeclaration) ast.Repr {
	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitCondition(node *ast.Condition) ast.Repr {
	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitImportDeclaration(*ast.ImportDeclaration) ast.Repr {
	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitContinueStatement(*ast.ContinueStatement) ast.Repr {
	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitBoolExpression(*ast.BoolExpression) ast.Repr {
	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitNilExpression(*ast.NilExpression) ast.Repr {
	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitIntExpression(*ast.IntExpression) ast.Repr {
	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitArrayExpression(node *ast.ArrayExpression) ast.Repr {
	for _, value := range node.Values {
		analyzer.visitNode(value)
	}

	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitDictionaryExpression(node *ast.DictionaryExpression) ast.Repr {
	for _, entry := range node.Entries {
		analyzer.visitNode(entry.Key)
		analyzer.visitNode(entry.Value)
	}

	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitIdentifierExpression(*ast.IdentifierExpression) ast.Repr {
	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitMemberExpression(node *ast.MemberExpression) ast.Repr {
	if !analyzer.isSelfExpression(node.Expression) {
		return analyzer.assignments
	}

	if !analyzer.assignments.Contains(node.Identifier) {
		*analyzer.errors = append(*analyzer.errors, &UnassignedFieldError{
			Identifier: node.Identifier,
			StartPos:   node.StartPosition(),
			EndPos:     node.EndPosition(),
		})
	}

	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitIndexExpression(node *ast.IndexExpression) ast.Repr {
	analyzer.visitNode(node.Expression)
	analyzer.visitNode(node.Index)

	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitUnaryExpression(node *ast.UnaryExpression) ast.Repr {
	analyzer.visitNode(node.Expression)
	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitBinaryExpression(node *ast.BinaryExpression) ast.Repr {
	analyzer.visitNode(node.Left)
	analyzer.visitNode(node.Right)

	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitFunctionExpression(node *ast.FunctionExpression) ast.Repr {
	// TODO: how to handle this?
	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitStringExpression(*ast.StringExpression) ast.Repr {
	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitFailableDowncastExpression(node *ast.FailableDowncastExpression) ast.Repr {
	analyzer.visitNode(node.Expression)
	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitCreateExpression(node *ast.CreateExpression) ast.Repr {
	return analyzer.assignments
}

func (analyzer *AssignmentAnalyzer) VisitDestroyExpression(expression *ast.DestroyExpression) ast.Repr {
	return analyzer.assignments
}
