package interpreter

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
)

func TestInterpreterOptionalBoxing(t *testing.T) {

	inter, err := NewInterpreter(nil)
	assert.Nil(t, err)

	value, newType := inter.boxOptional(
		BoolValue(true),
		&sema.BoolType{},
		&sema.OptionalType{Type: &sema.BoolType{}},
	)
	assert.Equal(t, value, SomeValue{BoolValue(true)})
	assert.Equal(t, newType, &sema.OptionalType{Type: &sema.BoolType{}})

	value, newType = inter.boxOptional(
		SomeValue{BoolValue(true)},
		&sema.OptionalType{Type: &sema.BoolType{}},
		&sema.OptionalType{Type: &sema.BoolType{}},
	)
	assert.Equal(t, value, SomeValue{BoolValue(true)})
	assert.Equal(t, newType, &sema.OptionalType{Type: &sema.BoolType{}})

	value, newType = inter.boxOptional(
		SomeValue{BoolValue(true)},
		&sema.OptionalType{Type: &sema.BoolType{}},
		&sema.OptionalType{Type: &sema.OptionalType{Type: &sema.BoolType{}}},
	)
	assert.Equal(t, value, SomeValue{SomeValue{BoolValue(true)}})
	assert.Equal(t, newType, &sema.OptionalType{Type: &sema.OptionalType{Type: &sema.BoolType{}}})

	// NOTE:
	value, newType = inter.boxOptional(
		NilValue{},
		&sema.OptionalType{Type: &sema.NeverType{}},
		&sema.OptionalType{Type: &sema.OptionalType{Type: &sema.BoolType{}}},
	)
	assert.Equal(t, value, NilValue{})
	assert.Equal(t, newType, &sema.OptionalType{Type: &sema.NeverType{}})

	// NOTE:
	value, newType = inter.boxOptional(
		SomeValue{NilValue{}},
		&sema.OptionalType{Type: &sema.OptionalType{Type: &sema.NeverType{}}},
		&sema.OptionalType{Type: &sema.OptionalType{Type: &sema.BoolType{}}},
	)
	assert.Equal(t, value, NilValue{})
	assert.Equal(t, newType, &sema.OptionalType{Type: &sema.NeverType{}})
}

func TestInterpreterAnyBoxing(t *testing.T) {

	inter, err := NewInterpreter(nil)
	assert.Nil(t, err)

	assert.Equal(t,
		inter.boxAny(
			BoolValue(true),
			&sema.BoolType{},
			&sema.AnyType{},
		), AnyValue{
			Value: BoolValue(true),
			Type:  &sema.BoolType{},
		},
	)

	assert.Equal(t,
		inter.boxAny(
			SomeValue{BoolValue(true)},
			&sema.OptionalType{Type: &sema.BoolType{}},
			&sema.OptionalType{Type: &sema.AnyType{}},
		),

		SomeValue{
			Value: AnyValue{
				Value: BoolValue(true),
				Type:  &sema.BoolType{},
			},
		},
	)

	// don't box already boxed
	assert.Equal(t,
		inter.boxAny(
			AnyValue{
				Value: BoolValue(true),
				Type:  &sema.BoolType{},
			},
			&sema.AnyType{},
			&sema.AnyType{},
		),
		AnyValue{
			Value: BoolValue(true),
			Type:  &sema.BoolType{},
		},
	)

}

func TestInterpreterBoxing(t *testing.T) {

	inter, err := NewInterpreter(nil)
	assert.Nil(t, err)

	assert.Equal(t,
		inter.box(
			BoolValue(true),
			&sema.BoolType{},
			&sema.OptionalType{Type: &sema.AnyType{}},
		),
		SomeValue{
			Value: AnyValue{
				Value: BoolValue(true),
				Type:  &sema.BoolType{},
			},
		},
	)

	assert.Equal(t,
		inter.box(
			SomeValue{BoolValue(true)},
			&sema.OptionalType{Type: &sema.BoolType{}},
			&sema.OptionalType{Type: &sema.AnyType{}},
		),
		SomeValue{
			Value: AnyValue{
				Value: BoolValue(true),
				Type:  &sema.BoolType{},
			},
		},
	)
}
