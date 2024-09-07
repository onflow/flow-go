package generate_authorization_fixes

import (
	"testing"

	"github.com/onflow/cadence/runtime/sema"
	. "github.com/onflow/cadence/runtime/tests/checker"
	"github.com/stretchr/testify/require"
)

func TestFindMinimalEntitlementsSet(t *testing.T) {

	t.Parallel()

	checker, err := ParseAndCheck(t, `
      entitlement E1
      entitlement E2
      entitlement E3
      resource interface I1 {
          access(E1) fun method1()
      }
      resource interface I2 {
          access(E1, E2) fun method2()
          access(E2) fun method3()
      }
      resource interface I3 {
          access(E3) fun method4()
      }
      // Assume this is the intersection-type that is left now.
      // It may have some set of entitlements, but we are not interested:
      // We are going to re-derive the minimal set of entitlements needed,
      // depending on what methods to keep.
      var intersectionType: &{I1, I2, I3}?  = nil
	`)
	require.NoError(t, err)

	vValueType := RequireGlobalValue(t, checker.Elaboration, "intersectionType")

	require.IsType(t, &sema.OptionalType{}, vValueType)
	optionalType := vValueType.(*sema.OptionalType)

	require.IsType(t, &sema.ReferenceType{}, optionalType.Type)
	referenceType := optionalType.Type.(*sema.ReferenceType)

	require.IsType(t, &sema.IntersectionType{}, referenceType.Type)
	intersectionType := referenceType.Type.(*sema.IntersectionType)

	e1 := RequireGlobalType(t, checker.Elaboration, "E1").(*sema.EntitlementType)
	e2 := RequireGlobalType(t, checker.Elaboration, "E2").(*sema.EntitlementType)
	e3 := RequireGlobalType(t, checker.Elaboration, "E3").(*sema.EntitlementType)

	t.Run("method1, method2", func(t *testing.T) {
		entitlements := findMinimalEntitlementSet(
			intersectionType,
			[]string{
				"method1",
				"method2",
			},
		)
		require.Equal(t,
			[]*sema.EntitlementType{e1},
			entitlements,
		)
	})

	t.Run("method1, method2, method3", func(t *testing.T) {
		entitlements := findMinimalEntitlementSet(
			intersectionType,
			[]string{
				"method1",
				"method2",
				"method3",
			},
		)
		require.Equal(t,
			[]*sema.EntitlementType{e1, e2},
			entitlements,
		)
	})

	t.Run("method1, method2, method4", func(t *testing.T) {
		entitlements := findMinimalEntitlementSet(
			intersectionType,
			[]string{
				"method1",
				"method2",
				"method4",
			},
		)
		require.Equal(
			t,
			[]*sema.EntitlementType{e1, e3},
			entitlements,
		)
	})

	t.Run("method1, method2, method3, method4", func(t *testing.T) {
		entitlements := findMinimalEntitlementSet(
			intersectionType,
			[]string{
				"method1",
				"method2",
				"method3",
				"method4",
			},
		)
		require.Equal(t,
			[]*sema.EntitlementType{e1, e2, e3},
			entitlements,
		)
	})

	t.Run("method4", func(t *testing.T) {
		entitlements := findMinimalEntitlementSet(
			intersectionType,
			[]string{
				"method4",
			},
		)
		require.Equal(t,
			[]*sema.EntitlementType{e3},
			entitlements,
		)
	})
}
