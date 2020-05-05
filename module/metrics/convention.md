# Metrics Conventions
Rough overview of how metrics should be named and used

## Naming
### Namespace
The component of code this metric is under.
- If it's under a module, use the module name.
  eg: `hotstuff`, `network`, `storage`, `mempool`, `interpreter`, `crypto`
- If it's a core metric from a node, use the node type.
  eg: `consensus`, `verification`, `access`

### Subsystem
Within the component, describe the part or function referred to.

Subsystem is optional if the entire namespace is small enough to not be segmented further.

eg: under `hotstuff`: `follower`, `...`

eg: under `storage`: `badger`, `trie`, `cache`

## Constant Labels
Add labels for constant information

Const Label | Value
------------|------
node_role   | [collection, consensus, execution, verification, access]
beta_metric | `true`
