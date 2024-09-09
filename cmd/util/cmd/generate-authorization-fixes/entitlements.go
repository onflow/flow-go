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

	// Iterate over the members of the type, and check if they are accessible.
	// This constructs the set of entitlements needed to access the members.
	// If a member is accessible, the entitlements needed to access it are added to the entitlements set.
	// If a member is not accessible, it is added to the unresolved members map.

	// NOTE: GetMembers might return members that are actually not accessible, for DX purposes.
	// We need to resolve the members and filter out the inaccessible members,
	// using the error reported when resolving

	memberResolvers := ty.GetMembers()

	sortedMemberNames := make([]string, 0, len(memberResolvers))
	for memberName := range memberResolvers {
		sortedMemberNames = append(sortedMemberNames, memberName)
	}
	sort.Strings(sortedMemberNames)

	for _, memberName := range sortedMemberNames {
		if _, ok := neededMembers[memberName]; !ok {
			continue
		}

		memberResolver := memberResolvers[memberName]

		var resolveErr error
		member := memberResolver.Resolve(nil, memberName, ast.EmptyRange, func(err error) {
			resolveErr = err
		})
		if resolveErr != nil {
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

	// Check if all needed members were resolved

	for memberName := range neededMembers {
		if _, ok := unresolvedMembers[memberName]; ok {
			continue
		}
		unresolvedMembers[memberName] = errors.New("member does not exist")
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
