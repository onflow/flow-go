# libp2p Resource Manager Configuration in Flow Go

## Overview
In Flow Go, the libp2p Resource Manager plays a crucial role in managing network resources effectively. This document provides guidance on setting various limits through configuration files and runtime flags, helping you optimize resource usage based on specific network conditions or protocol behaviors.

### What are These Limits?
The libp2p Resource Manager in Flow Go allows setting limits on different types of network resources like streams, connections, file descriptors, and memory. These limits are categorized under different scopes: `system`, `transient`, `protocol`, `peer`, and `peer-protocol`. Each scope serves a distinct purpose in resource management.

### How to Set Limits

#### In Configuration File (`default-config.yaml`)
You can define these limits in the `default-config.yaml` file under the `libp2p-resource-manager` section. Each limit can be set for different scopes as shown:

```yaml
libp2p-resource-manager:
  memory-limit-ratio: <value>
  file-descriptors-ratio: <value>
  limits-override:
    <scope>:
      streams-inbound: <value>
      streams-outbound: <value>
      ...
```

#### Via Runtime Flags
Each limit can also be dynamically set using runtime flags in the format:
`--libp2p-resource-manager-<scope>-<limit>`

For example:
- To set inbound stream limits for the system scope: `--libp2p-resource-manager-system-streams-inbound=<value>`
- For outbound streams in the protocol scope: `--libp2p-resource-manager-protocol-streams-outbound=<value>`

**Exceptions:** The `memory-limit-ratio` and `file-descriptors-ratio` limits are set as the following flags and both must be **between 0 and 1**:
- `--libp2p-resource-manager-memory-limit-ratio=<value>`
- `--libp2p-resource-manager-file-descriptors-ratio=<value>`
- For example: `--libp2p-resource-manager-memory-limit-ratio=0.5` means that the memory limit for libp2p resources is set to 50% of the available memory, i.e.,
    the libp2p can take up to 50% of the available memory on the system.

  
### Importance of Each Resource Scope
In the libp2p Resource Manager, scopes are organized hierarchically; `system`, `protocol`, `peer`, and `peer-protocol` scopes are arranged in a _descending order of priority_.
This means that the `system` scope has the highest priority `protocol` scope, `peer` scope, and `peer-protocol` scope.
As we explain later in this documentation, the `transient` scope is a special case and is not strictly fit in the hierarchy of scopes.

#### What does each scope mean?
   - **System Scope**: sets the global limits for the entire system and ensures overall stability and prevents resource hogging by any single component.
   - **Transient Scope**: manages resources for partially established connections or streams and prevents resource drainage during the establishment phase. 
       Transient resources are those that are not yet fully established, like a connection that's not yet fully established or a stream that's not yet fully opened. The transient scope is critical
       for guarding against resource drainage during the establishment phase.
   - **Protocol Scope**: sets limits for specific protocols (e.g., DHT, gossipsub) and prevents any single protocol from dominating resource usage. The protocol scope is essential for 
     protocol-specific resource tuning and preventing abuse by any single protocol.
   - **Peer Scope**: manages resources used by individual peers and prevents a single peer from exhausting resources. The peer scope is critical for preventing abuse by any single peer.
   - **Peer-Protocol Scope**: sets limits for specific peers on specific protocols and prevents any single peer from dominating resource usage on a specific protocol. It also prevents a single protocol 
      to dominate resource usage on a specific peer among all the protocols the peer is participating in. 

#### Scope Hierarchy
The higher scopes **"override"** limits on lower scopes:
1. **System Scope vs. Protocol/Peer Scopes**:
    - The system scope sets global limits. If the system scope has a lower limit than a protocol or peer scope, the system limit will be the effective constraint 
      because it's the upper bound for the entire system.
    - For example, if the system scope has an inbound stream limit of 10,000 and a protocol scope has a limit of 15,000, 
        the effective limit will be 10,000 because the system scope's limit applies globally.

2. **Protocol Scope vs. Peer Scope**:
    - The protocol scope sets limits for specific protocols, while the peer scope sets limits for individual peers. These are independent of each other but both are under the overarching system scope.
    - A peer can't exceed the limits set by the protocol scope, and vice versa. They operate within their own contexts but under the maximum limits imposed by the system scope.

