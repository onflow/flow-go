package interpreter

import (
	. "github.com/onsi/gomega"
	"testing"
)

func TestActivations(t *testing.T) {
	gomega := NewWithT(t)

	activations := &Activations{}

	activations.Set("a", 1)

	gomega.Expect(activations.Find("a")).To(Equal(1))
	gomega.Expect(activations.Find("b")).To(BeNil())

	activations.Push()

	activations.Set("a", 2)
	activations.Set("b", 3)

	gomega.Expect(activations.Find("a")).To(Equal(2))
	gomega.Expect(activations.Find("b")).To(Equal(3))
	gomega.Expect(activations.Find("c")).To(BeNil())

	activations.Push()

	activations.Set("a", 5)
	activations.Set("c", 4)

	gomega.Expect(activations.Find("a")).To(Equal(5))
	gomega.Expect(activations.Find("b")).To(Equal(3))
	gomega.Expect(activations.Find("c")).To(Equal(4))

	activations.Pop()

	gomega.Expect(activations.Find("a")).To(Equal(2))
	gomega.Expect(activations.Find("b")).To(Equal(3))
	gomega.Expect(activations.Find("c")).To(BeNil())

	activations.Pop()

	gomega.Expect(activations.Find("a")).To(Equal(1))
	gomega.Expect(activations.Find("b")).To(BeNil())
	gomega.Expect(activations.Find("c")).To(BeNil())

	activations.Pop()
	gomega.Expect(activations.Find("a")).To(BeNil())
	gomega.Expect(activations.Find("b")).To(BeNil())
	gomega.Expect(activations.Find("c")).To(BeNil())

}
