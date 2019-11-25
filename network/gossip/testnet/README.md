# TestNet

TestNet contains a multitude of integration tests on the gossip layer. The tests are on two different implementations of nodes:

## ChatNode

A ChatNode is a node that has only one function: `DisplayMessage(msg)` which displays the message `msg` on the screen and stores all received messages.

## HasherNode

A HasherNode is a node that has only one function: `receive(msg)` which computes the hash of the received message and stores it.

## The Tests

The test cases cover the different configurations that are possible when sending messages over the network:
* Sync One-to-one Gossip
* Async One-to-all Gossip
* Sync One-to-many Gossip
* Async One-to-many Gossip
* Sync One-to-all Gossip
* Async One-to-all Gossip

The tests utilize all the different possible modes to send messages in the different implemented nodes and test whether the messages are received correctly.