It's essential to understand that the lowest limit in the hierarchy of applicable scopes will effectively be the operational limit. 
If the system inbound stream limit is lower than the protocol inbound stream limit, the system limit will effectively cap the maximum number of inbound streams, regardless of the higher limit set at the protocol level.
Also, the higher scopes limits must be configured in a way that they don't override the limits of lower scopes; rather, they add another layer of constraint. 
Each scope must independently satisfy its own limits without violating the limits of the scopes above it.
When configuring limits, it's crucial to consider the hierarchical nature of these scopes. 
Ensure that the limits at lower scopes (like protocol or peer) are set within the bounds of higher scopes (like system) to maintain a coherent and effective resource management strategy.

#### On Transient Scope
The `transient` scope in the libp2p Resource Manager hierarchy has a specific and unique role. 
It is placed **alongside** other scopes like `system`, `protocol`, `peer`, and `peer-protocol`, but it serves a distinct purpose. Here's how the `transient` scope fits into the hierarchy:
The `transient` scope is designed to manage resources for connections and streams that are in the process of being established but haven't yet been fully negotiated or associated with a specific peer or protocol.
This includes streams that are awaiting protocol negotiation or connections that are in the initial stages of establishment.

In terms of hierarchy, the `transient` scope is below `system`, but is not strictly above or below other scopes like `protocol`. 
Instead, it operates more as a parallel scope that specifically handles resources in a temporary, intermediate state.
While the `system` scope sets the global limits, the `transient` scope sets limits on resources that are not yet fully categorized into other specific scopes (like `peer` or `protocol`).
The limits set in the `transient` scope are independent of those in the `protocol`, `peer`, and `peer-protocol` scopes but still operate under the overarching constraints of the `system` scope. 
Once a connection or stream transitions out of the `transient` state (i.e., when a protocol is negotiated, or a peer is identified), it then falls under the appropriate scope (such as `protocol` or `peer`) and adheres to the limits set within those scopes.
The `transient` scope is critical for managing resources during the negotiation phase of connections and streams. It helps in protecting the system against resource exhaustion attacks that can occur at the initial stages of connection or stream establishment.

**Example:** For example, when the limit for system-wide connections is set lower than the limit for transient-wide connections in the libp2p Resource Manager, the system-wide limit effectively becomes the constraining factor
In this example, the system-wide connections limit acts as the global cap for all connections in the libp2p network, regardless of their state (established, transient, etc.).
If this limit is lower than the transient-wide limit, it essentially restricts the total number of connections (including transient ones) to this lower system-wide limit.
The transient-wide limit is intended to manage connections that are in the process of being fully established. 

### Case Study: what does scopes mean in terms of one resource?
As an example, we study the default limits for "Streams Inbound/Outbound" at different scopes in the libp2p Resource Manager. The limtis on other resources follow a similar pattern.
Here's an explanation of what these default limits mean at each scope:

### System Scope
- **Streams Inbound/Outbound (e.g., 15,000)**:
    - **Meaning**: This limit defines the maximum number of inbound and outbound streams that can be active across the entire system, regardless of the specific protocols or peers involved.
    - **Implication**: It is a global cap ensuring that the total number of streams at any time does not exceed this limit, thus preventing system-wide resource exhaustion due to too many streams.

### Transient Scope
- **Streams Inbound/Outbound (e.g., 15,000)**:
    - **Meaning**: This limit controls the number of streams in the transient state, i.e., streams that are being set up but not yet fully established or associated with a peer/protocol.
    - **Implication**: It provides a buffer for handling stream negotiations, ensuring the system can manage a high volume of initiating connections without overwhelming the resources during the setup phase.

### Protocol Scope
- **Streams Inbound/Outbound (e.g., 5,000)**:
    - **Meaning**: This limit specifies the maximum number of inbound and outbound streams for each protocol. It applies to each protocol independently.
    - **Implication**: It ensures that no single protocol can dominate the network's resources, maintaining a balance in resource allocation among various protocols.

### Peer Scope
- **Streams Inbound/Outbound (e.g., 1,000)**:
    - **Meaning**: This sets the maximum number of inbound and outbound streams allowed per (remote) peer on the local peer.
    - **Implication**: It restricts the resource usage by individual peers, ensuring no single (remote) peer can exhaust network resources with too many streams.

### Peer-Protocol Scope
- **Streams Inbound/Outbound (e.g., 500)**:
    - **Meaning**: This limit is the most granular, applying to streams from each (remote) peer for each protocol on the local peer.
    - **Implication**: It offers fine-grained control, preventing any (remote) peer from using excessive resources in a specific protocol on the local peer, thus ensuring balanced resource use.

