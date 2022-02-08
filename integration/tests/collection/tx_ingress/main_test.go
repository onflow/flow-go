package tx_ingress

import (
	"testing"

	"github.com/onflow/flow-go/integration/tests/collection"
	"github.com/stretchr/testify/suite"
)

func TestIngress(t *testing.T) {
	suite.Run(t, new(collection.IngressSuite))
}
