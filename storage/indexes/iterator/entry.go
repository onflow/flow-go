package iterator

// type Entry[T any, K any] struct {
// 	key       []byte
// 	decodeKey storage.DecodeKeyFunc[K]
// 	getValue  func(K, *T) error
// }

// func NewEntry[T any, K any](key []byte, decodeKey storage.DecodeKeyFunc[K], getValue func(K, *T) error) Entry[T, K] {
// 	return Entry[T, K]{
// 		key:       key,
// 		decodeKey: decodeKey,
// 		getValue:  getValue,
// 	}
// }

// func (i Entry[T, K]) KeyParts() (K, error) {
// 	return i.decodeKey(i.key)
// }

// func (i Entry[T, K]) Value() (T, error) {
// 	var v T
// 	decodedKey, err := i.decodeKey(i.key)
// 	if err != nil {
// 		return v, err
// 	}

// 	err = i.getValue(decodedKey, &v)
// 	return v, err
// }
