# Topology
In Flow, the term _topology_ captures the distributed protocol by which a node determines its
fanout set. The _fanout set_ of a node is a subset of other nodes in the system by which the node 
interacts in the event of epidemic dissemination of information. The epidemic dissemination happens when a 
node _multicasts_ or _broadcasts_ a message. The former refers to the event when a node sends a message
targeted for a subgroup of nodes in the system, while the latter refers to the event when a node aims at sending
a message to the entire system. Note that the communications over the fanout set are assumed unidirectional from the 
node to its fanout set. The distributed and independent invocations of topology protocol on nodes of Flow
hence results in a _directed_ graph where the vertices are the Flow nodes, and the edges are the fanout set of the nodes.
This directed graph is called the _topology graph_. It is required that the topology graph is connected with a very
high probability, where the probability is taken over the number of times the topology is constructed. The topology is constructed
once at the beginning of each epoch. Hence, it implies that the topology construction protocol should result a connected graph with a very
high probability over the life-time of Flow, which theoretically is infinitely-many epochs.

Figure below shows an example of topology graph of 7 nodes in Flow network, which is a directed and connected graph. There is a path between every 
two node and traversing the topology graph with BFS or DFS visits all the vertices. Also, the fanout set of each node is colored the same 
as the node itself, e.g., fanout set of the red node is illustrated using red edges from it to other nodes of the network. Also, in this example, 
every node has a fanout size of 3. 

![alt text](topology.svg)

