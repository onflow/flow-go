# Protocol BFT Tests
This package contains BFT tests concerning the core Flow protocol. These tests are run as part of the integration test suite.


## Admin Command Disallow List Test
The `AdminCommandDisallowListTestSuite` in the `disallowlisting` package is designed to test the functionality of the disallow-list admin command within a network context. 
It ensures that connections to a blocked node are immediately pruned and incoming connection requests are blocked.
The test simulates the setup of two corrupt nodes (a sender and a receiver) and examines the behavior of the network before and after the sender node is disallowed. 
It includes steps to send authorized messages to verify normal behavior, implement disallow-listing, send unauthorized messages, and validate the expectation that no unauthorized messages are received. 
Various timing controls are put in place to handle asynchronous processes and potential race conditions. 
The entire suite assures that the disallow-listing command behaves as intended, safeguarding network integrity.

## Wintermute Attack Test
The `WintermuteTestSuite` in the `wintermute` package is focused on validating a specific attack scenario within the network, termed the "wintermute attack." 
This attack involves an Attack Orchestrator corrupting an execution result and then leveraging corrupt verification nodes to verify it.
The suite includes a constant timeout to define the attack window and a detailed test sequence.
The `TestWintermuteAttack` method carries out the attack process. 
It first waits for an execution result to be corrupted and identifies the corresponding victim block. 
It ensures that the corrupt execution nodes generate the correct result for the victim block and then waits for a specific number of approvals from corrupt verification nodes for each chunk of the corrupted result.
Further, the test waits for a block height equal to the victim block height to be sealed and verifies that the original victim block is correctly identified. 
Additional methods and logging information assist in detailing and controlling the flow of the attack.
The entire suite is instrumental in evaluating the system's behavior under this specific attack condition and ensuring that the expected actions and responses are observed.