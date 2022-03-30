package api

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/integration/tests/access"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestAccessSuite(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_TODO, "broken in CI")
	suite.Run(t, new(access.AccessSuite))
}
