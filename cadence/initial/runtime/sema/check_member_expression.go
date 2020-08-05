/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package sema

import (
	"github.com/onflow/cadence-initial/runtime/ast"
	"github.com/onflow/cadence-initial/runtime/common"
)

// NOTE: only called if the member expression is *not* an assignment
//
func (checker *Checker) VisitMemberExpression(expression *ast.MemberExpression) ast.Repr {
	member, isOptional := checker.visitMember(expression)

	if member == nil {
		return &InvalidType{}
	}

	accessedSelfMember := checker.accessedSelfMember(expression)
	if accessedSelfMember != nil {

		functionActivation := checker.functionActivations.Current()

		// Prevent an access to a field before it was initialized.
		//
		// If this is not an assignment to a `self` member, and the member is a field
		// which must be initialized, ensure the field has been initialized.
		//
		// An access of a member which is not a field / which must not be initialized, is safe
		// (e.g. a composite function call)

		info := functionActivation.InitializationInfo
		isInInitializer := info != nil

		if isInInitializer {
			fieldInitialized := info.InitializedFieldMembers.Contains(accessedSelfMember)

			field := info.FieldMembers[accessedSelfMember]
			if field != nil && !fieldInitialized {

				checker.report(
					&UninitializedFieldAccessError{
						Name: field.Identifier.Identifier,
						Pos:  field.Identifier.Pos,
					},
				)
			}
		}
	}

	memberType := member.TypeAnnotation.Type

	// If the member access is optional chaining, only wrap the result value
	// in an optional, if it is not already an optional value

	if isOptional {
		if _, ok := memberType.(*OptionalType); !ok {
			return &OptionalType{Type: memberType}
		}
	}
	return memberType
}

