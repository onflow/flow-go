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

type FunctionActivation struct {
	ReturnType           Type
	Loops                int
	ValueActivationDepth int
	ReturnInfo           *ReturnInfo
	ReportedDeadCode     bool
	InitializationInfo   *InitializationInfo
}

func (a FunctionActivation) InLoop() bool {
	return a.Loops > 0
}

type FunctionActivations struct {
	activations []*FunctionActivation
}

func (a *FunctionActivations) IsLocal() bool {
	currentFunctionDepth := -1

	currentFunctionActivation := a.Current()
	if currentFunctionActivation != nil {
		currentFunctionDepth = currentFunctionActivation.ValueActivationDepth
	}

	return currentFunctionDepth > 0
}

func (a *FunctionActivations) EnterFunction(functionType *FunctionType, valueActivationDepth int) {
	a.activations = append(a.activations,
		&FunctionActivation{
			ReturnType:           functionType.ReturnTypeAnnotation.Type,
			ValueActivationDepth: valueActivationDepth,
			ReturnInfo:           &ReturnInfo{},
		},
	)
}

func (a *FunctionActivations) LeaveFunction() {
	lastIndex := len(a.activations) - 1
	a.activations = a.activations[:lastIndex]
}

func (a *FunctionActivations) WithFunction(functionType *FunctionType, valueActivationDepth int, f func()) {
	a.EnterFunction(functionType, valueActivationDepth)
	defer a.LeaveFunction()
	f()
}

func (a *FunctionActivations) Current() *FunctionActivation {
	lastIndex := len(a.activations) - 1
	if lastIndex < 0 {
		return nil
	}
	return a.activations[lastIndex]
}

func (a *FunctionActivations) EnterLoop() {
	a.Current().Loops++
}

func (a *FunctionActivations) LeaveLoop() {
	a.Current().Loops--
}

func (a *FunctionActivations) WithLoop(f func()) {
	a.EnterLoop()
	defer a.LeaveLoop()
	f()
}
