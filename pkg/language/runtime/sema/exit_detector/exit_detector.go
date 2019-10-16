package exit_detector

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
)

// FunctionBlockExits determines if the given function is guaranteed to terminate.
//
// A function block terminates if all branches of its AST end in either a
// return statement or simple infinite loop (such as while(true)).
func FunctionBlockExits(node *ast.FunctionBlock) bool {
	detector := &ExitDetector{enclosingBlockContainsBreak: false}
	return node.Accept(detector).(bool)
}

type ExitDetector struct {
	enclosingBlockContainsBreak bool
}

func (detector *ExitDetector) visitStatements(statements []ast.Statement) bool {
	if statements == nil {
		return false
	}

	for _, statement := range statements {
		if statement.Accept(detector).(bool) {
			// TODO: report dead code after returning statement: https://github.com/dapperlabs/flow-go/issues/682
			return true
		}
	}

	return false
}

func (detector *ExitDetector) nodeExits(node ast.Element) bool {
	if node == nil {
		return false
	}

	return node.Accept(detector).(bool)
}

func (detector *ExitDetector) VisitReturnStatement(*ast.ReturnStatement) ast.Repr {
	return true
}

func (detector *ExitDetector) VisitBlock(node *ast.Block) ast.Repr {
	if node == nil {
		return false
	}

	return detector.visitStatements(node.Statements)
}

func (detector *ExitDetector) VisitFunctionBlock(node *ast.FunctionBlock) ast.Repr {
	return detector.nodeExits(node.Block)
}

func (detector *ExitDetector) VisitBreakStatement(*ast.BreakStatement) ast.Repr {
	detector.enclosingBlockContainsBreak = true
	return false
}

func (detector *ExitDetector) VisitIfStatement(node *ast.IfStatement) ast.Repr {
	test := node.Test.(ast.Element)

	// if the conditional exits, the whole statement exits
	if detector.nodeExits(test) {
		return true
	}

	// if conditional is a boolean literal, check then/else
	// TODO: evaluate more complex cases
	if booleanLiteral, ok := test.(*ast.BoolExpression); ok {
		if booleanLiteral.Value {
			return detector.nodeExits(node.Then)
		} else if node.Else != nil {
			return detector.nodeExits(node.Else)
		}
	}

	// otherwise, statement only exits if both cases exit
	thenExits := detector.nodeExits(node.Then)
	elseExits := detector.nodeExits(node.Else)

	return thenExits && elseExits
}

func (detector *ExitDetector) VisitWhileStatement(node *ast.WhileStatement) ast.Repr {
	outerBreakValue := detector.enclosingBlockContainsBreak
	detector.enclosingBlockContainsBreak = false
	defer func() {
		detector.enclosingBlockContainsBreak = outerBreakValue
	}()

	test := node.Test.(ast.Element)

	if detector.nodeExits(test) {
		return true
	}

	// visit enclosing block
	node.Block.Accept(detector)

	// TODO: evaluate more complex cases
	if booleanLiteral, ok := test.(*ast.BoolExpression); ok {
		// while(true) exits unless there is a break
		if booleanLiteral.Value && !detector.enclosingBlockContainsBreak {
			return true
		}
	}

	return false
}

func (detector *ExitDetector) VisitVariableDeclaration(node *ast.VariableDeclaration) ast.Repr {
	return detector.nodeExits(node.Value)
}

func (detector *ExitDetector) VisitAssignmentStatement(node *ast.AssignmentStatement) ast.Repr {
	return detector.nodeExits(node.Target) || detector.nodeExits(node.Value)
}

func (detector *ExitDetector) VisitSwapStatement(node *ast.SwapStatement) ast.Repr {
	return detector.nodeExits(node.Left) || detector.nodeExits(node.Right)
}

func (detector *ExitDetector) VisitExpressionStatement(node *ast.ExpressionStatement) ast.Repr {
	return detector.nodeExits(node.Expression)
}

func (detector *ExitDetector) VisitInvocationExpression(node *ast.InvocationExpression) ast.Repr {
	// TODO: handle invocations that do not return (i.e. have a `Never` return type)
	return false
}

func (detector *ExitDetector) VisitConditionalExpression(node *ast.ConditionalExpression) ast.Repr {
	return false
}

func (detector *ExitDetector) VisitProgram(node *ast.Program) ast.Repr {
	return false
}

func (detector *ExitDetector) VisitFunctionDeclaration(*ast.FunctionDeclaration) ast.Repr {
	return false
}

func (detector *ExitDetector) VisitCompositeDeclaration(*ast.CompositeDeclaration) ast.Repr {
	return false
}

func (detector *ExitDetector) VisitInterfaceDeclaration(*ast.InterfaceDeclaration) ast.Repr {
	return false
}

func (detector *ExitDetector) VisitFieldDeclaration(*ast.FieldDeclaration) ast.Repr {
	return false
}

func (detector *ExitDetector) VisitCondition(node *ast.Condition) ast.Repr {
	return false
}

func (detector *ExitDetector) VisitImportDeclaration(*ast.ImportDeclaration) ast.Repr {
	return false
}

func (detector *ExitDetector) VisitEventDeclaration(*ast.EventDeclaration) ast.Repr {
	return false
}

func (detector *ExitDetector) VisitEmitStatement(*ast.EmitStatement) ast.Repr {
	return false
}

func (detector *ExitDetector) VisitContinueStatement(*ast.ContinueStatement) ast.Repr {
	return false
}

func (detector *ExitDetector) VisitBoolExpression(*ast.BoolExpression) ast.Repr {
	return false
}

func (detector *ExitDetector) VisitNilExpression(*ast.NilExpression) ast.Repr {
	return false
}

func (detector *ExitDetector) VisitIntExpression(*ast.IntExpression) ast.Repr {
	return false
}

func (detector *ExitDetector) VisitArrayExpression(*ast.ArrayExpression) ast.Repr {
	return false
}

func (detector *ExitDetector) VisitDictionaryExpression(*ast.DictionaryExpression) ast.Repr {
	return false
}

func (detector *ExitDetector) VisitIdentifierExpression(*ast.IdentifierExpression) ast.Repr {
	return false
}

func (detector *ExitDetector) VisitMemberExpression(*ast.MemberExpression) ast.Repr {
	return false
}

func (detector *ExitDetector) VisitIndexExpression(*ast.IndexExpression) ast.Repr {
	return false
}

func (detector *ExitDetector) VisitUnaryExpression(*ast.UnaryExpression) ast.Repr {
	return false
}

func (detector *ExitDetector) VisitBinaryExpression(*ast.BinaryExpression) ast.Repr {
	return false
}

func (detector *ExitDetector) VisitFunctionExpression(*ast.FunctionExpression) ast.Repr {
	return false
}

func (detector *ExitDetector) VisitStringExpression(*ast.StringExpression) ast.Repr {
	return false
}

func (detector *ExitDetector) VisitFailableDowncastExpression(*ast.FailableDowncastExpression) ast.Repr {
	return false
}

func (detector *ExitDetector) VisitCreateExpression(*ast.CreateExpression) ast.Repr {
	return false
}

func (detector *ExitDetector) VisitDestroyExpression(expression *ast.DestroyExpression) ast.Repr {
	return false
}
