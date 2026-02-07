package filter

import "github.com/onflow/flow-go/model/flow"

type Matcher interface {
	Match(event *flow.Event) (bool, error)
}
