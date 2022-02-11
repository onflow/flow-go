package proposal

import (
	"testing"

	"github.com/onflow/flow-go/integration/tests/collection"
	"github.com/stretchr/testify/suite"
)

func TestMultiCluster(t *testing.T) {
	suite.Run(t, new(collection.MultiClusterSuite))
}
