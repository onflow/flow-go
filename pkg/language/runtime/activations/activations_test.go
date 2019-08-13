package activations

import (
	. "github.com/onsi/gomega"
	"testing"
)

func TestActivations(t *testing.T) {
	RegisterTestingT(t)

	activations := &Activations{}

	activations.Set("a", 1)

	Expect(activations.Find("a")).To(Equal(1))
	Expect(activations.Find("b")).To(BeNil())

	activations.PushCurrent()

	activations.Set("a", 2)
	activations.Set("b", 3)

	Expect(activations.Find("a")).To(Equal(2))
	Expect(activations.Find("b")).To(Equal(3))
	Expect(activations.Find("c")).To(BeNil())

	activations.PushCurrent()

	activations.Set("a", 5)
	activations.Set("c", 4)

	Expect(activations.Find("a")).To(Equal(5))
	Expect(activations.Find("b")).To(Equal(3))
	Expect(activations.Find("c")).To(Equal(4))

	activations.Pop()

	Expect(activations.Find("a")).To(Equal(2))
	Expect(activations.Find("b")).To(Equal(3))
	Expect(activations.Find("c")).To(BeNil())

	activations.Pop()

	Expect(activations.Find("a")).To(Equal(1))
	Expect(activations.Find("b")).To(BeNil())
	Expect(activations.Find("c")).To(BeNil())

	activations.Pop()

	Expect(activations.Find("a")).To(BeNil())
	Expect(activations.Find("b")).To(BeNil())
	Expect(activations.Find("c")).To(BeNil())

}
