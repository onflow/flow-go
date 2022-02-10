package views_progress

import (
	"testing"

	"github.com/onflow/flow-go/integration/tests/epochs"
	"github.com/stretchr/testify/suite"
)

func TestViewsProgress(t *testing.T) {
	suite.Run(t, new(epochs.ViewsProgressSuite))
}
