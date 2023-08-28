# Framework BFT Tests
The framework BFT tests are designed to assess the health of the BFT testing framework. 

## Passthrough Sealing and Verification Test
The `PassThroughTestSuite` test within the `framework` package includes a specific test method `TestSealingAndVerificationPassThrough`.
This test evaluates the health of the BFT testing framework for Byzantine Fault Tolerance (BFT) testing.
1. **Simulates a Scenario**: It sets up a scenario with two corrupt execution nodes and one corrupt verification node, controlled by a dummy orchestrator that lets all incoming events pass through.
2. **Deploys Transaction and Verifies Chunks**: Deploys a transaction leading to an execution result with multiple chunks, assigns them to a verification node, and verifies the generation of result approvals for all chunks.
3. **Sealing and Verification**: Enables sealing based on result approvals and verifies the sealing of a block with a specific multi-chunk execution result.
4. **Evaluates Events**: The test also assesses whether critical sealing-and-verification-related events from corrupt nodes are passed through the orchestrator, by checking both egress and ingress events.
