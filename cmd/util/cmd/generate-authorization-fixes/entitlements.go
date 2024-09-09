package generate_authorization_fixes

import (
	"errors"
	"fmt"
	"sort"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common/orderedmap"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
)

func findMinimalAuthorization(
	ty sema.Type,
	neededMembers map[string]struct{},
) (
	authorization interpreter.Authorization,
	unresolvedMembers map[string]error,
) {
	entitlements := &sema.EntitlementSet{}
	unresolvedMembers = map[string]error{}

	// NOTE: GetMembers might return members that are actually not accessible, for DX purposes.
	// We need to resolve the members and filter out the inaccessible members,
	// using the error reported when resolving

	sortedNeededMembers := make([]string, 0, len(neededMembers))
	for memberName := range neededMembers {
		sortedNeededMembers = append(sortedNeededMembers, memberName)
	}
	sort.Strings(sortedNeededMembers)

	memberResolvers := ty.GetMembers()

	for _, memberName := range sortedNeededMembers {
		memberResolver, ok := memberResolvers[memberName]
		if !ok {
			unresolvedMembers[memberName] = errors.New("member does not exist")
			continue
		}

		var resolveErr error
		member := memberResolver.Resolve(nil, memberName, ast.EmptyRange, func(err error) {
			resolveErr = err
		})
		if resolveErr != nil {
			unresolvedMembers[memberName] = resolveErr
			continue
		}

		switch access := member.Access.(type) {
		case sema.EntitlementSetAccess:
			switch access.SetKind {
			case sema.Conjunction:
				access.Entitlements.Foreach(func(entitlementType *sema.EntitlementType, _ struct{}) {
					entitlements.Add(entitlementType)
				})

			case sema.Disjunction:
				entitlements.AddDisjunction(access.Entitlements)

			default:
				panic(fmt.Errorf("unsupported set kind: %v", access.SetKind))
			}

			delete(neededMembers, memberName)

		case *sema.EntitlementMapAccess:
			unresolvedMembers[memberName] = fmt.Errorf(
				"member requires entitlement map access: %s",
				access.QualifiedKeyword(),
			)

		case sema.PrimitiveAccess:
			if access == sema.PrimitiveAccess(ast.AccessAll) {
				// member is always accessible
				delete(neededMembers, memberName)
			} else {
				unresolvedMembers[memberName] = fmt.Errorf(
					"member is inaccessible (%s)",
					access.QualifiedKeyword(),
				)
			}

		default:
			panic(fmt.Errorf("unsupported access kind: %T", member.Access))
		}
	}

	return entitlementSetMinimalAuthorization(entitlements), unresolvedMembers
}

// entitlementSetMinimalAuthorization returns the minimal authorization required to access the entitlements in the set.
// It is similar to `EntitlementSet.Access()`, but it returns the minimal authorization,
// i.e. does not return a disjunction if there is only one disjunction in the set,
// and only grants one entitlement for each disjunction.
func entitlementSetMinimalAuthorization(s *sema.EntitlementSet) interpreter.Authorization {

	s.Minimize()

	var entitlements *sema.EntitlementOrderedSet
	if s.Entitlements != nil && s.Entitlements.Len() > 0 {
		entitlements = orderedmap.New[sema.EntitlementOrderedSet](s.Entitlements.Len())
		entitlements.SetAll(s.Entitlements)
	}

	if s.Disjunctions != nil && s.Disjunctions.Len() > 0 {
		if entitlements == nil {
			// There are no entitlements, but disjunctions.
			// Allocate a new ordered map for all entitlements in the disjunctions
			// (at minimum there are two entitlements in each disjunction).
			entitlements = orderedmap.New[sema.EntitlementOrderedSet](s.Disjunctions.Len() * 2)
		}

		// Add one entitlement for each of the disjunctions to the entitlements
		s.Disjunctions.Foreach(func(_ string, disjunction *sema.EntitlementOrderedSet) {
			// Only add the first entitlement in the disjunction
			entitlements.Set(disjunction.Oldest().Key, struct{}{})
		})
	}

	if entitlements == nil {
		return interpreter.UnauthorizedAccess
	}

	entitlementTypeIDs := orderedmap.New[sema.TypeIDOrderedSet](entitlements.Len())
	entitlements.Foreach(func(entitlement *sema.EntitlementType, _ struct{}) {
		entitlementTypeIDs.Set(entitlement.ID(), struct{}{})
	})

	return interpreter.EntitlementSetAuthorization{
		Entitlements: entitlementTypeIDs,
		SetKind:      sema.Conjunction,
	}
}