func (checker *Checker) visitMember(expression *ast.MemberExpression) (member *Member, isOptional bool) {
	memberInfo, ok := checker.Elaboration.MemberExpressionMemberInfos[expression]
	if ok {
		return memberInfo.Member, memberInfo.IsOptional
	}

	accessedExpression := expression.Expression

	var expressionType Type

	func() {
		previousMemberExpression := checker.currentMemberExpression
		checker.currentMemberExpression = expression
		defer func() {
			checker.currentMemberExpression = previousMemberExpression
		}()

		expressionType = accessedExpression.Accept(checker).(Type)
	}()

	checker.checkUnusedExpressionResourceLoss(expressionType, accessedExpression)

	// If the the access is to a member of `self` and a resource,
	// its use must be recorded/checked, so that it isn't used after it was invalidated

	accessedSelfMember := checker.accessedSelfMember(expression)
	if accessedSelfMember != nil &&
		accessedSelfMember.TypeAnnotation.Type.IsResourceType() {

		// NOTE: Preventing the capturing of the resource field is already implicitly handled:
		// By definition, the resource field can only be nested in a resource,
		// so `self` is a resource, and the capture of it is checked separately

		checker.checkResourceUseAfterInvalidation(accessedSelfMember, expression.Identifier)
		checker.resources.AddUse(accessedSelfMember, expression.Identifier.Pos)
	}

	origins := checker.memberOrigins[expressionType]

	identifier := expression.Identifier.Identifier
	identifierStartPosition := expression.Identifier.StartPosition()
	identifierEndPosition := expression.Identifier.EndPosition()

	// Check if the type instance actually has members. For most types (e.g. composite types)
	// this is known statically (in the sense of this host language (Go), not the implemented language),
	// i.e. a Go type switch would be sufficient.
	// However, for some types (e.g. reference types) this depends on what type is referenced

	getMemberForType := func(expressionType Type) {
		if ty, ok := expressionType.(MemberAccessibleType); ok && ty.CanHaveMembers() {
			targetRange := ast.NewRangeFromPositioned(expression.Expression)
			member = ty.GetMember(identifier, targetRange, checker.report)
		}
	}

	// Get the member from the accessed value based
	// on the use of optional chaining syntax

	if expression.Optional {

		// If the member expression is using optional chaining,
		// check if the accessed type is optional

		if optionalExpressionType, ok := expressionType.(*OptionalType); ok {
			// The accessed type is optional, get the member from the wrapped type

			getMemberForType(optionalExpressionType.Type)
			isOptional = true
		} else {
			// Optional chaining was used on a non-optional type, report an error

			if !expressionType.IsInvalidType() {
				checker.report(
					&InvalidOptionalChainingError{
						Type:  expressionType,
						Range: ast.NewRangeFromPositioned(expression),
					},
				)
			}

			// NOTE: still try to get member for non-optional expression
			// to avoid spurious error that member does not exist,
			// even if the non-optional accessed type has the member

			getMemberForType(expressionType)
		}
	} else {
		// The member is accessed directly without optional chaining.
		// Get the member directly from the accessed type

		getMemberForType(expressionType)
		isOptional = false
	}

	if member == nil {
		if !expressionType.IsInvalidType() {
			checker.report(
				&NotDeclaredMemberError{
					Type: expressionType,
					Name: identifier,
					Range: ast.Range{
						StartPos: identifierStartPosition,
						EndPos:   identifierEndPosition,
					},
				},
			)
		}
	} else {
		origin := origins[identifier]
		checker.Occurrences.Put(
			identifierStartPosition,
			identifierEndPosition,
			origin,
		)

		// Check access and report if inaccessible

		if !checker.isReadableMember(member) {
			checker.report(
				&InvalidAccessError{
					Name:              member.Identifier.Identifier,
					RestrictingAccess: member.Access,
					DeclarationKind:   member.DeclarationKind,
					Range:             ast.NewRangeFromPositioned(expression),
				},
			)
		}

		// Check that the member access is not to a function of resource type
		// outside of an invocation of it.
		//
		// This would result in a bound method for a resource, which is invalid.

		if !checker.inAssignment &&
			!checker.inInvocation &&
			member.DeclarationKind == common.DeclarationKindFunction &&
			!expressionType.IsInvalidType() &&
			expressionType.IsResourceType() {

			checker.report(
				&ResourceMethodBindingError{
					Range: ast.NewRangeFromPositioned(expression),
				},
			)
		}
	}

	checker.Elaboration.MemberExpressionMemberInfos[expression] =
		MemberInfo{
			Member:     member,
			IsOptional: isOptional,
		}

	return member, isOptional
}

// isReadableMember returns true if the given member can be read from
// in the current location of the checker
//
func (checker *Checker) isReadableMember(member *Member) bool {
	if checker.isReadableAccess(member.Access) ||
		checker.containerTypes[member.ContainerType] {

		return true
	}

	switch member.Access {
	case ast.AccessContract:
		// If the member allows access from the containing contract,
		// check if the current location is contained in the member's contract

		contractType := containingContractKindedType(member.ContainerType)
		if checker.containerTypes[contractType] {
			return true
		}

	case ast.AccessAccount:
		// If the member allows access from the containing account,
		// check if the current location is the same as the member's container location

		location := member.ContainerType.(LocatedType).GetLocation()
		if ast.LocationsMatch(checker.Location, location) {
			return true
		}
	}

	return false
}

// isWriteableMember returns true if the given member can be written to
// in the current location of the checker
//
func (checker *Checker) isWriteableMember(member *Member) bool {
	return checker.isWriteableAccess(member.Access) ||
		checker.containerTypes[member.ContainerType]
}

// containingContractKindedType returns the containing contract-kinded type
// of the given type, if any.
//
// The given type itself might be the result.
//
func containingContractKindedType(t Type) CompositeKindedType {
	for {
		if compositeKindedType, ok := t.(CompositeKindedType); ok &&
			compositeKindedType.GetCompositeKind() == common.CompositeKindContract {

			return compositeKindedType
		}

		if containedType, ok := t.(ContainedType); ok {
			t = containedType.GetContainerType()
			continue
		}

		return nil
	}
}
