package generate_authorization_fixes

import (
	"errors"
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	. "github.com/onflow/cadence/runtime/tests/checker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newEntitlementSetAuthorizationFromEntitlementTypes(
	entitlements []*sema.EntitlementType,
	kind sema.EntitlementSetKind,
) interpreter.EntitlementSetAuthorization {
	return interpreter.NewEntitlementSetAuthorization(
		nil,
		func() []common.TypeID {
			typeIDs := make([]common.TypeID, len(entitlements))
			for i, e := range entitlements {
				typeIDs[i] = e.ID()
			}
			return typeIDs
		},
		len(entitlements),
		kind,
	)
}

func TestFindMinimalAuthorization(t *testing.T) {

	t.Parallel()

	checker, err := ParseAndCheck(t, `
      entitlement E1
      entitlement E2
      entitlement E3

      struct S {
          access(all) fun accessAll() {}
          access(self) fun accessSelf() {}
          access(contract) fun accessContract() {}
          access(account) fun accessAccount() {}

          access(E1) fun accessE1() {}
          access(E2) fun accessE2() {}
          access(E1, E2) fun accessE1AndE2() {}
          access(E1 | E2) fun accessE1OrE2() {}
      }
	`)
	require.NoError(t, err)

	ty := RequireGlobalType(t, checker.Elaboration, "S")

	e1 := RequireGlobalType(t, checker.Elaboration, "E1").(*sema.EntitlementType)
	e2 := RequireGlobalType(t, checker.Elaboration, "E2").(*sema.EntitlementType)

	t.Run("accessAll, accessSelf, accessContract, accessAccount", func(t *testing.T) {
		t.Parallel()

		authorization, unresolved := findMinimalAuthorization(
			ty,
			map[string]struct{}{
				"accessAll":      {},
				"accessSelf":     {},
				"accessContract": {},
				"accessAccount":  {},
				"undefined":      {},
			},
		)
		assert.Equal(t,
			interpreter.UnauthorizedAccess,
			authorization,
		)
		assert.Equal(t,
			map[string]error{
				"accessSelf":     errors.New("member is inaccessible (access(self))"),
				"accessContract": errors.New("member is inaccessible (access(contract))"),
				"accessAccount":  errors.New("member is inaccessible (access(account))"),
				"undefined":      errors.New("member does not exist"),
			},
			unresolved,
		)
	})

	t.Run("accessE1", func(t *testing.T) {
		t.Parallel()

		authorization, unresolved := findMinimalAuthorization(
			ty,
			map[string]struct{}{
				"accessE1":  {},
				"undefined": {},
			},
		)
		assert.Equal(t,
			newEntitlementSetAuthorizationFromEntitlementTypes(
				[]*sema.EntitlementType{
					e1,
				},
				sema.Conjunction,
			),
			authorization,
		)
		assert.Equal(t,
			map[string]error{
				"undefined": errors.New("member does not exist"),
			},
			unresolved,
		)
	})

	t.Run("accessE1, accessE2", func(t *testing.T) {
		t.Parallel()

		authorization, unresolved := findMinimalAuthorization(
			ty,
			map[string]struct{}{
				"accessE1":  {},
				"accessE2":  {},
				"undefined": {},
			},
		)
		assert.Equal(t,
			newEntitlementSetAuthorizationFromEntitlementTypes(
				[]*sema.EntitlementType{
					e1, e2,
				},
				sema.Conjunction,
			),
			authorization,
		)
		assert.Equal(t,
			map[string]error{
				"undefined": errors.New("member does not exist"),
			},
			unresolved,
		)
	})

	t.Run("accessE1AndE2", func(t *testing.T) {
		t.Parallel()

		authorization, unresolved := findMinimalAuthorization(
			ty,
			map[string]struct{}{
				"accessE1AndE2": {},
				"undefined":     {},
			},
		)
		assert.Equal(t,
			newEntitlementSetAuthorizationFromEntitlementTypes(
				[]*sema.EntitlementType{
					e1, e2,
				},
				sema.Conjunction,
			),
			authorization,
		)
		assert.Equal(t,
			map[string]error{
				"undefined": errors.New("member does not exist"),
			},
			unresolved,
		)
	})

	t.Run("accessE1OrE2", func(t *testing.T) {
		t.Parallel()

		authorization, unresolved := findMinimalAuthorization(
			ty,
			map[string]struct{}{
				"accessE1OrE2": {},
				"undefined":    {},
			},
		)
		assert.Equal(t,
			newEntitlementSetAuthorizationFromEntitlementTypes(
				[]*sema.EntitlementType{
					e1,
				},
				sema.Conjunction,
			),
			authorization,
		)
		assert.Equal(t,
			map[string]error{
				"undefined": errors.New("member does not exist"),
			},
			unresolved,
		)
	})

	t.Run("accessE1OrE2, accessE1AndE2", func(t *testing.T) {
		t.Parallel()

		authorization, unresolved := findMinimalAuthorization(
			ty,
			map[string]struct{}{
				"accessE1OrE2":  {},
				"accessE1AndE2": {},
				"undefined":     {},
			},
		)
		assert.Equal(t,
			newEntitlementSetAuthorizationFromEntitlementTypes(
				[]*sema.EntitlementType{
					e1, e2,
				},
				sema.Conjunction,
			),
			authorization,
		)
		assert.Equal(t,
			map[string]error{
				"undefined": errors.New("member does not exist"),
			},
			unresolved,
		)
	})
}
