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

```typescript,file=single-line-comment.bpl
// This is a comment on a single line.
// Another comment line that is not executed.
```

*Multi-line comments* start with a slash and an asterisk (`/*`) and end with an asterisk and a slash (`*/`):

```typescript,file=multi-line-comment.bpl
/* This is a comment which
spans multiple lines. */
```

Comments may be nested.

```text,file=nested-comment.bpl
/* /* this */ is a valid comment */
```

## Constants and Variable Declarations

Constants and variables are declarations that bind a value to a name. Constants can only be initialized with a value and cannot be reassigned afterwards. Variables can be initialized with a value and can be reassigned later. Declarations are valid in any scope, including the global scope.

Constant means that the *name* is constant, not the *value*, i.e., the value may still be changed if it allows it.

The `const` keyword is used to declare a constant and the `var` keyword is used to declare a variable.
The keywords are followed by the name, an optional [type annotation](#Type Annotations), an equals sign `=`, and the initial value.

```typescript,file=constants-and-variables.bpl
// Declare a constant named `a`
//
const a = 1

// Invalid: re-assigning to a constant
//
a = 2

// Declare a variable named `b`
//
var b = 3

// Assign a new value to the variable named `b`
//
b = 4
```

Variables and constants **must** be initialized.

```typescript
// Invalid: the constant has no initial value
//
const a
```

Once a constant or variable is declared, it can't be redeclared with the same name, with a different type, or changed into the corresponding other kind (variable to a constant and vice versa).


```typescript
// Declare a constant named `a`
//
const a = 1

// Invalid: can't re-declare a constant with name `a`,
// as it is already used in this scope
//
const a = 2

// Declare a variable named `b`
//
var b = 3
```

```typescript
// Invalid: can't re-declare a variable with name `b`,
// as it is already used in this scope
//
var b = 4

// Invalid: can't declare a variable with the name `a`,
// as it is already used in this scope,
// and it is declared as a constant
//
var a = 5
```

## Type Annotations

When declaring a constant or variable, an optional *type annotation* can be provided, to make it explicit what type the declaration has.

If no type annotation is provided, the type of the declaration is [inferred from the initial value](#type-inference).

```typescript,file=type-annotations.bpl
// Declare a variable named `initialized` which has an explicit type annotation.
//
// `Bool` is the type of booleans
//
var initialized: Bool = false

// Declare a constant named `inferred`, which has no type annotation
// and for which the type is inferred to be `Int`,
// based on the initial value
//
const inferred = 1
```

If a type annotation is provided, the initial value must be of this type, and new values assigned to variables must match the declaration's type. This type safety is explained in more detail in a [separate section](#type-safety).

```typescript
// Invalid: declare a variable with an explicit type `Bool`,
// but the initial value has type `Int`.
//
// `Int` is the type of arbitrary-precision integers
//
const booleanConstant: Bool = 1

// Declare a variable that has type `Bool`
//
var booleanVariable = false

// Invalid: assign a value with type `Int` to a variable which has type `Bool`
//
booleanVariable = 1
```

## Naming

Names may start with any upper and lowercase letter or an underscore. This may be followed by zero or more upper and lower case letters, underscores, and numbers.
Names may not begin with a number.

```typescript
// Valid, title-case
//
PersonID

// Valid, with underscore
//
token_name

// Valid, leading underscore
//
_balance

// Valid, with number
//
account2

// Invalid, leading number
//
1something

// Invalid, various invalid characters
//
!@#$%^&*
```

### Conventions

By convention, variables, constants, and functions have lowercase names; and types have title-case names.


## Semicolons

Semicolons may be used to separate statements, but are optional. They can be used to separate multiple statements on a single line.

```typescript,file=semicolons.bpl
// Declare a constant, without a semicolon
//
const a = 1

// Declare a variable, with a semicolon
//
var b = 2;

// Declare a constant and a variable on a single line, separated by semicolons
//
const d = 1; var e = 2
```

## Values and Types

Values are objects, like for example booleans, integers, or arrays. Values are typed.

### Booleans

The two boolean values `true` and `false` have the type `Bool`.

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

```typescript,file=numbers.bpl
// A decimal number
//
1234567890

// A binary number
//
0b101010

// An octal number
//
0o12345670

// A hexadecimal number
//
0x1234567890ABCDEFabcdef

// Invalid: unsupported prefix 0z
//
0z0
```

Decimal numbers may contain underscores (`_`) to logically separate components.

```typescript,file=number-underscores-decimal.bpl
const largeNumber = 1_000_000
```

Underscores are allowed for all numeral systems.

```typescript,file=number-underscores-binary.bpl
const binaryNumber = 0b10_11_01
```

### Arrays

Arrays are mutable, ordered collections of values. All values in an array must have the same type. Arrays may contain a value multiple times. Array literals start with an opening square bracket `[` and end with a closing square bracket `]`.

```typescript,file=arrays.bpl
// An empty array
//
[]

// An array with integers
//
[1, 2, 3]

// Invalid: mixed types
//
[1, true, 2, false]
```

#### Array Indexing

To get the element of an array at a specific index, the indexing syntax can be used: The array is followed by an opening square bracket `[`, the indexing value, and ends with a closing square bracket `]`.

```typescript,file=arrays-indexing.bpl
const numbers = [42, 23]

// Get the first number
//
numbers[0] // is 42

// Get the second number
//
numbers[1] // is 23
```

```typescript,file=arrays-nested-indexing.bpl
const arrays = [[1, 2], [3, 4]]

// Get the first number of the second array
//
arrays[1][0] // is 3
```

To set an element of an array at a specific index, the indexing syntax can be used as well.

```typescript,file=array-indexing-assignment.bpl
const numbers = [42, 23]

// Change the second number
//
numbers[1] = 2

// `numbers` is [42, 2]
```

#### Array Types

Arrays either have a fixed size or are variably sized, i.e., elements can be added and removed.

Fixed-size arrays have the type suffix `[N]`, where `N` is the size of the array. For example, a fixed-size array of 3 `Int8` elements has the type `Int8[3]`.

Variable-size arrays have the type suffix `[]`. For example, the type `Int16[]` specifies a variable-size array of elements that have type `Int16`.

```typescript,file=array-types.bpl
const array: Int8[2] = [1, 2]

const arrays: Int16[2][3] = [
    [1, 2, 3],
    [4, 5, 6]
]
```

<!--

TODO

#### Array Functions

- Length, concatenate, filter, etc. for all array types
- Append, remove, etc. for variable-size arrays
- Document and link to array concatenation operator `+` in operators section

-->


### Dictionaries

> 🚧 Status: Dictionaries are not implemented yet.

Dictionaries are mutable, unordered collections of key-value associations. In a dictionary, all keys must have the same type, and all values must have the same type. Dictionaries may contain a key only once and may contain a value multiple times.

Dictionary literals start with an opening brace `{` and end with a closing brace `}`. Keys are separated from values by a colon, and key-value associations are separated by commas.

```typescript,file=dictionaries.bpl
// An empty dictionary
//
{}

// A dictionary which associates integers with booleans
//
{
    1: true,
    2: false
}

// Invalid: mixed types
//
{
    1: true,
    false: 2
}
```

#### Dictionary Access

To get the value for a specific key from a dictionary, the access syntax can be used: The dictionary is followed by an opening square bracket `[`, the key, and ends with a closing square bracket `]`.

```typescript,file=dictionary-access.bpl
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

```typescript,file=dictionary-assignment.bpl
const booleans = {
    1: true,
    0: false
}
booleans[1] = false
booleans[0] = true
// `booleans` is {1: false, 0: true}
```


#### Dictionary Types

Dictionaries have the type suffix `[T]`, where `T` is the type of the key. For example, a dictionary with `Int` keys and `Bool` values has type `Bool[Int]`.

```typescript,file=dictionary-types.bpl
const booleans = {
    1: true,
    0: false
}
// `booleans` has type `Bool[Int]`

const integers = {
    true: 1,
    false: 0
}
// `integers` has type `Int[Bool]`
```

<!--

TODO

#### Dictionary Functions

-->


#### Dictionary Keys

Dictionary keys must be hashable and equatable, i.e., must implement the [`Hashable`](#hashable-interface) and [`Equatable`](#equatable-interface) [interfaces](#interfaces).

Most of the built-in types, like booleans, integers, are hashable and equatable, so can be used as keys in dictionaries.

## Operators

Operators are special symbols that perform a computation for one or more values. They are either unary, binary, or ternary.

- Unary operators perform an operation for a single value. The unary operator symbol appears before the value.

- Binary operators operate on two values. The binary operator symbol appears between the two values (infix).

- Ternary operators operate on three values. The operator symbols appear between the three values (infix).


### Negation

The `-` unary operator negates an integer:

```typescript,file=integer-negation.bpl
const a = 1
-a // is -1
```

The `!` unary operator logically negates a boolean:

```typescript,file=boolean-negation.bpl
const a = true
!a // is false
```

### Assignment

The binary assignment operator `=` can be used to assign a new value to a variable. It is only allowed in a statement and is not allowed in expressions.

```typescript,file=assignment.bpl
var a = 1
a = 2
// a is 2
```

The left-hand side of the assignment must be an identifier, followed by one or more index or access expressions.

```typescript,file=assignment-numbers.bpl
const numbers = [1, 2]

// Change the first number
//
numbers[0] = 3

// `numbers` is [3, 2]
```

```typescript,file=assignment-arrays.bpl
const arrays = [[1, 2], [3, 4]]

// Change the first number in the second array
//
arrays[1][0] = 5

// `arrays` is [[1, 2], [5, 4]]
```

```typescript,file=assignment-dictionaries.bpl
const dictionaries = {
  true: {1: 2},
  false: {3: 4}
}

dictionaries[false][3] = 0

// `dictionaries` is {
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

```typescript,file=operator-plus.bpl
const a = 1 + 2
// `a` is 3
```

Arithmetic operators don't cause values to overflow.

```typescript,file=operator-times.bpl
const a: Int8 = 100
const b: Int8 = 100
const c = a * b
// `c` is 10000, and has type `Int`
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

```typescript,file=operator-overflow-plus.bpl
const a: UInt8 = 255
a &+ 1 // is 0
```

Similarly, for the minimum value 0, subtracting 1 wraps around and results in the maximum value 255.

```typescript,file=operator-minus.bpl
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

  ```typescript,file=operator-and.bpl
  true && true // is true
  true && false // is false
  false && false // is false
  false && false // is false
  ```

- Logical OR: `a || b`

  ```typescript,file=operator-or.bpl
  true || true // is true
  true || false // is true
  false || false // is true
  false || false // is false
  ```

### Comparison operators

Comparison operators work with boolean and integer values.


- Equality: `==`, for booleans and integers

  ```typescript,file=operator-equal.bpl
  1 == 1 // is true
  1 == 2 // is false
  true == true // is true
  true == false // is false
  ```

- Inequality: `!=`, for booleans and integers

  ```typescript,file=operator-not-equal.bpl
  1 != 1 // is false
  1 != 2 // is true
  true != true // is false
  true != false // is true
  ```

- Less than: `<`, for integers

  ```typescript,file=operator-less.bpl
  1 < 1 // is false
  1 < 2 // is true
  2 < 1 // is false
  ```

- Less or equal than: `<=`, for integers

  ```typescript,file=operator-less-equals.bpl
  1 <= 1 // is true
  1 <= 2 // is true
  2 <= 1 // is false
  ```

- Greater than: `>`, for integers

  ```typescript,file=operator-greater.bpl
  1 > 1 // is false
  1 > 2 // is false
  2 > 1 // is true
  ```


- Greater or equal than: `>=`, for integers

  ```typescript,file=operator-greater-equals.bpl
  1 >= 1 // is true
  1 >= 2 // is false
  2 >= 1 // is true
  ```


### Ternary Conditional Operator

There is only one ternary conditional operator, the ternary conditional operator (`a ? b : c`).

It behaves like an if-statement, but is an expression: If the first operator value is true, the second operator value is returned. If the first operator value is false, the third value is returned.

```typescript,file=operator-ternary-conditional.bpl
const x = 1 > 2 ? 3 : 4
// `x` is 4
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

Expressions can be wrapped in parentheses to override precedence conventions, i.e. an alternate order should be indicated, or when the default order should be emphasized, e.g. to avoid confusion. For example, `(2 + 3) * 4` forces addition to precede multiplication, and `5 + (6 * 7)` reinforces the default order.


## Functions

Functions are sequences of statements that perform a specific task. Functions have parameters (inputs) and an optional return value (output). Functions are typed: the function type consists of the parameter types and the return type.

Functions are values, i.e., they can be assigned to constants and variables, and can be passed as arguments to other functions. This behavior is often called "first-class functions".

### Function Declarations

Functions can be declared by using the `fun` keyword, followed by the name of the declaration, the parameters, the optional return type, and the code that should be executed when the function is called.

The parameters need to be enclosed in parentheses. The return type, if any, is separated from the parameters using the `->` keyword (a hyphen followed by a right angle bracket). The function code needs to be enclosed in opening and closing braces.

Each parameter can have a label, the name that a function call needs to use to provide an argument value for the parameter. Argument labels precede the parameter name. The special argument label `_` indicates that a function call can omit the argument label. If no argument label is provided, the function call must use the parameter name.

Each parameter needs to have a type annotation, which follows the parameter name after a colon.

```typescript,file=function-double.bpl
// Declare a function named `double`, which multiples a number by two.
//
// The special argument label _ is specified for the parameter,
// so no argument label has to be provided in a function call
//
fun double(_ x: Int) -> Int {
    return x * 2
}

// Call the function named `double` with the value 4 for the first parameter.
//
// The argument label can be omitted in the function call as the declaration
// specifies the special argument label _ for the parameter
//
double(2) // returns 4
```

It is possible to require argument labels for some parameters, and not require argument labels for other parameters.

```typescript,file=function-clamp.bpl
// Declare a function named `clamp`. The function takes an integer value,
// the lower limit, and the upper limit. It returns an integer between
// the lower and upper limit.
//
// For the first parameter the special argument label _ is used,
// so no argument label has to be given for it in a function call.
//
// For the second and third parameter no argument label is given,
// so the parameter names are the argument labels, i.e., the parameter names
// have to be given as argument labels in a function call.
//
fun clamp(_ value: Int, min: Int, max: Int) -> Int {
    if value > max {
        return max
    }

    if value < min {
        return min
    }

    return value
}

// Declare a constant which has the result of a call to the function
// named `clamp` as its initial value.
//
// For the first argument no label is given, as it is not required by
// the function declaration (the special argument label `_` is specified).
//
// For the second and this argument the labels must be provided,
// as the function declaration does not specify the special argument label `_`
// for these two parameters.
//
// As the function declaration also does not specify argument labels
// for these parameters, the parameter names must be used as argument labels.
//
const clamped = clamp(123, min: 0, max: 100)
// clamped is 100
```

Argument labels make code more explicit and readable. For example, they avoid confusion about the order of arguments when there are multiple arguments that have the same type.

Argument labels should be named so they make sense from the perspective of the function call.

```typescript,file=function-send.bpl
// Declare a function named `send`, which transfers an amount
// from one account to another.
//
// The implementation is omitted for brevity.
//
// The first two parameters of the function have the same type, so there is
// a potential that a function call accidentally provides arguments in
// the wrong order.
//
// While the parameter names `sendingAccount` and `receivingAccount`
// are descriptive inside the function, they might be too verbose
// to require them as argument labels in function calls.
//
// For this reason the shorter argument labels `from` and `to` are specified,
// which still convey the meaning of the two parameters without being overly
// verbose.
//
// The name of the third parameter, `amount`, is both meaningful inside
// the function and also in a function call, so no argument label is given,
// and the parameter name is required as the argument label in a function call.
//
fun send(from sendingAccount: Account, to receivingAccount: Account, amount: Int) {
    // ...
}

// Declare a constant which refers to the sending account.
//
// The initial value is omitted for brevity
//
const sender: Account = // ...

// Declare a constant which refers to the receiving account.
//
// The initial value is omitted for brevity
//
const receiver: Account = // ...

// Call the function named `send`.
//
// The function declaration requires argument labels for all parameters,
// so they need to be provided in the function call.
//
// This avoids ambiguity. For example, in some languages (like C) it is
// a convention to order the parameters so that the receiver occurs first,
// followed by the sender. In other languages, it is common to have
// the sender be the first parameter, followed by the receiver.
//
// Here, the order is clear – send an amount from an account to another account.
//
send(from: sender, to: receiver, amount: 100)
```

The order of the arguments in a function call must match the order of the parameters in the function declaration.

```typescript,file=function-test.bpl
// Declare a function named `test`, which accepts two parameters, named `first` and `second`
//
fun test(first: Int, second: Int) {
    // ...
}

// Invalid: the arguments are provided in the wrong order,
// even though the argument labels are provided correctly
//
test(second: 1, first: 2)
```

Functions can be nested, i.e., the code of a function may declare further functions.

```typescript,file=function-doubleAndAddOne.bpl
// Declare a function which multiplies a number by two, and adds one
//
fun doubleAndAddOne(_ x: Int) -> Int {

    // Declare a nested function which multiplies a number by two
    //
    fun double(_ x: Int) {
        return x * 2
    }

    return double(x) + 1
}

doubleAndAddOne(2) // is 5
```

### Function Expressions

Functions can be also used as expressions. The syntax is the same as for function declarations, except that function expressions have no name, i.e., it is anonymous.

```typescript,file=function-expression.bpl
// Declare a constant named `double`, which has a function as its value.
//
// The function multiplies a number by two when it is called
//
const double =
    fun (_ x: Int) -> Int {
        return x * 2
    }
```

### Function Calls

Functions can be called (invoked). Function calls need to provide exactly as many argument values as the function has parameters.

```typescript,file=function-call.bpl
fun double(_ x: Int) -> Int {
     return x * 2
}

// Valid: the correct amount of arguments is provided
//
double(2) // is 4

// Invalid: too many arguments are provided
//
double(2, 3)

// Invalid: too few arguments are provided
//
double()
```

### Function Types

Function types consist of the function's parameter types and the function's return type.
The parameter types need to be enclosed in parentheses, followed by the `->` keyword, and end with the return type.

```typescript,file=function-type.bpl
// Declare a function named `add`, with the function type `(Int, Int) -> Int`
//
fun add(a: Int, b: Int) -> Int {
    return a + b
}
```

```typescript,file=function-type-expression.bpl
// Declare a constant named `add`, with the function type `(Int, Int) -> Int`
//
const add: (Int, Int) -> Int =
    fun (a: Int, b: Int) -> Int {
        return a + b
    }
```

If the function has no return type, it implicitly has the return type `Void`.

```typescript,file=function-doNothing.bpl
// Declare a constant named `doNothing`, which is a function
// that takes no parameters and returns nothing
//
const doNothing: () -> Void =
    fun () {}
```

Types can be enclosed in parentheses to change precedence.

For example, a function type `(Int) -> (() -> Int)` is the type for a function which accepts one argument with type `Int`, and which returns another function, that takes no arguments and returns an `Int`.

The type `((Int) -> Int)[2]` specifies an array type of two functions, which accept one integer and return one integer.

#### Argument Passing Behavior

When arguments are passed to a function, they are not copied. Instead, parameters act as new variable bindings and the values they refer to are identical to the passed values. Modifications to mutable values made within a function will be visible to the caller. This behavior is known as [call-by-sharing](https://en.wikipedia.org/w/index.php?title=Evaluation_strategy&oldid=896280571#Call_by_sharing).

```typescript,file=function-change.bpl
fun change(_ numbers: Int[]) {
     numbers[0] = 1
     numbers[1] = 2
}

const numbers = [0, 1]

change(numbers)
// numbers is [1, 2]
```

Parameters are constant, i.e., it is not allowed to assign to them.

```typescript,file=function-parameter-assignment.bpl
fun test(x: Int) {
     // Invalid: cannot assign to a parameter (constant)
     //
     x = 2
}
```

### Function Preconditions and Postconditions

> 🚧 Status: Function Preconditions and Postconditions are not implemented yet.

Functions may have preconditions and may have postconditions. Preconditions and postconditions can be used to restrict the inputs (values for parameters) and output (return value) of a function.

Preconditions must be true right before the execution of the function. Preconditions are part of the function and introduced by the `require` keyword, followed by the condition block.

Postconditions must be true right after the execution of the function. Postconditions are part of the function and introduced by the `ensure` keyword, followed by the condition block. Postconditions may only occur after preconditions, if any.

A conditions block consists of one or more conditions. Conditions are expressions evaluating to a boolean. They may not call functions, i.e., they can't have side-effects and must be pure expressions.

<!--

TODO:

For now, function calls are not allowed in preconditions and postconditions.
See https://github.com/dapperlabs/bamboo-runtime/issues/84

-->

Conditions may be written on separate lines, or multiple conditions can be written on the same line, separated by a semicolon. This syntax follows the syntax for [statements](#semicolons).

Following each condition, an optional description can be provided after a colon.

In postconditions, the special constant `result` refers to the result of the function.

```typescript,file=function-factorial.bpl
fun factorial(_ n: Int) -> Int {
    require {
        // Require the parameter `n` to be greater than or equal to zero
        //
        n >= 0:
            "factorial is only defined for integers greater than or equal to zero"
    }
    ensure {
        // Ensure the result will be greater than or equal to 1
        //
        result >= 1:
            "the result must be greater than or equal to 1"
    }

    var i = n
    var result = 1

    while i > 1 {
        result = result * i
        i = i - 1
    }

    return result
}

factorial(5) // returns 120

// Error: the given argument does not satisfy the precondition `n >= 0` of the function
//
factorial(-2)
```

In postconditions, the special function `before` can be used to get the value of an expression just before the function is called.

```typescript,file=function-increment.bpl
var n = 0

fun incrementN() {
    ensure {
        // Require the new value of `n` to be the old value of `n`, plus one
        //
        n == before(n) + 1:
            "n must be incremented by 1"
    }

    n = n + 1
}
```

## Control flow

Control flow statements control the flow of execution in a function.

### Conditional branching: if-statement

If-statements allow a certain piece of code to be executed only when a given condition is true.

The if-statement starts with the `if` keyword, followed by the condition, and the code that should be executed if the condition is true inside opening and closing braces. The condition must be boolean and the braces are required.


```typescript,file=control-flow-if.bpl
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

```typescript,file=control-flow-if-else.bpl
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

```typescript,file=control-flow-if-else-if.bpl
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

```typescript,file=control-flow-while.bpl
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

```typescript,file=scope.bpl
const x = 10

fun f() -> Int {
    const y = 10
    return x + y
}

f() // returns 20

// Invalid: the identifier `y` is not in scope
//
y
```

```typescript,file=scope-doubleAndAddOne.bpl
fun doubleAndAddOne(_ n: Int) -> Int {
    fun double(_ x: Int) {
        return x * 2
    }
    return double(n) + 1
}

// Invalid: the identifier `double` is not in scope
//
double(1)
```

Each scope can introduce new declarations, i.e., the outer declaration is shadowed.

```typescript,file=scope-test.bpl
const x = 2

fun test() -> Int {
    const x = 3
    return x
}

test() // returns 3
```

Scope is lexical, not dynamic.

```typescript,file=scope-lexical.bpl
const x = 10

fun f() -> Int {
   return x
}

fun g() -> Int {
   const x = 20
   return f()
}

g() // returns 10, not 20
```

Declarations are **not** moved to the top of the enclosing function (hoisted).

```typescript,file=scope-no-hoisting.bpl
const x = 2

fun f() -> Int {
    if x == 0 {
        const x = 3
        return x
    }
    return x
}
f() // returns 2
```

## Optionals

> Status: Optionals are not implemented yet.

Optionals are values which can represent the absence of a value. Optionals have two cases: either there is a value, or there is nothing.

An optional type is declared using the `?` suffix for another type. For example, `Int` is a non-optional integer, and `Int?` is an optional integer, i.e. either nothing, or an integer.

The value representing nothing is `nil`.

```typescript
// declare a constant which has an optional integer type,
// with nil as its initial value
//
const a: Int? = nil

// declare a constant which has an optional integer type,
// with 42 as its initial value
//
const b: Int? = 42
```

### Nil-Coalescing Operator

The nil-coalescing operator `??` returns the value inside an optional if it contains a value, or returns an alternative value if the optional has no value, i.e., the optional value is `nil`. 

```typescript
// declare a constant which has an optional integer type
//
const a: Int? = nil

// declare a constant with a non-optional integer type,
// which is initialized to b if it is non-nil, or 42 otherwise
//
const b: Int = a ?? 42
// integer is 42, as a is nil
```

The nil-coalescing operator can only be applied to values which have an optional type.

```typescript
// declare a constant with a non-optional integer type
//
const a = 1

// invalid: nil-coalescing operator is applied to a value which has a non-optional type
// (a has the non-optional type Int)
//
const b = a ?? 2
```

```typescript
// invalid: nil-coalescing operator is applied to a value which has a non-optional type
// (the integer literal is of type Int)
//
const c = 1 ?? 2
```

The alternative value, i.e. the right-hand side of the operator, must be the non-optional type matching the type of the left-hand side.

```typescript
// declare a constant with a non-optional integer type
const a = 1

// invalid: nil-coalescing operator is applied to a value of type Int,
// but alternative is of type Bool
//
const b = a ?? false
```


## Type Safety

> 🚧 Status: Type checking is not implemented yet.

The Bamboo programming language is a _type-safe_ language.

When assigning a new value to a variable, the value must be the same type as the variable. For example, if a variable has type `Bool`, it can _only_ be assigned a value that has type `Bool`, and not for example a value that has type `Int`.

```typescript,file=type-safety-assign-int-to-bool.bpl
// Declare a variable that has type `Bool`
var a = true

// Invalid: cannot assign a value that has type `Int` to a variable which has type `Bool`
//
a = 0
```

When passing arguments to a function, the types of the values must match the function parameters' types. For example, if a function expects an argument that has type `Bool`, _only_ a value that has type `Bool` can be provided, and not for example a value which has type `Int`.

```typescript,file=type-safety-nand.bpl
fun nand(_ a: Bool, _ b: Bool) -> Bool {
    return !(a && b)
}

nand(false, false) // returns true

// Invalid: integers are not booleans
//
nand(0, 0)
```

Types are **not** automatically converted. For example, an integer is not automatically converted to a boolean, nor is an `Int32` automatically converted to an `Int8`, nor is an optional integer `Int?`  automatically converted to a non-optional integer `Int`.


```typescript,file=type-safety-add.bpl
fun add(_ a: Int8, _ b: Int8) -> Int {
    return a + b
}

add(1, 2) // returns 3

// Declare two constants which have type `Int32`
//
const a: Int32 = 3_000_000_000
const b: Int32 = 3_000_000_000

// Invalid: cannot pass arguments which have type `Int32` to parameters which have type `Int8`
//
add(a, b)
```


## Type Inference

> 🚧 Status: Type inference is not implemented yet.

If a variable or constant is not annotated explicitly with a type, it is inferred from the value.

Integer literals are inferred to type `Int`.

```typescript,file=type-inference-int.bpl
const a = 1

// `a` has type `Int`
```

Array literals are inferred based on the elements of the literal, and to be variable-size.

```typescript,file=type-inference-intergers.bpl
const integers = [1, 2]
// `integers` has type `Int[]`

// Invalid: mixed types
//
const invalidMixed = [1, true, 2, false]
```

Dictionary literals are inferred based on the keys and values of the literal.

```typescript,file=type-inference-dictionary.bpl
const booleans = {
    1: true,
    2: false
}
// `booleans` has type `Bool[Int]`

// Invalid: mixed types
//
const invalidMixed = {
    1: true,
    false: 2
}
```

Functions are inferred based on the parameter types and the return type.

```typescript,file=type-inference-function.bpl
const add = (a: Int8, b: Int8) -> Int {
    return a + b
}

// `add` has type `(Int8, Int8) -> Int`
```

## Structures and Classes

> 🚧 Status: Structures and classes are not implemented yet.

Structures and classes are composite types. Structures and classes consist of one or more values, which are stored in named fields. Each field may have a different type.

Structures are declared using the `struct` keyword. Classes are declared using the `class` keyword. The keyword is followed by the name.

```typescript,file=struct-and-class.bpl
struct SomeStruct {
    // ...
}

class SomeClass {
    // ...
}
```

Structures and classes are types.

Values of a structure or class type are created (instantiated) by calling the type like a function.

```typescript,file=struct-and-class-instantiation.bpl
const someStruct: SomeStruct = SomeStruct()

const someClass: SomeClass = SomeClass()
```

Structures and classes mainly differ in their behaviour: Structures are *copied*, i.e. they are value types, whereas classes are *referenced*, i.e., they are reference types. This is explained in detail in a [separate section](#structure-and-class-behaviour).


### Structure and Class Fields

Fields are declared like variables and constants, however, they have no initial value. The initial values for fields are set in the initializer. All fields **must** be initialized in the initializer. The initializer is declared using the `init` keyword. Just like a function, it takes parameters. However, it has no return type, i.e., it is always `Void`. The initializer always follows any fields.

There are three kinds of fields.

Variable fields are stored in the structure or class value, can have new values assigned to them. They are declared using the `var` keyword.

Constant fields are also stored in the structure or class value, but they can **not** have new values assigned to them. They are declared using the `const` keyword.

Synthetic fields are **not** stored in the structure or class value, i.e. they are derived/computed from other values. They can have new values assigned to them and are declared using the `synthetic` keyword. Synthetic fields must have a getter and a setter. Getters and setters are explained in the [next section](#structure-and-class-field-getters-and-setters). Synthetic fields are explained in a [separate section](#synthetic-structure-and-class-fields).

| Field Kind           | Stored in memory | Assignable         | Keyword     |
|----------------------|------------------|--------------------|-------------|
| **Variable field**   | Yes              | Yes                | `var`       |
| **Constant field**   | Yes              | **No**             | `const`     |
| **Synthetic field**  | **No**           | Yes                | `synthetic` |

```typescript,file=struct-and-class-fields-and-init.bpl
// Declare a structure named `Token`, which has a constant field
// named `id` and a variable field named `balance`.
//
// Both fields are initialized through the initializer
//
struct Token {
    const id: Int
    var balance: Int

    init(id: Int, balance: Int) {
        self.id = id
        self.balance = balance
    }
}
```

In initializers, the special constant `self` refers to the structure or class value that is to be initialized.

Fields can be read (if they are constant or variable) and set (if they are variable), using the access syntax: the structure or class instance is followed by a dot (`.`) and the name of the field.

```typescript,file=struct-and-class-fields-assignment.bpl
const token = Token(id: 42, balance: 1_000_00)

token.id // is 42
token.balance // is 1_000_000

token.balance = 1
// token.balance is 1

// Invalid: assignment to constant field
//
token.id = 23
```

### Structure and Class Field Getters and Setters

Fields may have an optional getter and an optional setter. Getters are functions that are called when a field is read, and setters are functions that are called when a field is written.

Getters and setters are enclosed in opening and closing braces, after the field's type.

Getters are declared using the `get` keyword. Getters have no parameters and their return type is implicitly the type of the field.

```typescript,file=struct-and-class-field-getter.bpl
struct GetterExample {

    // Declare a variable field named `balance` with a getter
    // which ensures the read value is always positive
    //
    var balance: Int {
        get {
           ensure {
               result >= 0
           }

           if self.balance < 0 {
               return 0
           }

           return self.balance
        }
    }

    init(balance: Int) {
        self.balance = balance
    }
}

const example = GetterExample(balance: 10)
// example.balance is 10

example.balance = -50
// example.balance is 0. without the getter it would be -50
```

Setters are declared using the `set` keyword, followed by the name for the new value enclosed in parentheses. The parameter has implicitly the type of the field. Another type can not be specified. Setters have no return type.

The types of values assigned to setters must always match the field's type.

```typescript,file=struct-and-class-field-setter.bpl
struct SetterExample {

    // Declare a variable field named `balance` with a setter
    // which requires written values to be positive
    //
    var balance: Int {
       set(newBalance) {
           require {
               newBalance >= 0
           }
           self.balance = newBalance
       }
    }

    init(balance: Int) {
        self.balance = balance
    }
}

const example = SetterExample(balance: 10)
// example.balance is 10

// error: precondition of setter for field balance failed
example.balance = -50
```

### Synthetic Structure and Class Fields

Fields which are not stored in the structure or class value are *synthetic*, i.e., the field value is computed. Synthetic can be either read-only, or readable and writable.

Synthetic fields are declared using the `synthetic` keyword.

Synthetic fields are read-only when only a getter is provided.

```swift,file=struct-and-class-synthetic-field-getter-only.bpl
struct Rectangle {
    var width: Int
    var height: Int

    // Declare a synthetic field named `area`,
    // which computes the area based on the width and height
    //
    synthetic area: Int {
        get {
            return width * height
        }
    }
}
```

Synthetic fields are readable and writable when both a getter and a setter is declared.

```swift,file=struct-and-class-synthetic-field-setter-getter.bpl
// Declare a struct named `GoalTracker` which stores a number
// of target goals, a number of completed goals,
// and has a synthetic field to provide the left number of goals
//
// NOTE: the tracker only implements some functionality to demonstrate
// synthetic fields, it is incomplete (e.g. assignments to `goal` are not handled properly)
//
struct GoalTracker {

    var goal: Int
    var completed: Int

    // Declare a synthetic field which is both readable
    // and writable.
    //
    // When the field is read from (in the getter),
    // the number of left goals is computed from
    // the target number of goals and
    // the completed number of goals.
    //
    // When the field is written to (in the setter),
    // the number of completed goals is updated,
    // based on the number of target goals
    // and the new remaining number of goals
    //
    synthetic left: Double {
        get {
            return self.goal - self.completed
        }

        set(newLeft) {
            self.completed = self.goal - newLeft
        }
    }

    init(goal: Int, completed: Int) {
        self.goal = goal
        self.completed = completed
    }
}

const tracker = GoalTracker(goal: 10, completed: 0)
// tracker.goal is 10
// tracker.completed is 0
// tracker.left is 10

tracker.completed = 1
// tracker.left is 9

tracker.left = 8
// tracker.completed is 2
```

It is invalid to declare a synthetic field with only a setter.


### Structure and Class Functions

Structures and classes may contain functions. Just like in the initializer, the special constant `self` refers to the structure or class value that the function is called on.

```typescript,file=struct-and-class-function.bpl
struct Token {
    const id: Int
    var balance: Int

    init(id: Int, initialBalance balance: Int) {
        self.id = id
        self.balance = balance
    }

    fun mint(amount: Int) {
        self.balance = self.balance + amount
    }
}

const token = Token(id: 32, initialBalance: 0)
token.mint(amount: 1_000_000)
// token.balance is 1_000_000
```

### Structure and Class Behaviour

The only difference between structures and classes is their behavior when used as an initial value for another constant or variable, when assigned to a different variable, or passed as an argument to a function: Structures are *copied*, i.e. they are value types, whereas classes are *referenced*, i.e., they are reference types.

```typescript,file=struct-behavior.bpl
// Declare a structure named `SomeStruct`, with a variable integer field
//
struct SomeStruct {
    var value: Int

    init(value: Int) {
        self.value = value
    }

}

// Declare a constant with value of structure type `SomeStruct`
//
const structA = SomeStruct(value: 0)

// *Copy* the structure value into a new constant
//
const structB = structA

structB.value = 1

// structA.value is *0*
```

```typescript,file=class-behavior.bpl
// Declare a class named `SomeClass`, with a variable integer field
//
class SomeClass {
    var value: Int

    init(value: Int) {
        self.value = value
    }
}

// Declare a constant with value of class type `SomeClass`
//
const classA = SomeClass(value: 0)

// *Reference* the class value with a new constant
//
const classB = classA

classB.value = 1

// classA.value is *1*
```

Note the different values in the last line of each example.

There is **no** support for nulls, i.e., a constant or variable of a reference type must always be bound to an instance of the type. There is *no* `null`.


### Class Inheritance and Abstract Classes

There is **no** support for class inheritance. Class inheritance is a feature common in other programming languages, that allows including the fields and functions of a class (known as the superclass) in another class (known as the subclass).

Instead, follow "Composition over inheritance", the idea of composing functionality from multiple individual parts, rather than building an inheritance tree.

Furthermore, there is also **no** support for abstract classes. An abstract class is a feature common in other programming languages, that prevents creating values of a superclass and only allows the creation of values of a subclass.

In addition, abstract classes may declare functions, but omit the implementation of them and require   subclasses to implement them.

Instead, use [interfaces](#interfaces).

## Access control

> 🚧 Status: Access control is not implemented yet.

Access control allows making certain parts of the program accessible/visible and making other parts inaccessible/invisible. Top-level declarations (variables, constants, functions, structures, classes, interfaces) and fields (in structures, classes) are either private or public.

**Private** means the declaration is only accessible/visible in the current and inner scopes. For example, a private field in a class can only be accessed by functions of the class, not by code that uses an instance of the class in an outer scope.

**Public** means the declaration is accessible/visible in all scopes, the current and inner scopes like for private, and the outer scopes. For example, a private field in a class can be accessed using the access syntax on an instance of the class in an outer scope.

By default, everything is private. The `pub` keyword is used to make declarations public.

The `(set)` suffix can be used to make variables also publicly writable.

To summarize the behavior for variable declarations, constant declarations, and fields:

| Declaration kind | Access modifier    | Read scope        | Write scope       |
|:-----------------|:-------------------|:------------------|:------------------|
| `const`          |                    | Current and inner | *None*            |
| `const`          | `pub`              | **All**           | *None*            |
| `var`            |                    | Current and inner | Current and inner |
| `var`            | `pub`              | **All**           | Current and inner |
| `var`            | `pub(set)`         | **All**           | **All**           |

To summarize the behavior for functions, structures, classes, and interfaces:

| Declaration kind                         | Access modifier       | Access scope      |
|:-----------------------------------------|:----------------------|:------------------|
| `fun`, `struct`, `class`, `interface`    |                       | Current and inner |
| `fun`, `struct`, `class`, `interface`    | `pub`                 | **All**           |



```typescript,file=access-control.bpl
// Declare a private constant, inaccessible/invisible in outer scope
//
const a = 1

// Declare a public constant, accessible/visible in all scopes
//
pub const b = 2

// Declare a public class, accessible/visible in all scopes
//
pub class SomeClass {

    // Declare a private constant field,
    // only readable in the current and inner scopes
    //
    const a: Int

    // Declare a public constant field, readable in all scopes
    //
    pub const b: Int

    // Declare a private variable field,
    // only readable and writable in the current and inner scopes
    //
    var c: Int

    // Declare a public variable field, not settable,
    // only writable in the current and inner scopes,
    // readable in all scopes
    //
    pub var d: Int

    // Declare a public variable field, settable,
    // readable and writable in all scopes
    //
    pub(set) var e: Int

    // NOTE: initializer implementation skipped

    // Declare a private function,
    // only callable in the current and inner scopes
    //
    fun privateTest() {
        // ...
    }

    // Declare a public function,
    // callable in all scopes
    //
    pub fun privateTest() {
        // ...
    }
}
```

### Permissions

> 🚧 Status: Permissions are not implemented yet.

Initializers, fields, and functions of classes can be made accessible in other types by *permitting* them to do so.

<!-- TODO this is in basically adding additional scopes aside from the class in which the initialzer, field, or function is defined in -->

Permissions for classes are declared using the `permit` keyword, followed by the type that should be permitted access, the `to` keyword, and the type that should be accessible by the permitted type.

<!-- TODO: can be used e.g. for authorizations, values that represent access rights/privileges to resources. -->

```swift,file=permissions-purse.bpl
// Declare a class named `purse`, which holds a balance and
// allows amounts to be deposited from a purse to another.
//
// Purses are associated with an account.
// Deposits require an authorization.
//
class Purse {

    pub const account: Account
    pub var balance: Int

    init(initialBalance: Int) {
        self.balance = initialBalance
    }

    // Declare a function named `deposit`, which transfers an amount
    // from this purses's balance to another purses's balance,
    // but only if given a deposit authorization for this purse.
    //
    pub fun deposit(to: Purse, amount: Int, auth: DepositAuth) {
        require {
            amount > 0:
                "the amount must be positive"

            amount <= self.balance:
                "the amount must be smaller or equal to the balance"

            auth.purse == self:
                "the given authorization must be for this purse"

            amount <= auth.limit:
                "the amount must be smaller or equal to the authorization's limit"
        }

        ensure {
            self.balance == before(self.balance) - amount:
                "the amount must be deducted from this purse's balance"

            to.balance == before(to.balance) + amount:
                "the amount must be added to the receiving purse's balance"

            auth.limit == before(auth.limit) - amount:
                "the amount must be deducted from the authorization's limit"
        }

        self.balance = self.balance - amount
        to.amount = to.amount + amount

        // NOTE: as the class `Purse` is granted access to private fields
        // of class `DepoitAuth`, and the field `limit` is private and variable,
        // it can be written to by this class (`Purse`).
        //
        auth.limit = auth.limit - amount
    }

    // Declare a function named `makeDepositAuth` which creates a deposit
    // authorization with a given limit.
    //
    // NOTE: To illustrate that a deposit authorization can only be created
    // by an authorized account, the function requires passing
    // a storage authorization (`StorageAuth`; the declaration is omitted here).
    //
    // This ensures that just having a reference to this purse is not enough
    // to create a deposit authorization for it.
    //
    pub fun makeDepositAuth(limit: Int, auth: StorageAuth) -> DepositAuth {
        require {
            limit > 0:
                "the limit must be positive"

            auth.account == self.account:
                "the given authorization's account must be the account this purse belongs to"
        }

        ensure {
            result.purse == self:
                "the deposit authorization's purse must be this purse"

            result.limit == limit:
                "the deposit authorization's limit must be the given limit"
        }

        return DepositAuth(purse: self, limit: limit)
    }
}

// Declare a class named `DepositAuth`.
//
// The type represents the authorization to transfer
// a limited amount from one purse to another purse.
//
class DepositAuth {

    // Declare a publicly readable field named `purse` that has type `Purse`.
    // The deposit authorization is for a specific account, which is checked
    // in the `transfer` function of `Purse`
    //
    pub const purse: Purse

    // Declare a private field named `limit` that has type `Int`.
    //
    // As the field is private, it can normally only be accessed
    // it can only be accessed in the scope of this class.
    //
    // However, the class `Purse` is given permission to access this class,
    // so it can also read and write to this field. See the `transfer` function
    // of the `Purse` class.
    //
    private var limit: Int

    // Declare a private initializer.
    //
    // As the initializer is private, it can normally only be accessed
    // used in the scope of this class, which makes it impossible to create.
    //
    // However, the class `Purse` is given permission to access this class,
    // so it can call this private initializer. See the `makeDepositAuth`
    // function of the `Purse` class.
    //
    private init(purse: Purse, limit: Int) {
        self.purse = purse
        self.limit = limit
    }
}

// Permit the class `Purse` access to the class `DepositAuth`.
//
// This allows class `Purse` to create values of class `DepositAuth`
// by calling its private initializer, i.e., purses are allowed
// to create deposit authorizations.
//
// Furthermore it allows the class `Purse` to access the private field
// `limit` of class `Deposit`, i.e., purses are allowed to adjust the
// remaining limit of deposit authorizations.
//
permit Purse to DepositAuth
```

## Interfaces

> 🚧 Status: Interfaces are not implemented yet.

An interface is an abstract type that specifies the behavior of types that *implement* the interface. Interfaces declare the required functions and fields, as well as the access for those declarations, that implementations need to provide.

Interfaces can be implemented by [classes](#structures-and-classes), [structures](#structures-and-classes), [contracts](#contracts), and [authorizations](#authorizations). These types may implement multiple interfaces.

Interfaces consist of the function and field requirements that a type implementing the interface must provide implementations for. Implementations of interface requirements must always be at least public. Variable field requirements may be annotated to require them to be publicly settable.

Function requirements consist of the name of the function, parameter types, an optional return type, and optional preconditions and postconditions.

Field requirements consist of the name and the type of the field. Field requirements may optionally declare a getter requirement and a setter requirement, each with preconditions and postconditions.


### Interface Declaration

Interfaces are declared using the `interface` keyword, followed by the name of the interface, and the requirements enclosed in opening and closing braces.

Field requirements can be annotated to require the implementation to be a variable field, by using the `var` keyword; require the implementation to be a constant field, by using the `const` keyword; or the field requirement may specify nothing, in which case the implementation may either be a variable field, a constant field, or a synthetic field.

Field requirements and function requirements must specify the required level of access. The access must be at least be public, so the `pub` keyword must be provided. Variable field requirements can be specified to also be publicly settable by using the `pub(set)` keyword.

The special type `Self` can be used to refer to the type implementing the interface. This can be seen in the following example, where the first parameter of the `transfer` function has the `Self` type.

```typescript,file=interface-declaration.bpl
// Declare an interface for a vault (a container for a balance)
//
interface Vault {

    // Require the implementation to provide a field for the balance that is
    // readable in all scopes (`pub`).
    //
    // Neither the `var` keyword, nor the `const` keyword is used,
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
            ensure {
                result >= 0
            }
        }
    }

    // Require the implementation to provide an initializer that
    // given the initial balance, must initialize the balance field
    //
    init(initialBalance: Int) {
        ensure {
            self.balance == initialBalance:
                "the balance must be initialized to the initial balance"
        }

        // NOTE: no code
    }

    // Require the implementation to provide a function that is
    // callable in all scopes.
    //
    // The given amount must be positive and the function implementation
    // must add the amount to the balance
    //
    pub fun add(amount: Int) {
        require {
            amount > 0:
                "the amount must be positive"
        }

        ensure {
            self.balance == before(self.balance) + amount:
                "the amount must be added to the balance"
        }

        // NOTE: no code
    }

    // Require the implementation to provide a function that is
    // callable in all scopes.
    //
    // The function must transfer an amount from this vault's balance
    // to another vault's balance.
    //
    // The receiving vault must be of the same type – a transfer to a vault
    // of another type is not possible, as the `balance` field of it is only
    // readable.
    //
    // NOTE: the first parameter has the type `Self`,
    // i.e. the type implementing this interface
    //
    pub fun transfer(to receivingVault: Self, amount: Int) {
        require {
            amount > 0:
                "the amount must be positive"

            amount <= self.balance:
                "the amount must be smaller or equal to the balance"
        }

        ensure {
            self.balance == before(self.balance) - amount:
                "the amount must be deducted from the balance"

            receivingVault.balance == before(receivingVault.balance) + amount:
                "the amount must be added to the receiving balance"
        }

        // NOTE: no code
    }
}
```

Note that the required initializer and function do not have any executable code.

### Interface Implementation

Implementations are declared using the `impl` keyword, followed by the name of interface, the `for` keyword, and the name of the type (class, structure, contract, or authorization) that provides the functionality required in the interface.

```typescript,file=interface-implementation.bpl

