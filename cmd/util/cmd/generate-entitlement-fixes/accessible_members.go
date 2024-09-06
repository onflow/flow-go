package generate_entitlement_fixes

import (
	"fmt"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/interpreter"
)

func getAccessibleMembers(
	inter *interpreter.Interpreter,
	staticType interpreter.StaticType,
) (
	accessibleMembers []string,
	err error,
) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	semaType, err := inter.ConvertStaticToSemaType(staticType)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to convert static type %s to semantic type: %w",
			staticType.ID(),
			err,
		)
	}
	if semaType == nil {
		return nil, fmt.Errorf(
			"failed to convert static type %s to semantic type",
			staticType.ID(),
		)
	}

	// NOTE: RestrictedType.GetMembers returns *all* members,
	// including those that are not accessible, for DX purposes.
	// We need to resolve the members and filter out the inaccessible members,
	// using the error reported when resolving

	memberResolvers := semaType.GetMembers()

	accessibleMembers = make([]string, 0, len(memberResolvers))

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

	return accessibleMembers, nil
}
