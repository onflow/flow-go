package keyvalue

// DBConnection abstracts a db connection
type DBConnection interface {
	NewQuery() QueryBuilder
	// migrateup
	// migratedowne
}

// QueryBuilder builds a key value query
type QueryBuilder interface {
	InTransaction()
	Get(namespace string, key string)
	Set(namespace string, key string, value string)
	Delete(namespace string, key string)
	MustBuild() // MustBuild is intended to be called once per query on server startup for performance considerations of some providers.
	Execute() (result string, err error)
}
