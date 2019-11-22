package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
)

func (checker *Checker) VisitTransactionDeclaration(declaration *ast.TransactionDeclaration) ast.Repr {
	transactionType := checker.Elaboration.TransactionDeclarationTypes[declaration]

	checker.checkTransactionFields(declaration)
	checker.checkTransactionBlocks(declaration)

	// enter a new scope for this transaction
	checker.enterValueScope()
	defer checker.leaveValueScope(true)

	checker.declareSelfValue(transactionType)

	checker.checkTransactionPrepareFunction(declaration, transactionType)

	checker.checkTransactionPreConditions(declaration.PreConditions)
	checker.checkTransactionExecuteFunction(declaration, transactionType)
	checker.checkTransactionPostConditions(declaration.PostConditions)

	checker.checkTransactionResourceFieldInvalidation(transactionType)

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

func (checker *Checker) checkTransactionBlocks(declaration *ast.TransactionDeclaration) {
	if declaration.Prepare != nil {
		prepareIdentifier := declaration.Prepare.Identifier
		if prepareIdentifier.Identifier != common.DeclarationKindPrepare.Keywords() {
			checker.report(&InvalidTransactionBlockError{
				Name: prepareIdentifier.Identifier,
				Pos:  prepareIdentifier.Pos,
			})
		}
	}

	if declaration.Execute != nil {
		executeIdentifier := declaration.Execute.Identifier
		if executeIdentifier.Identifier != common.DeclarationKindExecute.Keywords() {
			checker.report(&InvalidTransactionBlockError{
				Name: executeIdentifier.Identifier,
				Pos:  executeIdentifier.Pos,
			})
		}
	}
}

func (checker *Checker) checkTransactionPrepareFunction(
	declaration *ast.TransactionDeclaration,
	transactionType *TransactionType,
) {
	fields := declaration.Fields

	if declaration.Prepare == nil {
		if len(declaration.Fields) != 0 {
			// report error for first field
			firstField := fields[0]

			checker.report(
				&TransactionMissingPrepareError{
					ContainerType:  transactionType,
					FirstFieldName: firstField.Identifier.Identifier,
					FirstFieldPos:  firstField.Identifier.Pos,
				},
			)
		}

		return
	}

	fieldMembers := map[*Member]*ast.FieldDeclaration{}

	for _, field := range fields {
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

func (checker *Checker) checkTransactionPreConditions(conditions []*ast.Condition) {
	checker.visitConditions(conditions)
}

func (checker *Checker) checkTransactionExecuteFunction(
	declaration *ast.TransactionDeclaration,
	transactionType *TransactionType,
) {
	if declaration.Execute == nil {
		checker.report(&TransactionMissingExecuteError{
			Range: declaration.Range,
		})

		return
	}

	checker.enterValueScope()
	defer checker.leaveValueScope(true)

	checker.visitStatements(declaration.Execute.FunctionBlock.Statements)
}

func (checker *Checker) checkTransactionPostConditions(conditions []*ast.Condition) {
	if len(conditions) > 0 {
		checker.declareBefore()
	}

	checker.visitConditions(conditions)
}

func (checker *Checker) checkTransactionResourceFieldInvalidation(transactionType *TransactionType) {
	for name, member := range transactionType.Members {
		if !member.Type.IsResourceType() {
			return
		}

		info := checker.resources.Get(member)
		if !info.DefinitivelyInvalidated {
			// TODO: use different error here
			checker.report(
				&ResourceFieldNotInvalidatedError{
					FieldName: name,
					TypeName:  transactionType.String(),
					// TODO:
					Pos: ast.Position{},
				},
			)
		}
	}
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

	var prepareParameterTypeAnnotations []*TypeAnnotation
	if declaration.Prepare != nil {
		prepareParameterTypeAnnotations = checker.parameterTypeAnnotations(declaration.Prepare.ParameterList)
	}

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
