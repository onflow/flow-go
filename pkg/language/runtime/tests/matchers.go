package tests

import (
	"fmt"

	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/sema"
)

// matchers

type originMatcher struct {
	startPos sema.Position
	endPos   sema.Position
	kind     common.DeclarationKind
}

func (matcher *originMatcher) Match(actual interface{}) (success bool, err error) {
	origin, ok := actual.(sema.Origin)
	if !ok {
		return false, fmt.Errorf("originMatcher matcher expects a sema.Origin")
	}

	if origin.StartPos != matcher.startPos {
		return false, fmt.Errorf(
			"Expected\n\t%#v\nto have start position\n\t%#v",
			actual,
			matcher.startPos,
		)
	}

	if origin.EndPos != matcher.endPos {
		return false, fmt.Errorf(
			"Expected\n\t%#v\nto have start position\n\t%#v",
			actual,
			matcher.endPos,
		)
	}

	if origin.Variable.Kind != matcher.kind {
		return false, fmt.Errorf(
			"Expected\n\t%#v\nto have kind\n\t%s",
			actual,
			matcher.kind.Name(),
		)
	}

	return true, nil
}

func (matcher *originMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf(
		"Expected\n\t%#v\nto match origin with\n\t%s @ %#v",
		actual,
		matcher.kind.Name(), matcher.startPos,
	)
}

func (matcher *originMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf(
		"Expected\n\t%#v\nto not match origin with\n\t%s @ %#v",
		actual,
		matcher.kind.Name(), matcher.startPos,
	)
}
