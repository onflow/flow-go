package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckCastingIntLiteralToInt8(t *testing.T) {

	checker, err := ParseAndCheck(t, `
      let x = 1 as Int8
    `)

	require.NoError(t, err)

	assert.Equal(t,
		&sema.Int8Type{},
		checker.GlobalValues["x"].Type,
	)

	assert.NotEmpty(t, checker.Elaboration.CastingTargetTypes)
}

func TestCheckInvalidCastingIntLiteralToString(t *testing.T) {

	_, err := ParseAndCheck(t, `
      let x = 1 as String
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
}

func TestCheckCastingIntLiteralToAnyStruct(t *testing.T) {

	checker, err := ParseAndCheck(t, `
      let x = 1 as AnyStruct
    `)

	require.NoError(t, err)

	assert.Equal(t,
		&sema.AnyStructType{},
		checker.GlobalValues["x"].Type,
	)

	assert.NotEmpty(t, checker.Elaboration.CastingTargetTypes)
}

func TestCheckCastingResourceToAnyResource(t *testing.T) {

	checker, err := ParseAndCheck(t, `
      resource R {}

      fun test() {
          let r <- create R()
          let x <- r as @AnyResource
          destroy x
      }
    `)

	require.NoError(t, err)

	assert.NotEmpty(t, checker.Elaboration.CastingTargetTypes)
}

func TestCheckCastingArrayLiteral(t *testing.T) {

	_, err := ParseAndCheck(t, `
      fun zipOf3(a: [AnyStruct; 3], b: [Int; 3]): [[AnyStruct; 2]; 3] {
          return [
              [a[0], b[0]] as [AnyStruct; 2],
              [a[1], b[1]] as [AnyStruct; 2],
              [a[2], b[2]] as [AnyStruct; 2]
          ]
      }
    `)

	require.NoError(t, err)
}

func TestCheckCastStaticResourceType(t *testing.T) {

	// Supertype: Restricted resource

	t.Run("restricted resource -> restricted resource: fewer restrictions", func(t *testing.T) {

		checker, err := ParseAndCheckStorage(t, `
              resource interface I1 {}

              resource interface I2 {}

	          resource R: I1, I2 {}

	          let r: @R{I1, I2} <- create R()
              let r2 <- r as @R{I2}
	        `,
		)

		require.NoError(t, err)

		r2Type := checker.GlobalValues["r2"].Type

		require.IsType(t,
			&sema.RestrictedResourceType{},
			r2Type,
		)
	})

	t.Run("restricted resource -> restricted resource: more restrictions", func(t *testing.T) {

		checker, err := ParseAndCheckStorage(t, `
              resource interface I1 {}

              resource interface I2 {}

	          resource R: I1, I2 {}

	          let r: @R{I1} <- create R()
              let r2 <- r as @R{I1, I2}
	        `,
		)

		require.NoError(t, err)

		r2Type := checker.GlobalValues["r2"].Type

		require.IsType(t,
			&sema.RestrictedResourceType{},
			r2Type,
		)
	})

	t.Run("restricted resource -> restricted resource: different resource", func(t *testing.T) {

		_, err := ParseAndCheckStorage(t, `
              resource interface I {}

	          resource R1: I {}

	          resource R2: I {}

	          let r: @R1{I} <- create R1()
              let r2 <- r as @R2{I}
	        `,
		)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	})

	t.Run("unrestricted resource -> restricted resource: same resource", func(t *testing.T) {

		checker, err := ParseAndCheckStorage(t, `
	         resource interface I {}

	         resource R: I {}

	         let r: @R <- create R()
	         let r2 <- r as @R{I}
	       `,
		)

		require.NoError(t, err)

		r2Type := checker.GlobalValues["r2"].Type

		require.IsType(t,
			&sema.RestrictedResourceType{},
			r2Type,
		)
	})

	t.Run("unrestricted resource -> restricted resource: different resource", func(t *testing.T) {

		_, err := ParseAndCheckStorage(t, `
	         resource interface I {}

	         resource R1: I {}

             resource R2: I {}

	         let r: @R1 <- create R1()
	         let r2 <- r as @R2{I}
	       `,
		)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	})

	t.Run("resource interface -> conforming restricted resource", func(t *testing.T) {

		_, err := ParseAndCheckStorage(t, `
	        resource interface RI {}

	        resource R: RI {}

	        let r: @RI <- create R()
	        let r2 <- r as @R{RI}
	      `,
		)

		// NOTE: static cast not allowed, only dynamic

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	})

	// Supertype: Resource (unrestricted)

	t.Run("restricted resource -> unrestricted resource", func(t *testing.T) {

		checker, err := ParseAndCheckStorage(t, `
	         resource interface I {}

	         resource R: I {}

	         let r: @R{I} <- create R()
	         let r2 <- r as @R
	       `,
		)

		require.NoError(t, err)

		r2Type := checker.GlobalValues["r2"].Type

		require.IsType(t,
			&sema.CompositeType{},
			r2Type,
		)
	})

	t.Run("resource interface -> conforming resource", func(t *testing.T) {

		_, err := ParseAndCheckStorage(t, `
	         resource interface RI {}

	         resource R: RI {}

	         let r: @RI <- create R()
	         let r2 <- r as @R
	       `,
		)

		// NOTE: static cast not allowed, only dynamic

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	})

	// Supertype: Resource interface

	t.Run("resource -> non-conformance resource interface", func(t *testing.T) {

		_, err := ParseAndCheckStorage(t, `
	         resource interface RI {}

	         // NOTE: R does not conform to RI
	         resource R {}

	         let r: @R <- create R()
	         let r2 <- r as @RI
	       `,
		)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	})

	t.Run("resource -> conforming resource interface", func(t *testing.T) {

		_, err := ParseAndCheckStorage(t, `
	         resource interface RI {}

	         resource R: RI {}

	         let r: @R <- create R()
	         let r2 <- r as @RI
	       `,
		)

		require.NoError(t, err)
	})

	t.Run("restricted resource -> conforming resource interface", func(t *testing.T) {

		checker, err := ParseAndCheckStorage(t, `
	         resource interface I {}

	         resource R: I {}

	         let r: @R{I} <- create R()
	         let r2 <- r as @I
	       `,
		)

		require.NoError(t, err)

		r2Type := checker.GlobalValues["r2"].Type

		require.IsType(t,
			&sema.InterfaceType{},
			r2Type,
		)
	})
}

