package protocol

// DKGPhaseViews returns the DKG final phase views for an epoch.
// Error returns:
// * protocol.ErrNoPreviousEpoch - if the epoch represents a previous epoch which does not exist.
// * protocol.ErrNextEpochNotSetup - if the epoch represents a next epoch which has not been set up.
// * protocol.ErrNextEpochNotCommitted - if the epoch has not been committed.
// * state.ErrUnknownSnapshotReference - if the epoch is queried from an unresolvable snapshot.
func DKGPhaseViews(epoch Epoch) (phase1FinalView uint64, phase2FinalView uint64, phase3FinalView uint64, err error) {
	phase1FinalView, err = epoch.DKGPhase1FinalView()
	if err != nil {
		return
	}
	phase2FinalView, err = epoch.DKGPhase2FinalView()
	if err != nil {
		return
	}
	phase3FinalView, err = epoch.DKGPhase3FinalView()
	if err != nil {
		return
	}
	return
}
