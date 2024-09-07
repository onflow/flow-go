package generate_authorization_fixes

import (
	"sort"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/common/orderedmap"
	"github.com/onflow/cadence/runtime/sema"
	"golang.org/x/exp/slices"
)

// TODO: handle unsatisfiable case
func findMinimalEntitlementSet(
	ty sema.Type,
	neededMethods []string,
) []*sema.EntitlementType {

	entitlementsToMethod := orderedmap.OrderedMap[*sema.EntitlementType, []string]{}

	// NOTE: GetMembers might return members that are actually not accessible, for DX purposes.
	// We need to resolve the members and filter out the inaccessible members,
	// using the error reported when resolving

	memberResolvers := ty.GetMembers()

	for memberName, memberResolver := range memberResolvers {
		var resolveErr error
		member := memberResolver.Resolve(nil, memberName, ast.EmptyRange, func(err error) {
			resolveErr = err
		})
		if resolveErr != nil {
			continue
		}

		if member.DeclarationKind != common.DeclarationKindFunction {
			continue
		}

		entitlementSetAccess, ok := member.Access.(sema.EntitlementSetAccess)
		if !ok {
			continue
		}

		// TODO: handle SetKind
		entitlementSetAccess.Entitlements.Foreach(func(entitlementType *sema.EntitlementType, _ struct{}) {
			methodsForEntitlement, _ := entitlementsToMethod.Get(entitlementType)
			methodsForEntitlement = append(methodsForEntitlement, memberName)
			entitlementsToMethod.Set(entitlementType, methodsForEntitlement)
		})
	}

	var entitlements []*sema.EntitlementType
	entitlementsToMethod.Foreach(func(entitlement *sema.EntitlementType, methodsWithEntitlement []string) {
		if isSubset(methodsWithEntitlement, neededMethods) {
			entitlements = append(entitlements, entitlement)
		}
	})

	sort.Slice(
		entitlements,
		func(i, j int) bool {
			return entitlements[i].ID() < entitlements[j].ID()
		},
	)

	return entitlements
}

func isSubset(subSet, superSet []string) bool {
	for _, element := range subSet {
		if !slices.Contains(superSet, element) {
			return false
		}
	}

	return true
}
