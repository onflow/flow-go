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

## Signature Requirement Test
The `TestGossipSubSignatureRequirement` method sets up a test environment consisting of three corrupted nodes:two attackers and one victim. 
One attacker is configured without message signing, intending to send unsigned messages that should be rejected by the victim. 
The other attacker sends valid signed messages that should be received by the victim.
The test is broken down into the following main parts:
1. **Unauthorized Messages Testing**: The victim node should not receive any messages sent without correct signatures from the unauthorized attacker. The test checks for zero unauthorized messages received by the victim.
2. **Authorized Messages Testing**: Messages sent by the authorized attacker, with the correct signature, must pass the libp2p signature verification process and be delivered to the victim. The test checks for all authorized messages received by the victim within a certain time frame.
