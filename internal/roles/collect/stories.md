# Stories
Referred to
https://docs.google.com/spreadsheets/d/1WLtXFdShv5JolvRTT0ykjOezVdgI-CmWu5_bQXkKnLQ/edit?ts=5d93cbfb#gid=0

## Collection
- As a (Collection) Node, it should be in the right cluster, after joining a cluster
- As a (Collection) Node, it should not be in the wrong cluster, after joining a cluster
- As a CollectionNode, when called with acceptTx, it should reject a tx if the tx has syntax error
- As a CollectionNode, when called with acceptTx, it should reject a tx if the tx has an invalid signature
- As a CollectionNode, when called with acceptTx, it should reject a tx if the tx has invalid payload
- As a CollectionNode, when called with acceptTx, it should reject a tx if the payer has insufficent fund
- As a CollectionNode, when called with acceptTx, it should relay the tx to the right cluster
- As a CollectionNode, when called with acceptTx, it should store the tx in pending txpool if the tx belongs to the cluster of the node
- As a CollectionLeader, it should propose collection and broadcast it to collection followers.
- As a CollectionFollower, it should reject the proposed collection if it has invalid signature of its leader.
- As a CollectionFollower, it should vote on proposed collection with its identity, stake and Epoch start seed.
- As a CollectionFollower, it should raise CollectionConsensusEquivocationChallenge to ConsensusNode if it receives a collection that contains transactions that have already been included in a previous collection
- As a CollectionFollower, it should raise DoubleVotingChallenge if it receives double votes from the same collection node.
- As a CollectionFollower, it should finalize a collection and broadcast if it receives enough votes from followers of the same cluster.
- As a CollectionFollower, it should reject a fake finalized collection if the block didn't actually get enough votes
- As a CollectionNode, it should store a finalized collection into its collection pool when it receives the collection.
- As a CollectionNode, when it receives a MissingCollectionChallenge, it should respond with a collection
- As a CollectionNode, when it receives a MissingTransactionChallenge, it should respond with the missing transaction.
