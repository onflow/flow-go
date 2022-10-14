package attackernet

import (
	"testing"

	"github.com/onflow/flow-go/insecure"
)

func TestOrchestratorNetworkObserve_SingleMessage(t *testing.T) {
	testOrchestratorNetworkObserve(t, 1)
}

func TestOrchestratorNetworkObserve_MultipleConcurrentMessages(t *testing.T) {
	testOrchestratorNetworkObserve(t, 10)
}

func TestOrchestratorNetworkUnicast_SingleMessage(t *testing.T) {
	testOrchestratorNetwork(t, insecure.Protocol_UNICAST, 1)
}

func TestOrchestratorNetworkUnicast_ConcurrentMessages(t *testing.T) {
	testOrchestratorNetwork(t, insecure.Protocol_UNICAST, 10)
}

func TestOrchestratorNetworkMulticast_SingleMessage(t *testing.T) {
	testOrchestratorNetwork(t, insecure.Protocol_MULTICAST, 1)
}

func TestOrchestratorNetworkMulticast_ConcurrentMessages(t *testing.T) {
	testOrchestratorNetwork(t, insecure.Protocol_MULTICAST, 10)
}

func TestOrchestratorNetworkPublish_SingleMessage(t *testing.T) {
	testOrchestratorNetwork(t, insecure.Protocol_PUBLISH, 1)
}

func TestOrchestratorNetworkPublish_ConcurrentMessages(t *testing.T) {
	testOrchestratorNetwork(t, insecure.Protocol_PUBLISH, 10)
}
