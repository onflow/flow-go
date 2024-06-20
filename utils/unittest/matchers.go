package unittest

import (
	"fmt"

	"github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/model/flow"
)

// MatchEpochExtension matches an epoch extension argument passed to a mocked component.
func MatchEpochExtension(finalView, extensionLen uint64) any {
	return mock.MatchedBy(func(extension flow.EpochExtension) bool {
		fmt.Println("## ", extension.FirstView, extension.FinalView)
		fmt.Println("$$ ", finalView+1, finalView+extensionLen)
		return extension.FirstView == finalView+1 && extension.FinalView == finalView+extensionLen
	})
}
