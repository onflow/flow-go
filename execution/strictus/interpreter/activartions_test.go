package interpreter

import (
	"bamboo-runtime/execution/strictus/ast"
	. "github.com/onsi/gomega"
	"testing"
)

func TestActivations(t *testing.T) {
	RegisterTestingT(t)

	activations := &Activations{}

	activations.Set(
		"a",
		&Variable{
			Declaration: ast.VariableDeclaration{
				IsConst:    false,
				Identifier: "a",
				Type:       &ast.Int64Type{},
			},
			Value: 1,
		},
	)

	Expect(activations.Find("a").Value).To(Equal(1))
	Expect(activations.Find("b")).To(BeNil())

	activations.PushCurrent()

	activations.Set(
		"a",
		&Variable{
			Declaration: ast.VariableDeclaration{
				IsConst:    false,
				Identifier: "a",
				Type:       &ast.Int64Type{},
			},
			Value: 2,
		},
	)
	activations.Set(
		"b",
		&Variable{
			Declaration: ast.VariableDeclaration{
				IsConst:    false,
				Identifier: "b",
				Type:       &ast.Int64Type{},
			},
			Value: 3,
		},
	)

	Expect(activations.Find("a").Value).To(Equal(2))
	Expect(activations.Find("b").Value).To(Equal(3))
	Expect(activations.Find("c")).To(BeNil())

	activations.PushCurrent()

	activations.Set(
		"a",
		&Variable{
			Declaration: ast.VariableDeclaration{
				IsConst:    false,
				Identifier: "a",
				Type:       &ast.Int64Type{},
			},
			Value: 5,
		},
	)
	activations.Set(
		"c",
		&Variable{
			Declaration: ast.VariableDeclaration{
				IsConst:    false,
				Identifier: "c",
				Type:       &ast.Int64Type{},
			},
			Value: 4,
		},
	)

	Expect(activations.Find("a").Value).To(Equal(5))
	Expect(activations.Find("b").Value).To(Equal(3))
	Expect(activations.Find("c").Value).To(Equal(4))

	activations.Pop()

	Expect(activations.Find("a").Value).To(Equal(2))
	Expect(activations.Find("b").Value).To(Equal(3))
	Expect(activations.Find("c")).To(BeNil())

	activations.Pop()

	Expect(activations.Find("a").Value).To(Equal(1))
	Expect(activations.Find("b")).To(BeNil())
	Expect(activations.Find("c")).To(BeNil())

	activations.Pop()

	Expect(activations.Find("a")).To(BeNil())
	Expect(activations.Find("b")).To(BeNil())
	Expect(activations.Find("c")).To(BeNil())

}
