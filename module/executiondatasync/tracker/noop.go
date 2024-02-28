package tracker

type NoopStorage struct{}

var _ Storage = (*NoopStorage)(nil)

func (s *NoopStorage) Update(UpdateFn) error { return nil }
func (s *NoopStorage) GetFulfilledHeight() (uint64, error) {
	return 0, nil
}

func (s *NoopStorage) SetFulfilledHeight(uint64) error {
	return nil
}

func (s *NoopStorage) GetPrunedHeight() (uint64, error) {
	return 0, nil
}

func (s *NoopStorage) PruneUpToHeight(height uint64) error {
	return nil
}
