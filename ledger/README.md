# Flow Ledger Package

**Ledger** is a stateful fork-aware key/value storage. Any update (value change for a key) to the ledger generates a new ledger state. Updates can be applied to any recent state. These changes don't have to be sequential and ledger supports a tree of states. Ledger provides value lookup by key at a particular state (historic lookups) and can prove the existence/non-existence of a key-value pair at the given state. Ledger assumes the initial state includes all keys with an empty bytes slice as value.

This package provides two ledger implementations:

- **Complete Ledger** implements a fast, memory-efficient and reliable ledger. It holds a limited number of recently used states in memory (for speed) and uses write-ahead logs and checkpointing to provide reliability. Under the hood complete ledger uses a collection of MTries(forest). MTrie is a customized in-memory binary Patricia Merkle trie storing payloads at specific storage paths. The payload includes both key-value pair and storage paths are determined by the PathFinder. Forest utilizes unchanged sub-trie sharing between tries to save memory.

- **Partial Ledger** implements the ledger functionality for a limited subset of keys. Partial ledgers are designed to be constructed and verified by a collection of proofs from a complete ledger. The partial ledger uses a partial binary Merkle trie which holds intermediate hash value for the pruned branched and prevents updates to keys that were not part of proofs.

## Definitions

### binary Merkle tree

![binary Merkle tree image](/ledger/docs/binary_merkle_tree.png?raw=true "binary Merkle tree" )

In this context a *binary Merkle tree* is defined as a full binary tree with an specific hight including three type of nodes:

- leaf nodes: holds a payload (data), a path (where the node is located), and a hash value (hash of path and payload content)

- empty leaf nodes: doesn't hold any data and only stores a path, and a default hash value based on the height of tree

- intermediate nodes: holds a path and a hash value which is defined as hash of hash value of left and right children.

![node types image](/ledger/docs/node_types.png)

A *path* is a unique address of a node storing a payload. Paths are derived from the content of payloads (see common/pathfinder).

![paths image](/ledger/docs/paths.png?raw=true "paths")

#### Operations

Get: Fetching a payload from the binary Merkle tree is by traversing the tree based on path bits. (0: left branch, 1: right branch)

Update: Updates to the tree starts with traversing the tree to the leaf node, updating payload, hash value of that node and hash value of all the ancestor nodes (nodes on higher level connected to this node).

![update image](/ledger/docs/tree_update.gif?raw=true "update")

Prove: A binary Merkle tree can provide an inclusion proof for any given payload. A Merkle proof in this context includes all the information needed to walk through a tree branch from an specific leaf node (key) up to the root of the tree (yellow node hash values are needed for inclusion proof for the green node).

![proof image](/ledger/docs/proof.png?raw=true "proof")

### binary Merkle *trie*
A *binary Merkle trie* in this context is defined as a compact version of binary Merkle tree, providing exact same functionality but don’t store unnecessary nodes.

![binary partial trie image](/ledger/docs/trie_update.gif?raw=true "binary partial trie")

### forest 
A *forest* holds a set of binary Merkle tries, given a parent trie and a batch of updates it creates a new binary merkle trie and adds it to the forest.

![forest image](/ledger/docs/forest.png?raw=true "forest")

### compact forest 
A *compact forest* construct new trie after each update (copy on change) and reuses unchanged sub-tries from the parent.

![compact forest image](/ledger/docs/reuse_sub_trees.gif?raw=true "compact forest")

### path finder 
*Path finder* deterministically computes a path for a given payload. Path finder is responsible to make sure the trie grows in balance and relevant data are stored nearby so that the proof size can be optimized.

### partial binary Merkle trie
A *partial Merkle trie* is similar to a Merkel trie but only keeping a subset of nodes and having intermediate nodes without the full subtrie. It can be constructed from batch of inclusion and non-inclusion proofs. It provides functionality to verify outcome of updates to a trie without the need to have the full trie.

![partial trie image](/ledger/docs/partial_trie.png?raw=true "partial trie")
