package generate_authorization_fixes

import (
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/sema"
)

func getAccessibleMembers(ty sema.Type) []string {
	// NOTE: GetMembers might return members that are actually not accessible, for DX purposes.
	// We need to resolve the members and filter out the inaccessible members,
	// using the error reported when resolving

	memberResolvers := ty.GetMembers()

	accessibleMembers := make([]string, 0, len(memberResolvers))

	for memberName, memberResolver := range memberResolvers {
		var resolveErr error
		memberResolver.Resolve(nil, memberName, ast.EmptyRange, func(err error) {
			resolveErr = err
		})
		if resolveErr != nil {
			continue
		}
		accessibleMembers = append(accessibleMembers, memberName)
	}

	return accessibleMembers
}
