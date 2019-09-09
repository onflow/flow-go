# Gossip
The gossip layer of Bamboo provides a fast, reliable, and Byzantine fault-tolerant gossiping service. The gossip layer is a part of the bigger Bamboo's decentralized middleware that is executed independently at each of the Bamboo nodes. On receiving a message from its corresponding nodes, the instance of the gossip protocol delivers that message to all the intended recipients.

The gossip layer is an application-layer protocol that acts as a middleware between the operating system of the nodes and instances of the advanced sub-protocols of Bamboo (e.g., consensus). We will develop the gossip layer in two phases known as the naive and mature gossip protocols. 

The naive gossip layer, which also is known as the _first version (i.e., v1)_, provides a bare skeleton of the gossip protocol, which is neither efficient nor robust in presence of the Byzantine nodes. The naive gossip protocol is mainly designed over our research of the choice of the proper development framework and technologies, and to reach the prototype of the protocol software. The naive layer itself is solely meant for execution at a limited scale in our internal test network and should not be utilized as a production-level implementation. In the naive gossip layer, a single message intended for a subset of other nodes of the system is delivered by making distinct one-to-one connections to each of the recipients. 

The mature gossip protocol, also known as the _second version (i.e., v2)_, is our evolved version of the naive gossip protocol that guarantees the delivery of the received message with a high probability and in an efficient manner. Efficiency in this context refers to the absence of bottlenecks; a single node should not be solely responsible for delivering a message to a large number of peers. Instead, nodes will implement a decentralized gossip service that epidemically disseminates a message to the intended recipients by gossip layer of each node simply forwarding the received message to a small subset of other nodes.


## Terminology:
+ **Gossip Router**: A gossip router -- or simply a router --  is a Bamboo node that conveys a message between two distinct nodes of the gossip network. A router node hence is never the sender (or initiator) of a message, but it can be one of the intended receivers.
+ **Gossip Partners**: As we specify later on, each node in the mature gossip network of Bamboo solely accepts gossip messages from a predefined subset of other nodes _as routers_, which are called its gossip partners. 
+ **Network**: 
Also reffered as the _system_ in the networking stream, concerns an overlay of the all the Bamboo nodes. 
+ **Network size**:
Also referred as the _system size_ in the networking stream, corresponds to the number of nodes. 
+ **Gossip initiator**: 
Corresponds to the node that aims to epidemically disseminate a message through the network, and invokes the gossip protocol as the means of the epidemic dissemination. 
+ **Destinated nodes**:
Corresponds to the set of all the nodes that the gossip initiator aims to have the message be delivered to. 
+ **Message**:
Corresponds to whatever that is meant to be transferable within the system, e.g., transactions, collections, blocks, execution receipts, etc. 

## Security
The general idea of the gossip network is to converge to a state that is in favor of the honest players, while acting against the favor of the byzantine actors. Honest nodes in such a state have a significantly higher priority than the Byzantine nodes in utilizing the system resources on gossiping their messages fast and reliably to the destinated nodes.  Byzantine nodes, on the other hand, are getting slow down literally to a state that is analogous to isolation from the system. 

### Scoring factors: 
The scoring mechanism of Gossip layer considers the following factors:
+ **Gossip interest**, i.e., how much a gossip partner of a node prefers that node to have a fresh message sooner than the other gossip partners? 
+ **Speed**, i.e., how much a gossip partner cares about the node to start the gossip faster, as well as, having a higher upstream and downstream bandwidth?
+ **Reliability**, i.e., does the gossip partner ever disrupts the connection during a gossip exchange? 
+ **Integrity**, i.e., how much _correct_ gossip message that gossip partner provides the node with?


## Mature Gossip (V2)

### Availability
#### Dropping:
_Problem:_ Instead of helping on the dissemination of the received gossip messages from their neighbors, Byzantine nodes may aim at attacking the availability of the system by simply dropping the received gossip messages. If the gossip overlay is poorly organized, this results in low availability or unavailability of the gossip message across the network. 

_Bamboo's countermeasure:_ At each epoch, each node is allowed to solely exchange the gossip messages with a certain subset of other nodes called its **fanout set**. Fanout is chosen in a way such that each node has at least one honest gossip partner with a high probability (w.h.p). Each node inside the fanout set is called a **gossip partner**. A node forwards a gossip delegated message to all its gossip partners, hoping the at-least-the-honest one forwards its delegation. When the network is not under a severe attack, and less than 33% of the neighbors are Byzantine, the majority of the fanout set of a chosen node are honest actors and hence we expect the gossip message to be disseminated quickly within the network. Under a severe 33% attack of Byzantine actors, there is a very high probability that at least one gossip partner of a node act honestly and forward the message. So in a very unlikely to happen worst case the dissemination of a gossip message goes with a random walk on the network and reaches all the destined nodes. A random walk corresponds to the case where the gossip message traverses one node at each time sequentially. Although it is drastically slower than the concurrent dissemination, it is just a reliable countermeasure for the very unlikely case in which the majority of nodes in the system have all-but-one Byzantine gossip partners.

