package types

type AggregatedSignature struct{}

func (a *AggregatedSignature) Sigs() []*Signature {
	panic("TODO")
}
