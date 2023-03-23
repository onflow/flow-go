# Application Layer Spam Prevention (ALSP)
## Overview
The Application Layer Spam Prevention (ALSP) is a module that provides a mechanism to prevent the malicious nodes from 
spamming the Flow nodes at the application layer (i.e., the engines). ASLP is not a multi-party protocol, i.e., 
it does not require the nodes to exchange any messages with each other for the purpose of spam prevention. Rather, it is 
a local mechanism that is implemented by each node to protect itself from malicious nodes. AlSP is not meant to replace 
the existing spam prevention mechanisms at the network layer (e.g., the Libp2p and GossipSub). 
Rather, it is meant to complement the existing mechanisms by providing an additional layer of protection.
ALSP is concerned with the spamming of the application layer through messages that appear valid to the networking layer and hence
are not filtered out by the existing mechanisms.

ALSP relies on the application layer to detect and report the misbehaviors that 
lead to spamming. It enforces a penalty system to penalize the misbehaving nodes that are reported by the application layer. ALSP also takes 
extra measures to protect the network from malicious nodes that attempt on an active spamming attacks. Once the penalty of a remote node
reaches a certain threshold, the local node will disconnect from the remote node and no-longer accept any incoming connections from the remote node 
until the penalty is reduced to zero again through a decaying interval.

## Features
- Spam prevention at the application layer. 
- Penalizes misbehaving nodes based on their behavior. 
- Configurable penalty values and decay intervals. 
- Misbehavior reports with customizable penalty amplification. 
- Thread-safe and non-blocking implementation. 
- Maintains the safety and liveness of the Flow blockchain system by disallow-listing malicious nodes (i.e., application layer spammers).

## Architectural Principles
- **Non-intrusive**: ALSP is a local mechanism that is implemented by each node to protect itself from malicious nodes. It is not a multi-party protocol, i.e., it does not require the nodes to exchange any messages with each other for the purpose of spam prevention.
- **Non-blocking**: ALSP is non-blocking and does not affect the performance of the networking layer. It is implemented in a way that does not require the networking layer to wait for the ALSP to complete its operations. Non-blocking behavior is mandatory for the networking layer to maintain its performance. 
- **Thread-safe**: ALSP is thread-safe and can be used concurrently by multiple threads, e.g., concurrent engine calls on reporting misbehaviors.

## Usage
ALSP is enabled by default through the networking layer. It is not necessary to explicitly enable it. Once can disable it by setting the `alsp-enbale` flag to `false`.
The network.Conduit interface provides the following method to report misbehaviors: 
- `ReportMisbehavior(*MisbehaviorReport)`: Reports a misbehavior to the ALSP. The misbehavior report contains the misbehavior type and the penalty value. The penalty value is used to increase the penalty of the remote node. The penalty value is amplified by the penalty amplification factor before being applied to the remote node. 

By default, the penalty amplification factor is set to 0.01 * disallow-listing threshold. The disallow-listing threshold is the penalty threshold at which the local node will disconnect from the remote node and no-longer accept any incoming connections from the remote node until the penalty is reduced to zero again through a decaying interval.
Hence, by default, every time a misbehavior is reported, the penalty of the remote node is increased by 0.01 * disallow-listing threshold. 
