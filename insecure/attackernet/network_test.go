package attackernet

import (
	"testing"

	"github.com/onflow/flow-go/insecure"
)

func TestAttackerNetworkObserve_SingleMessage(t *testing.T) {
	testAttackerNetworkObserve(t, 1)
}

func TestAttackerNetworkObserve_MultipleConcurrentMessages(t *testing.T) {
	testAttackerNetworkObserve(t, 10)
}

func TestAttackerNetworkUnicast_SingleMessage(t *testing.T) {
	testAttackerNetwork(t, insecure.Protocol_UNICAST, 1)
}

func TestAttackerNetworkUnicast_ConcurrentMessages(t *testing.T) {
	testAttackerNetwork(t, insecure.Protocol_UNICAST, 10)
}

func TestAttackerNetworkMulticast_SingleMessage(t *testing.T) {
	testAttackerNetwork(t, insecure.Protocol_MULTICAST, 1)
}

func TestAttackerNetworkMulticast_ConcurrentMessages(t *testing.T) {
	testAttackerNetwork(t, insecure.Protocol_MULTICAST, 10)
}

func TestAttackerNetworkPublish_SingleMessage(t *testing.T) {
	testAttackerNetwork(t, insecure.Protocol_PUBLISH, 1)
}

func TestAttackerNetworkPublish_ConcurrentMessages(t *testing.T) {
	testAttackerNetwork(t, insecure.Protocol_PUBLISH, 10)
}
