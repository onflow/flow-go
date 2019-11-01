This is the old interface documentation that uses the impl syntax.  Saving here just in case we want to re-add it

## Interfaces

> 🚧 Status: Interfaces are implemented, but have the syntax `struct S: Interface1, ... {}`

An interface is an abstract type that specifies the behavior of types that *implement* the interface.
Interfaces declare the required functions and fields, as well as the access for those declarations, that implementing types need to provide.

There are two kinds of interfaces:

- **Structure interfaces**: implemented by [structures](#structures)
- **Resource interfaces**: implemented by [resources](#resources)

Structure and resource types may implement multiple interfaces.

Interfaces consist of the function and field requirements that a type implementing the interface must provide implementations for.
Interface requirements, and therefore also their implementations, must always be at least public.
Variable field requirements may be annotated to require them to be publicly settable.

Function requirements consist of the name of the function, parameter types, an optional return type,
and optional preconditions and postconditions.

Field requirements consist of the name and the type of the field.
Field requirements may optionally declare a getter requirement and a setter requirement, each with preconditions and postconditions.

Calling functions with preconditions and postconditions on interfaces instead of concrete implementations can improve the security of a program,
as it ensures that even if implementations change, some aspects of them will always hold.

### Interface Declaration

Interfaces are declared using the `struct` or `resource` keyword,
followed by the `interface` keyword,
the name of the interface,
and the requirements, which must be enclosed in opening and closing braces.

Field requirements can be annotated to require the implementation to be a variable field, by using the `var` keyword;
require the implementation to be a constant field, by using the `let` keyword;
or the field requirement may specify nothing,
in which case the implementation may either be a variable field, a constant field, or a synthetic field.

Field requirements and function requirements must specify the required level of access.
The access must be at least be public, so the `pub` keyword must be provided.
Variable field requirements can be specified to also be publicly settable by using the `pub(set)` keyword.

The special type `Self` can be used to refer to the type implementing the interface.

```bamboo,file=interface-declaration.bpl
// Declare a resource interface for a fungible token.
// Only resources can implement this resource interface
//
resource interface FungibleToken {

    // Require the implementing type to provide a field for the balance
    // that is readable in all scopes (`pub`).
    //
    // Neither the `var` keyword, nor the `let` keyword is used,
    // so the field may be implemented as either a variable field,
    // a constant field, or a synthetic field.
    //
    // The read balance must always be positive.
    //
    // NOTE: no requirement is made for the kind of field,
    // it can be either variable or constant in the implementation
    //
    pub balance: Int {
        get {
            post {
                result >= 0:
                    "Balances are always non-negative"
            }
        }
    }

    // Require the implementing type to provide an initializer that
    // given the initial balance, must initialize the balance field
    //
    init(balance: Int) {
        post {
            self.balance == balance:
                "the balance must be initialized to the initial balance"
        }

        // NOTE: no code
    }

    // Require the implementing type to provide a function that is
    // callable in all scopes, which withdraws an amount from
    // this fungible token and returns the withdrawn amount as
    // a new fungible token.
    //
    // The given amount must be positive and the function implementation
    // must add the amount to the balance.
    //
    // The function must return a new fungible token.
    //
    // NOTE: `<-Self` is the resource type implementing this interface
    //
    pub fun withdraw(amount: Int): <-Self {
        pre {
            amount > 0:
                "the amount must be positive"
            amount <= self.balance:
                "insufficient funds: the amount must be smaller or equal to the balance"
        }
        post {
            self.balance == before(self.balance) - amount:
                "the amount must be deducted from the balance"
        }

        // NOTE: no code
    }

    // Require the implementing type to provide a function that is
    // callable in all scopes, which deposits a fungible token
    // into this fungible token.
    //
    // The given token must be of the same type – a deposit of another
    // type is not possible.
    //
    // No precondition is required to check the given token's balance
    // is positive, as this condition is already ensured by
    // the field requirement.
    //
    // NOTE: the first parameter has the type `<-Self`,
    // i.e. the resource type implementing this interface
    //
    pub fun deposit(_ token: <-Self) {
        post {
            self.balance == before(self.balance) + token.balance:
                "the amount must be added to the balance"
        }

        // NOTE: no code
    }
}
```

Note that the required initializer and functions do not have any executable code.

Interfaces can only be declared globally, i.e. not inside of functions.

### Interface Implementation

Implementations for interfaces are declared using the `impl` keyword,
followed by the name of interface, the `for` keyword,
and the name of the composite data type (structure or resource) that provides the functionality required in the interface.

```bamboo,file=interface-implementation.bpl
// Declare a resource named `ExampleToken` with a variable field named `balance`,
// that can be written by functions of the type, but outer scopes can only read it
//
resource ExampleToken {

    // Implement the required field `balance` for the `FungibleToken` interface.
    // The interface does not specify if the field must be variable, constant,
    // so in order for this type (`ExampleToken`) to be able to write to the field,
    // but limit outer scopes to only read from the field, it is declared variable,
    // and only has public access (non-settable).
    //
    pub var balance: Int

    // Implement the required initializer for the `FungibleToken` interface:
    // accept an initial balance and initialize the `balance` field.
    //
    // This implementation satisfies the required postcondition
    //
    // NOTE: the postcondition declared in the interface
    // does not have to be repeated here in the implementation
    //
    init(balance: Int) {
        self.balance = balance
    }
}


// Declare the implementation of the interface `FungibleToken`
// for the resource `ExampleToken`
//
impl FungibleToken for ExampleToken {

    // Implement the required function named `withdraw` of the interface
    // `FungibleToken`, that withdraws an amount from the token's balance.
    //
    // The function must be public.
    //
    // This implementation satisfies the required postcondition.
    //
    // NOTE: neither the precondition nor the postcondition declared
    // in the interface have to be repeated here in the implementation
    //
    pub fun withdraw(amount: Int): <-ExampleToken {
        self.balance = self.balance - amount
        return create ExampleToken(balance: amount)
    }

    // Implement the required function named `deposit` of the interface
    // `FungibleToken`, that deposits the amount from the given token
    // to this token.
    //
    // The function must be public.
    //
    // NOTE: the type of the parameter is `<-ExampleToken`,
    // i.e., only a token of the same type can be deposited.
    //
    // This implementation satisfies the required postconditions.
    //
    // NOTE: neither the precondition nor the postcondition declared
    // in the interface have to be repeated here in the implementation
    //
    pub fun deposit(_ token: <-ExampleToken) {
        self.balance = self.balance + amount
        destroy token
    }
}

// Declare a constant which has type `ExampleToken`,
// and is initialized with such an example token
//
let token <- create ExampleToken(balance: 100)

// Withdraw 10 units from the token.
//
// The amount satisfies the precondition of the `withdraw` function
// in the `FungibleToken` interface
//
let withdrawn <- token.withdraw(amount: 10)

// The postcondition of the `withdraw` function in the `FungibleToken`
// interface ensured the balance field of the token was updated properly
//
// `token.balance` is 90
// `withdrawn.balance` is 10

// Deposit the withdrawn token into another one.
let receiver: ExampleToken <- // ...
receiver.deposit(<-withdrawn)

// Error: precondition of function `withdraw` in interface
// `FungibleToken` is not satisfied: the parameter `amount`
// is larger than the field `balance` (100 > 90)
//
token.withdraw(amount: 100)
```

The access level for variable fields in an implementation may be less restrictive than the interface requires. For example, an interface may require a field to be at least public (i.e. the `pub` keyword is specified), and an implementation may provide a variable field which is public, but also publicly settable (the `pub(set)` keyword is specified).

```bamboo
struct interface AnInterface {
    // Require the implementing type to provide a publicly readable
    // field named `a` that has type `Int`. It may be a constant field,
    // a variable field, or a synthetic field.
    //
    pub a: Int
}

struct AnImplementation {
    // Declare a publicly settable variable field named `a`that has type `Int`.
    // This implementation satisfies the requirement for interface `AnInterface`:
    // The field is at least publicly readable, but this implementation also
    // allows the field to be written to in all scopes
    //
    pub(set) var a: Int

    init(a: Int) {
        self.a = a
    }
}

impl AnInterface for AnImplementation {
    // This implementation is empty, as the declaration
    // of the structure `AnImplementation` already fully satisfies
    // the requirements of the interface `AnInterface`,
    // i.e. a field named `a` that has type `Int` must be provided
}
```

### Interface Type

Interfaces are types. Values implementing an interface can be used as initial values for constants and variables that have the interface as their type.

```bamboo,file=interface-type.bpl
// Declare an interface named `Shape`.
//
// Require implementing types to provide a field which returns the area,
// and a function which scales the shape by a given factor.
//
struct interface Shape {
    pub area: Int
    pub fun scale(factor: Int)
}

// Declare a structure named `Square`
//
struct Square {
    pub var length: Int

    pub synthetic area: Int {
        get {
            return self.length * self.length
        }
    }

    pub init(length: Int) {
        self.length = length
    }
}

// Implement the interface `Shape` for the structure `Square`
//
impl Shape for Square {

    pub fun scale(factor: Int) {
        self.length = self.length * factor
    }
}

// Declare a structure named `Rectangle`
//
struct Rectangle {
    pub var width: Int
    pub var height: Int

    pub synthetic area: Int {
        get {
            return self.width * self.height
        }
    }

    pub init(width: Int, height: Int) {
        self.width = width
        self.height = height
    }
}

// Implement the interface `Rectangle` for the structure `Square`
//
impl Shape for Rectangle {

    pub fun scale(factor: Int) {
        self.width = self.width * factor
        self.height = self.height * factor
    }
}

// Declare a constant that has type `Shape`, which has a value that has type `Rectangle`
//
var shape: Shape = Rectangle(width: 10, height: 20)
```

Values implementing an interface are assignable to variables that have the interface as their type.

```bamboo,file=interface-type-assignment.bpl
// Assign a value of type `Square` to the variable `shape` that has type `Shape`
//
shape = Square(length: 30)

// Invalid: cannot initialize a constant that has type `Rectangle`
// with a value that has type `Square`
//
let rectangle: Rectangle = Square(length: 10)
```

Fields declared in an interface can be accessed and functions declared in an interface can be called on values of a type that implements the interface.

```bamboo,file=interface-type-fields-and-functions.bpl
// Declare a constant which has the type `Shape`
// and is initialized with a value that has type `Rectangle`
//
let shape: Shape = Rectangle(width: 2, height: 3)

// Access the field `area` declared in the interface `Shape`
//
shape.area // is 6

// Call the function `scale` declared in the interface `Shape`
//
shape.scale(factor: 3)
```

### Interface Implementation Requirements

Interfaces can require implementing types to also implement other interfaces of the same kind.
Interface implementation requirements can be declared by following the interface name with a colon (`:`)
and one or more names of interfaces of the same kind, separated by commas.

```bamboo,file=interface-implementation-requirement.bpl
// Declare a structure interface named `Shape`
//
struct interface Shape {}

// Declare a structure interface named `Polygon`.
// Require implementing types to also implement
// the structure interface `Shape`
//
struct interface Polygon: Shape {}

// Declare a structure named `Hexagon`
//
struct Hexagon {}

// Implement the structure interface `Polygon`
// for the structure `Hexagon`
//
impl Polygon for Hexagon {}

// Implement the structure interface `Shape`
// for the structure `Hexagon`.
//
// This is required, as the interface `Polygon`
// specified this implementation requirement.
//
impl Shape for Hexagon {}
```

### Interface Nesting

Interfaces can be arbitrarily nested.
Declaring an interface inside another does not require implementing types of the outer interface to provide an implementation of the inner interfaces.

```bamboo,file=interface-nesting.bpl
// Declare a resource interface `OuterInterface`, which declares
// a nested structure interface named `InnerInterface`.
//
// Resources implementing `OuterInterface` do not need to provide
// an implementation of `InnerInterface`.
//
// Structures may just implement `InnerInterface`
//
resource interface OuterInterface {

    struct interface InnerInterface {}
}

// Declare a resource named `SomeOuter`
//
resource SomeOuter {}

// Implement the interface `OuterInterface` for the resource  `SomeOuter`.
//
// The resource is not required to implement `OuterInterface.InnerInterface`
//
impl OuterInterface for SomeOuter {}


// Declare a structure named `SomeInner`
//
struct SomeInner {}

// Implement the interface `InnerInterface` which is nested in
// interface `OuterInterface` for the structure `SomeInner`.
//
impl OuterInterface.InnerInterface for SomeInner {}
```

### Nested Type Requirements

Interfaces can require implementing types to provide concrete nested types.
For example, a resource interface may require an implementing type to provide a resource type.

```bamboo,file=interface-nested-type-requirement.bpl
// Declare a resource interface named `FungibleToken`.
//
// Require implementing types to provide a resource type named `Vault`
// which must have a field named `balance`
//
resource interface FungibleToken {

    pub resource Vault {
        pub balance: Int
    }
}

// Declare a resource named `ExampleToken`
//
resource ExampleToken {}

// Implement the resource interface `FungibleToken`
// for resource type `ExampleToken`.
//
// The nested type `Vault` must be provided
// to conform to the interface.
//
impl FungibleToken for ExampleToken {

    pub resource Vault {
        pub var balance: Int

        init(balance: Int) {
            self.balance = balance
        }
    }
}
```

### `Equatable` Interface

> 🚧 Status: The `Equatable` interface is not implemented yet.

An equatable type is a type that can be compared for equality. Types are equatable when they  implement the `Equatable` interface.

Equatable types can be compared for equality using the equals operator (`==`) or inequality using the unequals operator (`!=`).

Most of the built-in types are equatable, like booleans and integers. Arrays are equatable when their elements are equatable. Dictionaries are equatable when their values are equatable.

To make a type equatable the `Equatable` interface must be implemented, which requires the implementation of the function `equals`, which accepts another value that the given value should be compared for equality. Note that the parameter type is `Self`, i.e., the other value must have the same type as the implementing type.

```bamboo,file=equatable.bpl
struct interface Equatable {
    pub fun equals(_ other: Self): Bool
}
```

```bamboo,file=equatable-impl.bpl
// Declare a struct named `Cat`, which has one field named `id`
// that has type `Int`, i.e., the identifier of the cat.
//
struct Cat {
    pub let id: Int

    init(id: Int) {
        self.id = id
    }
}

// Implement the interface `Equatable` for the type `Cat`,
// to allow cats to be compared for equality.
//
impl Equatable for Cat {

    pub fun equals(_ other: Self): Bool {
        // Cats are equal if their identifier matches.
        //
        return other.id == self.id
    }
}

Cat(1) == Cat(2) // is false
Cat(3) == Cat(3) // is true
```

### `Hashable` Interface

> 🚧 Status: The `Hashable` interface is not implemented yet.

A hashable type is a type that can be hashed to an integer hash value, i.e., it is distilled into a value that is used as evidence of inequality. Types are hashable when they implement the `Hashable` interface.

Hashable types can be used as keys in dictionaries.

Hashable types must also be equatable, i.e., they must also implement the `Equatable` interface. This is because the hash value is only evidence for inequality: two values that have different hash values are guaranteed to be unequal. However, if the hash values of two values are the same, then the two values could still be unequal and just happen to hash to the same hash value. In that case equality still needs to be determined through an equality check. Without `Equatable`, values could be added to a dictionary, but it would not be possible to retrieve them.

Most of the built-in types are hashable, like booleans and integers. Arrays are hashable when their elements are hashable. Dictionaries are hashable when their values are equatable.

Hashing a value means passing its essential components into a hash function. Essential components are those that are used in the type's implementation of `Equatable`.

If two values are equal because their `equals` function returns true, then the implementation must return the same integer hash value for each of the two values.

The implementation must also consistently return the same integer hash value during the execution of the program when the essential components have not changed. The integer hash value must not necessarily be the same across multiple executions.

```bamboo,file=hashable.bpl
struct interface Hashable: Equatable {
    pub hashValue: Int
}
```

```bamboo,file=hashable-impl.bpl
// Declare a structure named `Point` with two fields
// named `x` and `y` that have type `Int`.
//
struct Point {

    pub(set) var x: Int
    pub(set) var y: Int

    init(x: Int, y: Int) {
        self.x = x
        self.y = y
    }
}

// Implement the interface `Equatable` for the type `Point`,
// to allow points to be compared for equality.
//
impl Equatable for Point {

    pub fun equals(_ other: Self): Bool {
        // Points are equal if their coordinates match.
        //
        // The essential components are therefore the fields
        // `x` and `y`, which must be used in the `Hashable`
        // implementation.
        //
        return other.x == self.x
            && other.y == self.y
    }
}

// Implement the interface `Equatable` for the type `Point`.
//
impl Hashable for Point {

    pub synthetic hashValue: Int {
        get {
            var hash = 7
            hash = 31 * hash + self.x
            hash = 31 * hash + self.y
            return hash
        }
    }
}
```