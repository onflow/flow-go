# GossipSub BFT Tests

## Topic Validator Test
The `TopicValidatorTestSuite` in the `topicvalidator` package is specifically designed to test the functionality of the
libp2p topic validator within a network scenario.
This suite includes an end-to-end test to verify the topic validator's behavior in different situations, 
focusing on both unauthorized and authorized message handling.
The method `TestTopicValidatorE2E` is a comprehensive test that mimics an environment with a corrupted byzantine attacker node 
attempting to send unauthorized messages to a victim node. 
These messages should be dropped by the topic validator, as they fail the message authorization validation. 
The test simultaneously sends authorized messages to the victim node, ensuring that they are processed correctly, 
demonstrating the validator's correct operation.
The test confirms two main aspects:
1. Unauthorized messages must be dropped, and the victim node should not receive any of them.
2. Authorized messages should be correctly delivered and processed by the victim node.
