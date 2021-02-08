/*

Package dkg implements a controller that manages the lifecycle of a Joint
Feldman DKG node.

The state-machine can be represented as follows:

+-------+  /Run() +---------+  /EndPhase0() +---------+  /EndPhase1() +---------+  /End()   +-----+     +----------+
| Init  | ----->  | Phase 0 | ------------> | Phase 1 | ------------> | Phase 2 | --------> | End | --> | Shutdown |
+-------+         +---------+               +---------+               +---------+           +-----+     +----------+
   |                   |                         |                         |                                 ^
   v___________________v_________________________v_________________________v_________________________________|
                                            /Shutdown()

The controller is always in one of 6 states:
	- Init: Default state before the instance is started
	- Phase 0: 1st phase of the JF DKG protocol while it's running
	- Phase 1: 2nd phase of the JF DKG protocol while it's running
	- Phase 2: 3rd phase of the JF DKG protocol while it's running
	- End: When the DKG protocol is finished
	- Shutdown: When the controller and all its routines are stopped

The controller exposes the following functions to trigger transitions:

Run(): Triggers transition from Init to Phase0. Starts the DKG protocol instance
       and background communication routines.

EndPhase0(): Triggers transition from Phase 0 to Phase 1.

EndPhase1(): Triggers transition from Phase 1 to Phase 2.

End(): Ends the DKG protocol and records the artifacts in controller. Triggers
       transition from Phase 2 to End.

Shutdown(): Can be called from any state to stop the DKG instance.

The End and Shutdown states differ in that the End state can only be arrived at
from Phase 2 and after successfully computing the DKG artifacts. Whereas the
Shutdown state can be reached from any other state.


XXX:
- add architecture diagram
- explain broker
- explain usage (one per epoch)
*/
package dkg
