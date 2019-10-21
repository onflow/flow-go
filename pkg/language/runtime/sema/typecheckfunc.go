package sema

type TypeCheckFunc = func() Type

func wrapTypeCheck(check TypeCheckFunc, outerChecks ...func(TypeCheckFunc) Type) TypeCheckFunc {
	for _, outerCheck := range outerChecks {
		innerCheck := check
		currentOuterCheck := outerCheck
		check = func() Type {
			return currentOuterCheck(innerCheck)
		}
	}

	return check
}
