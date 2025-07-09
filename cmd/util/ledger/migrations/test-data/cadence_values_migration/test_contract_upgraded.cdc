// Unused import. But keep it here for testing the import resolving.
import NonFungibleToken from 0xf8d6e0586b0a20c7

access(all) contract Test {

	access(all) struct interface Foo {}

	access(all) struct interface Bar {}

	access(all) struct interface Baz {}

	access(all) entitlement E

	access(all) resource R {
        access(E) fun foo() {}
	}

	access(all) fun createR(): @R {
		return <- create R()
	}
}
