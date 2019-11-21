package sema

import "github.com/dapperlabs/flow-go/language/runtime/ast"

func (checker *Checker) VisitTransactionDeclaration(declaration *ast.TransactionDeclaration) ast.Repr {
	// TODO: implement transaction type checking
	panic(" not implemented")
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
