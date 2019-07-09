# Sealing

A "Block Seal" is a structure that holds an execution result alongside proofs of verification by nodes who attest to the correctness of the result. 

A valid "Block Seal" is defined to require that its verification proofs meet a minimum threshold for stake by the nodes they represent.

At the block formation phase, a block proposer may include one or more "Block Seals" inside the block. To do so, a security node needs to aggregate "Execution Receipt" messages and matching "Results Approvals" in a "Execution Receipt - Results Approvals" pool (or "ERRA" pool for short).

A valid "Block Seal" does not guarantee it can be included in any block. For inclusion, a valid "Block Seal" must also be chainable with respect the chain's state.

A chainable "Block Seal" is defined by the following requirements in respect to its "Execution Receipt":
  - Its "Previous Execution Receipt" matches the latest sealed block's "Execution Receipt".
  - Its block's "Previous Block Hash" matches the latest sealed block's "Block Hash".
  - Its "Start state" matches the latest sealed block's "End state".
