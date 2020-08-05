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

package interpreter

import (
	"fmt"
	"strings"

	"github.com/onflow/cadence-initial/runtime/ast"
	"github.com/onflow/cadence-initial/runtime/common"
	"github.com/onflow/cadence-initial/runtime/sema"
)

// unsupportedOperation

type unsupportedOperation struct {
	kind      common.OperationKind
	operation ast.Operation
	ast.Range
}

func (e *unsupportedOperation) Error() string {
	return fmt.Sprintf(
		"cannot evaluate unsupported %s operation: %s",
		e.kind.Name(),
		e.operation.Symbol(),
	)
}

// ExternalError is an error that occurred externally.
// It contains the recovered value.
//
type ExternalError struct {
	Recovered interface{}
}

func (e ExternalError) Error() string {
	return fmt.Sprint(e.Recovered)
}

// NotDeclaredError

type NotDeclaredError struct {
	ExpectedKind common.DeclarationKind
	Name         string
}

func (e *NotDeclaredError) Error() string {
	return fmt.Sprintf(
		"cannot find %s in this scope: `%s`",
		e.ExpectedKind.Name(),
		e.Name,
	)
}

func (e *NotDeclaredError) SecondaryError() string {
	return "not found in this scope"
}

// NotInvokableError

type NotInvokableError struct {
	Value Value
}

func (e *NotInvokableError) Error() string {
	return fmt.Sprintf("cannot call value: %#+v", e.Value)
}

// ArgumentCountError

type ArgumentCountError struct {
	ParameterCount int
	ArgumentCount  int
}

func (e *ArgumentCountError) Error() string {
	return fmt.Sprintf(
		"incorrect number of arguments: expected %d, got %d",
		e.ParameterCount,
		e.ArgumentCount,
	)
}

// InvalidParameterTypeInInvocationError

type InvalidParameterTypeInInvocationError struct {
	InvalidParameterType sema.Type
}

func (e *InvalidParameterTypeInInvocationError) Error() string {
	return fmt.Sprintf("cannot invoke functions with parameter type: `%s`", e.InvalidParameterType)
}

// TransactionNotDeclaredError

type TransactionNotDeclaredError struct {
	Index int
}

func (e *TransactionNotDeclaredError) Error() string {
	return fmt.Sprintf(
		"cannot find transaction with index %d in this scope",
		e.Index,
	)
}

// ConditionError

type ConditionError struct {
	ConditionKind ast.ConditionKind
	Message       string
	LocationRange
}

func (e *ConditionError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("%s failed", e.ConditionKind.Name())
	}
	return fmt.Sprintf("%s failed: %s", e.ConditionKind.Name(), e.Message)
}

// RedeclarationError

type RedeclarationError struct {
	Name string
}

func (e *RedeclarationError) Error() string {
	return fmt.Sprintf("cannot redeclare: `%s` is already declared", e.Name)
}

// DereferenceError

type DereferenceError struct {
	LocationRange
}

func (e *DereferenceError) Error() string {
	return "dereference failed"
}

// OverflowError

type OverflowError struct{}

func (e OverflowError) Error() string {
	return "overflow"
}

// UnderflowError

type UnderflowError struct{}

func (e UnderflowError) Error() string {
	return "underflow"
}

// UnderflowError

type DivisionByZeroError struct{}

func (e DivisionByZeroError) Error() string {
	return "division by zero"
}

// DestroyedCompositeError

type DestroyedCompositeError struct {
	CompositeKind common.CompositeKind
	LocationRange
}

func (e *DestroyedCompositeError) Error() string {
	return fmt.Sprintf("%s is destroyed and cannot be accessed anymore", e.CompositeKind.Name())
}

// ForceAssignmentToNonNilResourceError

type ForceAssignmentToNonNilResourceError struct {
	LocationRange
}

func (e *ForceAssignmentToNonNilResourceError) Error() string {
	return "force assignment to non-nil resource-typed value"
}

// ForceNilError

type ForceNilError struct {
	LocationRange
}

func (e *ForceNilError) Error() string {
	return "unexpectedly found nil while forcing an Optional value"
}

// TypeMismatchError

type TypeMismatchError struct {
	ExpectedType sema.Type
	LocationRange
}

func (e *TypeMismatchError) Error() string {
	return fmt.Sprintf(
		"unexpectedly found non-`%s` while force-casting value",
		e.ExpectedType.QualifiedString(),
	)
}

// InvalidPathDomainError

type InvalidPathDomainError struct {
	ActualDomain    common.PathDomain
	ExpectedDomains []common.PathDomain
	LocationRange
}

func (e *InvalidPathDomainError) Error() string {
	return "invalid path domain"
}

func (e *InvalidPathDomainError) SecondaryError() string {

	domainNames := make([]string, len(e.ExpectedDomains))

	for i, domain := range e.ExpectedDomains {
		domainNames[i] = domain.Name()
	}

	return fmt.Sprintf(
		"expected %s, got `%s`",
		common.EnumerateWords(domainNames, "or"),
		e.ActualDomain.Identifier(),
	)
}

// OverwriteError

type OverwriteError struct {
	Address AddressValue
	Path    PathValue
	LocationRange
}

func (e *OverwriteError) Error() string {
	return fmt.Sprintf(
		"failed to save object: path %s in account %s already stores an object",
		e.Path,
		e.Address,
	)
}

// CyclicLinkError

type CyclicLinkError struct {
	Address AddressValue
	Paths   []PathValue
	LocationRange
}

func (e *CyclicLinkError) Error() string {
	var builder strings.Builder
	for i, path := range e.Paths {
		if i > 0 {
			builder.WriteString(" -> ")
		}
		builder.WriteString(path.String())
	}
	paths := builder.String()

	return fmt.Sprintf(
		"cyclic link in account %s: %s",
		e.Address,
		paths,
	)
}
