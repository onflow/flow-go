package hotstuff

type ViewState interface {
	IsSelfLeaderForView(view uint64) bool
	GetSelfIdxForView(view uint64) uint32
}