#### Delaying and slowing down: 
_Problem:_ On sending or receiving a gossip message from a partner, a Byzantine node may deliberately enforce a long delay or slow-down of the gossip procedure. Although the effect of one Byzantine node delaying or slowing down the gossip may be venial, considering 33% Byzantine threshold over the network, it can severely degenerate the availability.

_Bamboo's countermeasure:_ Each node scores its gossip partners based on several factors including their upstream and downstream bandwidth, and the number of the timeout the gossip partner meets. Each RPC call is hence subject to a specific timeout to be terminated once the timeout approached. Hence, a Byzantine partner being slow just degenerates its score on the other side of the line. 

_Important note 1:_ We prefer not to blacklist a slow partner since it might be occasional due to the congestions, device failures, or even it can be the fault of the node itself and not its partners by for example limiting its downstream rate. 

_Important note 2:_ The speed scoring also allows a node to adaptively choose a close-to-optimal bandwidth per partner, i.e., starting from an initial bandwidth, the node may tweak up and down to find its optimal bandwidth based on the reaction of its partner.

#### Disrupting: 
_Problem:_ Although slowing down the gossip partners and delays on communications are tackled by introducing the scoring mechanism on speed and timeouts, a Byzantine node gossip partner may disrupt the exchanges abruptly in middle. This is not counted as slowing down or delaying the gossip directly. However, it affects the speed on which the gossip is disseminated within the network, and deteriorates the availability of the system. 

_Bamboo's countermeasure:_ Number of the times a gossip partner disrupts in the middle of a gossip exchange is counted as a negative score on the reliability of that partner. Continuing in this trend, the gossip partner would be the last one ever that a node aims to send a gossip to or to receive a gossip from. 

#### Flooding:
_Problem:_ Aiming at crashing a node with failure, a gossip Byzantine partner may be performing a Denial of Service (DoS) attack on a node by indefinitely sending gossip messages to that node. 

_Bamboo's countermeasure:_ This is tackled by the node having a limited-size slow-growing queue with each gossip partner, as well as enabling a **firewall**. Once the queue corresponding to a gossip partner gets full, that gossip partner temporarily blocked by the firewall of the node to avoid any further inbound communication. The queue corresponding to a gossip partner grows _logarithmically_ upon each _good_ gossip message submitted by that partner being processed. The partner is unblocked by the firewall as soon as its corresponding queue gets free space. 


#### Circulating: 
_Problem:_ Aiming at slowing down the network, colluding Byzantine nodes may aim at circulating (even correct) repetitive messages through the network. As each node should queue the received messages and process them, the circulation of the messages results in the wast of nodes' resources and over the long term degenerates the availability of the system. 

_Bamboo's countermeasure:_ Having each node with a limited set of gossip partners, and all communication between two nodes being authenticated and stateful (e.g., message counter) a node keeps a list of all the messages received by each of its gossip partners, a repeated message delivery by a gossip partner is considered as a circulation attack on the availability of the node and results in the node to blacklist the gossip partner by the firewall. 

### Integrity: 
Integrity attacks are concerned with the altering and modifying of gossip message content by unauthorized parties. Considering our system model, integrity attacks involve any attempt to modify the content of a gossip message contents by any node other than the issuer of the message. 

#### Spoofing or Spamming: 
_Problem:_ Aiming at attacking the system as a whole, or a single node, as a Byzantine gossip router may spoof the received gossip messages before routing and route the spoofed version of the messages. Likewise, a Byzantine gossip router may start disseminating spam (e.g., garbage) messages within the network. 

