package leader

// NewBootstrapLeaderSelection creates a leader selection for bootstrapping process to create
// genesis QC.
// The returned leader selection does not have any pre-generated leader selections since
// the bootstrapping process don't need it.
func NewSelectionForBootstrap() *LeaderSelection {
	return &LeaderSelection{}
}
