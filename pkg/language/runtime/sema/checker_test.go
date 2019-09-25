package sema

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
)

func TestOptionalSubtyping(t *testing.T) {
	RegisterTestingT(t)

	checker, err := NewChecker(&ast.Program{}, nil, nil)

	Expect(err).
		To(Not(HaveOccurred()))

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
