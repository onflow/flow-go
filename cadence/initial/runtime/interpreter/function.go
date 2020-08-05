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

	"github.com/raviqqe/hamt"

	"github.com/onflow/cadence-initial/runtime/ast"
	"github.com/onflow/cadence-initial/runtime/common"
	"github.com/onflow/cadence-initial/runtime/errors"
	"github.com/onflow/cadence-initial/runtime/sema"

	. "github.com/onflow/cadence-initial/runtime/trampoline"
)

// Invocation

type Invocation struct {
	Self               *CompositeValue
	Arguments          []Value
	ArgumentTypes      []sema.Type
	TypeParameterTypes map[*sema.TypeParameter]sema.Type
	LocationRange      LocationRange
	Interpreter        *Interpreter
}

// FunctionValue

type FunctionValue interface {
	Value
	isFunctionValue()
	Invoke(Invocation) Trampoline
}

// InterpretedFunctionValue

type InterpretedFunctionValue struct {
	Interpreter      *Interpreter
	ParameterList    *ast.ParameterList
	Type             *sema.FunctionType
	Activation       hamt.Map
	BeforeStatements []ast.Statement
	PreConditions    ast.Conditions
	Statements       []ast.Statement
	PostConditions   ast.Conditions
}

func (f InterpretedFunctionValue) String() string {
	return fmt.Sprintf("Function%s", f.Type.String())
}

func (InterpretedFunctionValue) IsValue() {}

func (InterpretedFunctionValue) DynamicType(_ *Interpreter) DynamicType {
	return FunctionDynamicType{}
}

func (f InterpretedFunctionValue) Copy() Value {
	return f
}

func (InterpretedFunctionValue) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (InterpretedFunctionValue) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (InterpretedFunctionValue) IsModified() bool {
	return false
}

func (InterpretedFunctionValue) SetModified(_ bool) {
	// NO-OP
}

func (InterpretedFunctionValue) isFunctionValue() {}

func (f InterpretedFunctionValue) Invoke(invocation Invocation) Trampoline {
	return f.Interpreter.invokeInterpretedFunction(f, invocation)
}

// HostFunctionValue

type HostFunction func(invocation Invocation) Trampoline

type HostFunctionValue struct {
	Function HostFunction
	Members  map[string]Value
}

func (f HostFunctionValue) String() string {
	// TODO: include type
	return "Function(...)"
}

func NewHostFunctionValue(
	function HostFunction,
) HostFunctionValue {
	return HostFunctionValue{
		Function: function,
	}
}

func (HostFunctionValue) IsValue() {}

func (HostFunctionValue) DynamicType(_ *Interpreter) DynamicType {
	return FunctionDynamicType{}
}

func (f HostFunctionValue) Copy() Value {
	return f
}

func (HostFunctionValue) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (HostFunctionValue) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (HostFunctionValue) IsModified() bool {
	return false
}

func (HostFunctionValue) SetModified(_ bool) {
	// NO-OP
}

func (HostFunctionValue) isFunctionValue() {}

func (f HostFunctionValue) Invoke(invocation Invocation) Trampoline {
	return f.Function(invocation)
}

func (f HostFunctionValue) GetMember(_ *Interpreter, _ LocationRange, name string) Value {
	return f.Members[name]
}

func (f HostFunctionValue) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

// BoundFunctionValue

type BoundFunctionValue struct {
	Function FunctionValue
	Self     *CompositeValue
}

func (f BoundFunctionValue) String() string {
	return fmt.Sprint(f.Function)
}

func (BoundFunctionValue) IsValue() {}

func (BoundFunctionValue) DynamicType(_ *Interpreter) DynamicType {
	return FunctionDynamicType{}
}

func (f BoundFunctionValue) Copy() Value {
	return f
}

func (BoundFunctionValue) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (BoundFunctionValue) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (BoundFunctionValue) IsModified() bool {
	return false
}

func (BoundFunctionValue) SetModified(_ bool) {
	// NO-OP
}

func (BoundFunctionValue) isFunctionValue() {}

func (f BoundFunctionValue) Invoke(invocation Invocation) Trampoline {
	invocation.Self = f.Self
	return f.Function.Invoke(invocation)
}
