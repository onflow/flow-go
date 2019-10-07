package self_field_analyzer

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
)

// CheckSelfFieldInitializations performs a definite assignment analysis on the provided function block.
//
// This function checks that the provided fields are definitely assigned in the function block. It returns
// a list of all fields that are not definitely assigned, as well as a list of errors that occurred
// due to unassigned usages in the function block itself.
func CheckSelfFieldInitializations(
	fields []*ast.FieldDeclaration,
	block *ast.FunctionBlock,
) ([]*ast.FieldDeclaration, []error) {
	assignments := NewAssignmentSet()
	errors := make([]error, 0)

	a := &SelfFieldAssignmentAnalyzer{
		assignments: assignments,
		errors:      &errors,
	}
	assigned := a.visitNode(block)

	unassigned := make([]*ast.FieldDeclaration, 0)

	for _, field := range fields {
		if !assigned.Contains(field.Identifier) {
			unassigned = append(unassigned, field)
		}
	}

	return unassigned, errors
}

// SelfFieldAssignmentAnalyzer is a visitor that traverses an AST to perform definite assignment analysis.
//
// This analyzer accumulates field assignments as it traverses, meaning that each node is aware of all
// definitely-assigned fields in its logical path.
type SelfFieldAssignmentAnalyzer struct {
	assignments AssignmentSet
	errors      *[]error
}

// branch spawns a new analyzer to inspect a branch of the AST.
func (analyzer *SelfFieldAssignmentAnalyzer) branch() *SelfFieldAssignmentAnalyzer {
	return &SelfFieldAssignmentAnalyzer{analyzer.assignments, analyzer.errors}
}

// isSelfExpression returns true if the given expression is the `self` identifier referring
// to the composite declaration being analyzed.
func (analyzer *SelfFieldAssignmentAnalyzer) isSelfExpression(expr ast.Expression) bool {
	// TODO: perform more sophisticated check (i.e. `var self = 1` is not valid)

	if identifier, ok := expr.(*ast.IdentifierExpression); ok {
		return identifier.Identifier.Identifier == "self"
	}

	return false
}

// report reports an error that has occurred during the analysis.
func (analyzer *SelfFieldAssignmentAnalyzer) report(err error) {
	*analyzer.errors = append(*analyzer.errors, err)
}

func (analyzer *SelfFieldAssignmentAnalyzer) visitStatements(statements []ast.Statement) AssignmentSet {
	for _, statement := range statements {
		analyzer.assignments = analyzer.visitNode(statement)
	}

	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) visitNode(node ast.Element) AssignmentSet {
	if node == nil {
		return analyzer.assignments
	}

	return node.Accept(analyzer).(AssignmentSet)
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitBlock(node *ast.Block) ast.Repr {
	if node == nil {
		return analyzer.assignments
	}

	return analyzer.visitStatements(node.Statements)
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitFunctionBlock(node *ast.FunctionBlock) ast.Repr {
	return analyzer.visitNode(node.Block)
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitIfStatement(node *ast.IfStatement) ast.Repr {
	test := node.Test.(ast.Element)
	analyzer.visitNode(test)

	thenAssignments := analyzer.branch().visitNode(node.Then)
	elseAssignments := analyzer.branch().visitNode(node.Else)

	return thenAssignments.Intersection(elseAssignments)
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitWhileStatement(node *ast.WhileStatement) ast.Repr {
	analyzer.branch().visitNode(node.Test)
	analyzer.branch().visitNode(node.Block)

	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitVariableDeclaration(node *ast.VariableDeclaration) ast.Repr {
	analyzer.visitNode(node.Value)

	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitReturnStatement(*ast.ReturnStatement) ast.Repr {
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitBreakStatement(*ast.BreakStatement) ast.Repr {
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitAssignment(node *ast.AssignmentStatement) ast.Repr {
	node.Value.Accept(analyzer)

	if memberExpression, ok := node.Target.(*ast.MemberExpression); ok {
		if analyzer.isSelfExpression(memberExpression.Expression) {
			return analyzer.assignments.Insert(memberExpression.Identifier)
		}
	}

	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitExpressionStatement(node *ast.ExpressionStatement) ast.Repr {
	analyzer.visitNode(node.Expression)
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitInvocationExpression(node *ast.InvocationExpression) ast.Repr {
	for _, arg := range node.Arguments {
		analyzer.visitNode(arg.Expression)
	}

	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitConditionalExpression(node *ast.ConditionalExpression) ast.Repr {
	analyzer.visitNode(node.Test)

	thenAssignments := analyzer.branch().visitNode(node.Then)
	elseAssignments := analyzer.branch().visitNode(node.Else)

	return thenAssignments.Intersection(elseAssignments)
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitProgram(*ast.Program) ast.Repr {
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitFunctionDeclaration(*ast.FunctionDeclaration) ast.Repr {
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitCompositeDeclaration(*ast.CompositeDeclaration) ast.Repr {
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitInterfaceDeclaration(*ast.InterfaceDeclaration) ast.Repr {
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitFieldDeclaration(*ast.FieldDeclaration) ast.Repr {
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitInitializerDeclaration(*ast.InitializerDeclaration) ast.Repr {
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitCondition(*ast.Condition) ast.Repr {
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitImportDeclaration(*ast.ImportDeclaration) ast.Repr {
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitContinueStatement(*ast.ContinueStatement) ast.Repr {
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitBoolExpression(*ast.BoolExpression) ast.Repr {
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitNilExpression(*ast.NilExpression) ast.Repr {
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitIntExpression(*ast.IntExpression) ast.Repr {
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitArrayExpression(node *ast.ArrayExpression) ast.Repr {
	for _, value := range node.Values {
		analyzer.visitNode(value)
	}

	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitDictionaryExpression(node *ast.DictionaryExpression) ast.Repr {
	for _, entry := range node.Entries {
		analyzer.visitNode(entry.Key)
		analyzer.visitNode(entry.Value)
	}

	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitIdentifierExpression(*ast.IdentifierExpression) ast.Repr {
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitMemberExpression(node *ast.MemberExpression) ast.Repr {
	if !analyzer.isSelfExpression(node.Expression) {
		return analyzer.assignments
	}

	if !analyzer.assignments.Contains(node.Identifier) {
		analyzer.report(&UninitializedFieldAccessError{
			Identifier: node.Identifier,
			Pos:        node.Identifier.Pos,
		})
	}

	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitIndexExpression(node *ast.IndexExpression) ast.Repr {
	analyzer.visitNode(node.TargetExpression)
	if node.IndexingExpression != nil {
		analyzer.visitNode(node.IndexingExpression)
	}

	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitUnaryExpression(node *ast.UnaryExpression) ast.Repr {
	analyzer.visitNode(node.Expression)
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitBinaryExpression(node *ast.BinaryExpression) ast.Repr {
	analyzer.visitNode(node.Left)
	analyzer.visitNode(node.Right)

	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitFunctionExpression(*ast.FunctionExpression) ast.Repr {
	// TODO: decide how to handle `self` captured inside a function expression
	// https://github.com/dapperlabs/flow-go/issues/726
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitStringExpression(*ast.StringExpression) ast.Repr {
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitFailableDowncastExpression(node *ast.FailableDowncastExpression) ast.Repr {
	analyzer.visitNode(node.Expression)
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitCreateExpression(*ast.CreateExpression) ast.Repr {
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitDestroyExpression(*ast.DestroyExpression) ast.Repr {
	return analyzer.assignments
}
