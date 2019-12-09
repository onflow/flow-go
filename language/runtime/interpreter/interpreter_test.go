package interpreter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
)

func TestInterpreterOptionalBoxing(t *testing.T) {

	inter, err := NewInterpreter(nil)
	require.NoError(t, err)

	t.Run("", func(t *testing.T) {
		value, newType := inter.boxOptional(
			BoolValue(true),
			&sema.BoolType{},
			&sema.OptionalType{Type: &sema.BoolType{}},
		)
		assert.Equal(t,
			NewSomeValueOwningNonCopying(BoolValue(true)),
			value,
		)
		assert.Equal(t,
			&sema.OptionalType{Type: &sema.BoolType{}},
			newType,
		)
	})

	t.Run("", func(t *testing.T) {
		value, newType := inter.boxOptional(
			NewSomeValueOwningNonCopying(BoolValue(true)),
			&sema.OptionalType{Type: &sema.BoolType{}},
			&sema.OptionalType{Type: &sema.BoolType{}},
		)
		assert.Equal(t,
			NewSomeValueOwningNonCopying(BoolValue(true)),
			value,
		)
		assert.Equal(t,
			&sema.OptionalType{Type: &sema.BoolType{}},
			newType,
		)
	})

	t.Run("", func(t *testing.T) {
		value, newType := inter.boxOptional(
			NewSomeValueOwningNonCopying(BoolValue(true)),
			&sema.OptionalType{Type: &sema.BoolType{}},
			&sema.OptionalType{Type: &sema.OptionalType{Type: &sema.BoolType{}}},
		)
		assert.Equal(t,
			NewSomeValueOwningNonCopying(
				NewSomeValueOwningNonCopying(BoolValue(true)),
			),
			value,
		)
		assert.Equal(t,
			&sema.OptionalType{Type: &sema.OptionalType{Type: &sema.BoolType{}}},
			newType,
		)
	})

	t.Run("", func(t *testing.T) {
		// NOTE:
		value, newType := inter.boxOptional(
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
	})

	t.Run("", func(t *testing.T) {
		// NOTE:
		value, newType := inter.boxOptional(
			NewSomeValueOwningNonCopying(NilValue{}),
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
	})
}

func TestInterpreterAnyBoxing(t *testing.T) {

	inter, err := NewInterpreter(nil)
	require.NoError(t, err)

	for _, anyType := range []sema.Type{
		&sema.AnyStructType{},
		&sema.AnyResourceType{},
	} {
		t.Run(anyType.String(), func(t *testing.T) {

			t.Run("", func(t *testing.T) {
				assert.Equal(t,
					NewAnyValueOwningNonCopying(
						BoolValue(true),
						&sema.BoolType{},
					),
					inter.boxAny(
						BoolValue(true),
						&sema.BoolType{},
						anyType,
					),
				)
			})

			t.Run("", func(t *testing.T) {

				assert.Equal(t,
					NewSomeValueOwningNonCopying(
						NewAnyValueOwningNonCopying(
							BoolValue(true),
							&sema.BoolType{},
						),
					),
					inter.boxAny(
						NewSomeValueOwningNonCopying(BoolValue(true)),
						&sema.OptionalType{Type: &sema.BoolType{}},
						&sema.OptionalType{Type: anyType},
					),
				)
			})

			t.Run("", func(t *testing.T) {
				// don't box already boxed
				assert.Equal(t,
					NewAnyValueOwningNonCopying(
						BoolValue(true),
						&sema.BoolType{},
					),
					inter.boxAny(
						NewAnyValueOwningNonCopying(
							BoolValue(true),
							&sema.BoolType{},
						),
						anyType,
						anyType,
					),
				)
			})
		})
	}
}

func TestInterpreterBoxing(t *testing.T) {

	inter, err := NewInterpreter(nil)
	require.NoError(t, err)

	for _, anyType := range []sema.Type{
		&sema.AnyStructType{},
		&sema.AnyResourceType{},
	} {

		t.Run(anyType.String(), func(t *testing.T) {

			t.Run("", func(t *testing.T) {

				assert.Equal(t,
					NewSomeValueOwningNonCopying(
						NewAnyValueOwningNonCopying(
							BoolValue(true),
							&sema.BoolType{},
						),
					),
					inter.convertAndBox(
						BoolValue(true),
						&sema.BoolType{},
						&sema.OptionalType{Type: anyType},
					),
				)

			})

			t.Run("", func(t *testing.T) {

				assert.Equal(t,
					NewSomeValueOwningNonCopying(
						NewAnyValueOwningNonCopying(
							BoolValue(true),
							&sema.BoolType{},
						),
					),
					inter.convertAndBox(
						NewSomeValueOwningNonCopying(BoolValue(true)),
						&sema.OptionalType{Type: &sema.BoolType{}},
						&sema.OptionalType{Type: anyType},
					),
				)
			})
		})
	}
}
