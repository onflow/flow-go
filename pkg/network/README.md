# Networking (V1)
## Overview
In a nutshell, the networking stream concerns the dissemination of a message throughout the system. By system, we mean the set of all the nodes participating in Bamboo regardless of their type or role in the system. A node is said to participate in the system if it has an instance of the Bamboo node up and running, e.g., as a process. Likewise, our view of a message in the Networking stream is whatever that is meant to be transferable within the system, e.g., transactions, collections, blocks, execution receipts, etc. 

The list of all features aimed to be covered within the Networking stream are:
- A *one-to-one* functionality, which enables each node to make a Remote Procedure Call (RPC) to another arbitrary node of choice. RPC is a functionality that enables a node to remotely call and hence execute a local function of another (remote) node. We elaborate more on this within this documentation. 
- A *one-to-many* functionality, which enables each node to make the same RPC on multiple nodes of its choice. 
- A *one-to-all* functionality, which enables each node to make the same RPC on all the nodes of the system. 

The first version of the networking implementation provides the bare gossip networking functionality for the system and is supposed to be evolved into a drastically more efficient and secure implementation as it is described in the rest of this documentation. 

## Terminology:
#### Network: 
Also reffered as the **system** in the networking stream, concerns an overlay of the all the Bamboo nodes. 
#### Network size:
Also referred as the **system size** in the networking stream, corresponds to the number of nodes. 
#### Gossip initiator: 
Corresponds to the node that aims to epidemically disseminate a message through the network, and invokes the gossip protocol as the means of the epidemic dissemination. 
#### Destinated nodes:
Corresponds to the set of all the nodes that the gossip initiator aims to have the message be delivered to. 
### Message:
Corresponds to whatever that is meant to be transferable within the system, e.g., transactions, collections, blocks, execution receipts, etc. 


## Packages used by this stream:
### Remote Procedure Call (RPC)
Sending data to another node can be done in several ways. One naive way of doing so is to provide a forwarding functionality, where a node casts (i.e., encodes) whatever it aims to send into a *message* data structure, and sends it over a communication channel (e.g., TCP or UDP). Once the receiver node receives the *message* it decodes it and extracts the original data. That however has the clear disadvantage of a time, communication, and memory overhead of encoding and decoding functionalities. To act more efficiently with respect to such overheads, in this stream we employ the RPC functionality. RPC allows one node to send data to another node by simply calling a local function on the receiver. The interesting feature of the RPC is that the sender solely needs the function prototype (i.e., signature) while the actual complete function resides on the receiver that is assumed to reside in remote distance with respect to the sender. Implementing the networking layer of Bamboo in Golang, we rely on [gRPC](https://grpc.io/).

### go-libp2p
  The traditional RPC libraries (e.g., gRPC) provide the server and client architectural realization of the protocol. However, in Bamboo, we do not have specific clients and servers. Rather, Bamboo is a P2P protocol, i.e., we expect each node running the Bamboo node be able to act as both a client and server. This notion is known as a peer. To provide RPC over the P2P overlay of the Bamboo nodes, we are employing the [go-libp2p](https://github.com/libp2p/go-libp2p) library that provides us the RPC communication among the nodes. 

## Dependencies on other streams
The networking stream acts more as a utility rather than a stream in the sense that it is a building block for the other streams to function correctly. The only touchpoint of this stream with the others is its interface. The interface of the networking stream is supposed to be invoked by the other streams to provide gossiping functionality for them. 

## Assumptions:
In the version 1 (i.e., v1) of the Gossip Network implementation we aim to establish a functional Gossip Network among the nodes compromising on efficiency for the sake of functionality. We focus on the efficiency and security aspects in detail in later versions. Having that in mind, we have the following list of assumptions in the v1 implementation: 

- Every node is accessible via a public IP or DNS, in other words, we do not have any node behind NAT that is not accessible via an IP. We do not need any hole punching or port forwarding. 
- Every node knows every other node's (IP or DNS) address.
- All nodes are honest and there is no Byzantine behaviour.
- All nodes are online, available, and responsive at all times.
- The maximum number of nodes participating in the gossip protocol is 100. 



## Interface
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
