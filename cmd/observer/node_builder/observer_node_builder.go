package node_builder

// ObserverNodeBuilder extends cmd.NodeBuilder and declares additional functions needed to bootstrap an Observer node.
// The Staked network allows the staked nodes to communicate among themselves, while the observer network allows the
// observer services and a staked Access node to communicate. Observer nodes can be attached to an observer or staked
// access node to fetch read only block, and execution information. Observer nodes are scalable.
//
//                                 observer network                           staked network
//  +------------------------+
//  |     Observer Node 1    |
//  +------------------------+
//              |
//              v
//  +------------------------+
//  |     Observer Node 2    |<--------------------------|
//  +------------------------+                           |
//              |                                        |
//              v                                        v
//  +------------------------+                         +-------------------+                 +------------------------+
//  |     Observer Node 3    |<----------------------->|    Access Node    |<--------------->| All other staked Nodes |
//  +------------------------+                         +-------------------+                 +------------------------+
//  +------------------------+                           ^
//  |     Observer Node 4    |<--------------------------|
//  +------------------------+
