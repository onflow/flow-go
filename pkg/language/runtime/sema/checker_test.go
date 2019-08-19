package sema

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/ast"
)

func TestOptionalSubtyping(t *testing.T) {
	RegisterTestingT(t)

	checker := NewChecker(&ast.Program{})

	Expect(checker.IsSubType(
		&OptionalType{Type: &IntType{}},
		&OptionalType{Type: &IntType{}},
	)).
		To(BeTrue())

	Expect(checker.IsSubType(
		&OptionalType{Type: &IntType{}},
		&OptionalType{Type: &BoolType{}},
	)).
		To(BeFalse())

	Expect(checker.IsSubType(
		&OptionalType{Type: &Int8Type{}},
		&OptionalType{Type: &IntegerType{}},
	)).
		To(BeTrue())
}
