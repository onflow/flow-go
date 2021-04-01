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
Note that we do not introduce the concept of registers values whose values are `nil`. 

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
 * An **INTERIM** node is a vertex in the tree with
    - it has exactly two children, called `LeftChild` and `RightChild`, which are both of the same height;
      the children can either be leafs or interim nodes. 
    - the `height` of an interim node `n` is `n.height = LeftChild.height + 1 = RightChild.height + 1`;
      (Hence, an interim node `n` can only have a `n.height > 0`, as only leafs have height zero).  
    - the `hash` value is defined as `H(LeftChild, RightChild)`
   
#### Convention for mapping a register `key` to a path in the tree

**Conventions:**
* let `path[i]` be the bit with index `i` (we use zero-based indexing)
* a `key` can be converted into its `integer representation` though big-endian ordering
* given a `path` and an index `i`, we define:
  - the `prefix` as `path[:i]` (excluding the bit up to, but not including, the bit with index `i`)
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
   the path index `i := K - n.height`.    
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

The update algorithm `NewTrieWithUpdates` takes a trie `m` as input along with a set of 
pairs `(path, payload)`.
It outputs a new trie `new_m` such that each payload `payload[i]` is stored at the path `path[i]`. 
Any path that is not included in the input pairs keeps the payload from the original input `m`.

The algorithm is described first to update a single pair `(path, payload)`. The generalization to 
an arbitrary higher number of pairs follows with a few updates. Given a root node of a trie, a path and a payload,
we look for the register addressed by the given path in a recursive bottom-down manner. Each recursive step 
operates at a certain height of the tree, starting from the root. Looking at the correct bit index of the path
(`p[i]` with `i = 256 - height`), we move one height down to look for the path at the left or right sub-trie. Each step 
generates 2 new sub-tries (left and right), unless no update is required. A new inter-node is created at each height level
and points to the newly created left and right sub-tries. This is the generic recursion step (Step 0). This process continues 
over the tree on a depth-first manner till it reaches a `nil` sub-trie or a leaf:

 - 1. a leaf means that we reached either a tree leaf (height zero) or a compactified leaf (height larger than zero). 
      - 1. If the leaf path is equal to the input path, then we have found the node with input `path` we're looking for. 
This new node represents either a tree leaf or a compactified leaf. The tree update is done by creating
a new node with the new input `payload`. 
      - 2. However, if the leaf path is not equal to the input `path`, then 
we are necessarily on a compactified leaf that needs to be further developed into a sub-tree in order to hold 
both the new input `(path, payload)` and the old register represented by the old compactified leaf. This means
a new inter-node with `nil` children is created and the recursion is called on a lower level, but this 
time carrying forward the old compactified node. The sub-tree that used to contain only one allocated 
registered, will now contain two allocated registers and can't be represented by a compactified leaf anymore. 
 - 2. a `nil` sub-trie means that the sub-trie is empty and at least a new leaf will be created: 
      - 1. If there is only one leaf to create (either the one representing the input `(path, payload)`, 
or the one representing a compactified leaf carried over from a higher height), then a new leaf is created.
The new leaf either represents a tree leaf or a new trie compactified leaf. 
      - 2. If there are 2 leafs to create (both the input `(path, payload)` and the compactified leaf carried over),
then we are still at an inter-node height, we create a new inter-node, check the path index of both the input `path`
and the compactified node `path` and continue the recursion. Eventually the recursion calls will fall into 2.a 
as we reach the first different bit index between the 2 paths. We note that this case is very similar to the 
generic Step 0 above, but just called with a node == nil. 

We define the following recursive step `Update` to implement `NewTrieWithUpdates`. 
`Update` takes the current node where the recursion applies, the node height, the input pair `(path, payload)`
as well as the possible compactified leaf carried over from higher heights. If no compactfied leaf is carried over, 
the parameter is simply `nil`. We note that if the compactified leaf is not `nil`, the current node is necessarily `nil`. The inverse is also valid, if the current node is non `nil`, there is necessarily no compactified node carried over. 
To generalize this algorithm to an arbitrary number of pairs `(path, payload)`, we make the following changes:
 - `Update` takes a list (slice) of `paths` and `payloads`. 
 -  when moving to a lower height, paths are split into two groups depending on `p[i][k]` with `k = 256 - height`. 
The first group has all bits `p[i][k] = 0` and goes into the left child recursion, while the second group has bits
`p[i][k] = 1` and goes into the right child recursion. The split is done lineraly over the input paths to `Update`. 
This results in sorting the overall input `paths` of `NewTrieWithUpdates` using an implicit quick sort.
 - if `len(paths) == 0` and there is no compact leaf carried over (`compactLeaf == nil`), no update will be done 
and the original sub-trie can be re-used in the new trie.
 - Step 0 is very similar. An inter-node is created, the paths are split into left and right.
 - In Step 1, instead of comparing the path of the leaf with the unique input path, the leaf path is linearly searched within
all the input paths. Step 1.a is when the leaf path is found among the inputs, step 1.b is when the leaf path is not found. We note that the linear search in the recursive step still gives a `O(n.log(n))` overall search complexity where `n` is the size of the input `paths`.
 - Step 1.a is now split into two subcases:
    - when `len(paths) == 1`, a new node is created with the new updated payload. This would be a leaf in the new trie.
    - when `len(paths) > 1`, we are necessarily on a compactified leaf and we don't care about its own payload as it will get
