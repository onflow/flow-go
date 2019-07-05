package interpreter

import (
	. "github.com/onsi/gomega"
	"testing"
)

func TestConstantSizedType_String(t *testing.T) {
	RegisterTestingT(t)

	ty := &ConstantSizedType{
		Type: &VariableSizedType{Type: &IntType{}},
		Size: 2,
	}

	Expect(ty.String()).To(Equal("Int[2][]"))
}

func TestConstantSizedType_String_OfFunctionType(t *testing.T) {
	RegisterTestingT(t)

	ty := &ConstantSizedType{
		Type: &FunctionType{
			ParameterTypes: []Type{
				&Int8Type{},
			},
			ReturnType: &Int16Type{},
		},
		Size: 2,
	}

	Expect(ty.String()).To(Equal("((Int8) -> Int16)[2]"))
}

func TestVariableSizedType_String(t *testing.T) {
	RegisterTestingT(t)

	ty := &VariableSizedType{
		Type: &ConstantSizedType{
			Type: &IntType{},
			Size: 2,
		},
	}

	Expect(ty.String()).To(Equal("Int[][2]"))
}

func TestVariableSizedType_String_OfFunctionType(t *testing.T) {
	RegisterTestingT(t)

	ty := &VariableSizedType{
		Type: &FunctionType{
			ParameterTypes: []Type{
				&Int8Type{},
			},
			ReturnType: &Int16Type{},
		},
	}

	Expect(ty.String()).To(Equal("((Int8) -> Int16)[]"))
}
