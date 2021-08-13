/*

Package dkg implements engines for the DKG protocol.

ReactorEngine

ReactorEngine implements triggers to control the lifecycle of DKG runs. A new
DKG protocol is started when an EpochSetup event is sealed and finalized. The
subsequent phase transitions are triggered when specified views are encountered
(specifically when the first block of a given view is finalized). In between
phase transitions the engine regularly queries the DKG smart-contract to read
broadcast messages.

MessagingEngine

MessagingEngine is a network engine that enables consensus nodes to securely
exchange private DKG messages. Note that broadcast messages are not exchanged
through this engine, but rather via the DKG smart-contract.

Architecture

For every new epoch, the ReactorEngine instantiates a new DKGController with a
new Broker using the provided ControllerFactory. The ControllerFactory ties new
DKGControllers to the MessagingEngine via a BrokerTunnel which exposes channels
to relay incoming and outgoing messages (cf. module/dkg).

    EpochSetup/OnView
          |
          v
   +---------------+
   | ReactorEngine |
   +---------------+
          |
          v
*~~~~~~~~~~~~~~~~~~~~~* (one/epoch)
|  +---------------+  |
|  | Controller    |  |
|  +---------------+  |
|         |           |
|         v           |
|  +---------------+  |
|  | Broker        |  |
|  +---------------+  |
*~~~~~~~~|~~~~~~~~~\~~*
       tunnel      smart-contract client
         |           \
   +--------------+   +------------------+
   | Messaging    |   | DKGSmartContract |
   | Engine       |   |                  |
   +--------------+   +------------------+

*/

package dkg
