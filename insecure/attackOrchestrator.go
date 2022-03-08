package insecure

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/network"
)

// AttackOrchestrator represents the stateful interface that implements a certain type of attack, e.g., wintermute attack.
type AttackOrchestrator interface {
	component.Component

	// HandleEventFromCorruptedNode implements logic of processing the events received from a corrupted node.
	//
	// In Corruptible Conduit Framework for BFT testing, corrupted nodes relay their outgoing events to
	// the attacker instead of dispatching them to the network.
	HandleEventFromCorruptedNode(*CorruptedNodeEvent) error
}

type CorruptedNodeEvent struct {
	CorruptedId flow.Identifier
	Channel     network.Channel
	Event       interface{}
	Protocol    Protocol
	TargetNum   uint32
	TargetIds   flow.IdentifierList
}
