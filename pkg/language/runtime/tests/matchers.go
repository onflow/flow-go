package tests

import (
	"fmt"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
)

// matchers

type occurrenceMatcher struct {
	startPos        sema.Position
	endPos          sema.Position
	originStartPos  *sema.Position
	originEndPos    *sema.Position
	declarationKind common.DeclarationKind
}

func (matcher *occurrenceMatcher) Match(actual interface{}) (success bool, err error) {
	occurrence, ok := actual.(sema.Occurrence)
	if !ok {
		return false, fmt.Errorf("occurrenceMatcher matcher expects a sema.Occurrence")
	}

	if occurrence.StartPos != matcher.startPos {
		return false, fmt.Errorf(
			"Expected\n\t%#v\nto have start position\n\t%#v",
			actual,
			matcher.startPos,
		)
	}

	if occurrence.EndPos != matcher.endPos {
		return false, fmt.Errorf(
			"Expected\n\t%#v\nto have start position\n\t%#v",
			actual,
			matcher.endPos,
		)
	}

	if occurrence.Origin.DeclarationKind != matcher.declarationKind {
		return false, fmt.Errorf(
			"Expected\n\t%#v\nto have declarationKind\n\t%s",
			actual,
			matcher.declarationKind.Name(),
		)
	}

	if occurrence.Origin.StartPos != nil {
		if occurrence.Origin.StartPos.Line != matcher.originStartPos.Line ||
			occurrence.Origin.StartPos.Column != matcher.originStartPos.Column {
			return false, fmt.Errorf(
				"Expected\n\t%#v\nto have origin start position\n\t%s",
				actual,
				matcher.originStartPos,
			)
		}
	}

	if occurrence.Origin.EndPos != nil {
		if occurrence.Origin.EndPos.Line != matcher.originEndPos.Line &&
			occurrence.Origin.EndPos.Column != matcher.originEndPos.Column {
			return false, fmt.Errorf(
				"Expected\n\t%#v\nto have origin end position\n\t%s",
				actual,
				matcher.originEndPos,
			)
		}
	}

	return true, nil
}

func (matcher *occurrenceMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf(
		"Expected\n\t%#v\nto match occurrence with\n\t%s @ %#v",
		actual,
		matcher.declarationKind.Name(), matcher.startPos,
	)
}

func (matcher *occurrenceMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf(
		"Expected\n\t%#v\nto not match occurrence with\n\t%s @ %#v",
		actual,
		matcher.declarationKind.Name(), matcher.startPos,
	)
}
