# Memory-Trie: `MTrie`

At its heart, an `MTrie` is an in-memory key-value store, with the ability to generate cryptographic proofs
for the states of the stored registers.
`MTrie` is a flavour of a Merkle [Radix Tree](https://en.wikipedia.org/wiki/Radix_tree). 

## Storage Model
Formally, an `MTrie` represents a *perfect*, *full*, *binary* Merkle tree with *uniform height*. 
We follow the established [graph-theoretic terminology](https://en.wikipedia.org/wiki/Binary_tree). 
In our terminology, we explicitly differentiate between:
 * **tree**: full binary Merkle tree with uniform height. The storage model is defined for the tree.
 * `MTrie`: is an optimized in-memory structure representing the tree
 
### Underling Graph-Theoretical Storage Model

The storage model is defined for the tree. At its heart, it is a key-value store.
In the store, there are a fixed number of storage slots, which we refer to as **registers**.
By convention, each register has a key (memory address) and a value 
(the binary blob stored in that memory slot). While all register keys have the same fixed length
(measured in bits), the values are variable-length byte slices.
We define an **unallocated register** as holding no data, i.e. an empty byte slice.
By default, each register is unallocated. In contrast, an **allocated_ register**
holds a value with positive storage size, i.e. a byte slice with length larger than zero.
Note that do not introduce the concept of registers values being `nil`. 

The theoretical storage model is a *perfect*, *full*, *binary* Merkle tree, which
spans _all_ registers (even if they are unallocated).   
Therefore, we have two different node types in the tree:
 * A **LEAF** node represents a register:
    - holding a `key` and a `value`; 
    - following establish graph-theoretic conventions, the `height` of a leaf is zero.
    - the `hash` value is defined as:
      - For and _unallocated_ register, the `hash` is just the hash of a global constant.
        Therefore, the leafs for all unallocated registers have the same hash.
        We refer to the hash of an unallocated register as `default hash at height 0`.
      - For  _allocated_ registers, the `hash` value is `H(key, value)` for `H` the hash function.
 * An **INTERIM** node is a vertex in the tree with
    - has exactly two children, called `LeftChild` and `RightChild`, which are both of the same height;
      the children can either be leaf or interim nodes; 
    - the `height` of an interim node `n` is `n.height = LeftChild.height + 1 = RightChild.height + 1`;
      (Hence, an interim node `n` can only have a `n.height > 0`, as only leafs have height zero.)  
    - the `hash` value is defined as `H(LeftChild, RightChild)`
   
#### convention for mapping a register `key` to a path in the tree

**Conventions:**
* let `key[i]` be the bit with index `i` (we use zero-based indexing)
* a `key` can be converted into its `integer representation` though big-endian ordering
* given a `key` and an index `i`, we define:
  - the `prefix` as `key[:i]` (excluding the bit up to, but not including, the bit with index `i`)
* the tree's root node partitions the register set into to sub-sets 
  depending on value `key[0]` :
  - all registers `key[0] == 0` fall into the `LeftChild` subtree
  - all registers `key[0] == 1` fall into the `RightChild` subtree
* All registers in `LeftChild`'s subtree, now have the prefix `key[0:1] = [0]`.
  `LeftChild`'s respective two children partition the register set further 
  into all registers with the common key prefix `0,0` vs `0,1`.  
* Let `n` be interim node with a path length to the root node of `d` [edges].
  Then, all registers that fall in `n`'s subtree share the same prefix `key[0:d]`.
  Furthermore, partition this register set further into 
  - all registers `key[d] == 0` fall into the `n.LeftChild` subtree
  - all registers `key[d] == 1` fall into the `n.RightChild` subtree
    
Therefore, we have the following relation between tree height and key length:
 * Let the tree hold registers with key length `len(key) = K` [bits].
   Therefore, the tree has _interim nodes_ with `height` values: `K` (tree root),
   `K-1` (root's children), ..., `1`. The interim nodes with `height = 1`
   partition the resisters according to their last bit. Their children a leaf nodes
   (which have zero height).   
 * Let `n` be an interim node with height `n.height`. Then, we can associate `n` with 
   the key index `i := K - n.height`.    
    - `n`'s prefix is then the defined as `p = key[:i]`, which is shared by 
      all registers that fall into `n`'s subtree. 
    - `n` partitions its register set further:
      all registers with prefix `p,0` fall into `n.LeftChild`'s subtree;      
      all registers with `p,1` fall into `n.LeftChild`'s subtree.     

Note that our definition of height follows established graph-theoretic conventions: 
```
The HEIGHT of a NODE v in a tree is the number of edges on the longest downward path between v and a tree leaf.
The HEIGHT of a TREE is the height of its root node.
``` 

Our storage model generates the following property, which is very beneficial
for optimizing the implementation:   
* A sub-tree holding only _unallocated_ registers hashes to a value that 
  only depends on the height of the subtree 
  (but _not_ on which specific registers are included in the tree).
  Specifically, we define the `defaultHash`, which is recursively defined. 
  The `defaultHash[0]` of an unallocated leaf node is a global constant. 
  Furthermore, `defaultHash[h]` is the subtree-root hash of 
  a subtree with height `h` that holds only _unallocated_ registers.


#### `MTrie` as an Optimized Storage implementation

Storing the perfect, full, binary Merkle tree with uniform height in its raw form is very
memory intensive. Therefore, the `MTrie` data structure employs a variety of optimizations
to reduce its memory and CPU footprint. Nevertheless, from an `MTrie`, the full tree can be constructed.  

On a high level, `MTrie` has the following optimizations: 
* **sparse**: all subtrees holding only _unallocated_ register are pruned:
  - Consider an interim node with height `h`. 
    Let `c` be one of its children, i.e. either `LeftChild` or `RightChild`. 
  - If `c == nil`, the subtree-root hash for `c` is `defaultHash[h-1]` 
    (correctness follows directly from the storage model) 
* **Compactification**:
  Consider a register with its repsective path from the root to the leaf in the _full binary tree_.
  When traversing the tree from the root down towards the leaf, there will come a node `Ω`, which only 
  contains a _single_ allocated register. Hence, `MTrie` pre-computes the root hash of such trees and 
  store them as a **compactified leaf**. Formally, a compactified leaf stores 
    - a `key` and a `value`; 
    - a height value `h`, which can zero or larger
    - its hash is: subtree-root hash for a tree that only holds `key` and `value`. 
      To compute this hash, we essentially start with `H(key, value)` and hash our way 
      upwards the tree until we hit height `h`. While klimbing the tree upwards, 
      we use the respective `defaultHash[..]` for the other branch which we are merger with. 
   
Furthermore, an `MTrie` 
* uses `SHA-3-256` as hash function `H`
* the registers have keys with `len(key) = 8*l [bits]`, for `l` the key size in bytes
* the height of `MTrie` (per definition, the `height` of the root node) is also `8*l`,
  for `l` the key size in bytes  



