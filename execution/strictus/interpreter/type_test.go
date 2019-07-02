package interpreter

import (
	. "github.com/onsi/gomega"
	"testing"
)

func TestConstantSizedType_String(t *testing.T) {
	RegisterTestingT(t)

	ty := ConstantSizedType{
		Type: &VariableSizedType{Type: &IntType{}},
		Size: 2,
	}

	Expect(ty.String()).To(Equal("Int[2][]"))
}

func TestVariableSizedType_String(t *testing.T) {
	RegisterTestingT(t)

	ty := VariableSizedType{
		Type: &ConstantSizedType{
			Type: &IntType{},
			Size: 2,
		},
	}

	Expect(ty.String()).To(Equal("Int[][2]"))
}
