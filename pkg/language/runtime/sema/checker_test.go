package sema

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestOptionalSubtyping(t *testing.T) {
	RegisterTestingT(t)

	Expect(IsSubType(
		&OptionalType{Type: &IntType{}},
		&OptionalType{Type: &IntType{}},
	)).
		To(BeTrue())

	Expect(IsSubType(
		&OptionalType{Type: &IntType{}},
		&OptionalType{Type: &BoolType{}},
	)).
		To(BeFalse())

	Expect(IsSubType(
		&OptionalType{Type: &Int8Type{}},
		&OptionalType{Type: &IntegerType{}},
	)).
		To(BeTrue())
}
