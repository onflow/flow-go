package storage

// BatchStorage
type BatchStorage interface {
	Set(key, val []byte) error
}
