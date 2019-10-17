package self_field_analyzer

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
)

// CheckSelfFieldInitializations performs a definite assignment analysis
// on the provided function block.
//
// This function checks that the provided fields are definitely assigned
// in the function block.
//
// It returns a list of all fields that are not definitely assigned,
// as well as a list of errors that occurred due to unassigned field uses
// in the function block itself.
//
func CheckSelfFieldInitializations(
	fields []*ast.FieldDeclaration,
	block *ast.FunctionBlock,
) ([]*ast.FieldDeclaration, []error) {
	analyzer := NewSelfFieldAssignmentAnalyzer()
	assigned := analyzer.VisitNode(block)

	unassigned := make([]*ast.FieldDeclaration, 0)

	for _, field := range fields {
		if !assigned.Contains(field.Identifier) {
			unassigned = append(unassigned, field)
		}
	}

	return unassigned, *analyzer.Errors
}

func NewSelfFieldAssignmentAnalyzer() *SelfFieldAssignmentAnalyzer {
	return &SelfFieldAssignmentAnalyzer{
		assignments: NewAssignmentSet(),
		Errors:      &[]error{},
	}
}

// SelfFieldAssignmentAnalyzer is a visitor that traverses an AST to perform definite assignment analysis.
//
// This analyzer accumulates field assignments as it traverses, meaning that each node is aware of all
// definitely-assigned fields in its logical path.
type SelfFieldAssignmentAnalyzer struct {
	assignments   AssignmentSet
	maybeReturned bool
	Errors        *[]error
}

// branch spawns a new analyzer to inspect a branch of the AST.
func (analyzer *SelfFieldAssignmentAnalyzer) branch() *SelfFieldAssignmentAnalyzer {
	return &SelfFieldAssignmentAnalyzer{
		assignments: analyzer.assignments,
		Errors:      analyzer.Errors,
	}
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
	*analyzer.Errors = append(*analyzer.Errors, err)
}

func (analyzer *SelfFieldAssignmentAnalyzer) visitStatements(statements []ast.Statement) AssignmentSet {
	for _, statement := range statements {
		analyzer.assignments = analyzer.VisitNode(statement)
	}

	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitNode(node ast.Element) AssignmentSet {
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
	return analyzer.VisitNode(node.Block)
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitIfStatement(node *ast.IfStatement) ast.Repr {
	test := node.Test.(ast.Element)
	analyzer.VisitNode(test)

	thenAnalyzer := analyzer.branch()
	thenAssignments := thenAnalyzer.VisitNode(node.Then)

	elseAnalyzer := analyzer.branch()
	elseAssignments := elseAnalyzer.VisitNode(node.Else)

	analyzer.maybeReturned = analyzer.maybeReturned ||
		thenAnalyzer.maybeReturned || elseAnalyzer.maybeReturned

	return thenAssignments.Intersection(elseAssignments)
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitWhileStatement(node *ast.WhileStatement) ast.Repr {
	testAnalyzer := analyzer.branch()
	testAnalyzer.VisitNode(node.Test)

	analyzer.maybeReturned = analyzer.maybeReturned ||
		testAnalyzer.maybeReturned

	blockAnalyzer := analyzer.branch()
	blockAnalyzer.VisitNode(node.Block)

	analyzer.maybeReturned = analyzer.maybeReturned ||
		blockAnalyzer.maybeReturned

	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitVariableDeclaration(node *ast.VariableDeclaration) ast.Repr {
	analyzer.VisitNode(node.Value)

	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitReturnStatement(*ast.ReturnStatement) ast.Repr {
	analyzer.maybeReturned = true
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitBreakStatement(*ast.BreakStatement) ast.Repr {
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitAssignmentStatement(node *ast.AssignmentStatement) ast.Repr {
	node.Value.Accept(analyzer)

	if !analyzer.maybeReturned {
		if memberExpression, ok := node.Target.(*ast.MemberExpression); ok {
			if analyzer.isSelfExpression(memberExpression.Expression) {
				return analyzer.assignments.Insert(memberExpression.Identifier)
			}
		}
	}

	return analyzer.assignments
}
func (analyzer *SelfFieldAssignmentAnalyzer) VisitSwapStatement(node *ast.SwapStatement) ast.Repr {
	if memberExpression, ok := node.Left.(*ast.MemberExpression); ok {
		if analyzer.isSelfExpression(memberExpression.Expression) {
			return analyzer.assignments.Insert(memberExpression.Identifier)
		}
	}

	if memberExpression, ok := node.Right.(*ast.MemberExpression); ok {
		if analyzer.isSelfExpression(memberExpression.Expression) {
			return analyzer.assignments.Insert(memberExpression.Identifier)
		}
	}

	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitExpressionStatement(node *ast.ExpressionStatement) ast.Repr {
	analyzer.VisitNode(node.Expression)
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitInvocationExpression(node *ast.InvocationExpression) ast.Repr {
	for _, arg := range node.Arguments {
		analyzer.VisitNode(arg.Expression)
	}

	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitConditionalExpression(node *ast.ConditionalExpression) ast.Repr {
	analyzer.VisitNode(node.Test)

	thenAnalyzer := analyzer.branch()
	thenAssignments := thenAnalyzer.VisitNode(node.Then)

	elseAnalyzer := analyzer.branch()
	elseAssignments := elseAnalyzer.VisitNode(node.Else)

	analyzer.maybeReturned = thenAnalyzer.maybeReturned || elseAnalyzer.maybeReturned

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

func (analyzer *SelfFieldAssignmentAnalyzer) VisitCondition(*ast.Condition) ast.Repr {
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitImportDeclaration(*ast.ImportDeclaration) ast.Repr {
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitEventDeclaration(*ast.EventDeclaration) ast.Repr {
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitEmitStatement(*ast.EmitStatement) ast.Repr {
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
		analyzer.VisitNode(value)
	}

	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitDictionaryExpression(node *ast.DictionaryExpression) ast.Repr {
	for _, entry := range node.Entries {
		analyzer.VisitNode(entry.Key)
		analyzer.VisitNode(entry.Value)
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
	analyzer.VisitNode(node.TargetExpression)
	if node.IndexingExpression != nil {
		analyzer.VisitNode(node.IndexingExpression)
	}

	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitUnaryExpression(node *ast.UnaryExpression) ast.Repr {
	analyzer.VisitNode(node.Expression)
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitBinaryExpression(node *ast.BinaryExpression) ast.Repr {
	analyzer.VisitNode(node.Left)
	analyzer.VisitNode(node.Right)

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
	analyzer.VisitNode(node.Expression)
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitCreateExpression(*ast.CreateExpression) ast.Repr {
	return analyzer.assignments
}

func (analyzer *SelfFieldAssignmentAnalyzer) VisitDestroyExpression(*ast.DestroyExpression) ast.Repr {
	return analyzer.assignments
}
