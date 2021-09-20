package votecollector

// 1. if the proposal is invalid, it should return InvalidBlockError
// 2. if the proposal is valid, then a vote processor should be created. The status of created processor is Verifying
// 3. Block() return the right block, Status() should return VoteCollectorStatusVerifying
// 4. if minimum required stakes have not been reached, no QC will be built
// 5. if minimum required stakes have been reached, but not sufficient beacon shares have been received, no QC will be built
// 6. if minimum required stakes have been reached, and sufficient beacon shares have been received, QC will be built
// 7. if more than minimum required stakes have been reached, and sufficient beacon shares have been received, QC will be built
// 8. when a QC is built, the signer ids should include all staking signers and random beacon signers.
// 9. if a QC has been built, additional votes won't trigger QC to be built again. (onQCCreated callback will only be called once)
// 10. if a vote is invalid, it should return InvalidVoteError
// 11. concurrency: when sending concurrently with votes that have just enough stakes and more beacon shares, a QC should be built
// 12. concurrency: when sending concurrently with votes that have more stakes and just enough beacon shares, a QC should be built
