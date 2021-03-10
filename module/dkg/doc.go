/*

Package dkg implements a controller that manages the lifecycle of a Joint
Feldman DKG node, as well as a broker that enables the controller to communicate
with other nodes

Controller

A new controller must be instantiated for every epoch.

The state-machine can be represented as follows:

+-------+  /Run() +---------+  /EndPhase1() +---------+  /EndPhase2() +---------+  /End()   +-----+     +----------+
| Init  | ----->  | Phase 1 | ------------> | Phase 2 | ------------> | Phase 3 | --------> | End | --> | Shutdown |
+-------+         +---------+               +---------+               +---------+           +-----+     +----------+
   |                   |                         |                         |                                 ^
   v___________________v_________________________v_________________________v_________________________________|
                                            /Shutdown()

The controller is always in one of 6 states:

- Init: Default state before the instance is started
- Phase 1: 1st phase of the JF DKG protocol while it's running
- Phase 2: 2nd phase ---
- Phase 3: 3rd phase ---
- End: When the DKG protocol is finished
- Shutdown: When the controller and all its routines are stopped

The controller exposes the following functions to trigger transitions:

Run(): Triggers transition from Init to Phase1. Starts the DKG protocol instance
       and background communication routines.

EndPhase1(): Triggers transition from Phase 1 to Phase 2.

EndPhase2(): Triggers transition from Phase 2 to Phase 3.

End(): Ends the DKG protocol and records the artifacts in controller. Triggers
       transition from Phase 3 to End.

Shutdown(): Can be called from any state to stop the DKG instance.

The End and Shutdown states differ in that the End state can only be arrived at
from Phase 3 and after successfully computing the DKG artifacts. Whereas the
Shutdown state can be reached from any other state.

Broker

The controller requires a broker to communicate with other nodes over the
network and to read broadcast messages from the DKG smart-contract. A new broker
must be instantiated for every epoch.

The Broker is responsible for:

- converting to and from the message format used by the underlying crypto DKG
  package.
- appending dkg instance id to messages to prevent replay attacks
- checking the integrity of incoming messages
- signing and verifying broadcast messages (broadcast messages are signed with
  the staking key of the sender)
- forwarding incoming messages (private and broadcast) to the controller via a
  channel
- forwarding outgoing messages (private and broadcast) to other nodes.

+------------+  +-------------+
|            |  |             | <--------(tunnel)-----------> network engine <--> Other nodes
| Controller |--| Broker      |
|            |  |             | <--(smart-contract client)--> DKG smart-contract
+------------+  +-------------+

To relay private messages, the broker uses a BrokerTunnel to communicate with a
network engine.

To send and receive broadcast messages, the broker communicates with the DKG
smart-contract via a smart-contract client. The broker's Poll method must be
called regularly to read broadcast messages from the smart-contract.

*/
package dkg
