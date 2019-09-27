package interpreter

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
)

func TestInterpreterOptionalBoxing(t *testing.T) {
	RegisterTestingT(t)

	inter, err := NewInterpreter(nil, nil)
	Expect(err).
		To(Not(HaveOccurred()))

	value, newType := inter.boxOptional(
		BoolValue(true),
		&sema.BoolType{},
		&sema.OptionalType{Type: &sema.BoolType{}},
	)
	Expect(value).
		To(Equal(SomeValue{BoolValue(true)}))
	Expect(newType).
		To(Equal(&sema.OptionalType{Type: &sema.BoolType{}}))

	value, newType = inter.boxOptional(
		SomeValue{BoolValue(true)},
		&sema.OptionalType{Type: &sema.BoolType{}},
		&sema.OptionalType{Type: &sema.BoolType{}},
	)
	Expect(value).
		To(Equal(SomeValue{BoolValue(true)}))
	Expect(newType).
		To(Equal(&sema.OptionalType{Type: &sema.BoolType{}}))

	value, newType = inter.boxOptional(
		SomeValue{BoolValue(true)},
		&sema.OptionalType{Type: &sema.BoolType{}},
		&sema.OptionalType{Type: &sema.OptionalType{Type: &sema.BoolType{}}},
	)
	Expect(value).
		To(Equal(SomeValue{SomeValue{BoolValue(true)}}))
	Expect(newType).
		To(Equal(&sema.OptionalType{Type: &sema.OptionalType{Type: &sema.BoolType{}}}))

	// NOTE:
	value, newType = inter.boxOptional(
		NilValue{},
		&sema.OptionalType{Type: &sema.NeverType{}},
		&sema.OptionalType{Type: &sema.OptionalType{Type: &sema.BoolType{}}},
	)
	Expect(value).
		To(Equal(NilValue{}))
	Expect(newType).
		To(Equal(&sema.OptionalType{Type: &sema.NeverType{}}))

	// NOTE:
	value, newType = inter.boxOptional(
		SomeValue{NilValue{}},
		&sema.OptionalType{Type: &sema.OptionalType{Type: &sema.NeverType{}}},
		&sema.OptionalType{Type: &sema.OptionalType{Type: &sema.BoolType{}}},
	)
	Expect(value).
		To(Equal(NilValue{}))
	Expect(newType).
		To(Equal(&sema.OptionalType{Type: &sema.NeverType{}}))
}

func TestInterpreterAnyBoxing(t *testing.T) {
	RegisterTestingT(t)

	inter, err := NewInterpreter(nil, nil)
	Expect(err).
		To(Not(HaveOccurred()))

	Expect(inter.boxAny(
		BoolValue(true),
		&sema.BoolType{},
		&sema.AnyType{},
	)).
		To(Equal(AnyValue{
			Value: BoolValue(true),
			Type:  &sema.BoolType{},
		}))

	Expect(inter.boxAny(
		SomeValue{BoolValue(true)},
		&sema.OptionalType{Type: &sema.BoolType{}},
		&sema.OptionalType{Type: &sema.AnyType{}},
	)).
		To(Equal(
			SomeValue{
				Value: AnyValue{
					Value: BoolValue(true),
					Type:  &sema.BoolType{},
				},
			}))

	// don't box already boxed
	Expect(inter.boxAny(
		AnyValue{
			Value: BoolValue(true),
			Type:  &sema.BoolType{},
		},
		&sema.AnyType{},
		&sema.AnyType{},
	)).
		To(Equal(
			AnyValue{
				Value: BoolValue(true),
				Type:  &sema.BoolType{},
			},
		))
}

func TestInterpreterBoxing(t *testing.T) {
	RegisterTestingT(t)

	inter, err := NewInterpreter(nil, nil)
	Expect(err).
		To(Not(HaveOccurred()))

	Expect(inter.box(
		BoolValue(true),
		&sema.BoolType{},
		&sema.OptionalType{Type: &sema.AnyType{}},
	)).
		To(Equal(
			SomeValue{
				Value: AnyValue{
					Value: BoolValue(true),
					Type:  &sema.BoolType{},
				},
			}))

	Expect(inter.box(
		SomeValue{BoolValue(true)},
		&sema.OptionalType{Type: &sema.BoolType{}},
		&sema.OptionalType{Type: &sema.AnyType{}},
	)).
		To(Equal(
			SomeValue{
				Value: AnyValue{
					Value: BoolValue(true),
					Type:  &sema.BoolType{},
				},
			}))
}
