package distributer_test

import (
	"testing"

	"github.com/onflow/flow-go/network/p2p/distributer"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestGossipSubInspectorNotification(t *testing.T) {
	_ = distributer.DefaultGossipSubInspectorNotification(unittest.Logger())
}
