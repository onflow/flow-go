package virtualmachine

type Ledger interface {
	Set(key string, value []byte)
	Get(key string) ([]byte, error)
	Delete(key string)
}