// Declare a class named `ExampleVault` with a variable field named `balance`,
// that can be written by functions of the class, but outer scopes can only read it
//
class ExampleVault {

    // Implement the required field `balance` for the `Vault` interface.
    // The interface does not specify if the field must be variable, constant,
    // so in order for this type (`ExampleVault`) to be able to write to the field,
    // but limit outer scopes to only read from the field, it is declared variable,
    // and only has public access (non-settable).
    //
    pub var balance: Int

    // Implement the required initializer for the `Vault` interface:
    // accept an initial balance and initialize the `balance` field.
    //
    // This implementation satisfies the required postcondition
    //
    // NOTE: the postcondition declared in the interface
    // does not have to be repeated here in the implementation
    //
    init(initialBalance: Int) {
        self.balance = initialBalance
    }
}


// Declare the implementation of the interface `Vault` for the class `ExampleVault`
//
impl Vault for ExampleVault {

    // Implement the required function named `add` of the interface `Vault`,
    // that adds an amount to the vault's balance. It must be public.
    //
    // This implementation satisfies the required postcondition.
    //
    // NOTE: neither the precondition nor the postcondition declared
    // in the interface have to be repeated here in the implementation
    //
    pub fun add(amount: Int) {
        self.balance = self.balance + amount
    }

    // Implement the required function named `transfer` of the interface `Vault`,
    // that subtracts the amount from this vault's balance, and adds the amount
    // to the receiving vault's balance. It must be public.
    //
    // NOTE: the type of the receiving vault parameter is `ExampleVault`,
    // i.e., an amount can only be transferred to a vault of the same type.
    //
    // This implementation satisfies the required postconditions.
    //
    // NOTE: neither the precondition nor the postcondition declared
    // in the interface have to be repeated here in the implementation
    //
    pub fun transfer(to receivingVault: ExampleVault, amount: Int) {
        self.balance -= amount
        receivingVault.amount += amount
    }
}

