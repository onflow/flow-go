package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
)

func (checker *Checker) VisitTransactionDeclaration(declaration *ast.TransactionDeclaration) ast.Repr {
	transactionType := checker.Elaboration.TransactionDeclarationTypes[declaration]

	// enter a new scope for this transaction
	checker.enterValueScope()
	defer checker.leaveValueScope(true)

	checker.checkTransactionFields(declaration)

	checker.declareSelfValue(transactionType)

	checker.checkTransactionPrepareFunction(declaration, transactionType)
	checker.checkTransactionExecuteFunction(declaration, transactionType)

	return nil
}

func (checker *Checker) checkTransactionFields(declaration *ast.TransactionDeclaration) {
	for _, field := range declaration.Fields {
		if field.Access != ast.AccessNotSpecified {
			// error: no access modifier required
		}
	}

	// TODO: error for redeclaration
}

func (checker *Checker) checkTransactionPrepareFunction(
	declaration *ast.TransactionDeclaration,
	transactionType *TransactionType,
) {
	if declaration.Prepare == nil {
		// prepare is optional
		return
	}

	fieldMembers := map[*Member]*ast.FieldDeclaration{}

	for _, field := range declaration.Fields {
		fieldName := field.Identifier.Identifier
		member := transactionType.Members[fieldName]
		fieldMembers[member] = field
	}

	initializationInfo := NewInitializationInfo(transactionType, fieldMembers)

	prepareFunction := declaration.Prepare
	prepareFunctionType := transactionType.Prepare

	checker.checkFunction(
		prepareFunction.ParameterList,
		ast.Position{},
		prepareFunctionType.FunctionType,
		prepareFunction.FunctionBlock,
		true,
		initializationInfo,
		true,
	)

	checker.checkTransactionPrepareFunctionParameters(
		prepareFunction.ParameterList,
		prepareFunctionType.ParameterTypeAnnotations,
	)
}

func (checker *Checker) checkTransactionPrepareFunctionParameters(
	parameterList *ast.ParameterList,
	parameterTypeAnnotations []*TypeAnnotation,
) {
	for i, parameter := range parameterList.Parameters {
		parameterTypeAnnotation := parameterTypeAnnotations[i]

		_ = parameter
		_ = parameterTypeAnnotation

		// TODO: only allow Account type
		// should Account become a built-in type? :/
	}

}

func (checker *Checker) checkTransactionExecuteFunction(
	declaration *ast.TransactionDeclaration,
	transactionType *TransactionType,
) {
	if declaration.Execute == nil {
		// TODO error: execute block is required
		panic("NO execute BLOCK")
	}

	// execute always has a void return type
	returnType := &TypeAnnotation{
		Move: false,
		Type: &VoidType{},
	}

	checker.visitFunctionBlock(
		declaration.Execute,
		returnType,
		true,
	)
}

func (checker *Checker) declareTransactionDeclaration(declaration *ast.TransactionDeclaration) {
	transactionType := &TransactionType{}

	members, origins := checker.membersAndOrigins(
		transactionType,
		declaration.Fields,
		nil,
		true,
	)

	checker.memberOrigins[transactionType] = origins

	prepareParameterTypeAnnotations := checker.parameterTypeAnnotations(declaration.Prepare.ParameterList)

	prepareFunctionType := &SpecialFunctionType{
		FunctionType: &FunctionType{
			ParameterTypeAnnotations: prepareParameterTypeAnnotations,
			ReturnTypeAnnotation:     NewTypeAnnotation(&VoidType{}),
		},
	}

	transactionType.Members = members
	transactionType.Prepare = prepareFunctionType

	checker.Elaboration.TransactionDeclarationTypes[declaration] = transactionType
}
