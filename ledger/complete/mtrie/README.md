# Memory-Trie: `MTrie`

At its heart, an `MTrie` is an in-memory key-value store, with the ability to generate cryptographic proofs
for the states of the stored registers.  `MTrie` combines features of [Merkle trees](https://en.wikipedia.org/wiki/Merkle_tree) 
(for generating cryptographic proofs for the stored register) and [Radix Trees](https://en.wikipedia.org/wiki/Radix_tree)
(for optimized memory consumption).

By construction, `MTrie`s are _immutable data structures_. Essentially, they represent a snapshot of the key-value store
for one specific point in time. Updating register values is implemented through
copy-on-write, which creates a new `MTrie`, i.e. a new snapshot of the updated key-value store.
For minimal memory consumption, all sub-tries that were not affected by the write 
operation are shared between the original `MTrie` (before the register updates) and the updated `MTrie`
(after the register writes).

## Storage Model
Formally, an `MTrie` represents a *perfect*, *full*, *binary* Merkle tree with *uniform height*. 
We follow the established [graph-theoretic terminology](https://en.wikipedia.org/wiki/Binary_tree). 
We explicitly differentiate between:
 * **tree**: full binary Merkle tree with uniform height. The storage model is defined for the tree.
 * `MTrie`: is an optimized in-memory structure representing the tree.
 
### Underling Graph-Theoretical Storage Model

The storage model is defined for the tree. At its heart, it is a key-value store.
In the store, there are a fixed number of storage slots, which we refer to as **registers**.
By convention, each register has a key (identifying the storage slot) and a value 
(binary blob) stored in that memory slot. A key identifies the storage slot through an address 
derived from the key, called path. While all register paths have the same fixed length
(measured in bits), the keys and values are variable-length byte slices. A register holds both the key and value, 
which forms a payload. A path is derived deterministically from the key part of the payload. 
We define an **unallocated register** as holding no value, i.e. a nil payload or an empty value byte slice.
By default, each register is unallocated. In contrast, an **allocated_ register**
holds a non-nil payload and a value with positive storage size, i.e. a byte slice with length larger than zero.
Note that we do not introduce the concept of registers with `nil` values. 

The theoretical storage model is a *perfect*, *full*, *binary* Merkle tree, which
spans _all_ registers (even if they are unallocated).   
Therefore, we have two different node types in the tree:
 * A **LEAF** node represents a register:
    - holding a payload, i.e a `key` and a `value`. 
    - holding a path, which is derived from the payload key.
    - following established graph-theoretic conventions, the `height` of a leaf is zero.
    - the `hash` value is defined as:
      - For an _unallocated_ register, the `hash` is just the hash of a global constant.
        Therefore, the leafs for all unallocated registers have the same hash.
        We refer to the hash of an unallocated register as `default hash at height 0`.
      - For  _allocated_ registers, the `hash` value is `H(path, value)` for `H` the hash function.
 * An **INTERIM** node is a vertex in the tree:
    - it has exactly two children, called `LeftChild` and `RightChild`, which are both of the same height;
      the children can either be leafs or interim nodes. 
    - the `height` of an interim node `n` is `n.height = LeftChild.height + 1 = RightChild.height + 1`;
      (Hence, an interim node `n` can only have a `n.height > 0`, as only leafs have height zero).  
    - the `hash` value is defined as `H(LeftChild, RightChild)`
   
#### Convention for mapping a register `key` to a path in the tree

**Conventions:**
* let `path[i]` be the bit with index `i` (we use zero-based indexing)
* a `path` can be converted into its `integer representation` through big-endian ordering
* given a `path` and an index `i`, we define:
  - the `prefix` as `path[:i]` (excluding the bit with index `i`)
* the tree's root node partitions the register set into two sub-sets 
  depending on value `path[0]` :
  - all registers `path[0] == 0` fall into the `LeftChild` subtree
  - all registers `path[0] == 1` fall into the `RightChild` subtree
* All registers in `LeftChild`'s subtree, now have the prefix `path[0:1] = [0]`.
  `LeftChild`'s respective two children partition the register set further 
  into all registers with the common key prefix `0,0` vs `0,1`.  
* Let `n` be an interim node with a path length to the root node of `d` [edges].
  Then, all registers that fall in `n`'s subtree share the same prefix `path[0:d]`.
  Furthermore, partition this register set further into 
  - all registers `path[d] == 0` fall into the `n.LeftChild` subtree
  - all registers `path[d] == 1` fall into the `n.RightChild` subtree
    
Therefore, we have the following relation between tree height and path length:
 * Let the tree hold registers with path length `len(path) = K` [bits].
   Therefore, the tree has _interim nodes_ with `height` values: `K` (tree root),
   `K-1` (root's children), ..., `1`. The interim nodes with `height = 1`
   partition the registers according to their last bit. Their children are leaf nodes
   (which have zero height).   
 * Let `n` be an interim node with height `n.height`. Then, we can associate `n` with 
   the path index `i = K - n.height`.    
    - `n`'s prefix is then the defined as `p = path[:i]`, which is shared by 
      all registers that fall into `n`'s subtree. 
    - `n` partitions its register set further:
      all registers with prefix `p,0` fall into `n.LeftChild`'s subtree;      
      all registers with `p,1` fall into `n.RightChild`'s subtree.     

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
  Specifically, we define the `defaultHash` in a recursive manner. 
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
  Consider a register with its respective path from the root to the leaf in the _full binary tree_.
  When traversing the tree from the root down towards the leaf, there will come a node `Ω`, which only 
  contains a _single_ allocated register. Hence, `MTrie` pre-computes the root hash of such trees and 
  store them as a **compactified leaf**. Formally, a compactified leaf stores 
    - a payload with a `key` and a `value`;
    - a path derived from the payload key. 
    - a height value `h`, which can be zero or larger
    - its hash is: subtree-root hash for a tree that only holds `key` and `value`. 
      To compute this hash, we essentially start with `H(path, value)` and hash our way 
      upwards the tree until we hit height `h`. While climbing the tree upwards, 
      we use the respective `defaultHash[..]` for the other branch which we are merging with. 
   
Furthermore, an `MTrie` 
* uses `SHA3-256` as the hash function `H`
* the registers have paths with `len(path) = 8*l [bits]`, for `l` the path size in bytes.
* the height of `MTrie` (per definition, the `height` of the root node) is also `8*l`,
  for `l` the path size in bytes.  
* l is fixed to 32 in the current implementation, which makes paths be 256-bits long 
and the trie root at a height 256.
  
### The Mtrie Update algorithm:

Updating register payloads of the mtrie is implemented through copy-on-write, 
which creates a new `MTrie`, i.e. a new snapshot of the updated key-value store.
For minimal memory consumption, all sub-tries that were not affected by the update 
operation are shared between the original `MTrie` and the updated `MTrie`. This means 
children of some new nodes of the new `Mtrie` point to existing sub-tries from the 
original `Mtrie`.

The update algorithm takes a trie `m` as input along with a set of `K` 
pairs: `(paths[k], payloads[k])` where `k = 0, 1, ..., K-1`.
It outputs a new trie `new_m` such that each payload `payloads[k]` is stored at the path `paths[k]`.  
Any path that is not included in the input pairs keeps the payload from the original input `m`.

We first describe the algorithm to perform only a single register update `(path, payload)` 
and subsequently generalize to an arbitrary number of `K` register updates. 
Given a root node of a trie, a path and a payload,
we look for the register addressed by the given path in a recursive top-down manner. Each recursive step 
operates at a certain height of the tree, starting from the root. Looking at the respective bit 
`path[i]` (with `i = 256 - height`) of the path, we recursively descend into the left or right child to apply the update.
For each node visited on the recursive descent, we create a new node at the respective height to represent the updated 
sub-trie. If the sub-trie is not affected by the update, we re-use the same visited node. 

We define the function `Update` to implement the recursive algorithm to apply the register update.
`Update` takes as inputs:
* `node` is the vertex of the trie before the register update. The `Update` method should return `node` if there are no updates in the respective sub-trie (with root `node`), to avoid unnecessary data duplication. If `node` is `nil`, then there is no candidate in the trie before the register update that could be returned and a new node must necessarily be created.
* Height is the `height` of the returned node in the new trie.
* The update `(path, payload)` which is supposed to be written to this particular sub-trie.
* (optional) The compactified leaf (denoted as `compactLeaf`) carried over from a larger height.
  If no compactfied leaf is carried over, then `compactLeaf = nil`. (The recursive algorithm uses this parameter, when it needs to expand a compactified 
  leaf into a sub-trie holding multiple registers.)


During the recursion, we can encounter the following cases:
* **Case 0: `node` is an interim node.** (generic recursion step) As described, we further descend down the left or right child, depending on the bit value `path[i]`. 
* **Case 1: `node` is a leaf.** A leaf can be either fully expanded (height zero) or compactified (height larger than zero). 
  - **case 1.a: `node.path == path`**, i.e. the leaf's represents the register that we are looking to update. 
    The tree update is done by creating a new node with the new input `payload`. 
  - **case 1.b: `node.path ≠ path`**, i.e. the leaf represents a _different_ register than the one we want to update.
    While the register with `path`, falls in the same sub-trie as the allocated register, it is still unallocated.  
    This implies that `node` must be a compactified leaf. 
    Therefore, in the updated trie, the previously compactified leaf has to be replaced by sub-trie containing
    two allocated registers. We recursively proceed to 
    write the contents of the previously existing register as well as the new register `(path, payload)` to the
    interim-node's children. We set `compactLeaf := node` and continue the recursive construction 
    of the new the sub-tree. 
* **Case 2: `node == nil`**: A `nil` sub-trie means that the sub-trie is empty and at least a new leaf has to be created. 
  - **case 2.a: there is only one leaf to create**. If there is only one leaf to create (either the one representing the input `(path, payload)`, 
or the one representing a compactified leaf carried over from a higher height), then a new leaf is created.
The new leaf can be either fully expanded or compactified. 
  - **case 2.b: there are 2 leafs to create**. If there are 2 leafs to create (both the input `(path, payload)` and the compactified leaf carried over),
then we are still at an interim-node height. Hence, we create a new interim-node with `nil` children, check the path index of both the input `path`
and the compactified node `path` and continue the recursion over the children. Eventually the recursion calls will fall into 2.a 
as we reach the first different bit index between the 2 paths. This case can be seen as a special case of the  
generic case 0 above, but just called with a `node = nil`. 

#### General algorithm

We now generalize this algorithm to an arbitrary number of `K` register updates: `(paths[k], payloads[k])` where `k = 0, 1, ..., K-1`.

- `Update` takes a list (slice) of `paths` and `payloads`. 
 -  When moving to a lower height, `paths` and `payloads` are partitioned depending on `paths[k][i]` with `i = 256 - height`. 
    The first partition has all updates for registers with `paths[k][i] = 0` and goes into the left child recursion, while the second partition has the updates
    pertaining to registers with `paths[k][i] = 1` and goes into the right child recursion. 
    This results in sorting the overall input `paths` using an implicit quick sort.
 - if `len(paths) == 0` and there is no compact leaf carried over (`compactLeaf == nil`), no update will be done 
and the original sub-trie can be re-used in the new trie.


* **Case 0: `node` is an interim node.** An interim-node is created, the paths are split into left and right.
* **Case 1: `node` is a leaf.** Instead of comparing the path of the leaf with the unique input path, the leaf path is linearly searched within
all the input paths. Case 1.a is when the leaf path is found among the inputs, Case 1.b is when the leaf path is not found. 
  The linear search in the recursive step has an overall complexity `O(K)` (for all recursion steps combined). 
  Case 1.a is now split into two subcases:
    - **case 1.a.i: `node.path ∈ path` and `len(paths) == 1`**. A new node is created with the new updated payload. This would be a leaf in the new trie.
    - **case 1.a.ii: `node.path ∈ path` and `len(paths) > 1`**. We are necessarily on a compactified leaf and we don't care about its own payload as it will get 
      updated by the new input payload. We therefore continue the recursion with `compactLeaf = nil` and the same input paths and payloads.
    - **case 1.b: `node.path ∉ path`**. If the leaf path is not found among the inputs, `node` must be a compactified leaf
      (as multiple different registers fall in its respective sub-trie). We call the recursion with the same inputs but with `compactLeaf` being set to the current node. 
* **Case 2: `node == nil`** : The sub-trie is empty
    - **Case 2a: `node == nil` and there is only one leaf to create**, i.e. `len(paths) == 1 && compactLeaf == nil` or `len(paths) == 0 && compactLeaf ≠ nil`.
    - **Case 2b: there are 2 or more leafs to create**. An interim-node is created, the paths are split into left and right, and `compactLeaf` is carried over into the left or right child. We note that this case is very similar to Case 0 where the current node is `nil`. The pseudo-code below will treat case 0 and 2.b in the same code section. 

**Lemma**:
_Consider a trie `m` before the update. The following condition holds for the `Update` algorithm: If `compactLeaf ≠ nil` then `node == nil`._
By inversion, the following condition also holds: _If `node ≠ nil` then `compactLeaf == nil`._

Proof of Lemma:

Initially, the `Update` algorithm starts with:
* `node` is set to the trie root
* `compactLeaf` is `nil`
The initial condition satisfies the lemma. 

Let's consider the first recursion step where `compactLeaf` may switch from `nil` (initial value)
to a non-`nil` value. This switch happens only in Case 1.b where we replace a compactified leaf by a trie holding multiple
registers. In this case (1.b), a new interim-node with `nil` children is created, and the recursion is carried forward with `node` being set to the `nil` children. The following steps will necessary fall under case 2 since `node` is `nil`. Subcases of case 2 would always keep `node` set to `nil`. 

Q.E.D.

#### Further optimization and resource-exhaustion attack:
In order to counter a resource-exhaustion attack where an existing allocated register is being updated with the same payload, resulting in creating new unnecessary nodes, we slightly adjust step 1.a. When `len(paths)==1` and the input path is equal to the current leaf path, we only create a new leaf if the input payload is different than the one stored initially in the leaf. If the two payloads are equal, we just re-cycle the initial leaf.
Morever, we create a new interim-node from the left and right children only if the returned children are different than the original node children. If the children are equal, we just re-cycle the same interim-node. 

#### Putting everything together:
This results in the following `Update` algorithm. When applying the updates `(paths, payloads)` to a trie with root node `root` 
(at height 256), the root node of the updated trie is returned by `Update(256, root, paths, payloads, nil)`.


```golang
FUNCTION Update(height Int, node Node, paths []Path, payloads []Payload, compactLeaf Node, prune bool) Node {
 if len(paths) == 0 {
  // If a compactLeaf from a higher height is carried over, then we are necessarily in case 2.a 
  // (node == nil and only one register to create)
  if compactLeaf != nil {
   return NewLeaf(compactLeaf.path, compactLeaf.payload, height)
  }
  // No updates to make, re-use the same sub-trie
  return node
 }
 
 // The remaining sub-case of 2.a (node == nil and only one register to create): 
 // the register payload is the input and no compactified leaf is to be carried over. 
 if len(paths) == 1 && node == nil && compactLeaf == nil {
  return NewLeaf(paths[0], payloads[0], height)
 }
 
 // case 1: we reach a non-nil leaf. Per Lemma, compactLeaf is necessarily nil
 if node != nil && node.IsLeaf() { 
  if node.path ∈ paths {
    if len(paths) == 1 { // case 1.a.i
     // the resource-exhaustion counter-measure
     if !node.payload == payloads[i] {
      return NewLeaf(paths[i], payloads[i], height)
     }
     return node  // re-cycle the same node
    }
    // case 1.a.ii: len(paths)>1
    // Value of compactified leaf will be overwritten. Hence, we don't have to carry it forward. 
    // Case 1.a.ii is the call: Update(height, nil, paths, payload, nil), but we can optimize the extra call and just continue the function to case 2.b with the same parameters.
  } else {
   // case 1.b: node's path was not found among the inputs and we should carry the node to lower heights as a compactLeaf parameter.
   // Case 1.b is the call: Update(height, nil, paths, payload, node), but we can optimize the extra call and just continue the function to case 2.b with 
   // compactLeaf set as node.
   compactLeaf = node
  }
 }
 
 // The remaining logic below handles the remaining recursion step which is common for the 
 // case 0: node ≠ nil and there are many paths to update (len(paths)>1)
 // case 1.a.ii: node ≠ nil and node.path ∈ path and len(paths) > 1
 // case 1.b: node ≠ nil and node.path ∉ path
 // case 2.b: node == nil and there is more than one register to update: 
 //     - len(paths) == 1 and compactLeaf != nil 
 //     - or alternatively len(paths) > 1

 // Split paths and payloads according to the bit of path[i] at index (256 - height):
 // lpaths contains all paths that have `0` at the bit index
 // rpaths contains all paths that have `1` at the bit index
 lpaths, rpaths, lpayloads, rpayloads = Split(paths, payloads, 256 - height)
	
 // As part of cases 1.b and 2.b, we have to determine whether compactLeaf falls into the left or right sub-trie:
 if compactLeaf != nil {
  // if yes, check which branch it will go to.
  if Bit(compactLeaf.path, 256 - height) == 0 {
   lcompactLeaf = compactLeaf
   rcompactLeaf = nil
  } else {
   lcompactLeaf = nil
   rcompactLeaf = compactLeaf
  } 
 } else { // for cases 0 and 1.a.ii, we don't have a compactified leaf to carry forward
  lcompactLeaf = nil
  rcompactLeaf = nil
 }
 
 // the difference between cases with node ≠ nil vs the case with node == nil
 if node != nil { // cases 0, 1.a.ii, and 1.b
  lchild = node.leftChild
  rchild = node.rightChild
 } else {  // case 2.b
  lchild = nil
  rchild = nil
 }
 
 // recursive descent into the childred
 newlChild = Update(height-1, lchild, lpaths, lpayloads, lcompactLeaf)
 newrChild = Update(height-1, rchild, rpaths, rpayloads, rcompactLeaf)
 
 // mitigate storage exhaustion attack: avoids creating a new interim-node when the same
 // payload is re-written at a register, resulting in the same children being returned.
 if lChild == newlChild && rChild == newrChild {
  return node
 }

nodeToBeReturned := NewInterimNode(height, newlChild, newrChild)
 // if pruning is enabled, check if we could compactify the nodes after the update
 // a common example of this is when we update a register payload to nil from a non-nil value
 // therefore at least one of the children might be a default node (any node that has hashvalue equal to the default hashValue for the given height)
 if prune { 
    return nodeToBeReturned.Compactify()
 }

 return nodeToBeReturned
}
```

