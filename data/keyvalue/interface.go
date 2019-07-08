package keyvalue

// DBConnecter abstracts a db connection
type DBConnecter interface {
	NewQuery() QueryBuilder
	MigrateUp() error
	MigrateDown() error
}

// QueryBuilder builds a key value query
type QueryBuilder interface {
	InTransaction() QueryBuilder
	Get(namespace string, key string) QueryBuilder
	Set(namespace string, key string) QueryBuilder
	Delete(namespace string, key string) QueryBuilder
	MustBuild() QueryBuilder // MustBuild is intended to be called once per query on server startup for performance considerations of some providers.
	Execute(setParams ...string) (result string, err error)
}
