// Package keyvalue provides an abstract interface for connecting to DBs and building/executing key-value queries on them.
package keyvalue

// DBConnector abstracts a db connection
type DBConnector interface {
	NewQuery() QueryBuilder
	MigrateUp() error
	MigrateDown() error
}

/*
QueryBuilder builds a key value query and allow
For exmaple:
	dbConn := NewPostgresDB(options...)

	setFooAndBar := dbConn.NewQuery().
		Set("foo", "keyA").
		Set("bar", "keyB").
		InTransaction().
		MustBuild()

	setFooAndBar.Execute("value1", "value2")
*/
type QueryBuilder interface {
	// InTransaction sets a query to run in a multi statement transaction
	InTransaction() QueryBuilder
	Get(namespace string, key string) QueryBuilder
	Set(namespace string, key string) QueryBuilder
	Delete(namespace string, key string) QueryBuilder
	MustBuild() QueryBuilder // MustBuild is intended to be called once per query on server startup for performance considerations of some providers.
	Execute(setParams ...string) (result string, err error)
}