// Declare two constants which have type `ExampleVault`,
// and are initialized to two different example vaults
//
const vault = ExampleVault(initialBalance: 100)
const otherVault = ExampleVault(initialBalance: 0)

// Transfer 10 units from the first vault to the second vault.
//
// The amount satisfies the precondition of the `transfer` function
// in the `Vault` interface
//
vault.transfer(to: otherVault, amount: 10)

// The postcondition of the `transfer` function in the `Vault` interface
// ensured the balances fields of the vaults were updated properly
//
// vault.balance is 90
// otherVault.balance is 10

// Error: precondition not satisfied: the parameter `amount` is larger than
// the field `balance` (100 > 90)
//
vault.transfer(to: otherVault, amount: 100)
```

The access level for variable fields in an implementation may be less restrictive than the interface requires. For example, an interface may require a field to be at least public (i.e. the `pub` keyword is specified), and an implementation may provide a variable field which is public, but also publicly settable (the `pub(set)` keyword is specified).

```typescript
interface AnInterface {
    // Require the implementation to provide a publicly readable
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

Interfaces are types. Values implementing an interface can be used as initial values for constants that have the interface as their type.

```typescript,file=interface-type.bpl
// Declare a constant that has type `Vault`, which has a value that has type `ExampleVault`
//
const vault: Vault = ExampleVault(initialBalance: 100)
```

Values implementing an interface are assignable to variables that have the interface as their type.

```typescript,file=interface-type-assignment.bpl
// Assume there is a declaration for another implementation
// of the interface  `Vault` which is named `CoolVault`

// Declare a variable that has type `Vault`,
// which has an initial value that has type `CoolVault`
//
var someVault: Vault = CoolVault(initialBalance: 100)

// Assign a different type of vault to the variable `someVault`,
// which has type `Vault`
//
someVault = ExampleVault(initialBalance: 50)


// Invalid: cannot assign a value that has type `CoolVault`
// to a constant that has type `ExampleVault`
//
const exampleVault: ExampleVault = CoolVault(initialBalance: 100)
```

Fields declared in an interface can be accessed and functions declared in an interface can be called on values of a type that implements the interface.

```typescript,file=interface-type-fields-and-functions.bpl
// Declare a constant which has the type `Vault`, and a value that has type `ExampleVault`
//
const someVault: Vault = ExampleVault(initialBalance: 100)

// Access the field `balance` declared in the interface `Vault`
//
someVault.balance // is 100

// Call the function named `add` declared in the interface `Vault`
someVault.add(amount: 50)

// someVault.balance is 150
```

### `Equatable` Interface

> 🚧 Status: The `Equatable` interface is not implemented yet.

An equatable type is a type that can be compared for equality. Types are equatable when they  implement the `Equatable` interface.

Equatable types can be compared for equality using the equals operator (`==`) or inequality using the unequals operator (`!=`).

Most of the built-in types are equatable, like booleans and integers. Arrays are equatable when their elements are equatable. Dictionaries are equatable when their values are equatable.

To make a type equatable the `Equatable` interface must be implemented, which requires the implementation of the function `equals`, which accepts another value that the given value should be compared for equality. Note that the parameter type is `Self`, i.e., the other value must have the same type as the implementing type.

```swift,file=equatable.bpl
interface Equatable {
    pub fun equals(_ other: Self) -> Bool
}
```

```swift,file=equatable-impl.bpl
// Declare a class named `Cat`, which has one field named `id`
// that has type `Int`, i.e., the identifier of the cat.
//
class Cat {
    pub const id: Int

    init(id: Int) {
        self.id = id
    }
}

// Implement the interface `Equatable` for the type `Cat`,
// to allow cats to be compared for equality.
//
impl Equatable for Cat {

    pub fun equals(_ other: Self) -> Bool {
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

Hashable types must also be equatable, i.e., they must also implement the `Equatable` interface. This is because the hash value is only evidence for inequality: two values that have different hash values are guaranteed to be unequal. However, if the hash value of two values is the same, then the two values could still be unequal and just happen to hash to the same hash value. In that case equality still needs to be determined through an equality check. Without `Equatable`, values could be added to a dictionary, but it would not be possible to retrieve them.

<!-- TODO: once interface inheritance is defined, describe how Hashable inherits from Equatable -->

Most of the built-in types are hashable, like booleans and integers. Arrays are hashable when their elements are hashable. Dictionaries are hashable when their values are equatable.

Hashing a value means passing its essential components into a hash function. Essential components are those that are used in the type's implementation of `Equatable`.

If two values are equal because their `equals` function returns true, then the implementation must return the same integer hash value for each of the two values.

The implementation must also consistently return the same integer hash value during the execution of the program when the essential components have not changed. The integer hash value must not necessarily be the same across multiple executions.

```swift,file=hashable.bpl
interface Hashable {
    pub hashValue: Int
}
```

```swift,file=hashable-impl.bpl
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

