// Package keyvalue provides an abstract interface for connecting to DBs and building/executing key-value queries on them.
/*
Some examples:

```go
  dbConn := NewPostgresDB(options...)

  setFooAndBar := dbConn.NewQueryBuilder().
	  AddSet("foo_collection").
	  AddSet("bar_collection").
	  InTransaction().
	  MustBuild()

  _, err := setFooAndBar.Execute("key1", "value1", "key2", "value2")
```

```go
  dbConn := NewPostgresDB(options...)

  result, err := dbConn.NewQueryBuilder().GetQuery.Execute("foo_collection", "key1")
  _, err := dbConn.NewQueryBuilder().SetQuery.Execute("foo_collection", "key1", "value1")
  _, err := dbConn.NewQueryBuilder().DeleteQuery.Execute("foo_collection", "key1")
```
*/
package keyvalue

// DBConnector abstracts a db connection
type DBConnector interface {
	// NewQueryBuilder returns an instance of a new QueryBuilder. Intended to be used when building a custom multi statement query
	NewQueryBuilder() QueryBuilder
	// GetQuery returns a pre-built QueryBuilder instance ready to be executed as a get statement
	GetQuery() Query
	// SetQuery returns a pre-built QueryBuilder instance ready to be executed as a get statement
	SetQuery() Query
	// DeleteQuery returns a pre-built QueryBuilder instance ready to be executed as a delete statement
	DeleteQuery() Query
	// MigrateUp performs all the steps required to bring the backing DB into an initialised state
	MigrateUp() error
	// MigrateDown is the inverse of MigrateUp and intended to be used in testing environment to achieve a "clean slate".
	MigrateDown() error
}

// QueryBuilder builds a key value query and allow
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
