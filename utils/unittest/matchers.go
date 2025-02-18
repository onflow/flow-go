package unittest

import (
	"github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// MatchEpochExtension matches an epoch extension argument passed to a mocked component.
func MatchEpochExtension(finalView, extensionLen uint64) any {
	return mock.MatchedBy(func(extension flow.EpochExtension) bool {
		return extension.FirstView == finalView+1 && extension.FinalView == finalView+extensionLen
	})
}

// MatchInvalidServiceEventError matches an error that is an InvalidServiceEventError.
var MatchInvalidServiceEventError = mock.MatchedBy(func(err error) bool { return protocol.IsInvalidServiceEventError(err) })
