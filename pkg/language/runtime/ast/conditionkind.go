package ast

import "github.com/dapperlabs/bamboo-node/pkg/language/runtime/errors"

//go:generate stringer -type=ConditionKind

type ConditionKind int

const (
	ConditionKindUnknown ConditionKind = iota
	ConditionKindPre
	ConditionKindPost
)

func (k ConditionKind) Name() string {
	switch k {
	case ConditionKindPre:
		return "pre-condition"
	case ConditionKindPost:
		return "post-condition"
	}

	panic(&errors.UnreachableError{})
}
