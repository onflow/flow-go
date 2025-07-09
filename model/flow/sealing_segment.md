# Sealing Segment

The `SealingSegment` is a section of the finalized chain. It is part of the data need to
initialize a new node to join the network. Informally, the `SealingSegment` is continuous section
of recently finalized blocks that is long enough for the new node to execute its business logic.

## History length covered by the Sealing Segment

The `SealingSegment` is created from a `protocol.Snapshot` via the method `SealingSegment`.
Lets denote the block that the `protocol.Snapshot` refers to as `head`. Per convention,
`head` must be a finalized block.

### Part 1: from `head` back to the latest sealed block

The SealingSegment is a chain segment such that the last block (greatest height)
is this snapshot's reference block (i.e. `head`) and the first (least height) is the most
recently sealed block as of this snapshot.
In other words, the most recently incorporated seal as of the highest block
references the lowest block. The highest block does not need to contain this seal.
* Example 1: block `E` seals `A`
  ```
  A <- B <- C <- D <- E(SA)
  ```
  Here, `SA` denotes the seal for block `A`.
  In the sealing segment's last block (`E`) has a seal for block `A`, which is the first block of the sealing segment.

* Example 2: `E` contains no seals, but latest seal prior to `E` seals `A`
  ```
  A <- B <- C <- D(SA) <- E
  ```
* Example 3: `E` contains multiple seals
  ```
  B <- C <- D <- E(SA, SB)
  ```

Per convention, the blocks from Part 1, go into the slice `SealingSegment.Blocks`:
```golang
type SealingSegment struct {
   // Blocks contain the chain [block sealed by `head`] <- ... <- [`head`] in ascending height order.
   Blocks []*Block

   ⋮
}
```

**Minimality Requirement for `SealingSegment.Blocks`**:
In example 3, note that block `B` is the highest sealed block as of `E`. Therefore, the
lowest block in `SealingSegment.Blocks` must be `B`. Essentially, this is a minimality
requirement for the history: it shouldn't be longer than necessary. So
extending the chain segment above to `A <- B <- C <- D <- E(SA, SB)` would
be an invalid value for field `SealingSegment.Blocks`.

### Part 2: `ExtraBlocks`

In addition, the `SealingSegment` contains the field `ExtraBlocks`:

```golang
// ExtraBlocks [optional] holds ancestors of `Blocks` in ascending height order. These blocks
// are connecting to `Blocks[0]` (the lowest block of sealing segment). Formally, let `l`
// be the length of `ExtraBlocks`, then ExtraBlocks[l-1] is the _parent_ of `Blocks[0]`.
// These extra blocks are included in order to ensure that a newly bootstrapped node
// knows about all entities which might be referenced by blocks which extend from
// the sealing segment.
// ExtraBlocks are stored separately from Blocks, because not all node roles need
// the same amount of history. Blocks represents the minimal possible required history;
// ExtraBlocks represents additional role-required history.
ExtraBlocks []*Block
```

**In case `head` contains multiple seals, we need _all_ the sealed blocks**, for the following reason:
* All nodes locally maintain a copy of the protocol state. A service event may change the state of the protocol state.
* For Byzantine resilience, we don't want protocol-state changes to take effect immediately. Therefore, we process
  system events only after receiving a QC for the block.

  Now let us consider the situation where a newly initialized node comes online and processes the first child of `head`.
  Lets reuse the example from above, where our head was block `E` and we are now processing the child `X`
  ```
            A  <- B <- C <- D <- E(SA, SB) <- X
  ├═══════════┤  ├───────────────────────┤   new
   ExtraBlocks              Blocks          block
  ```
  `X` carries the QC for `E`, hence the protocol-state changes in `E` take effect for `X`. Therefore, when processing `X`,
  we go through the seals in `E` and look through the sealed execution results for service events.
* As the service events are order-sensitive, we need to process the seals in the correct order, which is by increasing height
  of the sealed block. The seals don't contain the block's height information, hence we need to resolve the block.

**Extended history to check for duplicated collection guarantees in blocks** is required by nodes that _validate_ block
payloads (e.g. consensus nodes). Also Access Nodes require these blocks. Collections expire after `flow.DefaultTransactionExpiry` blocks.
Hence, we desire a history of `flow.DefaultTransactionExpiry` blocks. However, there is the edge case of a recent spork (or genesis),
where the history is simply less that `flow.DefaultTransactionExpiry`.

### Formal definition

The descriptions from the previous section can be formalized as follows

* (i) The highest sealed block as of `head` needs to be included in the sealing segment.
  This is relevant if `head` does not contain any seals.
* (ii) All blocks that are sealed by `head`. This is relevant if `head` contains _multiple_ seals.
* (iii) The sealing segment should contain the history back to (including):
  ```
  limitHeight := max(blockSealedAtHead.Height - flow.DefaultTransactionExpiry, SporkRootBlockHeight)
  ```
   where blockSealedAtHead is the block sealed by `head` block.
Note that all three conditions have to be satisfied by a sealing segment. Therefore, it must contain the longest history
required by any of the three conditions. The 'Spork Root Block' is the cutoff.

Per convention, we include the blocks for (i) in the `SealingSegment.Blocks`, while the
additional blocks for (ii) and optionally (iii) are contained in as `SealingSegment.ExtraBlocks`.


Condition (i) and (ii) are necessary for the sealing segment for _any node_. In contrast, (iii) is
necessary to bootstrap nodes that _validate_ block payloads (e.g. consensus nodes), to verify that
collection guarantees are not duplicated (collections expire after `flow.DefaultTransactionExpiry` blocks).

## Special case: Root Sealing Segment

The spork is initialized with a single 'spork root block'. A root sealing segment is a sealing segment containing root block:
* the root block is a self-sealing block with an empty payload
* the root block must be the first block (least height) in the segment
* no blocks in the segment may contain any seals (by the minimality requirement)
* it is possible (but not necessary) for root sealing segments to contain _only_ the root block

Examples:
* Example 1: one self-sealing root block
  ```
  ROOT
  ```
  The above sealing segment is the form of sealing segments within root snapshots,
  for example those snapshots used to bootstrap a new network, or spork.
* Example 2: one self-sealing root block followed by any number of seal-less blocks
  ```
  ROOT <- A <- B
  ```
  All non-root sealing segments contain more than one block.
  Sealing segments are in ascending height order.

In addition to storing the blocks within the sealing segment, as defined above,
the `SealingSegment` structure also stores any resources which are referenced
by blocks in the segment, but not included in the payloads of blocks within
the segment. In particular:
* results referenced by receipts within segment payloads
* results referenced by seals within segment payloads
* seals which represent the latest state commitment as of a segment block

## Outlook

In its current state, the sealing segment has been evolving driven by different needs. Most likely, there is some room for simplifications
and other improvements. However, an important aspect of the sealing segment is to allow newly-joining nodes to build an internal representation
of the protocol state, in particular the identity table. There are large changes coming around when we move to the dynamic identity table.
Therefore, we accept that the Sealing Segment currently has some technical debt and unnecessary complexity. Once we have implemented the
dynamic identity table, we will have a much more solidified understanding of the data in the sealing segment.

