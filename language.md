# Bamboo Programming Language

The Bamboo Programming Language is a new high-level programming language intended for smart contract development.

The language's goals are, in order of importance:

- *Safety and security*: Focus on safe code: provide a strong static type system, design by contract, a capability system, and linear types.
- *Auditability*: Focus on readability: make it easy to verify what the code is doing.
- *Simplicity*: Focus on developer productivity and usability: make it easy to write code, provide good tooling.


## Syntax and Behavior

The programming language's syntax and behavior is inspired by Kotlin, Swift, Rust, TypeScript, and Solidity.


## Comments

Comments can be used to document code. A comment is text that is not executed.

*Single-line comments* start with two slashes (`//`):

```
// This is a comment on a single line.
// Another comment line that is not executed.
```

*Multi-line comments* start with a slash and an asterisk (`/*`) and end with an asterisk and a slash (`*/`):

```
/* This is a comment which
spans multiple lines. */
```

Comments may be nested.

```
/* /* this */ is a valid comment */
```

## Constants and Variable Declarations

Constants and variables are declarations that bind a value to a name. Constants can only be initialized with a value and cannot be reassigned afterwards. Variables can be initialized with a value and can be reassigned later. Declarations are valid in any scope, including the global scope.

Constant means that the *name* is constant, not the *value*, i.e., the value may still be changed if it allows it.

