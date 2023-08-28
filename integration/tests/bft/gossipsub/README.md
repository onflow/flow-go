# GossipSub BFT Tests
GossipSub BFT tests are designed to test the behavior of the GossipSub protocol in a network environment with Byzantine nodes.

## Topic Validator Test
The `TopicValidatorTestSuite` in the `topicvalidator` package is specifically designed to test the functionality of the
libp2p topic validator within a network scenario.
This suite includes an end-to-end test to verify the topic validator's behavior in different situations, 
focusing on both unauthorized and authorized message handling.
The method `TestTopicValidatorE2E` is a comprehensive test that mimics an environment with a corrupt byzantine attacker node
attempting to send unauthorized messages to a victim node. 
These messages should be dropped by the topic validator, as they fail the message authorization validation. 
The test simultaneously sends authorized messages to the victim node, ensuring that they are processed correctly, 
demonstrating the validator's correct operation.
The test confirms two main aspects:
1. Unauthorized messages must be dropped, and the victim node should not receive any of them.
2. Authorized messages should be correctly delivered and processed by the victim node.

## Signature Requirement Test
The `TestGossipSubSignatureRequirement` test sets up a test environment consisting of three corrupt nodes: two attackers and one victim.
One (malicious) attacker is configured without message signing, intending to send unsigned messages that should be rejected by the victim.
The other (benign) attacker sends valid signed messages that should be received by the victim.
The test is broken down into the following main parts:
1. **Unauthorized Messages Testing**: The victim node should not receive any messages sent without correct signatures from the unauthorized attacker. The test checks for zero unauthorized messages received by the victim.
2. **Authorized Messages Testing**: Messages sent by the authorized attacker, with the correct signature, must pass the libp2p signature verification process and be delivered to the victim. The test checks for all authorized messages received by the victim within a certain time frame.

## RPC Inspector False Positive Test
The `GossipsubRPCInspectorFalsePositiveNotificationsTestSuite` test within the `rpc_inspector` package test suite aims to ensure that the underlying libp2p libraries related to GossipSub RPC control message inspection do not trigger false positives during their validation processes.
Here's a breakdown of the `TestGossipsubRPCInspectorFalsePositiveNotifications` method:
1. **Configuration and Context Setup**: A specific duration for loading and intervals is defined, and a context with a timeout is created for the test scenario.
2. **Simulating Network Activity**: The method triggers a "loader loop" with a specific number of test accounts and intervals, intending to create artificial network activity. It does this by submitting transactions to create Flow accounts, waiting for them to be sealed.
3. **State Commitments**: The method waits for 20 state commitment changes, ensuring that the simulated network load behaves as expected.
4. **Verification of Control Messages**: After simulating network activity, the method checks to ensure that no node in the network has disseminated an invalid control message notification. This is done by collecting metrics from the network containers and verifying that no false notifications are detected.
