# Verification

The execution of a block follows after a block is finalised. The security gained from the consensus algorithm does not cover the block's execution. It follows that there is a need to construct a process to attain security for the execution step.

We construct a verification process to attain security over the execution process. This process is a form of "Independent Verification" (See IEEE Standard 1012). The verification process is also given enforcement power, as we enable it to request an execution node slashing.

"Independent verification" in our context is re-computing the block.
The block computation is done by a compute-optimised node, furthermore, by the fastest within the set of these nodes. We therefore assume that a block re-computation by any other node, as part of the independent verification process, will always be slower. If not addressed, this bottleneck will hinder the speed gain in the execution step.
To address the bottleneck concern, we take an approach of split parallel verification. We define the "Execution Receipt" to be composed of separate chunks, constructed to be independently verifiable. A verification process is defined to be on a subset of these chunks.

Since we expect to verify multiple identical "Execution Receipt"s by different Execution nodes, we want the implementation to be efficient and reuse previously cached verification results. The following algorithm details this:
![verification-flow](https://github.com/dapperlabs/shoot/blob/master/designs/algorithms/post-computation/receipt-verification.png?raw=true)
<TODO: move diagrams into repo>

Combining independent verification with split parallel verification, results in a process that can be described with terminology borrowed from "acceptance sampling" theory. Namely, we design the process to follow zero-defect acceptance sampling (AQL=100%).

<TODO: explain Z or link to Z reademe file here >

A successful verification process results in a "Result Approval" message sent from the verifier to the security node.
We construct the verification process to be self contained. Any "Execution Receipt" can be verified, in isolation, without further knowledge about the chain. While some of the data is only linked to the "Execution Receipt" and needs to be fetched, each fetched piece can be checked to match the expected data via hashing. Therefore, errors found in the verification process can be attributed to the "Execution Receipt" publisher, with a public proof.
