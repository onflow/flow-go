# Application Layer Spam Prevention (ALSP)
## Overview
The Application Layer Spam Prevention (ALSP) is a module that provides a mechanism to prevent the malicious nodes from 
spamming the Flow nodes at the application layer (i.e., the engines). ALSP is not a multi-party protocol, i.e., 
it does not require the nodes to exchange any messages with each other for the purpose of spam prevention. Rather, it is 
a local mechanism that is implemented by each node to protect itself from malicious nodes. ALSP is not meant to replace 
the existing spam prevention mechanisms at the network layer (e.g., the Libp2p and GossipSub). 
Rather, it is meant to complement the existing mechanisms by providing an additional layer of protection.
ALSP is concerned with the spamming of the application layer through messages that appear valid to the networking layer and hence
are not filtered out by the existing mechanisms.

ALSP relies on the application layer to detect and report the misbehaviors that 
lead to spamming. It enforces a penalty system to penalize the misbehaving nodes that are reported by the application layer. ALSP also takes 
extra measures to protect the network from malicious nodes that attempt an active spamming attack. Once the penalty of a remote node
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
ALSP is enabled by default through the networking layer. It is not necessary to explicitly enable it. One can disable it by setting the `alsp-enable` flag to `false`.
The network.Conduit interface provides the following method to report misbehaviors: 
- `ReportMisbehavior(*MisbehaviorReport)`: Reports a misbehavior to the ALSP. The misbehavior report contains the misbehavior type and the penalty value. The penalty value is used to increase the penalty of the remote node. The penalty value is amplified by the penalty amplification factor before being applied to the remote node. 

By default, the penalty amplification factor is set to 0.01 * disallow-listing threshold. The disallow-listing threshold is the penalty threshold at which the local node will disconnect from the remote node and no-longer accept any incoming connections from the remote node until the penalty is reduced to zero again through a decaying interval.
Hence, by default, every time a misbehavior is reported, the penalty of the remote node is increased by 0.01 * disallow-listing threshold. This penalty value is configurable through an option function on the `MisbehaviorReport` struct.
The example below shows how to create a misbehavior report with a penalty amplification factor of 10, i.e., the penalty value of the misbehavior report is amplified by 10 before being applied to the remote node. This is equal to
increasing the penalty of the remote node by 10 * 0.01 * disallow-listing threshold. The `misbehavingId` is the Flow identifier of the remote node that is misbehaving. The `misbehaviorType` is the reason for reporting the misbehavior.
```go
report, err := NewMisbehaviorReport(misbehavingId, misbehaviorType, WithPenaltyAmplification(10))
if err != nil {
    // handle the error
}
```

Once a misbehavior report is created, it can be reported to the ALSP by calling the `ReportMisbehavior` method on the network conduit. The example below shows how to report a misbehavior to the ALSP.
```go
// let con be network.Conduit
err := con.ReportMisbehavior(report)
if err != nil {
    // handle the error
}
```

## Misbehavior Types (`MisbehaviorType`)
ALSP package defines several constants that represent various types of misbehaviors that can be reported by engines. These misbehavior types help categorize node behavior and improve the accuracy of the penalty system.

### Constants
The following constants represent misbehavior types that can be reported:

- `StaleMessage`: This misbehavior is reported when an engine receives a message that is deemed stale based on the local view of the engine. A stale message is one that is outdated, irrelevant, or already processed by the engine.
- `ResourceIntensiveRequest`: This misbehavior is reported when an engine receives a request that takes an unreasonable amount of resources for the engine to process, e.g., a request for a large number of blocks. The decision to consider a request heavy is up to the engine. Heavy requests can potentially slow down the engine, causing performance issues.
- `RedundantMessage`: This misbehavior is reported when an engine receives a message that is redundant, i.e., the message is already known to the engine. The decision to consider a message redundant is up to the engine. Redundant messages can increase network traffic and waste processing resources.
- `UnsolicitedMessage`: This misbehavior is reported when an engine receives a message that is not solicited by the engine. The decision to consider a message unsolicited is up to the engine. Unsolicited messages can be a sign of spamming or malicious behavior.
- `InvalidMessage`: This misbehavior is reported when an engine receives a message that is invalid and fails the validation logic as specified by the engine, i.e., the message is malformed or does not follow the protocol specification. The decision to consider a message invalid is up to the engine. Invalid messages can be a sign of spamming or malicious behavior.
## Thresholds and Parameters
The ALSP provides various constants and options to customize the penalty system:
- `misbehaviorDisallowListingThreshold`: The threshold for concluding a node behavior is malicious and disallow-listing the node. Once the penalty of a remote node reaches this threshold, the local node will disconnect from the remote node and no-longer accept any incoming connections from the remote node until the penalty is reduced to zero again through a decaying interval.
- `defaultPenaltyValue`: The default penalty value for misbehaving nodes. This value is used when the penalty value is not specified in the misbehavior report. By default, the penalty value is set to `0.01 * misbehaviorDisallowListingThreshold`. However, this value can be amplified by a positive integer in [1-100] using the `WithPenaltyAmplification` option function on the `MisbehaviorReport` struct. Note that amplifying at 100 means that a single misbehavior report will disallow-list the remote node.
- `misbehaviorDecayHeartbeatInterval`: The interval at which the penalty of the misbehaving nodes is decayed. Decaying is used to reduce the penalty of the misbehaving nodes over time. So that the penalty of the misbehaving nodes is reduced to zero after a certain period of time and the node is no-longer considered misbehaving. This is to avoid persisting the penalty of a node forever.
- `defaultDecayValue`: The default value that is deducted from the penalty of the misbehaving nodes at each decay interval.
- `decayValueSpeedPenalty`: The penalty for the decay speed. This is a multiplier that is applied to the `defaultDecayValue` at each decay interval. The purpose of this penalty is to slow down the decay process of the penalty of the nodes that make a habit of misbehaving.
- `minimumDecayValue`: The minimum decay value that is used to decay the penalty of the misbehaving nodes. The decay value is capped at this value. 
