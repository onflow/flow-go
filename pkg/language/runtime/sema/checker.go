package sema

import (
	"fmt"
	goRuntime "runtime"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/activations"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
)

type Checker struct {
	Program     *ast.Program
	activations *activations.Activations
	Globals     map[string]*Variable
}

func NewChecker(program *ast.Program) *Checker {
	return &Checker{
		Program:     program,
		activations: &activations.Activations{},
		Globals:     map[string]*Variable{},
	}
}

func (checker *Checker) Check() (err error) {
	// recover internal panics and return them as an error
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			// don't recover Go errors
			err, ok = r.(goRuntime.Error)
			if ok {
				panic(err)
			}
			err, ok = r.(error)
			if !ok {
				err = fmt.Errorf("%v", r)
			}
		}
	}()

	checker.Program.Accept(checker)

	return nil
}

func (checker *Checker) VisitProgram(program *ast.Program) ast.Repr {
	for _, declaration := range program.Declarations {
		declaration.Accept(checker)
		checker.declareGlobal(declaration)
	}
	return nil
}

func (checker *Checker) VisitFunctionDeclaration(declaration *ast.FunctionDeclaration) ast.Repr {
	// TODO:
	return nil
}

func (checker *Checker) VisitVariableDeclaration(declaration *ast.VariableDeclaration) ast.Repr {
	valueType := declaration.Value.Accept(checker).(Type)
	declarationType := valueType
	// does the declaration have an explicit type annotation?
	if declaration.Type != nil {
		// TODO: check value type is subtype of declaration type
		// TODO: use explicit declaration type
	}
	checker.declareVariable(declaration, declarationType)

	return nil
}

func (checker *Checker) declareVariable(declaration *ast.VariableDeclaration, ty Type) {
	// check if variable with this name is already declared in the current scope
	variable := checker.findVariable(declaration.Identifier)
	depth := checker.activations.Depth()
	if variable != nil && variable.Depth == depth {
		panic(&RedeclarationError{
			Name: declaration.Identifier,
			Pos:  declaration.GetIdentifierPosition(),
		})
	}

	// variable with this name is not declared in current scope, declare it
	variable = &Variable{
		Declaration: declaration,
		Depth:       depth,
		Type:        ty,
	}
	checker.setVariable(declaration.Identifier, variable)
}

func (checker *Checker) setVariable(name string, variable *Variable) {
	checker.activations.Set(name, variable)
}

func (checker *Checker) findVariable(name string) *Variable {
	value := checker.activations.Find(name)
	if value == nil {
		return nil
	}
	variable, ok := value.(*Variable)
	if !ok {
		return nil
	}
	return variable
}

func (checker *Checker) declareGlobal(declaration ast.Declaration) {
	name := declaration.DeclarationName()
	if _, exists := checker.Globals[name]; exists {
		panic(&RedeclarationError{
			Name: name,
			Pos:  declaration.GetIdentifierPosition(),
		})
	}
	checker.Globals[name] = checker.findVariable(name)
}

func (checker *Checker) VisitBlock(block *ast.Block) ast.Repr {
	// TODO:
	return nil
}

func (checker *Checker) VisitReturnStatement(statement *ast.ReturnStatement) ast.Repr {
	// TODO:
	return nil
}

func (checker *Checker) VisitIfStatement(statement *ast.IfStatement) ast.Repr {
	// TODO:
	return nil
}

func (checker *Checker) VisitWhileStatement(statement *ast.WhileStatement) ast.Repr {
	// TODO:
	return nil
}

func (checker *Checker) VisitAssignment(assignment *ast.AssignmentStatement) ast.Repr {
	// TODO:
	return nil
}

func (checker *Checker) VisitIdentifierExpression(expression *ast.IdentifierExpression) ast.Repr {
	// TODO:
	return nil
}

func (checker *Checker) VisitBinaryExpression(expression *ast.BinaryExpression) ast.Repr {
	// TODO:
	return nil
}

func (checker *Checker) VisitUnaryExpression(expression *ast.UnaryExpression) ast.Repr {
	// TODO:
	return nil
}

func (checker *Checker) VisitExpressionStatement(statement *ast.ExpressionStatement) ast.Repr {
	// TODO:
	return nil
}

func (checker *Checker) VisitBoolExpression(expression *ast.BoolExpression) ast.Repr {
	return &BoolType{}
}

func (checker *Checker) VisitIntExpression(expression *ast.IntExpression) ast.Repr {
	return &IntType{}
}

func (checker *Checker) VisitArrayExpression(expression *ast.ArrayExpression) ast.Repr {
	// TODO:
	return nil
}

func (checker *Checker) VisitMemberExpression(*ast.MemberExpression) ast.Repr {
	// TODO:
	return nil
}

func (checker *Checker) VisitIndexExpression(expression *ast.IndexExpression) ast.Repr {
	// TODO:
	return nil
}

func (checker *Checker) VisitConditionalExpression(expression *ast.ConditionalExpression) ast.Repr {
	// TODO:
	return nil
}

func (checker *Checker) VisitInvocationExpression(invocationExpression *ast.InvocationExpression) ast.Repr {
	// TODO:
	return nil
}

func (checker *Checker) VisitFunctionExpression(expression *ast.FunctionExpression) ast.Repr {
	// TODO:
	return nil
}
