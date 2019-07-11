// Package keyvalue provides an abstract interface for connecting to DBs and building/executing key-value queries on them.
package keyvalue

// DBConnector abstracts a db connection
type DBConnector interface {
	// NewQueryBuilder returns an instance of a new QueryBuilder. Intended to be used when building a custom multi statement query
	NewQueryBuilder() QueryBuilder
	// MigrateUp performs all the steps required to bring the backing DB into an initialised state
	MigrateUp() error
	// MigrateDown is the inverse of MigrateUp and intended to be used in testing environment to achieve a "clean slate".
	MigrateDown() error
}

/*
QueryBuilder builds a key value query and allow
For exmaple:
```go
  dbConn := NewPostgresDB(options...)

  setFooAndBar := dbConn.NewQueryBuilder().
	  AddSet("foo_collection").
	  AddSet("bar_collection").
	  InTransaction().
	  MustBuild()

  setFooAndBar.Execute("key1", "value1", "key2", "value2")
```
*/
type QueryBuilder interface {
	// InTransaction sets a query to run in a multi statement transaction
	InTransaction() QueryBuilder
	// AddGet adds a get statement
	AddGet(namespace string) QueryBuilder
	// AddSet adds a set statement
	AddSet(namespace string) QueryBuilder
	// AddDelete adds a delete statement
	AddDelete(namespace string) QueryBuilder
	// MustBuild is intended to be called once per query on server startup for performance considerations of some providers.
	MustBuild() Query
}

// Query provides a way to execute a query
type Query interface {
	// Execute runs the query and returns its result
	Execute(params ...string) (result string, err error)
}
