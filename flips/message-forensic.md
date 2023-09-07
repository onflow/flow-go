# GossipSub Message Forensic (GMF)

## Abstract
In GossipSub the sender of a message signs its published messages. 
Upon reception of a new gossiped message, the receiver verifies the signature prior to delivering it to the application 
layer or forwarding it to the other nodes (as part of the GossipSub protocol). There are several cases where we need the 
GossipSub authentication data to be shared with the Flow protocol as part of the protocol-level decision makings, 
e.g., to attribute a protocol-level violation to the malicious sender that originally sent it. The challenge is the signature
is done on the message payload at the GossipSub level, which is not available to the Flow protocol. In this FLIP we propose
a mechanism to share the GossipSub authentication data with the Flow protocol. We call this mechanism GossipSub Message 
Forensic (GMF). We also discuss the use cases of GMF in the Flow protocol, its interface, and the implementation details.
Moreover, we articulate the advantages and disadvantages of GMF, and propose a set of alternatives to GMF and compare 
them with GMF. The purpose of this FLIP is to provide a fair comparison between all the alternatives and GMF, and to
evaluate the feasibility of the suitable option considering the implementation complexity, performance overhead, and the security
guarantees.

## Notes
GMF canfind and penalize the malicious GossipSub node that has routed an invalid message to the node. 