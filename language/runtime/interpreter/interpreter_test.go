package interpreter

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
)

func TestInterpreterOptionalBoxing(t *testing.T) {

	inter, err := NewInterpreter(nil)
	assert.Nil(t, err)

	value, newType := inter.boxOptional(
		BoolValue(true),
		&sema.BoolType{},
		&sema.OptionalType{Type: &sema.BoolType{}},
	)
	assert.Equal(t,
		&SomeValue{BoolValue(true)},
		value,
	)
	assert.Equal(t,
		&sema.OptionalType{Type: &sema.BoolType{}},
		newType,
	)

	value, newType = inter.boxOptional(
		&SomeValue{BoolValue(true)},
		&sema.OptionalType{Type: &sema.BoolType{}},
		&sema.OptionalType{Type: &sema.BoolType{}},
	)
	assert.Equal(t,
		&SomeValue{BoolValue(true)},
		value,
	)
	assert.Equal(t,
		&sema.OptionalType{Type: &sema.BoolType{}},
		newType,
	)

	value, newType = inter.boxOptional(
		&SomeValue{BoolValue(true)},
		&sema.OptionalType{Type: &sema.BoolType{}},
		&sema.OptionalType{Type: &sema.OptionalType{Type: &sema.BoolType{}}},
	)
	assert.Equal(t,
		&SomeValue{&SomeValue{BoolValue(true)}},
		value,
	)
	assert.Equal(t,
		&sema.OptionalType{Type: &sema.OptionalType{Type: &sema.BoolType{}}},
		newType,
	)

	// NOTE:
	value, newType = inter.boxOptional(
		NilValue{},
		&sema.OptionalType{Type: &sema.NeverType{}},
		&sema.OptionalType{Type: &sema.OptionalType{Type: &sema.BoolType{}}},
	)
	assert.Equal(t,
		NilValue{},
		value,
	)
	assert.Equal(t,
		&sema.OptionalType{Type: &sema.NeverType{}},
		newType,
	)

	// NOTE:
	value, newType = inter.boxOptional(
		&SomeValue{NilValue{}},
		&sema.OptionalType{Type: &sema.OptionalType{Type: &sema.NeverType{}}},
		&sema.OptionalType{Type: &sema.OptionalType{Type: &sema.BoolType{}}},
	)
	assert.Equal(t,
		NilValue{},
		value,
	)
	assert.Equal(t,
		&sema.OptionalType{Type: &sema.NeverType{}},
		newType,
	)
}

func TestInterpreterAnyBoxing(t *testing.T) {

	inter, err := NewInterpreter(nil)
	assert.Nil(t, err)

	assert.Equal(t,
		AnyValue{
			Value: BoolValue(true),
			Type:  &sema.BoolType{},
		},
		inter.boxAny(
			BoolValue(true),
			&sema.BoolType{},
			&sema.AnyType{},
		),
	)

	assert.Equal(t,
		&SomeValue{
			Value: AnyValue{
				Value: BoolValue(true),
				Type:  &sema.BoolType{},
			},
		},
		inter.boxAny(
			&SomeValue{BoolValue(true)},
			&sema.OptionalType{Type: &sema.BoolType{}},
			&sema.OptionalType{Type: &sema.AnyType{}},
		),
	)

	// don't box already boxed
	assert.Equal(t,
		AnyValue{
			Value: BoolValue(true),
			Type:  &sema.BoolType{},
		},
		inter.boxAny(
			AnyValue{
				Value: BoolValue(true),
				Type:  &sema.BoolType{},
			},
			&sema.AnyType{},
			&sema.AnyType{},
		),
	)

}

func TestInterpreterBoxing(t *testing.T) {

	inter, err := NewInterpreter(nil)
	assert.Nil(t, err)

	assert.Equal(t,
		&SomeValue{
			Value: AnyValue{
				Value: BoolValue(true),
				Type:  &sema.BoolType{},
			},
		},
		inter.convertAndBox(
			BoolValue(true),
			&sema.BoolType{},
			&sema.OptionalType{Type: &sema.AnyType{}},
		),
	)

	assert.Equal(t,
		&SomeValue{
			Value: AnyValue{
				Value: BoolValue(true),
				Type:  &sema.BoolType{},
			},
		},
		inter.convertAndBox(
			&SomeValue{BoolValue(true)},
			&sema.OptionalType{Type: &sema.BoolType{}},
			&sema.OptionalType{Type: &sema.AnyType{}},
		),
	)
}
