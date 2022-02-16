package verification

type VerificationStreamNegotiationSuite struct {
	Suite
}

// TestVerificationNodeHappyPath verifies the integration of verification and execution nodes over the
// happy path of successfully issuing a result approval for the first chunk of the first block of the testnet.
// Note that gzip stream compression is enabled between verification and execution nodes.
func (suite *VerificationStreamNegotiationSuite) TestVerificationNodeHappyPath() {
	testVerificationNodeHappyPath(suite.T(), suite.exe1ID, suite.verID, suite.BlockState, suite.ReceiptState, suite.ApprovalState)
}