updated by the new input payload. We therefore continue the recursion with `compactLeaf = nil` and the same input paths 
and payloads.
 - Step 1.b is similar. If the leaf path is not found among the inputs, then we are on a compactified leaf and we call the recursion
with the same inputs but with `compactLeaf` being set to the current node. 
 - Step 2.a checks for a single leaf to create. This check would become `len(paths) == 1 && compactLeaf == nil` or
`len(paths) == 0 && compactLeaf != nil`.
 - Step 2.b is very similar. An inter-node is created, the paths are split into left and right, and the compact leaf
is carried over into the left or right child. We note that this case is also very similar to Step 0 where the current node is `nil`. The pseudo-code below will treat case 0 and 2.b in the same code section. 


#### Further optimization and resource-exhaustion attack:
In order to counter a resource-exhaustion attack where an existing allocated register is being updated with the same payload, resulting in creating new unnecessary nodes, we slightly adjust step 1.a. When `len(paths)==1` and the input path is equal to the current leaf path, we only create a new leaf if the input payload is different than the one stored initially in the leaf. If the two payloads are equal, we just re-cycle the initial leaf.
Morever, we create a new inter-node from the left and right children only if the returned children are different than the original node children. If the children are equal, we just re-cycle the same inter-node. 

#### Putting everything together:
This results in the following algorithm:

```
function NewTrieWithUpdates(OriginalTrie MTrie, paths []Path, payloads []Payload) MTrie {
 newMtrieRoot  = Update(256, OriginalTrie.root, paths, payloads, nil)
 return  MTrie( root: newMtrieRoot)
}

function Update(height Int, node Node, paths []Path, payloads []Payload, compactLeaf Node) Node{
 if len(paths) == 0 {
  // If a compactLeaf from a higher height is carried over, then we are necessarily in case 2.a (node == nil and only one register to create)
  if compactLeaf != nil {
   return NewLeaf(compactLeaf.path, compactLeaf.payload, height)
  }
  // No updates to make, re-use the same sub-trie
  return node
 }

 // The remaining sub-case of 2.a (node == nil and only one register to create), this time the register payload is the input and not the one from the compactified leaf carried over. 
 if len(paths) == 1 && parentNode == nil && compactLeaf == nil {
  return NewLeaf(paths[0], payloads[0], height)
 }

 // case 1: we reach a non-nil leaf. As noted above, in this case compactLeaf is necessarily nil
 if node != nil && node.IsLeaf() { 
  // case 1: check if the node node is among the updated paths
  found = false
  for i = range len(paths) {
   // case 1.a : the node path is found among the inputs
   if p[i] == node.path) {
    // the first subcase of 1.a
    if len(paths) == 1 {
     // the resource-exhaustion counter-measure
     if !node.payload == payloads[i] {
      return NewLeaf(paths[i], payloads[i], height)
     }
     return node  // re-cycle the same node
    }
    // the second subcase of 1.a (len(paths)>1)
    found = true
    break
   }
  }

  // Check whether we are still in 1.a or should switch to 1.b
  if !found {
   // Switch to 1.b: the node path was not found among the inputs and we should carry the node to lower heights as a compactLeaf parameter.
   // Case 1.b can be written as Update(height, nil, paths, payload, node), but we can optimize and just continue the function to case 2.b with 
   // compactLeaf set as node
   compactLeaf = node
  }
  // if the path is found, nothing to change, we remain in 1.a
 }

 // Below are the cases 2.b or the generic case 0: 
 //   - case 0: node != nil and there are many paths to update (len(paths)>1)
 //   - case 2.b: node == nil and there is more than one register to update : 
 //     - len(paths) == 1 and compactLeaf != nil 
 //     - or simply len(paths) > 1

 // Split paths and payloads according to the bit of path[i] at index (256 - height):
 // lpaths contains all paths that have `0` at the bit index
 // rpaths contains all paths that have `1` at the bit index
 lpaths, rpaths, lpayloads, rpayloads = Split(paths, payloads, 256 - height)
	
 // As part of case 2.b, check whether there is a compactLeaf that needs to get deep to height 0, check which side it should go:
 if compactLeaf != nil {
  // if yes, check which branch it will go to.
  if Bit(compactLeaf, 256-nodeHeight) == 0 {
   lcompactLeaf = compactLeaf
   rcompactLeaf = nil
  } else {
   lcompactLeaf = nil
   rcompactLeaf = compactLeaf
  } 
 } else {
  lcompactLeaf = nil
  rcompactLeaf = nil
 }

 // the difference between case 0 and case 2.b
 if node != nil { // case 0
  lchild = node.leftChild
  rchild = node.rightChild
 } else {  // case 2.b
  lchild = nil
  rchild = nil
 }

 // recurse over each child
 newlChild = Update(height-1, lchild, lpaths, lpayloads, lcompactLeaf)
 newrChild = Update(height-1, rchild, rpaths, rpayloads, rcompactLeaf)

 // mitigate storage exhaustion attack: avoids creating a new inter-node when the same
 // payload is re-written at a register, resulting in the same children being returned.
 if lChild == newlChild && rChild == newrChild {
  return node
 }
 return NewInterimNode(height, newlChild, newrChild)
}
```

