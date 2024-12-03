// Package dkg implements engines for the DKG protocol.
//
// # Reactor Engine
//
// The [ReactorEngine] implements triggers to control the lifecycle of DKG instances.
// A new DKG instance is started when an EpochSetup service event is sealed.
// The subsequent phase transitions are triggered when specified views are encountered.
// Specifically, phase transitions for a view V are triggered when the first block with view >=V is finalized.
// Between phase transitions, we periodically query the DKG smart-contract ("whiteboard") to read broadcast messages.
// Before transitioning the state machine to the next phase, we query the whiteboard w.r.t. the final view
// of the phase - this ensures all participants eventually observe the same set of messages for each phase.
//
// # Messaging Engine
//
// The [MessagingEngine] is a network engine that enables consensus nodes to securely exchange
// private (not broadcast) DKG messages. Broadcast messages are sent via the DKG smart contract.
//
// # Architecture
//
// In the happy path, one DKG instance runs every epoch. For each DKG instance, the [ReactorEngine]
// instantiates a new, epoch-scoped module.DKGController and module.DKGBroker using the provided dkg.ControllerFactory.
// The dkg.ControllerFactory ties new module.DKGController's to the [MessagingEngine] via a dkg.BrokerTunnel,
// which exposes channels to relay incoming and outgoing messages (see package module/dkg for details).
//
//	  EpochSetup/EpochCommit/OnView events
//	            |
//	            v
//	     +---------------+
//	     | ReactorEngine |
//	     +---------------+
//	            |
//	            v
//	 *~~~~~~~~~~~~~~~~~~~~~* <- Epoch-scoped components
//	 |  +---------------+  |
//	 |  | Controller    |  |
//	 |  +---------------+  |
//	 |         |           |
//	 |         v           |
//	 |  +---------------+  |
//	 |  | Broker        |  |
//	 |  +---------------+  |
//	 *~~~~~~~~|~~~~~~~~~\~~*
//	          |          \
//	    BrokerTunnel      DKGContractClient
//	          |            \
//	+--------------+   +------------------+
//	| Messaging    |   | FlowDKG smart    |
//	| Engine       |   | contract         |
//	+--------------+   +------------------+
package dkg
