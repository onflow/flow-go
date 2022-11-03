# Corruptible Conduit Framework

Corruptible Conduit Framework is an integration testing framework for Byzantine Fault Tolerant (BFT) setups.
As shown by figure below, this framework is composed of Corruptible Conduits and Corruptible Conduit Factory.
A Corruptible Conduit Factory (CCF) is configured on each node that is meant to play _malicious_ during the test scenario. 
The CCF utilizes Corruptible Conduits (CC)s to connect the engines of its node to the networking adaptor.
In contrast to the normal conduits, the CCs do not relay the events from their engine to the network adaptor. 
Instead, they relay the events from their engine to the CCF. On receiving an event from a CC, the CCF forwards 
the message to a remote attacker. The attacker is in charge of orchestrating specific attacks through its 
programmable attack vectors. The attacker controls the set of malicious nodes. 
All messages from engines of attacker-controlled corrupted nodes are directly wired to the attacker. 
The attacker runs its registered attack vectors on each received message from its corrupted nodes' engines. 
As the result of running the attack vectors, the attacker may ask CCF of some corrupted nodes to send some certain messages on its behalf.
In this way, the Corruptible Conduit Framework empowers testing attack vectors by just replacing the default conduit factory of malicious nodes
with a CCF, and without any further modification to the application-layer implementation of the node. 

![Corruptible Conduit Framework](/insecure/corruptible/corruptible.png)