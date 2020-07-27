# Flow Ledger Package**

**Ledger** is a stateful fork-aware key/value storage. Any update (value change for a key) to the ledger generates a new ledger state. Updates can be applied to any recent state. These changes don't have to be sequential and ledger supports a tree of states. Ledger provides value lookup by key at a particular state (historic lookups) and can prove the existence/non-existence of a key-value pair at the given state. Ledger assumes the initial state includes all keys with an empty bytes slice as value.

This package provides two various implementation of ledger:

- **Complete Ledger** implements a fast, memory-efficient and reliable ledger. It holds a limited number of recently active states in memory (for speed) and uses write-ahead logs and checkpointing to provide reliability. Under the hood complete ledger uses a collection of MTries (forest). MTrie is a customized in-memory binary Patricia Merkle trie storing payloads at specific storage paths. The payload includes both key-value pair and storage paths are determined by the PathFinder. Forest utilizes unchanged sub-trie sharing between tries to save memory.

- **Partial Ledger** implements the ledger functionality for a limited subset of keys. Partial ledgers are designed to be constructed and verified by a collection of proofs from a complete ledger. The partial ledger uses a partial binary Merkle trie which holds intermediate hash value for the pruned branched and prevents updates to keys that were not part of proofs.