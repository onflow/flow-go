package signature

// RandomBeaconThreshold returns the threshold (t) to allow the largest number of
// malicious nodes (m) assuming the protocol requires:
//   m<=t for unforgeability
//   n-m>=t+1 for robustness
func RandomBeaconThreshold(size int) int {
	// avoid initializing the thershold to 0 when n=2
	if size == 2 {
		return 1
	}
	return (size - 1) / 2
}
