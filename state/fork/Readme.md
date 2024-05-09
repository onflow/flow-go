# Traversing a Fork

Generally, when traversing a fork (in either direction), there are two distinct blocks:
 * the `head` of the fork that should be traversed
 * the `lowestBlock` in that fork, that should be included in the traversal

The traversal the walks `head <--> lowestBlock` (in either direction).

There are a variety of ways to precisely specify `head` and `lowestBlock`:
 * At least one block, `head` or `lowestBlock`, must be specified by its ID
   to unambiguously identify the fork that should be traversed.
 * The other block can either be specified by ID or height.
 * If both `head` and `lowestBlock` are specified by their ID,
   they must both be on the same fork.

### Convention

**For the core traversal algorithms, we use the following conventions:** 
1. The `head` of the fork that should be traversed is specified by its `ID`
2. The `lowestBlock` is specified by its height: `lowestHeightToVisit`.
3. If `head.Height < lowestHeightToVisit`, no blocks are visited and the 
   traversal returns immediately (without error). 

This design is inspired by a `for` loop. For example, lets consider the case where we start 
at the fork `head` and traverse backwards until we reach a block with height `lowestHeightToVisit`:
```golang
for block := head; block.Height >= lowestHeightToVisit; block = block.Parent {
	visit(block)
}
```

For the core traversal algorithms, the parametrization of `head` [type `flow.ID`],
`lowestHeightToVisit` [type `uint64`], and direction is beneficial, as there are no inconsistent parameter sets.
(There is the edge-case of the root block, which we ignore in this high-level discussion)




### Terminals

Higher-level algorithms that want to collect specific data from a fork have a variety of
different termination conditions, such as:
* walk backwards up to (_including_) the block with ID `xyz` or _excluding_ the block with ID `xyz`

To provide higher-level primitives for terminating the fork traversal, we include
`Terminals`. Essentially, a `Terminal` converts a high-level condition for the `lowestBlock`
to a suitable parametrization for the core traversal algorithm:

* The height `lowestHeightToVisit` of the lowest block that should be visited.
* A consistency check `ConfirmTerminalReached` for the lowest visited block.
  This check is predominantly useful
  in case both `head` and `lowestBlock` are specified by their ID. It allows to enforce that
  `head` and `lowestBlock` are both on the same fork and error otherwise.
  
  However, the precise implementation of `ConfirmTerminalReached` is left to the Terminal.
  Other, more elaborate conditions  are possible.