_Bamboo's countermeasures:_ We acknowledge that there is no more communication-wise efficient approach than having the messages being signed, verified hop-by-hop, and blacklist Byzantine gossip partners once their signed received message either fails to be verified or is a corrupted message (i.e., the signature is verified but the content is corrupted). This solution is however not time efficient as gossiping is slowed down by time overhead of the intermediate verifications at intermediate nodes. To act efficiently concerning the tradeoff of time and communication overheads, Bamboo handles the spoofing and spamming attacks by a hybrid of countermeasures, which do not require absolute verification of signatures on the intermediate nodes. The goal of Bamboo on tackling the Byzantine nodes is to limit their benefit of the network utilization at a reasonable speed and slow down them ultimately. A Byzantine node that is slowed down by its neighbors is supposed to not be able to exploit the network literally. 
  - No blind routing: Our mature gossip protocol ensures that the router of a gossip message upon reception is fully capable of verifying the content of the message, that is to determine whether the message is _correct_ and _useful_ or it is _spam_, _garbage_, or malicious. In other words, our gossip network does not utilize a node as a router without making sure that the node is capable of verifying the authenticity of what it routes. For example, an Access node is never expected to route a gossip message that is meant for a Security node.
  - Deterministic ultimate verification: Our mature gossip protocol ensures that although the router of a gossip message instantly routes the received message, it nevertheless, ultimately is going to verify the correctness and authenticity of the message. The verification should take place within the same block interval as the reception of the message. Any failure of the ultimate verification of a message will degenerate the score of the gossip partner of the node that routes the message to that router node.  
  - Probabilistic instantaneous verification: Despite the ultimate verification functionality of mature gossip, each received message at a node is subject to a chance of being verified instantly before further routing. This is mainly done as a countermeasure for the Byzantine node aiming to perform an attack by leveraging the ultimate verification time and flooding their gossip partners with as many as messages till their ultimate verification. To counter this, the mature gossip protocol at each node introduces an _examination probability_ for each of the gossip partners of the node. Having the examination probability of _p_ for a gossip partner of a node, there is a chance of _p_ for a received message from that partner to be verified instantaneously upon reception instead of being postponed for ultimate verifications. The more a node has trust on its partners, the lower is the examination probability. The examination probability of a node for each of its partners is initially equal to one and drops at a specific predefined rate per correct and authenticated received message. The examination probability of a partner is reset to one if any of the ultimate or probabilistic verifications of a message received by that gossip partner fails. 


#### Replay Attack and Duplication:
_Problem:_ A Byzantine node may aim at attacking the integrity of the system by repetitive delivery of the same message to its gossip partner. Such attacks aim to make the targeted gossip partner executing the effects of the message repeatedly, which results in an (inconsistent) internal state of the victimized gossip partner. The replay attack can be for example a repetitive voting request over the consensus or a repetitive block request. In addition to being an attack on the integrity, the replay attack is also considered an attack on the availability of the system, as by indefinitely asking a partner to do a repetitive job, a replay attacker makes that gossip partner _unavailable_ of being responsive in the favor of the system. Additionally, a Byzantine node capturing the communication between two honest nodes may aim on performing a replay attack against any of them by masquerading as the other party and replaying all or a subset of the captured messages from their session. In addition to the replay attack that is considered as a malicious attack instantiated by the Byzantine nodes, a node may receive duplicative messages from all or a subset of its gossip partners all aiming on just helping the node with having the fresh information. However, for the node, receiving the same message from all its gossip partners is an expensive act. 

_Bamboo's countermeasures:_ We tackle the replay attack with the three following countermeasures: 
           -Stateful session: We divide the interaction between two nodes over time into sessions. Each session has a start and an end. All the application layer messages that are exchanged between two nodes within a session are tagged with a message authentication code and are subject to a state bookkeeping at both sides. Any attempt of an eavesdropper attacker to do a replay attack on behalf of each of the nodes engaged in a session against the other one is easily detected and neutralized. 
          -Digest first: This solution is to protect a node from expending bandwidth and resources on receiving duplicative messages from different gossip partners. Using the _Digest First_ mechanism, gossip partners of a node first send the digest of the message they aim to gossip with the node. Upon the verification by the node that it has not yet received such a message, the node replies positively to one of its gossip partners on receiving the message from it. Failure to provide a message to a node that is compliant with the previously sent digest is counted as misbehavior concerning the integrity and degenerates the score of the corresponding gossip partner in the perspective of the node.  
          -Probabilist duplication detection: For a single node, keeping track of all the received messages or even their digests to detect the duplication and replay attack is an inefficient solution concerning the time and space. We hence provide each node a probabilistic efficient solution concerning both time and memory that can detect the repetition of gossiped digests it receives from its gossip partners. The duplication detection secures a low false positive probability of duplication detection that is under further investigation. 

## Naive Gossip (V1)
### Overview
In a nutshell, the networking stream concerns the dissemination of a message throughout the system. By system, we mean the set of all the nodes participating in Bamboo regardless of their type or role in the system. A node is said to participate in the system if it has an instance of the Bamboo node up and running, e.g., as a process. Likewise, our view of a message in the Networking stream is whatever that is meant to be transferable within the system, e.g., transactions, collections, blocks, execution receipts, etc. 

The list of all features aimed to be covered within the Networking stream are:
- A *one-to-one* functionality, which enables each node to make a Remote Procedure Call (RPC) to another arbitrary node of choice. RPC is a functionality that enables a node to remotely call and hence execute a local function of another (remote) node. We elaborate more on this within this documentation. 
- A *one-to-many* functionality, which enables each node to make the same RPC on multiple nodes of its choice. 
- A *one-to-all* functionality, which enables each node to make the same RPC on all the nodes of the system. 

