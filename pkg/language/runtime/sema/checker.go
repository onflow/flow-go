package sema

import (
	"fmt"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/common"
	goRuntime "runtime"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/activations"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
)

type Checker struct {
	Program          *ast.Program
	valueActivations *activations.Activations
	typeActivations  *activations.Activations
	Globals          map[string]*Variable
}

func NewChecker(program *ast.Program) *Checker {
	return &Checker{
		Program:          program,
		valueActivations: &activations.Activations{},
		typeActivations:  &activations.Activations{},
		Globals:          map[string]*Variable{},
	}
}

func (checker *Checker) setVariable(name string, variable *Variable) {
	checker.valueActivations.Set(name, variable)
}

func (checker *Checker) setType(name string, ty Type) {
	checker.typeActivations.Set(name, ty)
}

func (checker *Checker) findVariable(name string) *Variable {
	value := checker.valueActivations.Find(name)
	if value == nil {
		return nil
	}
	variable, ok := value.(*Variable)
	if !ok {
		return nil
	}
	return variable
}

func (checker *Checker) findType(name string) Type {
	value := checker.typeActivations.Find(name)
	if value == nil {
		return nil
	}
	ty, ok := value.(Type)
	if !ok {
		return nil
	}
	return ty
}

func (checker *Checker) pushActivations() {
	checker.valueActivations.PushCurrent()
	checker.typeActivations.PushCurrent()
}

func (checker *Checker) popActivations() {
	checker.valueActivations.Pop()
	checker.typeActivations.Pop()
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
	checker.pushActivations()
	defer checker.popActivations()
	declaration.Block.Accept(checker)

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
	depth := checker.valueActivations.Depth()
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
	checker.pushActivations()
	defer checker.popActivations()

	for _, statement := range block.Statements {
		statement.Accept(checker)
	}

	return nil
}

func (checker *Checker) VisitReturnStatement(statement *ast.ReturnStatement) ast.Repr {
	statement.Expression.Accept(checker)

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
	variable := checker.findVariable(expression.Identifier)
	if variable == nil {
		panic(&NotDeclaredError{
			ExpectedKind: common.DeclarationKindValue,
			Name:         expression.Identifier,
			StartPos:     expression.StartPosition(),
			EndPos:       expression.EndPosition(),
		})
	}

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

// ConvertType converts an AST type representation to a sema type
func (checker *Checker) ConvertType(t ast.Type) Type {
	switch t := t.(type) {
	case *ast.BaseType:
		result := checker.findType(t.Identifier)
		if result == nil {
			panic(&NotDeclaredError{
				ExpectedKind: common.DeclarationKindType,
				Name:         t.Identifier,
				// TODO: add start and end position to ast.Type
				StartPos: t.Pos,
				EndPos:   t.Pos,
			})
		}
		return result

	case *ast.VariableSizedType:
		return &VariableSizedType{
			Type: checker.ConvertType(t.Type),
		}

	case *ast.ConstantSizedType:
		return &ConstantSizedType{
			Type: checker.ConvertType(t.Type),
			Size: t.Size,
		}

	case *ast.FunctionType:
		var parameterTypes []Type
		for _, parameterType := range t.ParameterTypes {
			parameterTypes = append(parameterTypes,
				checker.ConvertType(parameterType),
			)
		}

		returnType := checker.ConvertType(t.ReturnType)

		return FunctionType{
			ParameterTypes: parameterTypes,
			ReturnType:     returnType,
		}
	}

	panic(&astTypeConversionError{invalidASTType: t})
}