## Troubleshooting (For Flow Node Operators)
This troubleshooting guide works based on the case of excessive streams in the network. Similar guidelines can be applied to other resources as well.

### Observation
If you observe an excessive number of open streams (or open `goroutines` affiliated with a libp2p protocol) in your network, 
the appropriate action would be to adjust the stream limits within specific scopes, depending on the nature of the issue.

### 1. Excessive Streams Across All Protocols and Peers
- **Scope**: System Scope
- **Limits to Adjust**:
    - `streams-inbound`
    - `streams-outbound`
- **Reason**: The system scope applies globally across all peers and protocols. Adjusting these limits helps manage the overall number of streams in the network.

### 2. Excessive Streams in a Specific Protocol
- **Scope**: Protocol Scope
- **Limits to Adjust**:
    - `streams-inbound`
    - `streams-outbound`
- **Reason**: If a particular protocol (e.g., DHT, gossipsub) is opening too many streams, tightening limits in the protocol scope can restrict the resource usage by that specific protocol.

### 3. Excessive Streams from Individual Peers
- **Scope**: Peer Scope
- **Limits to Adjust**:
    - `streams-inbound`
    - `streams-outbound`
- **Reason**: When specific peers are opening too many streams, adjusting these limits can prevent any single peer from using an excessive number of streams.

### 4. Excessive Streams from a Specific Peer on a Specific Protocol
- **Scope**: Peer-Protocol Scope
- **Limits to Adjust**:
    - `streams-inbound`
    - `streams-outbound`
- **Reason**: This is the most granular level of control, where you can restrict stream usage for a specific protocol used by a specific peer.

### Wildcard: Increasing all limit overrides at scale
In order to preserve the hierarchy of scopes, you need to adjust the limits in each scope in a way that they don't override the limits of higher scopes.
One easy way is to increase all limits by a certain factor across all scopes. For example, if you want to increase all limits by 1.5 times, you can do so by adjusting the flags for each limit within each scope.

### System Scope
1. **Streams Inbound/Outbound**
    - `--libp2p-resource-manager-system-streams-inbound=<1.5 * current value>`
    - `--libp2p-resource-manager-system-streams-outbound=<1.5 * current value>`
2. **Connections Inbound/Outbound**
    - `--libp2p-resource-manager-system-connections-inbound=<1.5 * current value>`
    - `--libp2p-resource-manager-system-connections-outbound=<1.5 * current value>`
3. **File Descriptors**
    - `--libp2p-resource-manager-system-fd=<1.5 * current value>`
4. **Memory Bytes**
    - `--libp2p-resource-manager-system-memory-bytes=<1.5 * current value>`

### Transient Scope
1. **Streams Inbound/Outbound**
    - `--libp2p-resource-manager-transient-streams-inbound=<1.5 * current value>`
    - `--libp2p-resource-manager-transient-streams-outbound=<1.5 * current value>`
2. **Connections Inbound/Outbound**
    - `--libp2p-resource-manager-transient-connections-inbound=<1.5 * current value>`
    - `--libp2p-resource-manager-transient-connections-outbound=<1.5 * current value>`
3. **File Descriptors**
    - `--libp2p-resource-manager-transient-fd=<1.5 * current value>`
4. **Memory Bytes**
    - `--libp2p-resource-manager-transient-memory-bytes=<1.5 * current value>`

### Protocol Scope
1. **Streams Inbound/Outbound**
    - `--libp2p-resource-manager-protocol-streams-inbound=<1.5 * current value>`
    - `--libp2p-resource-manager-protocol-streams-outbound=<1.5 * current value>`

### Peer Scope
1. **Streams Inbound/Outbound**
    - `--libp2p-resource-manager-peer-streams-inbound=<1.5 * current value>`
    - `--libp2p-resource-manager-peer-streams-outbound=<1.5 * current value>`

### Peer-Protocol Scope
1. **Streams Inbound/Outbound**
    - `--libp2p-resource-manager-peer-protocol-streams-inbound=<1.5 * current value>`
    - `--libp2p-resource-manager-peer-protocol-streams-outbound=<1.5 * current value>`

### Notes
- Replace `<1.5 * current value>` with the actual calculated value from `default-config.yaml`. For example, if the current system streams inbound limit is 10,000, the new value would be `--libp2p-resource-manager-system-streams-inbound=15000`.