The `const` keyword is used to declare a constant and the `var` keyword is used to declare a variable.
The keywords is followed by the name, an optional [type annotation](#Type Annotations), an equals sign `=`, and the initial value.

```typescript
// declaring a constant
//
const a = 1

// error: re-assigning to a constant
//
a = 2

// declaring a variable
//
var b = 3

// assigning a new value to a variable
//
b = 4
```

Variables and constants **must** be initialized.

```typescript
// invalid: constant has no initial value
//
const a
```

Once a constant or variable is declared, it can't be redeclared with the same name, with a different type, or changed into the corresponding other kind.


```typescript
// declaring a constant
//
const a = 1

// invalid: re-declaring a constant with a name that is already used in this scope
//
const a = 2

// declaring a variable
//
var b = 3

// invalid: re-declaring a variable with a name that is already used in this scope
//
var b = 4

// invalid: declaring a variable with a name that was used for a constant
//
var a = 5
```

## Type Annotations

When declaring a constant or variable, an optional *type annotation* can be provided, to make it explicit what type the declaration has.
If no type annotation is provided, the type of the declaration is [inferred from the initial value](#type-inference).

```typescript
// declaring a variable with an explicit type
//
var initialized: bool = false

// declaring a constant with an inferred type
//
const a = 1
```

## Naming

Names may start with any upper and lowercase letter or an underscore. This may be followed by zero or more upper and lower case letters, underscores, and numbers.
Names may not begin with a number.

```typescript
// valid
//
PersonID

// valid
//
token_name

// valid
//
_balance

// valid
//
account2

// invalid
//
!@#$%^&*

// invalid
//
1something
```

### Conventions

By convention, variables, constants, and functions have lowercase names; and types have title-case names.


## Semicolons

Semicolons may be used to separate statements, but are optional. They can be used to separate multiple statements on a single line.

```typescript
// constant declaration statement without a semicolon
//
const a = 1

// variable declarations statement with a semicolon
//
var b = 2;

// multiple statements on a single line
//
const d = 1; var e = 2
```

## Values and Types

Values are objects like booleans and integers. Values are typed.

### Booleans

The two boolean values `true` and `false` are of type `Bool`.

### Integers

Integers are whole numbers without a fractional part. They are either *signed* (positive, zero, or negative) or *unsigned* (positive or zero) and are either 8 bits, 16 bits, 32 bits, 64 bits or arbitrarily large.

The names for the integer types follow this naming convention: Signed integer types have an `Int` prefix, unsigned integer types have a `UInt` prefix, i.e., the integer types are named `Int8`, `Int16`, `Int32`, `Int64`, `UInt8`, `UInt16`, `UInt32`, and `UInt64`.

 - **`Int8`**: -128 through 127
 - **`Int16`**: -32768 through 32767
 - **`Int32`**: -2147483648 through 2147483647
 - **`Int64`**: -9223372036854775808 through 9223372036854775807
 - **`UInt16`**: 0 through 65535
 - **`UInt32`**: 0 through 4294967295
 - **`UInt64`**: 0 through 18446744073709551615

In addition, the arbitrary precision integer type `Int` is provided.

### Floating-Point Numbers

There is no support for floating point numbers.

Contracts are not intended to work with values with error margins and therefore floating point arithmetic is not appropriate here. Fixed point numbers should be simulated using integers and a scale factor for now.

### Numeric Literals

Numbers can be written in various bases. Numbers are assumed to be decimal by default. Non-decimal literals have a specific prefix.

| Numeral system  | Prefix | Characters                                                            |
|:----------------|:-------|:----------------------------------------------------------------------|
| **Decimal**     | *None* | one or more numbers (`0` to `9`)                                      |
| **Binary**      | `0b`   | one or more zeros or ones (`0` or `1`)                                |
| **Octal**       | `0o`   | one or more numbers in the range `0` to `7`                           |
| **Hexadecimal** | `0x`   | one or more numbers, or characters `a` to `f`, lowercase or uppercase |

```typescript
// a decimal number
//
const dec = 1234567890

// a binary number
//
const bin = 0b101010

// an octal number
//
const oct = 0o12345670

// a hexadecimal number
//
const hex = 0x1234567890ABCDEFabcdef
```

Decimal numbers may contain underscores (`_`) to logically separate components.

```typescript
const aLargeNumber = 1_000_000
```

### Arrays

Arrays are mutable, ordered collections of values. All values in an array must have the same type. Arrays may contain a value multiple times. Array literals start with an opening square bracket `[` and end with a closing square bracket `]`.

```typescript
// an empty array
//
const empty = []

// an array with integers
//
const integers = [1, 2, 3]

// invalid: mixed types
//
const invalidMixed = [1, true, 2, false]
```

#### Array Indexing

To get the element of an array at a specific index, the indexing syntax can be used: The array is followed by an opening square bracket `[`, the indexing value, and ends with a closing square bracket `]`.

```typescript
const numbers = [42, 23]

// get the first number
//
numbers[0] // is 42

// get the second number
//
numbers[1] // is 23
```

```typescript
const arrays = [[1, 2], [3, 4]]

// get the first number of the second array
//
arrays[1][0] // is 3
```

To set an element of an array at a specific index, the indexing syntax can be used as well.

```typescript
const numbers = [42, 23]

// change the second number
//
numbers[1] = 2

// numbers is [42, 2]
```

#### Array Types

Arrays either have a fixed size or are variably sized, i.e., elements can be added an removed.

Fixed-size arrays have the type suffix `[N]`, where `N` is the size of the array. For example, a fixed-size array of 3 `Int8` elements has the type `Int8[3]`.

Variable-size arrays have the type suffix `[]`. For example, a variable-size array of `Int16` elements has the type `Int16[]`.


<!--

TODO

#### Array Functions

- Length, concatenate, filter, etc. for all array types
- Append, remove, etc. for variable-size arrays
- Document and link to array concatenation operator `+` in operators section

-->


## Dictionaries

> Status: Dictionaries are not implemented yet.

Dictionaries are mutable, unordered collections of key-value associations. In a dictionary, all keys must have the same type, and all values must have the same type. Dictionaries may contain a key only once and may contain a value multiple times.

Dictionary literals start with an opening brace `{` and end with a closing brace `}`. Keys are separated from values by a colon, and key-value associations are separated by commas.

```typescript
// an empty dictionary
//
const empty = {}

// a dictionary mapping integers to booleans
//
const dictionary = {
    1: true,
    2: false
}

// invalid: mixed types
//
const invalidMixed = {
    1: true,
    false: 2
}
```

#### Dictionary Access

To get the value for a specific key from a dictionary, the access syntax can be used: The dictionary is followed by an opening square bracket `[`, the key, and ends with a closing square bracket `]`.

```typescript
const booleans = {
    1: true,
    0: false
}
booleans[1] // is true
booleans[0] // is false

const integers = {
    true: 1,
    false: 0
}
integers[true] // is 1
integers[false] // is 0
```

To set the value for a key of a dictionary, the access syntax can be used as well.

```typescript
const booleans = {
    1: true,
    0: false
}
booleans[1] = false
booleans[0] = true
// booleans is {1: false, 0: true}
```


#### Dictionary Types

Dictionaries have the type suffix `[T]`, where `T` is the type of the key. For example, a dictionary with `Int` keys and `Bool` values has type `Bool[Int]`.

```typescript
const booleans = {
    1: true,
    0: false
}
// booleans has type Bool[Int]

const integers = {
    true: 1,
    false: 0
}
// integers has type Int[Bool]
```

<!--

TODO

#### Dictionary Functions

-->

## Operators

Operators are special symbols that perform a computation for one or more values. They are either unary, binary, or ternary.

- Unary operators perform an operation for a single value. The unary operator symbol appears before the value.

- Binary operators operate on two values. The binary operator symbol appears between the two values (infix).

- Ternary operators operate on three values. The operator symbols appear between the three values (infix).


### Negation

The `-` unary operator negates an integer:

```typescript
const a = 1
-a // is -1
```

The `!` unary operator logically negates a boolean:

```typescript
const a = true
!a // is false
```

### Assignment

The binary assignment operator `=` can be used to assign a new value to a variable. It is only allowed in a statement and is not allowed in expressions.

```typescript
var a = 1
a = 2
// a is 2
```

The left-hand side of the assignment must be an identifier, followed by one or more index or access expressions.

```typescript
const numbers = [1, 2]

// change the first number
//
numbers[0] = 3

// numbers is [3, 2]
```

```typescript
const arrays = [[1, 2], [3, 4]]

// change the first number in the second array
//
arrays[1][0] = 5

// arrays is [[1, 2], [5, 4]]
```

```typescript
const dictionaries = {
  true: {1: 2},
  false: {3: 4}
}

dictionaries[false][3] = 0

// dictionaries is {
//   true: {1: 2},
//   false: {3: 0}
//}
```

### Arithmetic

There are four arithmetic operators:

- Addition: `+`
- Subtraction: `-`
- Multiplication: `*`
- Division: `/`

```typescript
const a = 1 + 2
// a is 3
```

Arithmetic operators don't cause values to overflow.

```typescript
const a: Int8 = 100
const b: Int8 = 100
const c = a * b
// c is 10000, and of type `Int`.
```

If overflow behavior is intended, overflowing operators are available, which are prefixed with an `&`:

- Overflow addition: `&+`
- Overflow subtraction: `&-`
- Overflow multiplication: `&*`

For example, the maximum value of an unsigned 8-bit integer is 255 (binary 11111111). Adding 1 results in an overflow, truncation to 8 bits, and the value 0.

```typescript
//     11111111 = 255
// &+         1
//  = 100000000 = 0
```

```typescript
const a: UInt8 = 255
a &+ 1 // is 0
```

Similarly, for the minimum value 0, subtracting 1 wraps around and results in the maximum value 255.

```typescript
//     00000000
// &-         1
//  =  11111111 = 255
```

```typescript
const b: UInt8 = 0
b &- 1 // is 255
```

Signed integers are also affected by overflow. In a signed integer, the first bit is used for the sign. This leaves 7 bits for the actual value for an 8-bit signed integer, i.e., the range of values is -128 (binary 10000000) to 127 (01111111). Subtracting 1 from -128 results in 127.

```typescript
//    10000000 = -128
// &-        1
//  = 01111111 = 127
```

```typescript
const c: Int8 = -128
c &- 1 // is 127
```

### Logical Operators

Logical operators work with the boolean values `true` and `false`.

- Logical AND: `a && b`

  ```typescript
  true && true // is true
  true && false // is false
  false && false // is false
  false && false // is false
  ```

- Logical OR: `a || b`

  ```typescript
  true || true // is true
  true || false // is true
  false || false // is true
  false || false // is false
  ```

### Comparison operators

Comparison operators work with boolean and integer values.


- Equality: `==`, for booleans and integers

  ```typescript
  1 == 1 // is true
  1 == 2 // is false
  true == true // is true
  true == false // is false
  ```

- Inequality: `!=`, for booleans and integers

  ```typescript
  1 != 1 // is false
  1 != 2 // is true
  true != true // is false
  true != false // is true
  ```

- Less than: `<`, for integers

  ```typescript
  1 < 1 // is false
  1 < 2 // is true
  2 < 1 // is false
  ```

- Less or equal than: `<=`, for integers

  ```typescript
  1 <= 1 // is true
  1 <= 2 // is true
  2 <= 1 // is false
  ```

- Greater than: `>`, for integers

  ```typescript
  1 > 1 // is false
  1 > 2 // is false
  2 > 1 // is true
  ```


- Greater or equal than: `>=`, for integers

  ```typescript
  1 >= 1 // is true
  1 >= 2 // is false
  2 >= 1 // is true
  ```


### Ternary Conditional Operator

There is only one ternary conditional operator, the ternary conditional operator (`a ? b : c`).

It behaves like an if-statement, but is an expression: If the first operator value is true, the second operator value is returned. If the first operator value is false, the third value is returned.

```typescript
const x = 1 > 2 ? 3 : 4
x // is 4
```


### Parentheses

Expressions can be wrapped in parentheses to avoid ambiguity.

```typescript
const a = (1 + 2) * 3
// a is 9
```

### Precedence and Associativity

Operators have the following precedences, highest to lowest:

- Multiplication precedence: `*`, `&*`, `/`, `%`
- Addition precedence: `+`, `&+`, `-`, `&-`
- Relational precedence: `<`, `<=`, `>`, `>=`
- Equality precedence: `==`, `!=`
- Logical conjunction precedence: `&&`
- Logical disjunction precedence: `||`
- Ternary precedence: `? :`

All operators are left-associative, except for the ternary operator, which is right-associative.


## Functions

Functions are sequences of statements that perform a specific task. Functions have parameters and an optional return value. Functions are typed: the function type consists of the parameter types and the return type.

Functions are values, i.e., they can be assigned to constants and variables, and can be passed as arguments to other functions. This behavior is often called "first-class functions".

### Function Declarations

Functions can be declared by using the `fun` keyword, followed by the name of the declaration, the parameters, the optional return type, and the code that should be executed when the function is called.

The parameters need to be enclosed in parentheses. Each parameter needs to have a type annotation, which follows the parameter name after a colon. The return type is also separated from the parameters with a colon. The function code needs to be enclosed in opening and closing braces.

```typescript
// declare a function called double, which multiples a number by two
//
fun double(x: Int): Int {
    return x * 2
}
```

Functions can be nested, i.e., the code of a function may declare further functions.

```typescript
// declare a function which multiplies a number by two, and adds one
//
fun doubleAndAddOne(n: Int): Int {

    // declare a nested function which doubles, which multiplies a number by two
    //
    fun double(x: Int) {
        return x * 2
    }

    return double(n) + 1
}
```

### Function Expressions

Functions can be also used as expressions. The syntax is the same as for function declarations, except that function expressions have no name, i.e., it is anonymous.

```typescript
// declare a constant called double, which has a function as its value,
// which multiplies a number by two when called
//
const double = fun (x: Int): Int {
     return x * 2
}
```

### Function Calls

Functions can be called (invoked). Function calls need to provide exactly as many argument values as the function has parameters.

```typescript
fun double(x: Int): Int {
     return x * 2
}

double(2) // is 4

// invalid: too many arguments
//
double(2, 3)

// invalid: too few arguments
//
double()
```

### Function Types

Function types consist of the function's parameter types and the function's return type.
The parameter types need to be enclosed in parentheses, followed by the `=>` keyword, and end with the return type.

```typescript
// declare a function called add, with the function type (Int, Int) => Int
//
fun add(a: Int, b: Int): Int {
    return a + b
}
```

```typescript
// declare a constant called add, with the function type (Int, Int) => Int
//
const add: (Int, Int) => Int =
    fun (a: Int, b: Int): Int {
        return a + b
    }
```

If the function has no return type, it implicitly has the return type `Void`.

```typescript
// declare a constant called doNothing, which is a function
// that takes no parameters and returns nothing
//
const doNothing: () => Void =
    fun () {}
```

#### Argument Passing Behavior

When arguments are passed to a function, they are not copied. Instead, parameters act as new variable bindings and the values they refer to are identical to the passed values. Modifications to mutable values made within a function will be visible to the caller. This behavior is known as [call-by-sharing](https://en.wikipedia.org/w/index.php?title=Evaluation_strategy&oldid=896280571#Call_by_sharing).


```typescript
fun change(numbers: Int[]) {
     numbers[0] = 1
     numbers[1] = 2
}

const numbers = [0, 1]

change(numbers)
// numbers is [1, 2]
```

Parameters are constant, i.e., it is not allowed to assign to them.

```typescript
fun test(x: Int) {
     // invalid: assignment to parameter (constant)
     //
     x = 2
}
```

### Function Preconditions and Postconditions

> Status: Function Preconditions and Postconditions are not implemented yet.

Functions may have preconditions and may have postconditions.

Preconditions must be true right before the execution of the function. Preconditions are part of the function and introduced by the `require` keyword, followed by the condition block.

Postconditions must be true right after the execution of the function. Postconditions are part of the function and introduced by the `ensure` keyword, followed by the condition block. Postconditions may only occur after preconditions, if any.

A conditions block consists of one or more conditions. Conditions are expressions evaluating to a boolean. Conditions may be written on separate lines, or multiple conditions can be written on the same line, separated by a semicolon. This syntax follows the syntax for [statements](#semicolons).

```typescript
fun factorial(n: Int): Int {
    require {
        // factorial is only defined for integers greater than or equal to zero
        //
        n >= 0
    }
    ensure {
        // the result will always be greater than or equal to 1
        //
        result >= 1
    }

    var i = n
    var result = 1

    while i > 1 {
        result = result * i
        i = i - 1
    }

    return result
}

factorial(5) // is 120

// error: argument does not satisfy precondition n >= 0
//
factorial(-2)
```

In postconditions, the special function `before` can be used to get the value of an expression just before the function is called.

```typescript
var n = 0

fun incrementN() {
    ensure {
        // require the new value of n to be the old value of n, plus one
        //
        n == before(n) + 1
    }

    n = n + 1
}
```

## Control flow

Control flow statements control the flow of execution in a function.

### Conditional branching: if-statement

If-statements allow a certain piece of code to be executed only when a given condition is true.

The if-statement starts with the `if` keyword, followed by the condition, and the code that should be executed if the condition is true inside opening and closing braces. The condition must be boolean and the braces are required.


```typescript
const a = 0
var b = 0

if a == 0 {
   b = 1
}

if a != 0 {
   b = 2
}

// b is 1
```

An additional else-clause can be added to execute another piece of code when the condition is false.
The else-clause is introduced by the `else` keyword.

```typescript
const a = 0
var b = 0

if a == 1 {
   b = 1
} else {
   b = 2
}

// b is 2
```

The else-clause can contain another if-statement, i.e., if-statements can be chained together.

```typescript
const a = 0
var b = 0

if a == 1 {
   b = 1
} else if a == 2 {
   b = 2
} else {
   b = 3
}

// b is 3
```

### Looping: while-statement

While-statements allow a certain piece of code to be executed repeatedly, as long as a condition remains true.

The while-statement starts with the `while` keyword, followed by the condition, and the code that should be repeatedly executed if the condition is true inside opening and closing braces. The condition must be boolean and the braces are required.

The while-statement will first evaluate the condition. If the condition is false, the execution is done.
If it is true, the piece of code is executed and the evaluation of the condition is repeated. Thus, the piece of code is executed zero or more times.

```typescript
var a = 0
while a < 5 {
    a = a + 1
}

// a is 5
```

### Immediate function return: return-statement

The return-statement causes a function to return immediately, i.e., any code after the return-statement is not executed. The return-statement starts with the `return` keyword and is followed by an optional expression that should be the return value of the function call.

<!--
TODO: examples

- in function
- in while
- in if
-->

## Scope

Every function and block (`{` ... `}`) introduces a new scope for declarations. Each function and block can refer to declarations in its scope or any of the outer scopes.

```typescript
const x = 10

fun f(): Int {
    const y = 10
    return x + y
}

f() // is 20

// invalid: y is not in scope
//
y
```

```typescript
fun doubleAndAddOne(n: Int): Int {
    fun double(x: Int) {
        return x * 2
    }
    return double(n) + 1
}

// invalid: `double` is not in scope
//
double(1)
```

Each scope can introduce new declarations, i.e., the outer declaration is shadowed.

```typescript
const x = 2

fun test(): Int {
    const x = 3
    return x
}

test() // is 3
```

Scope is lexical, not dynamic.

```typescript
const x = 10

fun f(): Int {
   return x
}

fun g(): Int {
   const x = 20
   return f()
}

g() // is 10, not 20
```

Declarations are **not** moved to the top of the enclosing function (hoisted).

```typescript
const x = 2

fun f(): Int {
    if x == 0 {
        const x = 3
        return x
    }
    return x
}
f() // returns 2
```

## Type Safety

> Status: Type checking is not implemented yet.

The Bamboo programming language is a _type-safe_ language.

When assigning a new value to a variable, the value must be the same type as the variable. For example, if a variable is of type `Bool`, it can _only_ be assigned a `Bool`, and not for example an `Int`.

```typescript
var a = true

// invalid: integer is not a boolean
//
a = 0
```

When passing arguments to a function, the types of the values must match the function parameters' types. For example, if a function expects an argument of type `Bool`, _only_ a `Bool` can be provided, and not for example an `Int`.

```typescript
fun nand(a: Bool, b: Bool): Bool {
    return !(a && b)
}

nand(false, false) // is true

// invalid: integers are not booleans
//
nand(0, 0)
```

Types are *not* automatically converted. For example, an integer is not automatically converted to a boolean, nor is an `Int32` automatically converted to an `Int8`.

```typescript
fun add(a: Int8, b: Int8): Int {
    return a + b
}

add(1, 2) // is 3

const a: Int32 = 3_000_000_000
const b: Int32 = 3_000_000_000

// invalid: Int32 is not Int8
//
add(a, b)
```


## Type Inference

> Status: Type inference is not implemented yet.

If a variable or constant is not annotated explicitly with a type, it is inferred from the value.

Integer literals are inferred to type `Int`.

```typescript
const a = 1

// a has type Int
```

Array literals are inferred based on the elements of the literal, and to be variable-size.

```typescript
const integers = [1, 2]
// integers has type Int[]

// invalid: mixed types
//
const invalidMixed = [1, true, 2, false]
```

Dictionary literals are inferred based on the keys and values of the literal.

```typescript
const booleans = {
    1: true,
    2: false
}
// booleans has type Bool[Int]

// invalid: mixed types
//
const invalidMixed = {
    1: true,
    false: 2
}
```

Functions are inferred based on the parameter types and the return type.

```typescript
const add = (a: Int8, b: Int8): Int {
    return a + b
}

// add has type (Int8, Int8) => Int
```

## Structures and Classes

> Status: Structures and classes are not implemented yet.

Structures and classes are composite types. Structures and classes consist of one or more values, which are stored in named fields. Each field may have a different type.

Structures are declared using the `struct` keyword. Classes are declared using the `class` keyword. The keyword is followed by the name.

```typescript
struct SomeStruct {
    // ...
}

class SomeClass {
    // ...
}
```

Fields are declared like variables and constants, however, they have no initial value.The initial values for fields are set in the initializer. All fields **must** be initialized in the initializer. The initialier is declared using the `init` keyword. Just like a function, it takes parameters. However, it has no return type, i.e., it is always `Void`. The initializer always follows any fields.

```typescript
// declare a token struct, which has a constant field
// named id and a variable field named balance.
// both fields are initialized through the initializer.
//
struct Token {
    const id: Int
    var balance: Int

    init(id: Int, balance: Int) {
        this.id = id
        this.balance = balance
    }
}
```

In initializers, the special constant `this` refers to the structure or class that is to be initialized.

Values of a structure or class type are created (instantiated) by calling the type like functions.

```typescript
const token = Token(42, 1_000_00)
```

Fields can be read (if they are constant or variable) and set (if they are variable), using the access syntax: the structure or class instance is followed by a dot (`.`) and the name of the field.

```typescript
token.id // is 42
token.balance // is 1_000_000

token.balance = 1
// token.balance is 1

// invalid: assignment to constant field
//
token.id = 23
```

Structures and classes may contain functions. Just like in the initializer, the special constant `this` refers to the structure or class that the function is called on.

```typescript
struct Token {
    const id: Int
    var balance: Int

    init(id: Int, balance: Int) {
        this.id = id
        this.balance = balance
    }

    fun mint(value: Int) {
        this.balance = this.balance + value
    }
}

const token = Token(32, 0)
token.mint(1_000_000)
// token.balance is 1_000_000
```

The only difference between structures and classes is their behavior when used as an initial value for another constant or variable, when assigned to a different variable, or passed as an argument to a function: Structures are *copied*, i.e. they are value types, whereas classes are *referenced*, i.e., they are reference types.

```typescript
// declare a structure with a variable integer field
//
struct SomeStruct {
    var value: Int

    init(value: Int) {
        this.value = value
    }

}

// create a value of structure type SomeStruct
//
const structA = SomeStruct(0)

// *copy* the structure value into a new constant
//
const structB = structA

structB.value = 1

// structA.value is *0*
```

```typescript
// declare a class with a variable integer field
//
class SomeClass {
    var value: Int

    init(value: Int) {
        this.value = value
    }
}

// create a value of class type SomeClass
//
const classA = SomeClass(0)

// *reference* the class value with a new constant
//
const classB = classA

classB.value = 1

// classA.value is *1*
```

Note the different values in the last line of each example.

There is **no** support for nulls, i.e., a constant or variable of a reference type must always be bound to an instance of the type. There is *no* `null`.

## Access control

> Status: Access control is not implemented yet.

Access control allows making certain parts of the program accessible/visible and making other parts inaccessible/invisible. Top-level declarations (variables, constants, functions, structures, classes) and fields (in structures, classes) are either private or public.

**Private** means the declaration is only accessible/visible in the currrent and inner scopes. For example, a private field in a class can only be accessed by functions of the class, not by code that useses an instance of the class in an outer scope.

**Public** means the declaration is accessible/visible in all scopes, the current and inner scopes like for private, and the outer scopes. For example, a private field in a class can be accessed using the access syntax on an instance of the class in an outer scope.

The `private` keyword is used to make declarations private, and the `public` keyword is used to make declarations public.

The `(set)` suffix can be used to make variables also publicly writeable.

To summarize the behavior for variable and constant declarations and fields:

| Declaration kind | Access modifier    | Read scope        | Write scope       |
|:-----------------|:-------------------|:------------------|:------------------|
| `const`          |                    | Current and inner | *None*            |
| `const`          | `public`           | **All**           | *None*            |
| `var`            |                    | Current and inner | Current and inner |
| `var`            | `public`           | **All**           | Current and inner |
| `var`            | `public(set)`      | **All**           | **All**           |


```typescript
// private constant, inaccessible/invisible
//
private const a = 1

// public constant, accessible/visible
//
public const b = 2


public class SomeClass {

    // private constant field,
    // only readable in class functions
    //
    private const a: Int

    // public constant field,
    // readable in all scopes
    //
    public const b: Int

    // private variable field,
    // only readable and writeable in class functions
    //
    private var c: Int

    // public variable field, not settable,
    // only writeable in class functions,
    // readable in all scopes
    //
    public var d: Int

    // public variable field, settable,
    // readable and writeable in all scopes
    //
    public(set) var e: Int

    // initializer implementation skipped
}
```

## Interfaces

> Status: Interfaces are not implemented yet.

An interface is an abstract type that specifies the behavior of types that *implement* the interface. Interfaces declare the required functions and fields, as well as the access for those declarations, that implementations need to provide.
<!-- TODO also contracts, once documented -->
Interfaces can be implemented by classes and structures. Types may implement multiple interfaces.

Interfaces consist of the functions and fields, as well as their access, that an implementation must provide. Function requirements consist of the name of the function, parameter types, an optional return type, and optional preconditions and postconditions. Field requirements consist of the name and the type of the field.

### Inferface Declaration

Interfaces are declared using the `interface` keyword, followed by the name of the interface, and the requirements enclosed in opening and closing braces. Fields can be annotated to be variable or constant, and how they can be accessed, but do not have to be annotated.

The special type `This` can be used to refer to the type implementing the interface.

```typescript
// declare an interface for a vault: a container for a balance
//
interface Vault {

    // require the implementation to provide a field for the balance
    // that is readable in outer scopes.
    //
    // NOTE: no requirement is made for the kind of field,
    // it can be either variable or constant in the implementation
    //
    pub balance: Int

    // require the implementation to provide an initializer that
    // given the initial balance, must initialize the balance field
    //
    init(initialBalance: Int) {
        ensure {
            this.balance == initialBalance
        }

        // NOTE: no code
    }

    // require the implementation to provide a function that adds an amount to the balance.
    // the given amount must be positive
    //
    fun add(amount: Int) {
        require {
            amount > 0
        }
        ensure {
            this.balance == before(this.balance) + amount
        }

        // NOTE: no code
    }

    // require the implementation to provide a function that transfers an amount
    // from this vaults balance to another vault's balance. The receiving balance
    // must be of the same type – a transfer to a vault of another type is not possible
    // as the balance of it would only be readable
    //
    fun transfer(receivingVault: This, amount: Int) {
        require {
            amount > 0
            amount <= this.balance
        }
        ensure {
            this.balance == before(this.balance) - amount
            receivingVault.balance == before(receivingVault.balance) + amount
        }

        // NOTE: no code
    }
}
```

Note that the required initializer and function do not have any executable code.

### Interface Implementation

Implementations are declared using the `impl` keyword, followed by the name of interface, the `for` keyword, and the name of the class or structure that should provide the functionality.

```typescript
// declare a class called ExampleVault with a variable balance field,
// that can be written by functions of the class, but only read in outer scopes
//
class ExampleVault {

    // implement the required variable field balance for the Vault interface,
    // that the ExampleVault type can write to, and that is only readable
    // in outer scopes
    //
    pub var balance: Int

    // implement the required initializer for the Vault interface:
    // accept an initial balance and initialize the balance field.
    // this implementation satisfies the required postcondition
    //
    // NOTE: the postcondition does not have to be repeated
    //
    init(initialBalance: Int) {
        this.balance = initialBalance
    }
}


// declare the implementation of the Vault interface for the ExampleVault class
//
impl Vault for ExampleVault {

    // implement the required function add for the Vault interface,
    // that adds an amount to the vault's balance.
    //
    // this implementation satisfies the required postcondition
    //
    // NOTE: neither the precondition, nor the postcondition have to be repeated
    //
    fun add(amount: Int) {
        this.balance = this.balance + amount
    }

    // implement the required function transfer for the Vault interface,
    // that subtracts the amount from the this vault's balance,
    // and adds the amount to the receiving vault's balance.
    //
    // NOTE: the type of the receiving vault parameter is ExampleVault,
    // i.e., an amount can only be transfered to a vault of the same type
    //
    // this implementation satisfies the required postconditions
    //
    // NOTE: neither the precondition, nor the postcondition have to be repeated
    //
    fun transfer(receivingVault: ExampleVault, amount: Int) {
        this.balance -= amount
        receivingVault.amount += amount
    }
}

// declare and create two example vaults
//
const vault = ExampleVault(100)
const otherVault = ExampleVault(0)

// transfer 10 units from the first vault to the second vault.
// the amount satisifes the precondition of the Vault interface's transfer function
//
vault.transfer(otherVault, 10)

// the postcondition of the Vault interface's transfer function ensured
// the balances of the vaults were updated properly
//
// vault.balance is 90
// otherVault.balance is 10

// error: precondition not satisfied: amount is larger than balance (100 > 90)
//
vault.transfer(otherVault, 100)
```

### Interface Type

Interfaces are types. Values implementing an interface can be used as initial values for constants that have the interface as their type.

```typescript
// declare a constant that has type Vault, which has a value of type ExampleVault
//
const vault: Vault = ExampleVault(100)
```

Values implementing an interface are assignable to variables that have the interface as their type.

```typescript
// assume there is a declaration for another implementation
// of the Vault interface called CoolVault

// declare a variable that has type Vault, which has an initial value of type CoolVault
//
var someVault: Vault = CoolVault(100)

// assign a different type of vault
//
someVault = ExampleVault(50)


// invalid: type mismatch
//
const exampleVault: ExampleVault = CoolVault(100)
```

Fields declared in an interface can be accessed and functions declared in an interface can be called on values of a type implementing the interface.

```typescript
const someVault: Vault = ExampleVault(100)

// access the balance field of the Vault interface type
//
someVault.balance // is 100

// call the add function of Vault interface type
someVault.add(50)

// someVault.balance is 150
```

## Authorizations

> Status: Authorizations are not implemented yet.

Authorizations represent access rights/privileges to resources. An authorization is similar to a class in that it is a composite reference type, i.e., it consists of values and is referenced, that is has an initializer, and that it can have functions associated with it.

Authorizations differ from classes in that they can only be created (instantiated) from existing authorizations. To make this explicit, the initializer of an authorization *must* be declared, the initializer must have at least one parameter, and the type of the first parameter must be an authorization.

Furthermore, authorizations are unforgeable.

There is a global authorization `rootAuth` of type `RootAuth`. It represents the access rights/privileges to all resources.

Authorizations are declared using the `auth` keyword.

```typescript
auth SendTokens {
    const limit: Int

    init(auth: RootAuth, limit: Int) {
        this.limit = limit
    }
}
```

<!--

TODO:
- access control

-->


