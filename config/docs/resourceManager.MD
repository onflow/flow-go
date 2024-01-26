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
In the libp2p Resource Manager, scopes are organized hierarchically; `system`, `transient`, `protocol`, `peer`, and `peer-protocol` scopes are arranged in a _descending order of priority_.
This means that the `system` scope has the highest priority, followed by the `transient` scope, `protocol` scope, `peer` scope, and `peer-protocol` scope.

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

3. **Practical Implications**:
    - It's essential to understand that the lowest limit in the hierarchy of applicable scopes will effectively be the operational limit.
    - If the system inbound stream limit is lower than the protocol inbound stream limit, the system limit will effectively cap the maximum number of inbound streams, regardless of the higher limit set at the protocol level.

4. **No Override, but Cumulative Constraints**:
    - Higher scopes limits must be configured in a way that they don't override the limits of lower scopes; rather, they add another layer of constraint.
    - Each scope must independently satisfy its own limits without violating the limits of the scopes above it.

5. **Configuration Strategy**:
    - When configuring limits, it's crucial to consider the hierarchical nature of these scopes.
    - Ensure that the limits at lower scopes (like protocol or peer) are set within the bounds of higher scopes (like system) to maintain a coherent and effective resource management strategy.