    pub fun equals(_ other: Self) -> Bool {
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

## Storage

> 🚧 Status: Storage is not implemented yet.

Storable types can be persisted across multiple executions of the program. Values are storable if they have a type that implements the interface `Storable`.

All built-in types are storable: booleans, integers, arrays with storable elements, and dictionaries with storable keys and values.

[Structures and classes](#structures-and-classes) can be stored by implementing the [interface](#interfaces) `Storable`. The only requirement is that all field types of the implementing type need to be storable.

```swift,file=storable-class.bpl
// The declaration for interface `Storable` has no explicit requirements.
interface Storable {}


// Declare a structure named `Point`.
//
struct Point {
    pub const x: Int
    pub const y: Int

    // The initializer is omitted for brevity.
}

// Implement the `Storable` interface for structure `Point`.
//
// The structure has two fields which have the type `Int`, which implements
// the `Storable` interface. As all fields implement the `Storable` interface,
// this structure can implement the `Storable` interface.
//
// There are no further requirements to satisfy, so the declaration is empty.
//
impl Storable for Point {}
```

Storable values can be stored for an [account](#accounts) by using the `storeIfNotExists` function. The function requires a [storage authorization](#storage-authorization) (`StorageAuth`) for the account.

```swift
fun storeIfNotExists(_ value: Storable, auth: StorageAuth)
```

It is only possible to store **one** value of a type.

```swift,file=storage-storeIfNotExists.bpl
// Store the integer value `42` for an account, given a storage authorization for it
//
const value = 42
const storageAuth: StorageAuth = // ...
storeIfNotExists(value, auth: storageAuth)
```

Storable values can be retrieved for an account (`Account`) using the account's `getStored` function.

```swift,file=storage-getStored.bpl
// Retrieve a stored integer value, if any, for an account
//
const account: Account = // ...
const value: Int? = account.getStored(Int)
```

To store more complex data than just a single value, structures and classes have to be defined.

The requirement to declare and use composite data structures to store more complex data is intentional, and not a "workaround". Types declaratively specify what data is stored, specify what names are used, and specify what types the values have. This approach also enables static checking that the code is correct. Storage is declarative and explicit.

The storage API is intentionally not a key-value store with arbitrary strings as keys and values. Even though such an approach would be more flexible, it would require the developer to perform ad-hoc type casting, which is error-prone and has the potential for type confusion. Also, such an approach has the potential for bugs, like accidentally naming keys that are written and keys that are read in different places in different ways. 

```swift,file=storage-simplevault.bpl
// Declare a class named `SimpleVault`.
// It is a container for a balance.
//
class SimpleVault {
    pub var balance: Int

    init(balance: Int) {
        self.balance = balance
    }
}

// Implement the `Storable` interface for the class `SimpleVault`.
//
impl Storable for SimpleVault {}

// Create a new vault with an initial zero balance.
// Store it for an account, unless there already exists one
//
const vaultToBeStored = SimpleVault(balance: 0)
const storageAuth: StorageAuth = // ...
storeIfNotExists(vaultToBeStored, auth: storageAuth)

// Retrieve a vault for an account, if any
//
const account: Account = // ...
const storedVault: SimpleVault? = account.getStored(SimpleVault)
```

## Contracts

> 🚧 Status: Contracts are not implemented yet.

A contract is similar to a [class](#structures-and-classes) in that it is a composite data structure and a reference type, i.e., it consists of values, is referenced, has an initializer, and can have functions associated with it.

Contracts differ from classes in that all fields are [stored](#storage). To make this explicit, all fields must be annotated with the `stored` keyword.

```swift,file=contract-counter.bpl
// Declare a contract named `Counter`.
//
// The counter has an initial, positive value that
// can be incremented using the function `increment`.
//
// The count is stored, i.e. persisted.
//
contract Counter {

    // Declare a public variable field for the count.
    // It is stored, i.e. persisted.
    // The field is written to by the contract, and can be read
    // in outer scopes.
    //
    pub stored var count: Int

    // Declare an initializer which accepts the initial count
    // and initializes the stored field `count` with it.
    //
    init(initialCount: Int) {
        require {
            initialCount >= 0:
                "initial count must be positive"
        }

        self.count = initialCount
    }

    // Declare a public function named `increment`
    //
    pub fun increment(_ count: Int) {
        require {
            count >= 0:
                "number must be positive"
        }

        self.count = self.count + count
    }
}
```

<!-- TODO list storable types again? -->
The type of a stored field must be [storable](#storage). Many built-in types can be stored, even complex ones, like arrays and dictionaries.

Stored fields are implicitly stored in the [storage](#storage) of the [account](#accounts) the contract is associated with.

```swift,file=contract-funigble-token.bpl
// Declare a contract named `FungibleToken`.
//
// This is a very simple fungible token contract.
// The contract keeps track of balances for IDs.
// Units can be sent from one account to another.
//
contract FungibleToken {

    // Declare a private variable field for the balances.
    // It is stored, i.e. persisted
    //
    stored var balances: Int[Int]

    // Declare an initializer which initializes the balances
    //
    init(initialBalances: Int[Int]) {
        self.balances = initialBalances
    }

    // Declare a public function named `balance`,
    // which returns the balance for an ID
    //
    pub fun balance(of id: Int) -> Int? {
        return balances[id]
    }

    // Declare a public function named `transfer` which
    // sends units from one ID to another ID
    //
    pub fun transfer(from: Int, to: Int, amount: Int) {
        require {
            amount >= 0: "the amount must be positive"
        }

        self.balances[from] = self.balances[from] - amount
        self.balances[to] = self.balances[to] + amount
    }
}
```

## Accounts

> 🚧 Status: Accounts are not implemented yet.

<!-- TODO: explain accounts -->

<!-- TODO: can't express the signature for getStored yet -->

Accounts have a `getStored` function, which retrieves a value from [storage](#storage). The function takes a type that implements the `Storable` interface, and returns an optional value that has the type.

```swift
// Get the stored integer value for an account
//
const account: Account = // ...
const value: Int? = account.getStored(Int)
```


## Built-in Types

### Never

`Never` is the return type for functions which never return normally.

```swift
// Declare a function named `crashAndBurn` which will never return,
// because it calls the function named `fatalError`, which never returns
//
fun crashAndBurn() -> Never {
    fatalError("An unrecoverable error occurred")
}
```

### Root Authorization

The authorization type `RootAuth` represents access rights/privileges to all resources.

```swift
interface RootAuth {
    pub account: Account
    pub storageAuth: StorageAuth
}
```

### Storage Authorization

The authorization type `StorageAuth` represents storage rights for an account. It is created from a [root authorization](#root-authorization). The storage authorization's account is the root authorization's account.

```swift
interface StorageAuth {
    pub account: Account

    init(_ rootAuth: RootAuth)
}
```

<!--

TODO:
- access control

-->


