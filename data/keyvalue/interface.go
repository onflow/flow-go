package keyvalue

// DBConnecter abstracts a db connection
type DBConnecter interface {
	NewQuery() QueryBuilder
	MigrateUp() error
	MigrateDown() error
}

// QueryBuilder builds a key value query
type QueryBuilder interface {
	InTransaction()
	Get(namespace string, key string)
	Set(namespace string, key string)
	Delete(namespace string, key string)
	MustBuild() // MustBuild is intended to be called once per query on server startup for performance considerations of some providers.
	Execute(setParams ...string) (result string, err error)
}
