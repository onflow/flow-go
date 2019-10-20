package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
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
