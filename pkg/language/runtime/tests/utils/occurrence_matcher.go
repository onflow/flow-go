package utils

import (
	"fmt"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
)

type OccurrenceMatcher struct {
	StartPos        sema.Position
	EndPos          sema.Position
	OriginStartPos  *sema.Position
	OriginEndPos    *sema.Position
	DeclarationKind common.DeclarationKind
}

func (matcher *OccurrenceMatcher) Match(actual interface{}) (success bool, err error) {
	occurrence, ok := actual.(sema.Occurrence)
	if !ok {
		return false, fmt.Errorf("OccurrenceMatcher matcher expects a sema.Occurrence")
	}

	if occurrence.StartPos != matcher.StartPos {
		return false, fmt.Errorf(
			"Expected\n\t%#v\nto have start position\n\t%#v",
			actual,
			matcher.StartPos,
		)
	}

	if occurrence.EndPos != matcher.EndPos {
		return false, fmt.Errorf(
			"Expected\n\t%#v\nto have start position\n\t%#v",
			actual,
			matcher.EndPos,
		)
	}

	if occurrence.Origin.DeclarationKind != matcher.DeclarationKind {
		return false, fmt.Errorf(
			"Expected\n\t%#v\nto have declarationKind\n\t%s",
			actual,
			matcher.DeclarationKind.Name(),
		)
	}

	if occurrence.Origin.StartPos != nil {
		if occurrence.Origin.StartPos.Line != matcher.OriginStartPos.Line ||
			occurrence.Origin.StartPos.Column != matcher.OriginStartPos.Column {
			return false, fmt.Errorf(
				"Expected\n\t%#v\nto have origin start position\n\t%s",
				actual,
				matcher.OriginStartPos,
			)
		}
	}

	if occurrence.Origin.EndPos != nil {
		if occurrence.Origin.EndPos.Line != matcher.OriginEndPos.Line &&
			occurrence.Origin.EndPos.Column != matcher.OriginEndPos.Column {
			return false, fmt.Errorf(
				"Expected\n\t%#v\nto have origin end position\n\t%s",
				actual,
				matcher.OriginEndPos,
			)
		}
	}

	return true, nil
}

func (matcher *OccurrenceMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf(
		"Expected\n\t%#v\nto match occurrence with\n\t%s @ %#v",
		actual,
		matcher.DeclarationKind.Name(), matcher.StartPos,
	)
}

func (matcher *OccurrenceMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf(
		"Expected\n\t%#v\nto not match occurrence with\n\t%s @ %#v",
		actual,
		matcher.DeclarationKind.Name(), matcher.StartPos,
	)
}
