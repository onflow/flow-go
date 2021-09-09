package votecollector

// 1. if the proposal is invalid, it should return InvalidBlockError
// 2. if the proposal is valid, then a vote processor should be created. The status of created processor is Verifying
// 3. Block() return the right block, Status() should return VoteCollectorStatusVerifying
// 4. if minimum required stakes has not been reached, no QC will be built
// 5. if minimum required stakes has been reached, but not sufficient beacon shares have been received, no QC will be built
// 6. if minimum required stakes has been reached, and sufficient beacon shares have been received, QC will be built
// 7. if a QC has been built, additional votes won't trigger QC to be built again. (onQCCreated callback will only be called once)
// 8. if a vote is invalid, it should return InvalidVoteError
