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
	"example.com/cadence-initial/runtime/ast"
)

type InitializationInfo struct {
	ContainerType           Type
	FieldMembers            map[*Member]*ast.FieldDeclaration
	InitializedFieldMembers *MemberSet
}

func NewInitializationInfo(
	containerType Type,
	fieldMembers map[*Member]*ast.FieldDeclaration,
) *InitializationInfo {
	return &InitializationInfo{
		ContainerType:           containerType,
		FieldMembers:            fieldMembers,
		InitializedFieldMembers: &MemberSet{},
	}
}

// InitializationComplete returns true if all fields of the container
// were initialized, false if some fields are uninitialized
//
func (info *InitializationInfo) InitializationComplete() bool {
	for member := range info.FieldMembers {
		if !info.InitializedFieldMembers.Contains(member) {
			return false
		}
	}

	return true
}
