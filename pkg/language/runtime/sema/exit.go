package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
)

type ExitAnalyzer struct {
	enclosingBlockContainsBreak    bool
	enclosingBlockContainsContinue bool
}

func (analyzer *ExitAnalyzer) Exits(node ast.Element) bool {
	return node.Accept(analyzer).(bool)
}

func (analyzer *ExitAnalyzer) visitStatements(statements []ast.Statement) ast.Repr {
	if statements == nil {
		return false
	}

	for _, statement := range statements {
		if statement.Accept(analyzer).(bool) {
			return true
		}
	}

	return false
}

func (analyzer *ExitAnalyzer) nodeExits(node ast.Element) bool {
	if node == nil {
		return false
	}

	return node.Accept(analyzer).(bool)
}

func (analyzer *ExitAnalyzer) VisitProgram(node *ast.Program) ast.Repr {
	return false
}

func (analyzer *ExitAnalyzer) VisitFunctionDeclaration(*ast.FunctionDeclaration) ast.Repr {
	return false
}

func (analyzer *ExitAnalyzer) VisitBlock(node *ast.Block) ast.Repr {
	if node == nil {
		return false
	}

	return analyzer.visitStatements(node.Statements).(bool)
}

func (analyzer *ExitAnalyzer) VisitFunctionBlock(node *ast.FunctionBlock) ast.Repr {
	// how to handle pre/post?
	return analyzer.nodeExits(node.Block)
}

func (analyzer *ExitAnalyzer) VisitCompositeDeclaration(*ast.CompositeDeclaration) ast.Repr {
	return false
}

func (analyzer *ExitAnalyzer) VisitInterfaceDeclaration(*ast.InterfaceDeclaration) ast.Repr {
	return false
}

func (analyzer *ExitAnalyzer) VisitFieldDeclaration(*ast.FieldDeclaration) ast.Repr {
	return false
}

func (analyzer *ExitAnalyzer) VisitInitializerDeclaration(node *ast.InitializerDeclaration) ast.Repr {
	return node.FunctionBlock.Accept(analyzer)
}

func (analyzer *ExitAnalyzer) VisitCondition(node *ast.Condition) ast.Repr {
	return false
}

func (analyzer *ExitAnalyzer) VisitImportDeclaration(*ast.ImportDeclaration) ast.Repr {
	return false
}

func (analyzer *ExitAnalyzer) VisitReturnStatement(*ast.ReturnStatement) ast.Repr {
	return true
}

func (analyzer *ExitAnalyzer) VisitBreakStatement(*ast.BreakStatement) ast.Repr {
	analyzer.enclosingBlockContainsBreak = true
	return false
}

func (analyzer *ExitAnalyzer) VisitContinueStatement(*ast.ContinueStatement) ast.Repr {
	analyzer.enclosingBlockContainsContinue = true
	return false
}

func (analyzer *ExitAnalyzer) VisitIfStatement(node *ast.IfStatement) ast.Repr {
	test := node.Test.(ast.Element)

	// if the conditional exits, the whole statement exits
	if analyzer.nodeExits(test) {
		return true
	}

	// if conditional is a boolean literal, check then/else
	if booleanLiteral, ok := test.(*ast.BoolExpression); ok {
		if booleanLiteral.Value {
			return analyzer.nodeExits(node.Then)
		} else if node.Else != nil {
			return analyzer.nodeExits(node.Else)
		}
	}

	// otherwise, statement only exits if both sides exit
	thenExits := analyzer.nodeExits(node.Then)
	elseExits := analyzer.nodeExits(node.Else)

	return thenExits && elseExits
}

func (analyzer *ExitAnalyzer) VisitWhileStatement(node *ast.WhileStatement) ast.Repr {
	outerBreakValue := analyzer.enclosingBlockContainsBreak

	analyzer.enclosingBlockContainsBreak = false

	defer func() {
		analyzer.enclosingBlockContainsBreak = outerBreakValue
	}()

	test := node.Test.(ast.Element)

	if analyzer.nodeExits(test) {
		return true
	}

	// visit enclosing block
	node.Block.Accept(analyzer)

	if booleanLiteral, ok := test.(*ast.BoolExpression); ok {
		// while(true) exits unless there is a break
		if booleanLiteral.Value && !analyzer.enclosingBlockContainsBreak {
			return true
		}
	}

	return false
}

func (analyzer *ExitAnalyzer) VisitVariableDeclaration(node *ast.VariableDeclaration) ast.Repr {
	return analyzer.nodeExits(node.Value)
}

func (analyzer *ExitAnalyzer) VisitAssignment(node *ast.AssignmentStatement) ast.Repr {
	return analyzer.nodeExits(node.Target) || analyzer.nodeExits(node.Value)
}

func (analyzer *ExitAnalyzer) VisitExpressionStatement(node *ast.ExpressionStatement) ast.Repr {
	return analyzer.nodeExits(node.Expression)
}

func (analyzer *ExitAnalyzer) VisitBoolExpression(*ast.BoolExpression) ast.Repr {
	return false
}

func (analyzer *ExitAnalyzer) VisitNilExpression(*ast.NilExpression) ast.Repr {
	return false
}

func (analyzer *ExitAnalyzer) VisitIntExpression(*ast.IntExpression) ast.Repr {
	return false
}

func (analyzer *ExitAnalyzer) VisitArrayExpression(*ast.ArrayExpression) ast.Repr {
	return false
}

func (analyzer *ExitAnalyzer) VisitDictionaryExpression(*ast.DictionaryExpression) ast.Repr {
	return false
}

func (analyzer *ExitAnalyzer) VisitIdentifierExpression(*ast.IdentifierExpression) ast.Repr {
	return false
}

func (analyzer *ExitAnalyzer) VisitInvocationExpression(node *ast.InvocationExpression) ast.Repr {
	if analyzer.nodeExits(node.InvokedExpression) {
		return true
	}

	for _, argument := range node.Arguments {
		if analyzer.nodeExits(argument.Expression) {
			return true
		}
	}

	return false
}

func (analyzer *ExitAnalyzer) VisitMemberExpression(*ast.MemberExpression) ast.Repr {
	return false
}

func (analyzer *ExitAnalyzer) VisitIndexExpression(*ast.IndexExpression) ast.Repr {
	return false
}

func (analyzer *ExitAnalyzer) VisitConditionalExpression(node *ast.ConditionalExpression) ast.Repr {
	test := node.Test.(ast.Element)

	// if the conditional exits, the whole statement exits
	if analyzer.nodeExits(test) {
		return true
	}

	return analyzer.nodeExits(node.Then) && analyzer.nodeExits(node.Else)
}

func (analyzer *ExitAnalyzer) VisitUnaryExpression(*ast.UnaryExpression) ast.Repr {
	return false
}

func (analyzer *ExitAnalyzer) VisitBinaryExpression(*ast.BinaryExpression) ast.Repr {
	return false
}

func (analyzer *ExitAnalyzer) VisitFunctionExpression(*ast.FunctionExpression) ast.Repr {
	return false
}

func (analyzer *ExitAnalyzer) VisitStringExpression(*ast.StringExpression) ast.Repr {
	return false
}

func (analyzer *ExitAnalyzer) VisitFailableDowncastExpression(*ast.FailableDowncastExpression) ast.Repr {
	return false
}
