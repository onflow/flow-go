package tx_ingress

import (
	"testing"

	"github.com/onflow/flow-go/integration/tests/collection"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/suite"
)

func TestIngress(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_FLAKY, "flaky in CI")
	suite.Run(t, new(collection.IngressSuite))
}
