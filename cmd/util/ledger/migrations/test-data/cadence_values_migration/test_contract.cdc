pub contract Test {

	pub struct interface Foo {}

	pub struct interface Bar {}

	pub struct interface Baz {}

	pub resource R {
        pub fun foo() {}
	}

	pub fun createR(): @R {
		return <- create R()
	}
}
