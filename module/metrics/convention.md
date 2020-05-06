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

### Naming
The actual metric name should clearly identify what's actually being collected.
The name should be like an inverse domain, and get more specific left to right.

eg: `transaction_size_bytes` rather than `size_of_transaction`

If the metric is in the form of a verb, it should be in the past tense.
`finalized`, `issued`, `sent`

Metrics should always be suffixed with the unit the metric is in, pluralized.
`seconds`, `bytes`, `messages`, `transactions`, `chunks`

Use the [Prometheus standard base units](https://prometheus.io/docs/practices/naming/#base-units).
Avoid `milliseconds`, `bits`, etc

If the metrics is an ever accumulating counter, it should be additionally suffixed with `total` after the unit.
`seconds_total`, `transations_total`

Do not repeat any of the terms used in `Namespace` or `Subsystem` in the metric name.

## Constant Labels
Add labels for constant information

Const Label | Value
------------|------
node_role   | [collection, consensus, execution, verification, access]
beta_metric | `true` *only set if the metric is in beta