The first version of the networking implementation provides the bare gossip networking functionality for the system and is supposed to be evolved into a drastically more efficient and secure implementation as it is described in the rest of this documentation. 


### Packages used by this stream:
#### Remote Procedure Call (RPC)
Sending data to another node can be done in several ways. One naive way of doing so is to provide a forwarding functionality, where a node casts (i.e., encodes) whatever it aims to send into a *message* data structure, and sends it over a communication channel (e.g., TCP or UDP). Once the receiver node receives the *message* it decodes it and extracts the original data. That however has the clear disadvantage of a time, communication, and memory overhead of encoding and decoding functionalities. To act more efficiently with respect to such overheads, in this stream we employ the RPC functionality. RPC allows one node to send data to another node by simply calling a local function on the receiver. The interesting feature of the RPC is that the sender solely needs the function prototype (i.e., signature) while the actual complete function resides on the receiver that is assumed to reside in remote distance with respect to the sender. Implementing the networking layer of Bamboo in Golang, we rely on [gRPC](https://grpc.io/).

#### go-libp2p
  The traditional RPC libraries (e.g., gRPC) provide the server and client architectural realization of the protocol. However, in Bamboo, we do not have specific clients and servers. Rather, Bamboo is a P2P protocol, i.e., we expect each node running the Bamboo node be able to act as both a client and server. This notion is known as a peer. To provide RPC over the P2P overlay of the Bamboo nodes, we are employing the [go-libp2p](https://github.com/libp2p/go-libp2p) library that provides us the RPC communication among the nodes. 

### Dependencies on other streams
The networking stream acts more as a utility rather than a stream in the sense that it is a building block for the other streams to function correctly. The only touchpoint of this stream with the others is its interface. The interface of the networking stream is supposed to be invoked by the other streams to provide gossiping functionality for them. 

### Assumptions:
In the version 1 (i.e., v1) of the Gossip Network implementation we aim to establish a functional Gossip Network among the nodes compromising on efficiency for the sake of functionality. We focus on the efficiency and security aspects in detail in later versions. Having that in mind, we have the following list of assumptions in the v1 implementation: 

- Every node is accessible via a public IP or DNS, in other words, we do not have any node behind NAT that is not accessible via an IP. We do not need any hole punching or port forwarding. 
- Every node knows every other node's (IP or DNS) address.
- All nodes are honest and there is no Byzantine behaviour.
- All nodes are online, available, and responsive at all times.
- The maximum number of nodes participating in the gossip protocol is 100. 



### Interface
Gossip Network v1 provides a general purpose _gossip_ interface for the other streams of the system, which is shown in the following:

`Gossip(ctx context.Context, request Message, reply *Message)`

`ctx` is an instance of the Context package that carries deadlines, cancelation signals, and other request-scoped values across API boundaries and between processes.

`request` is a Gossip Network _message_ structure that is meant to to be gossiped within the network in one of the gossip _modes_. 

`reply` is a pointer to the Gossip Network _message_ that is sent back to the initiator of the gossip message as the reply of the gossip it initiates. 


### Gossip modes
The request message also contains a gossip mode that determines how the initiator of the message aims the message to be dessiminated within the network. By the initiator of the message we mean the node who initiates the first instance of the gossip message, not the nodes participating in gossiping the generated message within the network. 
#### One to one
In this mode, the initiator aims on sending the message to only a _single_ node of the system. Assuming that every node knows the address of every other node, this mode is accomplished by directly making a `Gossip` RPC call of the gossip initiator on the receiver node. 

#### One to all
In this mode, the initiator aims on sending the message to _all_ the nodes in the system. Assuming that every node knows the address of every other node and also the small size of the system in this version, the one-to-all gossip mode is accomplished by simply doing one-to-one gossips between the initiator and all the nodes in the system. For the later versions, however, the gossip network establishes and maintains an overlay of the nodes that handles the one-to-all gossip efficiently in the presence of the Byzantine nodes – that appear in the v2 of the gossip network – and without making direct one-to-one calls between the gossip initiator and all the nodes in the system. 

#### One to many 
In this mode, the initiator aims on sending the message to a _subset_ of the nodes of the system. That subset is something between 10 to one-tenth of the system size in the order of magnitude. For example, considering a system size of 10K nodes, the destinated nodes of one-to-many would be a set of 10 to 1000 nodes. Due to a small system size in v1 implementation, we are going to implement one-to-many gossip using as many as one-to-one gossips targeting all the destinated nodes. In later versions, however, we are going to develop a decision function that decides on the time and communication overhead of the one-to-many gossip based on the size of the destinated nodes set. For the corner cases of the very small or very large destinated nodes set, the decision function employs one-to-one or one-to-many gossips, respectively. For non-corner cases, however, it establishes and manages a gossip network, that handles the one-to-many gossip efficiently in the presence of the Byzantine nodes, that appear in the v2 of the gossip network.  

