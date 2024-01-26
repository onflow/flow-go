access(all) contract Test {

	access(all) struct interface Foo {}

	access(all) struct interface Bar {}

	access(all) struct interface Baz {}

	access(all) resource R {
        access(all) fun foo() {}
	}

	access(all) fun createR(): @R {
		return <- create R()
	}
}
