package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
)

func (checker *Checker) VisitTransactionDeclaration(declaration *ast.TransactionDeclaration) ast.Repr {
	transactionType := checker.Elaboration.TransactionDeclarationTypes[declaration]

	fieldMembers := map[*Member]*ast.FieldDeclaration{}

	for _, field := range declaration.Fields {
		fieldName := field.Identifier.Identifier
		member := transactionType.Members[fieldName]
		fieldMembers[member] = field
	}

	checker.checkTransactionFields(declaration)
	checker.checkTransactionBlocks(declaration)

	// enter a new scope for this transaction
	checker.enterValueScope()
	defer checker.leaveValueScope(true)

	checker.declareSelfValue(transactionType)

	checker.visitTransactionPrepareFunction(declaration.Prepare, transactionType, fieldMembers)

	checker.visitTransactionPreConditions(declaration.PreConditions)
	checker.visitTransactionExecuteFunction(declaration.Execute, transactionType)
	checker.visitTransactionPostConditions(declaration.PostConditions)

	checker.checkTransactionResourceFieldInvalidation(transactionType, fieldMembers)

	return nil
}

func (checker *Checker) checkTransactionFields(declaration *ast.TransactionDeclaration) {
	for _, field := range declaration.Fields {
		if field.Access != ast.AccessNotSpecified {
			checker.report(
				&InvalidTransactionFieldAccessModifierError{
					Name:   field.Identifier.Identifier,
					Access: field.Access.Keyword(),
					Pos:    field.StartPosition(),
				},
			)
		}
	}
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
	} else {
		if len(declaration.Fields) != 0 {
			// report error for first field
			firstField := declaration.Fields[0]

			checker.report(
				&TransactionMissingPrepareError{
					FirstFieldName: firstField.Identifier.Identifier,
					FirstFieldPos:  firstField.Identifier.Pos,
				},
			)
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
	} else {
		checker.report(&TransactionMissingExecuteError{
			Range: declaration.Range,
		})
	}
}

func (checker *Checker) visitTransactionPrepareFunction(
	prepareFunction *ast.SpecialFunctionDeclaration,
	transactionType *TransactionType,
	fieldMembers map[*Member]*ast.FieldDeclaration,
) {
	if prepareFunction == nil {
		return
	}

	initializationInfo := NewInitializationInfo(transactionType, fieldMembers)

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
	}

}

func (checker *Checker) visitTransactionPreConditions(conditions []*ast.Condition) {
	checker.visitConditions(conditions)
}

func (checker *Checker) visitTransactionExecuteFunction(
	executeFunction *ast.SpecialFunctionDeclaration,
	transactionType *TransactionType,
) {
	if executeFunction == nil {
		return
	}

	checker.enterValueScope()
	defer checker.leaveValueScope(true)

	checker.visitStatements(executeFunction.FunctionBlock.Statements)
}

func (checker *Checker) visitTransactionPostConditions(conditions []*ast.Condition) {
	if len(conditions) > 0 {
		checker.declareBefore()
	}

	checker.visitConditions(conditions)
}

func (checker *Checker) checkTransactionResourceFieldInvalidation(
	transactionType *TransactionType,
	fieldMembers map[*Member]*ast.FieldDeclaration,
) {
	for member, field := range fieldMembers {
		if !member.Type.IsResourceType() {
			return
		}

		info := checker.resources.Get(member)
		if !info.DefinitivelyInvalidated {

			checker.report(
				&ResourceFieldNotInvalidatedError{
					FieldName: field.Identifier.Identifier,
					TypeName:  transactionType.String(),
					Pos:       field.Identifier.StartPosition(),
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
