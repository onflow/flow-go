package hotstuff

type Trigger interface {
	ViewChange() error
}
