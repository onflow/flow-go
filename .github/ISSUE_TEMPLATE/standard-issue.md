---
name: Standard issue
about: Our standard issue
title: ''
labels: ''
assignees: ''

---

## Context

Why are we doing this work? How does it interact with other work being done? For example:

Access nodes need to receive transactions in order to store them, so we need a communication solution for relaying transactions between access nodes.

## Definition of Done

What do we need to achieve to consider this issue resolved? For example:

* Access nodes must send transactions to at least some of their peers
* Processing hooks are in place for received transactions, to be implemented in a later issue
* Transactions must be received by the entire network, although this doesn't have to be immediate (i.e. transactions could be gossiped)

## Further reading:

Any additional materials that provide context and background, beyond the typical scope for an issue. For example this might be links to the Bamboo technical primer.
