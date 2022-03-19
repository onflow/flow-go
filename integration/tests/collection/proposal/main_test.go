package proposal

import (
	"testing"

	"github.com/onflow/flow-go/integration/tests/collection"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/suite"
)

func TestMultiCluster(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_FLAKY, "flaky as it often hits port already allocated, since too many containers are created")
	suite.Run(t, new(collection.MultiClusterSuite))
}
