# Bamboo Programming Language

The Bamboo Programming Language is a new high-level programming language intended for smart contract development.

The language's goals are, in order of importance:

- *Safety and security*: Focus on safe code: provide a strong static type system, design by contract, a capability system, and linear types. 
- *Auditability*: Focus on readability: make it easy to verify what the code is doing.
- *Simplicity*: Focus on developer productivity and usability: make it easy to write code, provide good tooling.


## Syntax

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

The `const` keyword is used to define a constant and the `var` keyword is used to define a variable.
The keywords is followed by the name, an optional [type annotation](#Type Annotations), an equals sign `=`, and the initial value.

```typescript
// declaring a constant
const a = 1

// error: re-assigning to a constant
a = 2

// declaring a variable
var b = 3

// assigning a new value to a variable
b = 4
```

Variables and constants must be initialized.

```typescript
// invalid: constant has no initial value
const a
```

Once a constant or variable is declared, it can't be redeclared with the same name, with a different type, or changed into the corresponding other kind.


```typescript
// declaring a constant
const a = 1

// invalid: re-declaring a constant with a name that is already used in this scope
const a = 2

// declaring a variable
var b = 3

// invalid: re-declaring a variable with a name that is already used in this scope
var b = 4

// invalid: declaring a variable with a name that was used for a constant
var a = 5
```

## Type Annotations

When declaring a constant or variable, an optional *type annotation* can be provided, to make it explicit what type the declaration has. If no type annotation is provided, the type of the declaration is [inferred from the initial value](#type-inference). 

```typescript
// declaring a variable with an explicit type
var initialized: bool = false

// declaring a constant with an inferred type
const a = 1
```

## Naming

Names may start with any upper and lowercase letter or an underscore. This may be followed by zero or more upper and lower case letters, underscores, and numbers. Names may not begin with a number.

```typescript
// valid
PersonID

// valid
token_name

// valid
_balance

// valid 
account2

// invalid
!@#$%^&*

// invalid
1something
```

### Conventions

By convention, variables, constants, and functions have lowercase names; and types have title-case names.


## Semicolons

Semicolons may be used to separate statements, but are optional. They can be used to separate multiple statements on a single line.

```typescript
// constant declaration statement without a semicolon
const a = 1

// variable declarations statement with a semicolon
var b = 2;

// multiple statements on a single line
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

- **Decimal**: no prefix, one or more numbers (`0` to `9`)
- **Binary**: `0b` prefix, followed by one or more zeros or ones (`0` or `1`)
- **Octal**: `0o` prefix, followed by one or more numbers in the range `0` to `7`
- **Hexadecimal**: `0x` prefix, followed by one or more numbers or characters `a` to `f`, lowercase or uppercase

```typescript
// a decimal number
const dec = 1234567890

// a binary number
const bin = 0b101010

// an octal number
const oct = 0o12345670

// a hexadecimal number
const hex = 0x1234567890ABCDEFabcdef
```

Decimal numbers may contain underscores (`_`) to logically separate components.

```typescript
const aLargeNumber = 1_000_000
```

### Arrays

Arrays are mutable, ordered collections of values. All values in the array must have the same type. Arrays may contain a value multiple times. Array literals start with an opening square bracket `[` and end with a closing square bracket `]`. 

```typescript
// an empty array
const empty = []

// an array with integers
const integers = [1, 2, 3]

// invalid: mixed types
const invalidMixed = [1, true, 2, false]
```

#### Array Indexing

To get the element of an array at a specific index, the indexing syntax can be used.

```typescript
const numbers = [42, 23]
numbers[0] // is 42
numbers[1] // is 23

const arrays = [[1, 2], [3, 4]]
arrays[1][0] // is 3
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

The left-hand side of the assignment must be an identifier, followed by one or more indices.

```typescript
const numbers = [1, 2]
numbers[0] = 3
// numbers is [3, 2]

const arrays = [[1, 2], [3, 4]]
// change the first number in the second array
arrays[1][0] = 5
// arrays is [[1, 2], [5, 4]]
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
It behaves like an if-statement, but is an expression: If the first operator value is true, 
the second operator value is returned. If the first operator value is false, the third value is returned.

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
fun double(x: Int): Int {
    return x * 2
}
```

Functions can be nested, i.e., the code of a function may declare further functions.

```typescript
fun doubleAndAddOne(n: Int): Int {
    fun double(x: Int) {
        return x * 2    
    }
    return double(n) + 1
}
```


### Function Expressions 

Functions can be also used as expressions. The syntax is the same as for function declarations, except that function expressions have no name, i.e., it is anonymous.

```typescript
// declare a constant called double, which has a function as its value, which multiples a number by two
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
double(2, 3)

// invalid: too few arguments
double()
```

### Function Types

Function types consist of the function's parameter types and the function's return type. 
The parameter types need to be enclosed in parentheses, followed by the `=>` keyword, 
and end with the return type. 

```typescript
const add: (Int, Int) => Int = 
    fun (a: Int, b: Int): Int {
        return a + b
    }
```

If the function has no return type, it implicitly has the return type `Void`.

```typescript
const doNothing: () => Void = fun () {}
```

#### Argument Passing Behavior

When arguments are passed to a function, they are not copied. Instead, parameters act as new variable bindings and the values they refer to are identical to the passed values. Modifications to mutable values made within a function will be visible to the caller. This behavior is known as [call-by-sharing](https://en.wikipedia.org/wiki/Evaluation_strategy#Call_by_sharing).


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
     x = 2
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

Every function and block (`{` ... `}`) introduces a new scope for declarations. Each function and block can refer to declarations defined in its or any of the outer scopes.

```typescript
const x = 10

fun f(): Int {
    const y = 10
    return x + y
}

f() // is 20
// invalid: y is not in scope
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

The Bamboo programming language is a _type-safe_ language. 

When assigning a new value to a variable, the value must be the same type as the variable. For example, if a variable is of type `Bool`, it can _only_ be assigned a `Bool`, and not for example an `Int`. 

```typescript
var a = true

// invalid: integer is not a boolean
a = 0
```

When passing arguments to a function, the types of the values must match the function parameters' types. For example, if a function expects an argument of type `Bool`, _only_ a `Bool` can be provided, and not for example an `Int`.

```typescript
fun nand(a: Bool, b: Bool): Bool {
    return !(a && b)
}

nand(false, false) // is true

// invalid: integers are not booleans
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
add(a, b)
```


## Type Inference

If a variable or constant is not annotated explicitly with a type, it is inferred from the value.

Integer literals are inferred to type `Int`.

```typescript
const a = 1
// a has type Int
```

Array literals are inferred based on the elements of the literal, and to be variable-size.

```typescript
const a = [1, 2]
// a has type Int[]
```

<!--

TODO: dictionaries

- the key and value type is inferred from dictionaries literals

-->


