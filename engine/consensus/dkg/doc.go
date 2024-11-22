// Package dkg implements engines for the DKG protocol.
//
// # Reactor Engine
//
// The [ReactorEngine] implements triggers to control the lifecycle of DKG runs. A new
// DKG protocol is started when an EpochSetup event is sealed and finalized. The
// subsequent phase transitions are triggered when specified views are encountered
// (specifically when the first block of a given view is finalized). In between
// phase transitions the engine regularly queries the DKG smart-contract to read
// broadcast messages.
//
// # Messaging Engine
//
// The [MessagingEngine] is a network engine that enables consensus nodes to securely exchange
// private (not broadcast) DKG messages. Broadcast messages are sent via the DKG smart contract.
//
// # Architecture
//
// For every new epoch, the [ReactorEngine] instantiates a new [module.DKGController] with a
// new [module.DKGBroker] using the provided ControllerFactory. The ControllerFactory ties new
// DKGControllers to the [MessagingEngine] via a BrokerTunnel which exposes channels
// to relay incoming and outgoing messages (see package module/dkg).
//
//	  EpochSetup/EpochCommit/OnView
//	            |
//	            v
//	     +---------------+
//	     | ReactorEngine |
//	     +---------------+
//	            |
//	            v
//	 *~~~~~~~~~~~~~~~~~~~~~* <- Epoch-scoped
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
