<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Cadence Programming Language](#cadence-programming-language)
  - [Table of Contents](#table-of-contents)
  - [Introduction](#introduction)
  - [Terminology](#terminology)
  - [Syntax and Behavior](#syntax-and-behavior)
  - [Comments](#comments)
  - [Constants and Variable Declarations](#constants-and-variable-declarations)
  - [Type Annotations](#type-annotations)
  - [Naming](#naming)
    - [Conventions](#conventions)
  - [Semicolons](#semicolons)
  - [Values and Types](#values-and-types)
    - [Booleans](#booleans)
    - [Numeric Literals](#numeric-literals)
    - [Integers](#integers)
    - [Fixed-Point Numbers](#fixed-point-numbers)
    - [Floating-Point Numbers](#floating-point-numbers)
    - [Addresses](#addresses)
    - [AnyStruct and AnyResource](#anystruct-and-anyresource)
    - [Optionals](#optionals)
      - [Nil-Coalescing Operator](#nil-coalescing-operator)
      - [Force Unwrap (`!`)](#force-unwrap-)
      - [Force-assignment operator (`<-!`)](#force-assignment-operator--)
      - [Conditional Downcasting Operator](#conditional-downcasting-operator)
    - [Never](#never)
    - [Strings and Characters](#strings-and-characters)
      - [String Fields and Functions](#string-fields-and-functions)
    - [Arrays](#arrays)
      - [Array Types](#array-types)
      - [Array Indexing](#array-indexing)
      - [Array Fields and Functions](#array-fields-and-functions)
        - [Variable-size Array Functions](#variable-size-array-functions)
    - [Dictionaries](#dictionaries)
      - [Dictionary Types](#dictionary-types)
      - [Dictionary Access](#dictionary-access)
      - [Dictionary Fields and Functions](#dictionary-fields-and-functions)
      - [Dictionary Keys](#dictionary-keys)
  - [Operators](#operators)
    - [Negation](#negation)
    - [Assignment](#assignment)
    - [Swapping](#swapping)
    - [Arithmetic](#arithmetic)
    - [Logical Operators](#logical-operators)
    - [Comparison operators](#comparison-operators)
    - [Ternary Conditional Operator](#ternary-conditional-operator)
    - [Precedence and Associativity](#precedence-and-associativity)
  - [Functions](#functions)
    - [Function Declarations](#function-declarations)
    - [Function overloading](#function-overloading)
    - [Function Expressions](#function-expressions)
    - [Function Calls](#function-calls)
    - [Function Types](#function-types)
    - [Closures](#closures)
    - [Argument Passing Behavior](#argument-passing-behavior)
    - [Function Preconditions and Postconditions](#function-preconditions-and-postconditions)
  - [Control flow](#control-flow)
    - [Conditional branching: if-statement](#conditional-branching-if-statement)
    - [Optional Binding](#optional-binding)
    - [Looping](#looping)
      - [while-statement](#while-statement)
      - [For-in statement](#for-in-statement)
      - [continue and break](#continue-and-break)
    - [Immediate function return: return-statement](#immediate-function-return-return-statement)
  - [Scope](#scope)
  - [Type Safety](#type-safety)
  - [Type Inference](#type-inference)
  - [Composite Types](#composite-types)
    - [Composite Type Declaration and Creation](#composite-type-declaration-and-creation)
    - [Composite Type Fields](#composite-type-fields)
    - [Composite Data Initializer Overloading](#composite-data-initializer-overloading)
    - [Composite Type Field Getters and Setters](#composite-type-field-getters-and-setters)
    - [Synthetic Composite Type Fields](#synthetic-composite-type-fields)
    - [Composite Type Functions](#composite-type-functions)
    - [Composite Type Subtyping](#composite-type-subtyping)
    - [Composite Type Behaviour](#composite-type-behaviour)
      - [Structures](#structures)
      - [Accessing Fields and Functions of Composite Types Using Optional Chaining](#accessing-fields-and-functions-of-composite-types-using-optional-chaining)
      - [Resources](#resources)
      - [Resource Variables](#resource-variables)
      - [Resource Destructors](#resource-destructors)
      - [Nested Resources](#nested-resources)
      - [Resources in Closures](#resources-in-closures)
      - [Resources in Arrays and Dictionaries](#resources-in-arrays-and-dictionaries)
    - [Unbound References / Nulls](#unbound-references--nulls)
    - [Inheritance and Abstract Types](#inheritance-and-abstract-types)
  - [Access control](#access-control)
  - [Interfaces](#interfaces)
    - [Interface Declaration](#interface-declaration)
    - [Interface Implementation](#interface-implementation)
    - [Interfaces in Types](#interfaces-in-types)
    - [Interface Implementation Requirements](#interface-implementation-requirements)
    - [Interface Nesting](#interface-nesting)
    - [Nested Type Requirements](#nested-type-requirements)
    - [`Equatable` Interface](#equatable-interface)
    - [`Hashable` Interface](#hashable-interface)
  - [Restricted Types](#restricted-types)
  - [References](#references)
  - [Imports](#imports)
  - [Accounts](#accounts)
  - [Account Storage](#account-storage)
  - [Capability-based Access Control](#capability-based-access-control)
  - [Contracts](#contracts)
    - [Deploying and Updating Contracts](#deploying-and-updating-contracts)
    - [Contract Interfaces](#contract-interfaces)
  - [Events](#events)
    - [Emitting events](#emitting-events)
  - [Transactions](#transactions)
    - [Importing and using Deployed Contract Code](#importing-and-using-deployed-contract-code)
  - [Built-in Functions](#built-in-functions)
    - [Block and Transaction Information](#block-and-transaction-information)
    - [`panic`](#panic)
      - [Example](#example)
    - [`assert`](#assert)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# [](#cadence-programming-language)Cadence Programming Language

_Bastian Müller, Dieter Shirley, Joshua Hannan_

## [](#table-of-contents)Table of Contents

-   [Introduction](#introduction)

-   [Terminology](#terminology)

-   [Syntax and Behavior](#syntax-and-behavior)

-   [Comments](#comments)

-   [Constants and Variable Declarations](#constants-and-variable-declarations)

-   [Type Annotations](#type-annotations)

-   [Naming](#naming)

    -   [Conventions](#conventions)

-   [Semicolons](#semicolons)

-   [Values and Types](#values-and-types)

    -   [Booleans](#booleans)

    -   [Numeric Literals](#numeric-literals)

    -   [Integers](#integers)

    -   [Fixed-Point Numbers](#fixed-point-numbers)

    -   [Floating-Point Numbers](#floating-point-numbers)

    -   [Addresses](#addresses)

    -   [AnyStruct and AnyResource](#anystruct-and-anyresource)

    -   [Optionals](#optionals)

        -   [Nil-Coalescing Operator](#nil-coalescing-operator)
        -   [Force Unwrap (`!`)](#force-unwrap-)
        -   [Force-assignment operator (`<-!`)](#force-assignment-operator--)
        -   [Conditional Downcasting Operator](#conditional-downcasting-operator)

    -   [Never](#never)

    -   [Strings and Characters](#strings-and-characters)

        -   [String Fields and Functions](#string-fields-and-functions)

    -   [Arrays](#arrays)

        -   [Array Types](#array-types)

        -   [Array Indexing](#array-indexing)

        -   [Array Fields and Functions](#array-fields-and-functions)

            -   [Variable-size Array Functions](#variable-size-array-functions)

    -   [Dictionaries](#dictionaries)

        -   [Dictionary Types](#dictionary-types)
        -   [Dictionary Access](#dictionary-access)
        -   [Dictionary Fields and Functions](#dictionary-fields-and-functions)
        -   [Dictionary Keys](#dictionary-keys)

-   [Operators](#operators)

    -   [Negation](#negation)
    -   [Assignment](#assignment)
    -   [Swapping](#swapping)
    -   [Arithmetic](#arithmetic)
    -   [Logical Operators](#logical-operators)
    -   [Comparison operators](#comparison-operators)
    -   [Ternary Conditional Operator](#ternary-conditional-operator)
    -   [Precedence and Associativity](#precedence-and-associativity)

-   [Functions](#functions)

    -   [Function Declarations](#function-declarations)
    -   [Function overloading](#function-overloading)
    -   [Function Expressions](#function-expressions)
    -   [Function Calls](#function-calls)
    -   [Function Types](#function-types)
    -   [Closures](#closures)
    -   [Argument Passing Behavior](#argument-passing-behavior)
    -   [Function Preconditions and Postconditions](#function-preconditions-and-postconditions)

-   [Control flow](#control-flow)

    -   [Conditional branching: if-statement](#conditional-branching-if-statement)

    -   [Optional Binding](#optional-binding)

    -   [Looping](#looping)

        -   [while-statement](#while-statement)
        -   [For-in statement](#for-in-statement)
        -   [continue and break](#continue-and-break)

    -   [Immediate function return: return-statement](#immediate-function-return-return-statement)

-   [Scope](#scope)

-   [Type Safety](#type-safety)

-   [Type Inference](#type-inference)

-   [Composite Types](#composite-types)

    -   [Composite Type Declaration and Creation](#composite-type-declaration-and-creation)

    -   [Composite Type Fields](#composite-type-fields)

    -   [Composite Data Initializer Overloading](#composite-data-initializer-overloading)

    -   [Composite Type Field Getters and Setters](#composite-type-field-getters-and-setters)

    -   [Synthetic Composite Type Fields](#synthetic-composite-type-fields)

    -   [Composite Type Functions](#composite-type-functions)

    -   [Composite Type Subtyping](#composite-type-subtyping)

    -   [Composite Type Behaviour](#composite-type-behaviour)

        -   [Structures](#structures)
        -   [Accessing Fields and Functions of Composite Types Using Optional Chaining](#accessing-fields-and-functions-of-composite-types-using-optional-chaining)
        -   [Resources](#resources)
        -   [Resource Variables](#resource-variables)
        -   [Resource Destructors](#resource-destructors)
        -   [Nested Resources](#nested-resources)
        -   [Resources in Closures](#resources-in-closures)
        -   [Resources in Arrays and Dictionaries](#resources-in-arrays-and-dictionaries)

    -   [Unbound References / Nulls](#unbound-references--nulls)

    -   [Inheritance and Abstract Types](#inheritance-and-abstract-types)

-   [Access control](#access-control)

-   [Interfaces](#interfaces)

    -   [Interface Declaration](#interface-declaration)
    -   [Interface Implementation](#interface-implementation)
    -   [Interfaces in Types](#interfaces-in-types)
    -   [Interface Implementation Requirements](#interface-implementation-requirements)
    -   [Interface Nesting](#interface-nesting)
    -   [Nested Type Requirements](#nested-type-requirements)
    -   [`Equatable` Interface](#equatable-interface)
    -   [`Hashable` Interface](#hashable-interface)

-   [Restricted Types](#restricted-types)

-   [References](#references)

-   [Imports](#imports)

-   [Accounts](#accounts)

-   [Account Storage](#account-storage)

-   [Capability-based Access Control](#capability-based-access-control)

-   [Contracts](#contracts)

    -   [Deploying and Updating Contracts](#deploying-and-updating-contracts)
    -   [Contract Interfaces](#contract-interfaces)

-   [Events](#events)

    -   [Emitting events](#emitting-events)

-   [Transactions](#transactions)

    -   [Importing and using Deployed Contract Code](#importing-and-using-deployed-contract-code)

-   [Built-in Functions](#built-in-functions)

    -   [Block and Transaction Information](#block-and-transaction-information)

    -   [`panic`](#panic)

        -   [Example](#example)

    -   [`assert`](#assert)

## [](#introduction)Introduction

The Cadence Programming Language is a new high-level programming language
intended for smart contract development.

The language&#x27;s goals are, in order of importance:

-   **Safety and security**:
    Provide a strong static type system, design by contract (preconditions and postconditions),
    and resources (inspired by linear types).

-   **Auditability**:
    Focus on readability: Make it easy to verify what the code is doing,
    and make intentions explicit, at a small cost of verbosity.

-   **Simplicity**: Focus on developer productivity and usability:
    Make it easy to write code, provide good tooling.

## [](#terminology)Terminology

In this document, the following terminology is used to describe syntax
or behavior that is not allowed in the language:

-   `Invalid` means that the invalid program will not even be allowed to run.
    The program error is detected and reported statically by the type checker.

-   `Run-time error` means that the erroneous program will run,
    but bad behavior will result in the execution of the program being aborted.

## [](#syntax-and-behavior)Syntax and Behavior

Much of the language&#x27;s syntax is inspired by Swift, Kotlin, and TypeScript.

Much of the syntax, types, and standard library is inspired by Swift,
which popularized e.g. optionals, argument labels,
and provides safe handling of integers and strings.

Resources are based on liner types which were popularized by Rust.

Events are inspired by Solidity.

**Disclaimer:** In real Cadence code, all type definitions and code
must be declared and contained in [contracts](#contracts) or [transactions](#transactions),
but we omit these containers in examples for simplicity.

## [](#comments)Comments

Comments can be used to document code.
A comment is text that is not executed.

_Single-line comments_ start with two slashes (`//`).
These comments can go on a line by themselves or they can go directly after a line of code.

<code><pre><span style="color: #008000">// This is a comment on a single line.</span><span>
</span><span style="color: #008000">// Another comment line that is not executed.</span><span>
</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> x = </span><span style="color: #09885A">1</span><span style="color: #000000">  </span><span style="color: #008000">// Here is another comment after a line of code.</span><span>
</span></pre></code>

_Multi-line comments_ start with a slash and an asterisk (`/*`)
and end with an asterisk and a slash (`*/`):

<code><pre><span style="color: #008000">/* This is a comment which</span><span>
</span><span style="color: #008000">spans multiple lines. */</span><span>
</span></pre></code>

Comments may be nested.

<code><pre><span style="color: #008000">/* /* this */ is a valid comment */</span><span>
</span></pre></code>

Mutli-line comments are balanced.

<code><pre><span style="color: #008000">/* this is a // comment up to here */</span><span style="color: #000000"> this is not part of the comment </span><span style="color: #CD3131">*/</span><span>
</span></pre></code>

## [](#constants-and-variable-declarations)Constants and Variable Declarations

Constants and variables are declarations that bind
a value and [type](#type-safety) to an identifier.
Constants are initialized with a value and cannot be reassigned afterwards.
Variables are initialized with a value and can be reassigned later.
Declarations can be created in any scope, including the global scope.

Constant means that the _identifier&#x27;s_ association is constant,
not the _value_ itself –
the value may still be changed if is mutable.

Constants are declared using the `let` keyword. Variables are declared
using the `var` keyword.
The keywords are followed by the identifier,
an optional [type annotation](#type-annotations), an equals sign `=`,
and the initial value.

<code><pre><span style="color: #008000">// Declare a constant named `a`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> a = </span><span style="color: #09885A">1</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: re-assigning to a constant.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">a = </span><span style="color: #09885A">2</span><span>
</span><span>
</span><span style="color: #008000">// Declare a variable named `b`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> b = </span><span style="color: #09885A">3</span><span>
</span><span>
</span><span style="color: #008000">// Assign a new value to the variable named `b`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">b = </span><span style="color: #09885A">4</span><span>
</span></pre></code>

Variables and constants **must** be initialized.

<code><pre><span style="color: #008000">// Invalid: the constant has no initial value.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> a</span><span>
</span></pre></code>

The names of the variable or constant
declarations in each scope must be unique.
Declaring another variable or constant with a name that is already
declared in the current scope is invalid, regardless of kind or type.

<code><pre><span style="color: #008000">// Declare a constant named `a`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> a = </span><span style="color: #09885A">1</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: cannot re-declare a constant with name `a`,</span><span>
</span><span style="color: #008000">// as it is already used in this scope.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> a = </span><span style="color: #09885A">2</span><span>
</span><span>
</span><span style="color: #008000">// Declare a variable named `b`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> b = </span><span style="color: #09885A">3</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: cannot re-declare a variable with name `b`,</span><span>
</span><span style="color: #008000">// as it is already used in this scope.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> b = </span><span style="color: #09885A">4</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: cannot declare a variable with the name `a`,</span><span>
</span><span style="color: #008000">// as it is already used in this scope,</span><span>
</span><span style="color: #008000">// and it is declared as a constant.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> a = </span><span style="color: #09885A">5</span><span>
</span></pre></code>

However, variables can be redeclared in sub-scopes.

<code><pre><span style="color: #008000">// Declare a constant named `a`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> a = </span><span style="color: #09885A">1</span><span>
</span><span>
</span><span style="color: #0000FF">if</span><span style="color: #000000"> </span><span style="color: #0000FF">true</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Declare a constant with the same name `a`.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// This is valid because it is in a sub-scope.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// This variable is not visible to the outer scope.</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">let</span><span style="color: #000000"> a = </span><span style="color: #09885A">2</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// `a` is `1`</span><span>
</span></pre></code>

A variable cannot be used as its own initial value.

<code><pre><span style="color: #008000">// Invalid: Use of variable in its own initial value.</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> a = a</span><span>
</span></pre></code>

## [](#type-annotations)Type Annotations

When declaring a constant or variable,
an optional _type annotation_ can be provided,
to make it explicit what type the declaration has.

If no type annotation is provided, the type of the declaration is
[inferred from the initial value](#type-inference).

<code><pre><span style="color: #008000">// Declare a variable named `boolVarWithAnnotation`, which has an explicit type annotation.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// `Bool` is the type of booleans.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> boolVarWithAnnotation: </span><span style="color: #0000FF">Bool</span><span style="color: #000000"> = </span><span style="color: #0000FF">false</span><span>
</span><span>
</span><span style="color: #008000">// Declare a constant named `integerWithoutAnnotation`, which has no type annotation</span><span>
</span><span style="color: #008000">// and for which the type is inferred to be `Int`, the type of arbitrary-precision integers.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// This is based on the initial value which is an integer literal.</span><span>
</span><span style="color: #008000">// Integer literals are always inferred to be of type `Int`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> integerWithoutAnnotation = </span><span style="color: #09885A">1</span><span>
</span><span>
</span><span style="color: #008000">// Declare a constant named `smallIntegerWithAnnotation`, which has an explicit type annotation.</span><span>
</span><span style="color: #008000">// Because of the explicit type annotation, the type is not inferred.</span><span>
</span><span style="color: #008000">// This declaration is valid because the integer literal `1` fits into the range of the type `Int8`,</span><span>
</span><span style="color: #008000">// the type of 8-bit signed integers.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> smallIntegerWithAnnotation: </span><span style="color: #0000FF">Int8</span><span style="color: #000000"> = </span><span style="color: #09885A">1</span><span>
</span></pre></code>

If a type annotation is provided, the initial value must be of this type.
All new values assigned to variables must match its type.
This type safety is explained in more detail in a [separate section](#type-safety).

<code><pre><span style="color: #008000">// Invalid: declare a variable with an explicit type `Bool`,</span><span>
</span><span style="color: #008000">// but the initial value has type `Int`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> booleanConstant: </span><span style="color: #0000FF">Bool</span><span style="color: #000000"> = </span><span style="color: #09885A">1</span><span>
</span><span>
</span><span style="color: #008000">// Declare a variable that has the inferred type `Bool`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> booleanVariable = </span><span style="color: #0000FF">false</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: assign a value with type `Int` to a variable which has the inferred type `Bool`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">booleanVariable = </span><span style="color: #09885A">1</span><span>
</span></pre></code>

## [](#naming)Naming

Names may start with any upper or lowercase letter (A-Z, a-z)
or an underscore (`_`).
This may be followed by zero or more upper and lower case letters,
underscores, and numbers (0-9).
Names may not begin with a number.

<code><pre><span style="color: #008000">// Valid: title-case</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">PersonID</span><span>
</span><span>
</span><span style="color: #008000">// Valid: with underscore</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">token_name</span><span>
</span><span>
</span><span style="color: #008000">// Valid: leading underscore and characters</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">_balance</span><span>
</span><span>
</span><span style="color: #008000">// Valid: leading underscore and numbers</span><span>
</span><span style="color: #000000">_8264</span><span>
</span><span>
</span><span style="color: #008000">// Valid: characters and number</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">account2</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: leading number</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">1something</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: invalid character #</span><span>
</span><span style="color: #000000">_#</span><span style="color: #09885A">1</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: various invalid characters</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">!@#$%^&#x26;*</span><span>
</span></pre></code>

### [](#conventions)Conventions

By convention, variables, constants, and functions have lowercase names;
and types have title-case names.

## [](#semicolons)Semicolons

Semicolons (;) are used as statement separators.
A semicolon can be placed after any statement,
but can be omitted if only one statement appears on the line.
Semicolons must be used to separate multiple statements if they appear on the same line –
exactly one semicolon between each pair of statements.

<code><pre><span style="color: #008000">// Declare a constant, without a semicolon.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> a = </span><span style="color: #09885A">1</span><span>
</span><span>
</span><span style="color: #008000">// Declare a variable, with a semicolon.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> b = </span><span style="color: #09885A">2</span><span style="color: #000000">;</span><span>
</span><span>
</span><span style="color: #008000">// Declare a constant and a variable on a single line, separated by semicolons.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> d = </span><span style="color: #09885A">1</span><span style="color: #000000">; </span><span style="color: #0000FF">var</span><span style="color: #000000"> e = </span><span style="color: #09885A">2</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Multiple semicolons between statements.</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> f = </span><span style="color: #09885A">1</span><span style="color: #000000">;; </span><span style="color: #0000FF">let</span><span style="color: #000000"> g = </span><span style="color: #09885A">2</span><span>
</span></pre></code>

## [](#values-and-types)Values and Types

Values are objects, like for example booleans, integers, or arrays.
Values are typed.

### [](#booleans)Booleans

The two boolean values `true` and `false` have the type `Bool`.

### [](#numeric-literals)Numeric Literals

Numbers can be written in various bases. Numbers are assumed to be decimal by default.
Non-decimal literals have a specific prefix.

| Numeral system  | Prefix | Characters                                                            |
| :-------------- | :----- | :-------------------------------------------------------------------- |
| **Decimal**     | _None_ | one or more numbers (`0` to `9`)                                      |
| **Binary**      | `0b`   | one or more zeros or ones (`0` or `1`)                                |
| **Octal**       | `0o`   | one or more numbers in the range `0` to `7`                           |
| **Hexadecimal** | `0x`   | one or more numbers, or characters `a` to `f`, lowercase or uppercase |

<code><pre><span style="color: #008000">// A decimal number</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #09885A">1234567890</span><span style="color: #000000">  </span><span style="color: #008000">// is `1234567890`</span><span>
</span><span>
</span><span style="color: #008000">// A binary number</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #09885A">0b101010</span><span style="color: #000000">  </span><span style="color: #008000">// is `42`</span><span>
</span><span>
</span><span style="color: #008000">// An octal number</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #09885A">0o12345670</span><span style="color: #000000">  </span><span style="color: #008000">// is `2739128`</span><span>
</span><span>
</span><span style="color: #008000">// A hexadecimal number</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #09885A">0x1234567890ABCabc</span><span style="color: #000000">  </span><span style="color: #008000">// is `1311768467294898876`</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: unsupported prefix 0z</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">0z0</span><span>
</span><span>
</span><span style="color: #008000">// A decimal number with leading zeros. Not an octal number!</span><span>
</span><span style="color: #09885A">00123</span><span style="color: #000000"> </span><span style="color: #008000">// is `123`</span><span>
</span><span>
</span><span style="color: #008000">// A binary number with several trailing zeros.</span><span>
</span><span style="color: #09885A">0b001000</span><span style="color: #000000">  </span><span style="color: #008000">// is `8`</span><span>
</span></pre></code>

Decimal numbers may contain underscores (`_`) to logically separate components.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> largeNumber = </span><span style="color: #09885A">1_000_000</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Value is not a number literal, but a variable.</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> notNumber = _123</span><span>
</span></pre></code>

Underscores are allowed for all numeral systems.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> binaryNumber = </span><span style="color: #09885A">0b10_11_01</span><span>
</span></pre></code>

### [](#integers)Integers

Integers are numbers without a fractional part.
They are either _signed_ (positive, zero, or negative)
or _unsigned_ (positive or zero).

Signed integer types which check for overflow and underflow have an `Int` prefix
and can represent values in the following ranges:

-   **`Int8`**: −2^7 through 2^7 − 1 (-128 through 127)
-   **`Int16`**: −2^15 through 2^15 − 1 (-32768 through 32767)
-   **`Int32`**: −2^31 through 2^31 − 1 (-2147483648 through 2147483647)
-   **`Int64`**: −2^63 through 2^63 − 1 (-9223372036854775808 through 9223372036854775807)
-   **`Int128`**: −2^127 through 2^127 − 1
-   **`Int256`**: −2^255 through 2^255 − 1

Unsigned integer types which check for overflow and underflow have a `UInt` prefix
and can represent values in the following ranges:

-   **`UInt8`**: 0 through 2^8 − 1 (255)
-   **`UInt16`**: 0 through 2^16 − 1 (65535)
-   **`UInt32`**: 0 through 2^32 − 1 (4294967295)
-   **`UInt64`**: 0 through 2^64 − 1 (18446744073709551615)
-   **`UInt128`**: 0 through 2^128 − 1
-   **`UInt256`**: 0 through 2^256 − 1

Unsigned integer types which do **not** check for overflow and underflow,
i.e. wrap around, have the `Word` prefix
and can represent values in the following ranges:

-   **`Word8`**: 0 through 2^8 − 1 (255)
-   **`Word16`**: 0 through 2^16 − 1 (65535)
-   **`Word32`**: 0 through 2^32 − 1 (4294967295)
-   **`Word64`**: 0 through 2^64 − 1 (18446744073709551615)

The types are independent types, i.e. not subtypes of each other.

See the section about [artihmetic operators](#arithmetic) for further
information about the behavior of the different integer types.

<code><pre><span style="color: #008000">// Declare a constant that has type `UInt8` and the value 10.</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> smallNumber: </span><span style="color: #0000FF">UInt8</span><span style="color: #000000"> = </span><span style="color: #09885A">10</span><span>
</span></pre></code>

<code><pre><span style="color: #008000">// Invalid: negative literal cannot be used as an unsigned integer</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> invalidNumber: </span><span style="color: #0000FF">UInt8</span><span style="color: #000000"> = </span><span style="color: #09885A">-10</span><span>
</span></pre></code>

In addition, the arbitrary precision integer type `Int` is provided.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> veryLargeNumber: </span><span style="color: #0000FF">Int</span><span style="color: #000000"> = </span><span style="color: #09885A">10000000000000000000000000000000</span><span>
</span></pre></code>

Integer literals are [inferred](#type-inference) to have type `Int`,
or if the literal occurs in a position that expects an explicit type,
e.g. in a variable declaration with an explicit type annotation.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> someNumber = </span><span style="color: #09885A">123</span><span>
</span><span>
</span><span style="color: #008000">// `someNumber` has type `Int`</span><span>
</span></pre></code>

Negative integers are encoded in two&#x27;s complement representation.

Integer types are not converted automatically. Types must be explicitly converted,
which can be done by calling the constructor of the type with the integer type.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> x: </span><span style="color: #0000FF">Int8</span><span style="color: #000000"> = </span><span style="color: #09885A">1</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> y: </span><span style="color: #0000FF">Int16</span><span style="color: #000000"> = </span><span style="color: #09885A">2</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: the types of the operands, `Int8` and `Int16` are incompatible.</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> z = x + y</span><span>
</span><span>
</span><span style="color: #008000">// Explicitly convert `x` from `Int8` to `Int16`.</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> a = Int16(x) + y</span><span>
</span><span>
</span><span style="color: #008000">// `a` has type `Int16`</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: The integer literal is expected to be of type `UInt8`,</span><span>
</span><span style="color: #008000">// but the large integer literal does not fit in the range of `UInt8`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> b = x + </span><span style="color: #09885A">1000000000000000000000000</span><span>
</span></pre></code>

### [](#fixed-point-numbers)Fixed-Point Numbers

> 🚧 Status: Currently only the 64-bit wide `Fix64` and `UFix64` types are available.
> More fixed-point number types will be added in a future release.

Fixed-point numbers are useful for representing fractional values.
They have a fixed number of digits after decimal point.

They are essentially integers which are scaled by a factor.
For example, the value 1.23 can be represented as 1230 with a scaling factor of 1/1000.
The scaling factor is the same for all values of the same type
and stays the same during calculations.

Fixed-point numbers in Cadence have a scaling factor with a power of 10, instead of a power of 2,
i.e. they are decimal, not binary.

Signed fixed-point number types have the prefix `Fix`,
have the following factors, and can represent values in the following ranges:

-   **`Fix64`**: Factor 1/100,000,000; -92233720368.54775808 through 92233720368.54775807

Unsigned fixed-point number types have the prefix `UFix`,
have the following factors, and can represent values in the following ranges:

-   **`UFix64`**: Factor 1/100,000,000; 0.0 through 184467440737.09551615

### [](#floating-point-numbers)Floating-Point Numbers

There is **no** support for floating point numbers.

Smart Contracts are not intended to work with values with error margins
and therefore floating point arithmetic is not appropriate here.

Instead, consider using [fixed point numbers](#fixed-point-numbers).

### [](#addresses)Addresses

The type `Address` represents an address.
Addresses are unsigned integers with a size of 160 bits (20 bytes).
Hexadecimal integer literals can be used to create address values.

<code><pre><span style="color: #008000">// Declare a constant that has type `Address`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> someAddress: </span><span style="color: #0000FF">Address</span><span style="color: #000000"> = </span><span style="color: #09885A">0x06012c8cf97bead5deae237070f9587f8e7a266d</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Initial value is not compatible with type `Address`,</span><span>
</span><span style="color: #008000">// it is not a number.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> notAnAddress: </span><span style="color: #0000FF">Address</span><span style="color: #000000"> = </span><span style="color: #A31515">""</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Initial value is not compatible with type `Address`.</span><span>
</span><span style="color: #008000">// The integer literal is valid, however, it is larger than 160 bits.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> alsoNotAnAddress: </span><span style="color: #0000FF">Address</span><span style="color: #000000"> = </span><span style="color: #09885A">0x06012c8cf97bead5deae237070f9587f8e7a266d123456789</span><span>
</span></pre></code>

Integer literals are not inferred to be an address.

<code><pre><span style="color: #008000">// Declare a number. Even though it happens to be a valid address,</span><span>
</span><span style="color: #008000">// it is not inferred as it.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> aNumber = </span><span style="color: #09885A">0x06012c8cf97bead5deae237070f9587f8e7a266d</span><span>
</span><span style="color: #008000">// `aNumber` has type `Int`</span><span>
</span></pre></code>

### [](#anystruct-and-anyresource)AnyStruct and AnyResource

`AnyStruct` is the top type of all non-resource types,
i.e., all non-resource types are a subtype of it.

`AnyResource` is the top type of all resource types.

<code><pre><span style="color: #008000">// Declare a variable that has the type `AnyStruct`.</span><span>
</span><span style="color: #008000">// Any non-resource typed value can be assigned to it, for example an integer,</span><span>
</span><span style="color: #008000">// but not resoure-typed values.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> someStruct: </span><span style="color: #0000FF">AnyStruct</span><span style="color: #000000"> = </span><span style="color: #09885A">1</span><span>
</span><span>
</span><span style="color: #008000">// Assign a value with a different non-resource type, `Bool`.</span><span>
</span><span style="color: #000000">someStruct = </span><span style="color: #0000FF">true</span><span>
</span><span>
</span><span style="color: #008000">// Declare a structure named `TestStruct`, create an instance of it,</span><span>
</span><span style="color: #008000">// and assign it to the `AnyStruct`-typed variable</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">struct</span><span style="color: #000000"> TestStruct {}</span><span>
</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> testStruct = TestStruct()</span><span>
</span><span>
</span><span style="color: #000000">someStruct = testStruct</span><span>
</span><span>
</span><span style="color: #008000">// Declare a resource named `TestResource`</span><span>
</span><span>
</span><span style="color: #0000FF">resource</span><span style="color: #000000"> Test {}</span><span>
</span><span>
</span><span style="color: #008000">// Declare a variable that has the type `AnyResource`.</span><span>
</span><span style="color: #008000">// Any resource-typed value can be assigned to it,</span><span>
</span><span style="color: #008000">// but not non-resource typed values.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> someResource: @</span><span style="color: #0000FF">AnyResource</span><span style="color: #000000"> &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> Test()</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Resource-typed values can not be assigned</span><span>
</span><span style="color: #008000">// to `AnyStruct`-typed variables</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">someStruct &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> Test()</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Non-resource typed values can not be assigned</span><span>
</span><span style="color: #008000">// to `AnyResource`-typed variables</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">someResource = </span><span style="color: #09885A">1</span><span>
</span></pre></code>

However, using `AnyStruct` and `AnyResource` does not opt-out of type checking.
It is invalid to access fields and call functions on these types,
as they have no fields and functions.

<code><pre><span style="color: #008000">// Declare a variable that has the type `AnyStruct`.</span><span>
</span><span style="color: #008000">// The initial value is an integer,</span><span>
</span><span style="color: #008000">// but the variable still has the explicit type `AnyStruct`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> a: </span><span style="color: #0000FF">AnyStruct</span><span style="color: #000000"> = </span><span style="color: #09885A">1</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Operator cannot be used for an `AnyStruct` value (`a`, left-hand side)</span><span>
</span><span style="color: #008000">// and an `Int` value (`2`, right-hand side).</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">a + </span><span style="color: #09885A">2</span><span>
</span></pre></code>

`AnyStruct` and `AnyResource` may be used like other types,
for example, they may be the element type of [arrays](#arrays)
or be the element type of an [optional type](#optionals).

<code><pre><span style="color: #008000">// Declare a variable that has the type `[AnyStruct]`,</span><span>
</span><span style="color: #008000">// i.e. an array of elements of any non-resource type.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> anyValues: [</span><span style="color: #0000FF">AnyStruct</span><span style="color: #000000">] = [</span><span style="color: #09885A">1</span><span style="color: #000000">, </span><span style="color: #A31515">"2"</span><span style="color: #000000">, </span><span style="color: #0000FF">true</span><span style="color: #000000">]</span><span>
</span><span>
</span><span style="color: #008000">// Declare a variable that has the type `AnyStruct?`,</span><span>
</span><span style="color: #008000">// i.e. an optional type of any non-resource type.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> maybeSomething: </span><span style="color: #0000FF">AnyStruct</span><span style="color: #000000">? = </span><span style="color: #09885A">42</span><span>
</span><span>
</span><span style="color: #000000">maybeSomething = </span><span style="color: #A31515">"twenty-four"</span><span>
</span><span>
</span><span style="color: #000000">maybeSomething = </span><span style="color: #0000FF">nil</span><span>
</span></pre></code>

`AnyStruct` is also the super-type of all non-resource optional types,
and `AnyResource` is the super-type of all resource optional types.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> maybeInt: </span><span style="color: #0000FF">Int</span><span style="color: #000000">? = </span><span style="color: #09885A">1</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> anything: </span><span style="color: #0000FF">AnyStruct</span><span style="color: #000000"> = maybeInt</span><span>
</span></pre></code>

[Conditional downcasting](#conditional-downcasting-operator) allows coercing
a value which has the type `AnyStruct` or `AnyResource` back to its orignal type.

### [](#optionals)Optionals

Optionals are values which can represent the absence of a value. Optionals have two cases:
either there is a value, or there is nothing.

An optional type is declared using the `?` suffix for another type.
For example, `Int` is a non-optional integer, and `Int?` is an optional integer,
i.e. either nothing, or an integer.

The value representing nothing is `nil`.

<code><pre><span style="color: #008000">// Declare a constant which has an optional integer type,</span><span>
</span><span style="color: #008000">// with nil as its initial value.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> a: </span><span style="color: #0000FF">Int</span><span style="color: #000000">? = </span><span style="color: #0000FF">nil</span><span>
</span><span>
</span><span style="color: #008000">// Declare a constant which has an optional integer type,</span><span>
</span><span style="color: #008000">// with 42 as its initial value.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> b: </span><span style="color: #0000FF">Int</span><span style="color: #000000">? = </span><span style="color: #09885A">42</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: `b` has type `Int?`, which does not support arithmetic.</span><span>
</span><span style="color: #000000">b + </span><span style="color: #09885A">23</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Declare a constant with a non-optional integer type `Int`,</span><span>
</span><span style="color: #008000">// but the initial value is `nil`, which in this context has type `Int?`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> x: </span><span style="color: #0000FF">Int</span><span style="color: #000000"> = </span><span style="color: #0000FF">nil</span><span>
</span></pre></code>

Optionals can be created for any value, not just for literals.

<code><pre><span style="color: #008000">// Declare a constant which has a non-optional integer type,</span><span>
</span><span style="color: #008000">// with 1 as its initial value.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> x = </span><span style="color: #09885A">1</span><span>
</span><span>
</span><span style="color: #008000">// Declare a constant which has an optional integer type.</span><span>
</span><span style="color: #008000">// An optional with the value of `x` is created.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> y: </span><span style="color: #0000FF">Int</span><span style="color: #000000">? = x</span><span>
</span><span>
</span><span style="color: #008000">// Declare a variable which has an optional any type, i.e. the variable</span><span>
</span><span style="color: #008000">// may be `nil`, or any other value.</span><span>
</span><span style="color: #008000">// An optional with the value of `x` is created.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> z: </span><span style="color: #0000FF">AnyStruct</span><span style="color: #000000">? = x</span><span>
</span></pre></code>

A non-optional type is a subtype of its optional type.

<code><pre><span style="color: #0000FF">var</span><span style="color: #000000"> a: </span><span style="color: #0000FF">Int</span><span style="color: #000000">? = </span><span style="color: #0000FF">nil</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> b = </span><span style="color: #09885A">2</span><span>
</span><span style="color: #000000">a = b</span><span>
</span><span>
</span><span style="color: #008000">// `a` is `2`</span><span>
</span></pre></code>

Optional types may be contained in other types, for example [arrays](#arrays) or even optionals.

<code><pre><span style="color: #008000">// Declare a constant which has an array type of optional integers.</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> xs: [</span><span style="color: #0000FF">Int</span><span style="color: #000000">?] = [</span><span style="color: #09885A">1</span><span style="color: #000000">, </span><span style="color: #0000FF">nil</span><span style="color: #000000">, </span><span style="color: #09885A">2</span><span style="color: #000000">, </span><span style="color: #0000FF">nil</span><span style="color: #000000">]</span><span>
</span><span>
</span><span style="color: #008000">// Declare a constant which has a double optional type.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> doubleOptional: </span><span style="color: #0000FF">Int</span><span style="color: #000000">?? = </span><span style="color: #0000FF">nil</span><span>
</span></pre></code>

#### [](#nil-coalescing-operator)Nil-Coalescing Operator

The nil-coalescing operator `??` returns
the value inside an optional if it contains a value,
or returns an alternative value if the optional has no value,
i.e., the optional value is `nil`.

If the left-hand side is non-nil, the right-hand side is not evaluated.

<code><pre><span style="color: #008000">// Declare a constant which has an optional integer type</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> a: </span><span style="color: #0000FF">Int</span><span style="color: #000000">? = </span><span style="color: #0000FF">nil</span><span>
</span><span>
</span><span style="color: #008000">// Declare a constant with a non-optional integer type,</span><span>
</span><span style="color: #008000">// which is initialized to `a` if it is non-nil, or 42 otherwise.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> b: </span><span style="color: #0000FF">Int</span><span style="color: #000000"> = a ?? </span><span style="color: #09885A">42</span><span>
</span><span style="color: #008000">// `b` is 42, as `a` is nil</span><span>
</span></pre></code>

The nil-coalescing operator can only be applied
to values which have an optional type.

<code><pre><span style="color: #008000">// Declare a constant with a non-optional integer type.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> a = </span><span style="color: #09885A">1</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: nil-coalescing operator is applied to a value which has a non-optional type</span><span>
</span><span style="color: #008000">// (a has the non-optional type `Int`).</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> b = a ?? </span><span style="color: #09885A">2</span><span>
</span></pre></code>

<code><pre><span style="color: #008000">// Invalid: nil-coalescing operator is applied to a value which has a non-optional type</span><span>
</span><span style="color: #008000">// (the integer literal is of type `Int`).</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> c = </span><span style="color: #09885A">1</span><span style="color: #000000"> ?? </span><span style="color: #09885A">2</span><span>
</span></pre></code>

The type of the right-hand side of the operator (the alternative value) must be a subtype
of the type of left-hand side, i.e. the right-hand side of the operator must
be the non-optional or optional type matching the type of the left-hand side.

<code><pre><span style="color: #008000">// Declare a constant with an optional integer type.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> a: </span><span style="color: #0000FF">Int</span><span style="color: #000000">? = </span><span style="color: #0000FF">nil</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> b: </span><span style="color: #0000FF">Int</span><span style="color: #000000">? = </span><span style="color: #09885A">1</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> c = a ?? b</span><span>
</span><span style="color: #008000">// `c` is `1` and has type `Int?`</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: nil-coalescing operator is applied to a value of type `Int?`,</span><span>
</span><span style="color: #008000">// but the alternative has type `Bool`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> d = a ?? </span><span style="color: #0000FF">false</span><span>
</span></pre></code>

#### [](#force-unwrap-)Force Unwrap (`!`)

The force-unwrap operator (`!`) returns
the value inside an optional if it contains a value,
or panics and aborts the execution if the optional has no value,
i.e., the optional value is `nil`.

<code><pre><span style="color: #008000">// Declare a constant which has an optional integer type</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> a: </span><span style="color: #0000FF">Int</span><span style="color: #000000">? = </span><span style="color: #0000FF">nil</span><span>
</span><span>
</span><span style="color: #008000">// Declare a constant with a non-optional integer type,</span><span>
</span><span style="color: #008000">// which is initialized to `a` if `a` is non-nil.</span><span>
</span><span style="color: #008000">// If `a` is nil, the program aborts.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> b: </span><span style="color: #0000FF">Int</span><span style="color: #000000"> = a!</span><span>
</span><span style="color: #008000">// The program aborts because `a` is nil.</span><span>
</span><span>
</span><span style="color: #008000">// Declare another optional integer constant</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> c: </span><span style="color: #0000FF">Int</span><span style="color: #000000">? = </span><span style="color: #09885A">3</span><span>
</span><span>
</span><span style="color: #008000">// Declare a non-optional integer</span><span>
</span><span style="color: #008000">// which is initialized to `c` if `a` is non-nil.</span><span>
</span><span style="color: #008000">// If `c` is nil, the program aborts.</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> d: </span><span style="color: #0000FF">Int</span><span style="color: #000000"> = c!</span><span>
</span><span style="color: #008000">// `d` is initialized to 3 because c isn't nil.</span><span>
</span><span>
</span></pre></code>

The force-unwrap operator can only be applied
to values which have an optional type.

<code><pre><span style="color: #008000">// Declare a constant with a non-optional integer type.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> a = </span><span style="color: #09885A">1</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: force-unwrap operator is applied to a value which has a</span><span>
</span><span style="color: #008000">// non-optional type (`a` has the non-optional type `Int`).</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> b = a!</span><span>
</span></pre></code>

<code><pre><span style="color: #008000">// Invalid: The force-unwrap operator is applied</span><span>
</span><span style="color: #008000">// to a value which has a non-optional type</span><span>
</span><span style="color: #008000">// (the integer literal is of type `Int`).</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> c = </span><span style="color: #09885A">1</span><span style="color: #000000">!</span><span>
</span></pre></code>

#### [](#force-assignment-operator--)Force-assignment operator (`<-!`)

The force-assignment operator (`<-!`) assigns a resource-typed value to an
optional-typed variable if the variable is nil.
If the variable being assigned to is non-nil,
the execution of the program aborts.

The force-assignment operator is only used for
[resource types](#resources) and the move operator (`<-`),
which are covered the resources section of this document.

#### [](#conditional-downcasting-operator)Conditional Downcasting Operator

> 🚧 Status: The conditional downcasting operator `as?` is implemented,
> but it only supports values that have the type `AnyStruct` and `AnyResource`.

The conditional downcasting operator `as?`
can be used to type cast a value to a type.
The operator returns an optional.
If the value has a type that is a subtype
of the given type that should be casted to,
the operator returns the value as the given type,
otherwise the result is `nil`.

The cast and check is performed at run-time, i.e. when the program is executed,
not statically, i.e. when the program is checked.

<code><pre><span style="color: #008000">// Declare a constant named `something` which has type `AnyStruct`,</span><span>
</span><span style="color: #008000">// with an initial value which has type `Int`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> something: </span><span style="color: #0000FF">AnyStruct</span><span style="color: #000000"> = </span><span style="color: #09885A">1</span><span>
</span><span>
</span><span style="color: #008000">// Conditionally downcast the value of `something` to `Int`.</span><span>
</span><span style="color: #008000">// The cast succeeds, because the value has type `Int`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> number = something as? Int</span><span>
</span><span style="color: #008000">// `number` is `1` and has type `Int?`</span><span>
</span><span>
</span><span style="color: #008000">// Conditionally downcast the value of `something` to `Bool`.</span><span>
</span><span style="color: #008000">// The cast fails, because the value has type `Int`,</span><span>
</span><span style="color: #008000">// and `Bool` is not a subtype of `Int`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> boolean = something as? Bool</span><span>
</span><span style="color: #008000">// `boolean` is `nil` and has type `Bool?`</span><span>
</span></pre></code>

Downcasting works for nested types (e.g. arrays),
interfaces (if a [resource](#resources) interface not to a concrete resource),
and optionals.

<code><pre><span style="color: #008000">// Declare a constant named `values` which has type `[AnyStruct]`,</span><span>
</span><span style="color: #008000">// i.e. an array of arbitrarily typed values.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> values: [</span><span style="color: #0000FF">AnyStruct</span><span style="color: #000000">] = [</span><span style="color: #09885A">1</span><span style="color: #000000">, </span><span style="color: #0000FF">true</span><span style="color: #000000">]</span><span>
</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> first = values[</span><span style="color: #09885A">0</span><span style="color: #000000">] as? Int</span><span>
</span><span style="color: #008000">// `first` is `1` and has type `Int?`</span><span>
</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> second = values[</span><span style="color: #09885A">1</span><span style="color: #000000">] as? Bool</span><span>
</span><span style="color: #008000">// `second` is `true` and has type `Bool?`</span><span>
</span></pre></code>

### [](#never)Never

`Never` is the bottom type, i.e., it is a subtype of all types.
There is no value that has type `Never`.
`Never` can be used as the return type for functions that never return normally.
For example, it is the return type of the function [`panic`](#panic).

<code><pre><span style="color: #008000">// Declare a function named `crashAndBurn` which will never return,</span><span>
</span><span style="color: #008000">// because it calls the function named `panic`, which never returns.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">fun</span><span style="color: #000000"> crashAndBurn(): </span><span style="color: #0000FF">Never</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">    panic(</span><span style="color: #A31515">"An unrecoverable error occurred"</span><span style="color: #000000">)</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Declare a constant with a `Never` type, but the initial value is an integer.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> x: </span><span style="color: #0000FF">Never</span><span style="color: #000000"> = </span><span style="color: #09885A">1</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Declare a function which returns an invalid return value `nil`,</span><span>
</span><span style="color: #008000">// which is not a value of type `Never`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">fun</span><span style="color: #000000"> returnNever(): </span><span style="color: #0000FF">Never</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">return</span><span style="color: #000000"> </span><span style="color: #0000FF">nil</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

### [](#strings-and-characters)Strings and Characters

Strings are collections of characters.
Strings have the type `String`, and characters have the type `Character`.
Strings can be used to work with text in a Unicode-compliant way.
Strings are immutable.

String and character literals are enclosed in double quotation marks (`"`).

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> someString = </span><span style="color: #A31515">"Hello, world!"</span><span>
</span></pre></code>

String literals may contain escape sequences. An escape sequence starts with a backslash (`\`):

-   `\0`: Null character
-   `\\`: Backslash
-   `\t`: Horizontal tab
-   `\n`: Line feed
-   `\r`: Carriage return
-   `\"`: Double quotation mark
-   `\'`: Single quotation mark
-   `\u`: A Unicode scalar value, written as `\u{x}`,
    where `x` is a 1–8 digit hexadecimal number
    which needs to be a valid Unicode scalar value,
    i.e., in the range 0 to 0xD7FF and 0xE000 to 0x10FFFF inclusive

<code><pre><span style="color: #008000">// Declare a constant which contains two lines of text</span><span>
</span><span style="color: #008000">// (separated by the line feed character `\n`), and ends</span><span>
</span><span style="color: #008000">// with a thumbs up emoji, which has code point U+1F44D (0x1F44D).</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> thumbsUpText =</span><span>
</span><span style="color: #000000">    </span><span style="color: #A31515">"This is the first line.\nThis is the second line with an emoji: \u{1F44D}"</span><span>
</span></pre></code>

The type `Character` represents a single, human-readable character.
Characters are extended grapheme clusters,
which consist of one or more Unicode scalars.

For example, the single character `ü` can be represented
in several ways in Unicode.
First, it can be represented by a single Unicode scalar value `ü`
(&quot;LATIN SMALL LETTER U WITH DIAERESIS&quot;, code point U+00FC).
Second, the same single character can be represented
by two Unicode scalar values:
`u` (&quot;LATIN SMALL LETTER U&quot;, code point U+0075),
and &quot;COMBINING DIAERESIS&quot; (code point U+0308).
The combining Unicode scalar value is applied to the scalar before it,
which turns a `u` into a `ü`.

Still, both variants represent the same human-readable character `ü`.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> singleScalar: </span><span style="color: #0000FF">Character</span><span style="color: #000000"> = </span><span style="color: #A31515">"\u{FC}"</span><span>
</span><span style="color: #008000">// `singleScalar` is `ü`</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> twoScalars: </span><span style="color: #0000FF">Character</span><span style="color: #000000"> = </span><span style="color: #A31515">"\u{75}\u{308}"</span><span>
</span><span style="color: #008000">// `twoScalars` is `ü`</span><span>
</span></pre></code>

Another example where multiple Unicode scalar values are rendered as a single,
human-readable character is a flag emoji.
These emojis consist of two &quot;REGIONAL INDICATOR SYMBOL LETTER&quot; Unicode scalar values.

<code><pre><span style="color: #008000">// Declare a constant for a string with a single character, the emoji</span><span>
</span><span style="color: #008000">// for the Canadian flag, which consists of two Unicode scalar values:</span><span>
</span><span style="color: #008000">// - REGIONAL INDICATOR SYMBOL LETTER C (U+1F1E8)</span><span>
</span><span style="color: #008000">// - REGIONAL INDICATOR SYMBOL LETTER A (U+1F1E6)</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> canadianFlag: </span><span style="color: #0000FF">Character</span><span style="color: #000000"> = </span><span style="color: #A31515">"\u{1F1E8}\u{1F1E6}"</span><span>
</span><span style="color: #008000">// `canadianFlag` is `🇨🇦`</span><span>
</span></pre></code>

#### [](#string-fields-and-functions)String Fields and Functions

Strings have multiple built-in functions you can use.

-   `let length: Int`: Returns the number of characters in the string as an integer.

    <code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> example = </span><span style="color: #A31515">"hello"</span><span>
    </span><span>
    </span><span style="color: #008000">// Find the number of elements of the string.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> length = example.length</span><span>
    </span><span style="color: #008000">// `length` is `5`</span><span>
    </span></pre></code>

-   `fun concat(_ other: String): String`:
    Concatenates the string `other` to the end of the original string,
    but does not modify the original string.
    This function creates a new string whose length is the sum of the lengths
    of the string the function is called on and the string given as a parameter.

    <code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> example = </span><span style="color: #A31515">"hello"</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> new = </span><span style="color: #A31515">"world"</span><span>
    </span><span>
    </span><span style="color: #008000">// Concatenate the new string onto the example string and return the new string.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> helloWorld = example.concat(new)</span><span>
    </span><span style="color: #008000">// `helloWorld` is now `"helloworld"`</span><span>
    </span></pre></code>

-   `fun slice(from: Int, upTo: Int): String`:
    Returns a string slice of the characters
    in the given string from start index `from` up to,
    but not including, the end index `upTo`.
    This function creates a new string whose length is `upto - from`.
    It does not modify the original string.
    If either of the parameters are out of
    the bounds of the string, the function will fail.

    <code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> example = </span><span style="color: #A31515">"helloworld"</span><span>
    </span><span>
    </span><span style="color: #008000">// Create a new slice of part of the original string.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> slice = example.slice(from: </span><span style="color: #09885A">3</span><span style="color: #000000">, upTo: </span><span style="color: #09885A">6</span><span style="color: #000000">)</span><span>
    </span><span style="color: #008000">// `slice` is now `"lowo"`</span><span>
    </span><span>
    </span><span style="color: #008000">// Run-time error: Out of bounds index, the program aborts.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> outOfBounds = example.slice(from: </span><span style="color: #09885A">2</span><span style="color: #000000">, upTo: </span><span style="color: #09885A">10</span><span style="color: #000000">)</span><span>
    </span></pre></code>

<!--

TODO

#### String Functions

- Document and link to string concatenation operator `&` in operators section

-->

### [](#arrays)Arrays

Arrays are mutable, ordered collections of values.
All values in an array must have the same type.
Arrays may contain a value multiple times.
Array literals start with an opening square bracket `[` and end with a closing square bracket `]`.

<code><pre><span style="color: #008000">// An empty array</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">[]</span><span>
</span><span>
</span><span style="color: #008000">// An array with integers</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">[</span><span style="color: #09885A">1</span><span style="color: #000000">, </span><span style="color: #09885A">2</span><span style="color: #000000">, </span><span style="color: #09885A">3</span><span style="color: #000000">]</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: mixed types</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">[</span><span style="color: #09885A">1</span><span style="color: #000000">, </span><span style="color: #0000FF">true</span><span style="color: #000000">, </span><span style="color: #09885A">2</span><span style="color: #000000">, </span><span style="color: #0000FF">false</span><span style="color: #000000">]</span><span>
</span></pre></code>

#### [](#array-types)Array Types

Arrays either have a fixed size or are variably sized, i.e., elements can be added and removed.

Fixed-size arrays have the form `[T; N]`, where `T` is the element type,
and `N` is the size of the array.  `N` has to be statically known, meaning
that it needs to be an integer literal.
For example, a fixed-size array of 3 `Int8` elements has the type `[Int8; 3]`.

Variable-size arrays have the form `[T]`, where `T` is the element type.
For example, the type `[Int16]` specifies a variable-size array of elements that have type `Int16`.

It is important to understand that arrays are value types and are only ever copied
when used as an initial value for a constant or variable,
when assigning to a variable,
when used as function argument,
or when returned from a function call.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> size = </span><span style="color: #09885A">2</span><span>
</span><span style="color: #008000">// Invalid: Array-size must be an integer literal</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> numbers: [</span><span style="color: #0000FF">Int</span><span style="color: #000000">; </span><span style="color: #0000FF">size</span><span style="color: #000000">] = []</span><span>
</span><span>
</span><span style="color: #008000">// Declare a fixed-sized array of integers</span><span>
</span><span style="color: #008000">// which always contains exactly two elements.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> array: [</span><span style="color: #0000FF">Int8</span><span style="color: #000000">; 2] = [</span><span style="color: #09885A">1</span><span style="color: #000000">, </span><span style="color: #09885A">2</span><span style="color: #000000">]</span><span>
</span><span>
</span><span style="color: #008000">// Declare a fixed-sized array of fixed-sized arrays of integers.</span><span>
</span><span style="color: #008000">// The inner arrays always contain exactly three elements,</span><span>
</span><span style="color: #008000">// the outer array always contains two elements.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> arrays: [[</span><span style="color: #0000FF">Int16</span><span style="color: #000000">; 3]; 2] = [</span><span>
</span><span style="color: #000000">    [</span><span style="color: #09885A">1</span><span style="color: #000000">, </span><span style="color: #09885A">2</span><span style="color: #000000">, </span><span style="color: #09885A">3</span><span style="color: #000000">],</span><span>
</span><span style="color: #000000">    [</span><span style="color: #09885A">4</span><span style="color: #000000">, </span><span style="color: #09885A">5</span><span style="color: #000000">, </span><span style="color: #09885A">6</span><span style="color: #000000">]</span><span>
</span><span style="color: #000000">]</span><span>
</span><span>
</span><span style="color: #008000">// Declare a variable length array of integers</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> variableLengthArray: [</span><span style="color: #0000FF">Int</span><span style="color: #000000">] = []</span><span>
</span></pre></code>

Array types are covariant in their element types.
For example, `[Int]` is a subtype of `[AnyStruct]`.
This is safe because arrays are value types and not reference types.

#### [](#array-indexing)Array Indexing

To get the element of an array at a specific index, the indexing syntax can be used:
The array is followed by an opening square bracket `[`, the indexing value,
and ends with a closing square bracket `]`.

Indexes start at 0 for the first element in the array.

Accessing an element which is out of bounds results in a fatal error at run-time
and aborts the program.

<code><pre><span style="color: #008000">// Declare an array of integers.</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> numbers = [</span><span style="color: #09885A">42</span><span style="color: #000000">, </span><span style="color: #09885A">23</span><span style="color: #000000">]</span><span>
</span><span>
</span><span style="color: #008000">// Get the first number of the array.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">numbers[</span><span style="color: #09885A">0</span><span style="color: #000000">] </span><span style="color: #008000">// is `42`</span><span>
</span><span>
</span><span style="color: #008000">// Get the second number of the array.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">numbers[</span><span style="color: #09885A">1</span><span style="color: #000000">] </span><span style="color: #008000">// is `23`</span><span>
</span><span>
</span><span style="color: #008000">// Run-time error: Index 2 is out of bounds, the program aborts.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">numbers[</span><span style="color: #09885A">2</span><span style="color: #000000">]</span><span>
</span></pre></code>

<code><pre><span style="color: #008000">// Declare an array of arrays of integers, i.e. the type is `[[Int]]`.</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> arrays = [[</span><span style="color: #09885A">1</span><span style="color: #000000">, </span><span style="color: #09885A">2</span><span style="color: #000000">], [</span><span style="color: #09885A">3</span><span style="color: #000000">, </span><span style="color: #09885A">4</span><span style="color: #000000">]]</span><span>
</span><span>
</span><span style="color: #008000">// Get the first number of the second array.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">arrays[</span><span style="color: #09885A">1</span><span style="color: #000000">][</span><span style="color: #09885A">0</span><span style="color: #000000">] </span><span style="color: #008000">// is `3`</span><span>
</span></pre></code>

To set an element of an array at a specific index, the indexing syntax can be used as well.

<code><pre><span style="color: #008000">// Declare an array of integers.</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> numbers = [</span><span style="color: #09885A">42</span><span style="color: #000000">, </span><span style="color: #09885A">23</span><span style="color: #000000">]</span><span>
</span><span>
</span><span style="color: #008000">// Change the second number in the array.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// NOTE: The declaration `numbers` is constant, which means that</span><span>
</span><span style="color: #008000">// the *name* is constant, not the *value* – the value, i.e. the array,</span><span>
</span><span style="color: #008000">// is mutable and can be changed.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">numbers[</span><span style="color: #09885A">1</span><span style="color: #000000">] = </span><span style="color: #09885A">2</span><span>
</span><span>
</span><span style="color: #008000">// `numbers` is `[42, 2]`</span><span>
</span></pre></code>

#### [](#array-fields-and-functions)Array Fields and Functions

Arrays have multiple built-in fields and functions
that can be used to get information about and manipulate the contents of the array.

The field `length`, and the functions `concat`, and `contains`
are available for both variable-sized and fixed-sized or variable-sized arrays.

-   `let length: Int`:
    Returns the number of elements in the array.

    <code><pre><span style="color: #008000">// Declare an array of integers.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> numbers = [</span><span style="color: #09885A">42</span><span style="color: #000000">, </span><span style="color: #09885A">23</span><span style="color: #000000">, </span><span style="color: #09885A">31</span><span style="color: #000000">, </span><span style="color: #09885A">12</span><span style="color: #000000">]</span><span>
    </span><span>
    </span><span style="color: #008000">// Find the number of elements of the array.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> length = numbers.length</span><span>
    </span><span>
    </span><span style="color: #008000">// `length` is `4`</span><span>
    </span></pre></code>

-   `fun concat(_ array: T): T`:
    Concatenates the parameter `array` to the end
    of the array the function is called on,
    but does not modify that array.

    Both arrays must be the same type `T`.

    This function creates a new array whose length is
    the sum of the length of the array
    the function is called on and the length of the array given as the parameter.

    <code><pre><span style="color: #008000">// Declare two arrays of integers.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> numbers = [</span><span style="color: #09885A">42</span><span style="color: #000000">, </span><span style="color: #09885A">23</span><span style="color: #000000">, </span><span style="color: #09885A">31</span><span style="color: #000000">, </span><span style="color: #09885A">12</span><span style="color: #000000">]</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> moreNumbers = [</span><span style="color: #09885A">11</span><span style="color: #000000">, </span><span style="color: #09885A">27</span><span style="color: #000000">]</span><span>
    </span><span>
    </span><span style="color: #008000">// Concatenate the array `moreNumbers` to the array `numbers`</span><span>
    </span><span style="color: #008000">// and declare a new variable for the result.</span><span>
    </span><span style="color: #008000">//</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> allNumbers = numbers.concat(moreNumbers)</span><span>
    </span><span>
    </span><span style="color: #008000">// `allNumbers` is `[42, 23, 31, 12, 11, 27]`</span><span>
    </span><span style="color: #008000">// `numbers` is still `[42, 23, 31, 12]`</span><span>
    </span><span style="color: #008000">// `moreNumbers` is still `[11, 27]`</span><span>
    </span></pre></code>

-   `fun contains(_ element: T): Bool`:
    Indicates whether the given element of type `T` is in the array.

    <code><pre><span style="color: #008000">// Declare an array of integers.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> numbers = [</span><span style="color: #09885A">42</span><span style="color: #000000">, </span><span style="color: #09885A">23</span><span style="color: #000000">, </span><span style="color: #09885A">31</span><span style="color: #000000">, </span><span style="color: #09885A">12</span><span style="color: #000000">]</span><span>
    </span><span>
    </span><span style="color: #008000">// Check if the array contains 11.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> containsEleven = numbers.contains(</span><span style="color: #09885A">11</span><span style="color: #000000">)</span><span>
    </span><span style="color: #008000">// `containsEleven` is `false`</span><span>
    </span><span>
    </span><span style="color: #008000">// Check if the array contains 12.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> containsTwelve = numbers.contains(</span><span style="color: #09885A">12</span><span style="color: #000000">)</span><span>
    </span><span style="color: #008000">// `containsTwelve` is `true`</span><span>
    </span><span>
    </span><span style="color: #008000">// Invalid: Check if the array contains the string "Kitty".</span><span>
    </span><span style="color: #008000">// This results in a type error, as the array only contains integers.</span><span>
    </span><span style="color: #008000">//</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> containsKitty = numbers.contains(</span><span style="color: #A31515">"Kitty"</span><span style="color: #000000">)</span><span>
    </span></pre></code>

##### [](#variable-size-array-functions)Variable-size Array Functions

The following functions can only be used on variable-sized arrays.
It is invalid to use one of these functions on a fixed-sized array.

-   `fun append(_ element: T): Void`:
    Adds the new element `element` of type `T` to the end of the array.

    The new element must be the same type as all the other elements in the array.

    <code><pre><span style="color: #008000">// Declare an array of integers.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> numbers = [</span><span style="color: #09885A">42</span><span style="color: #000000">, </span><span style="color: #09885A">23</span><span style="color: #000000">, </span><span style="color: #09885A">31</span><span style="color: #000000">, </span><span style="color: #09885A">12</span><span style="color: #000000">]</span><span>
    </span><span>
    </span><span style="color: #008000">// Add a new element to the array.</span><span>
    </span><span style="color: #000000">numbers.append(</span><span style="color: #09885A">20</span><span style="color: #000000">)</span><span>
    </span><span style="color: #008000">// `numbers` is now `[42, 23, 31, 12, 20]`</span><span>
    </span><span>
    </span><span style="color: #008000">// Invalid: The parameter has the wrong type `String`.</span><span>
    </span><span style="color: #000000">numbers.append(</span><span style="color: #A31515">"SneakyString"</span><span style="color: #000000">)</span><span>
    </span></pre></code>

-   `fun insert(at index: Int, _ element: T): Void`:
    Inserts the new element `element` of type `T`
    at the given `index` of the array.

    The new element must be of the same type as the other elements in the array.

    The `index` must be within the bounds of the array.
    If the index is outside the bounds, the program aborts.

    The existing element at the supplied index is not overwritten.

    All the elements after the new inserted element
    are shifted to the right by one.

    <code><pre><span style="color: #008000">// Declare an array of integers.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> numbers = [</span><span style="color: #09885A">42</span><span style="color: #000000">, </span><span style="color: #09885A">23</span><span style="color: #000000">, </span><span style="color: #09885A">31</span><span style="color: #000000">, </span><span style="color: #09885A">12</span><span style="color: #000000">]</span><span>
    </span><span>
    </span><span style="color: #008000">// Insert a new element at position 1 of the array.</span><span>
    </span><span style="color: #000000">numbers.insert(at: </span><span style="color: #09885A">1</span><span style="color: #000000">, </span><span style="color: #09885A">20</span><span style="color: #000000">)</span><span>
    </span><span style="color: #008000">// `numbers` is now `[42, 20, 23, 31, 12]`</span><span>
    </span><span>
    </span><span style="color: #008000">// Run-time error: Out of bounds index, the program aborts.</span><span>
    </span><span style="color: #000000">numbers.insert(at: </span><span style="color: #09885A">12</span><span style="color: #000000">, </span><span style="color: #09885A">39</span><span style="color: #000000">)</span><span>
    </span></pre></code>

-   `fun remove(at index: Int): T`:
    Removes the element at the given `index` from the array and returns it.

    The `index` must be within the bounds of the array.
    If the index is outside the bounds, the program aborts.

    <code><pre><span style="color: #008000">// Declare an array of integers.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> numbers = [</span><span style="color: #09885A">42</span><span style="color: #000000">, </span><span style="color: #09885A">23</span><span style="color: #000000">, </span><span style="color: #09885A">31</span><span style="color: #000000">]</span><span>
    </span><span>
    </span><span style="color: #008000">// Remove element at position 1 of the array.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> twentyThree = numbers.remove(at: </span><span style="color: #09885A">1</span><span style="color: #000000">)</span><span>
    </span><span style="color: #008000">// `numbers` is now `[42, 31]`</span><span>
    </span><span style="color: #008000">// `twentyThree` is `23`</span><span>
    </span><span>
    </span><span style="color: #008000">// Run-time error: Out of bounds index, the program aborts.</span><span>
    </span><span style="color: #000000">numbers.remove(at: </span><span style="color: #09885A">19</span><span style="color: #000000">)</span><span>
    </span></pre></code>

-   `fun removeFirst(): T`:
    Removes the first element from the array and returns it.

    The array must not be empty.
    If the array is empty, the program aborts.

    <code><pre><span style="color: #008000">// Declare an array of integers.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> numbers = [</span><span style="color: #09885A">42</span><span style="color: #000000">, </span><span style="color: #09885A">23</span><span style="color: #000000">]</span><span>
    </span><span>
    </span><span style="color: #008000">// Remove the first element of the array.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> fortytwo = numbers.removeFirst()</span><span>
    </span><span style="color: #008000">// `numbers` is now `[23]`</span><span>
    </span><span style="color: #008000">// `fortywo` is `42`</span><span>
    </span><span>
    </span><span style="color: #008000">// Remove the first element of the array.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> twentyThree = numbers.removeFirst()</span><span>
    </span><span style="color: #008000">// `numbers` is now `[]`</span><span>
    </span><span style="color: #008000">// `twentyThree` is `23`</span><span>
    </span><span>
    </span><span style="color: #008000">// Run-time error: The array is empty, the program aborts.</span><span>
    </span><span style="color: #000000">numbers.removeFirst()</span><span>
    </span></pre></code>

-   `fun removeLast(): T`:
    Removes the last element from the array and returns it.

    The array must not be empty.
    If the array is empty, the program aborts.

    <code><pre><span style="color: #008000">// Declare an array of integers.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> numbers = [</span><span style="color: #09885A">42</span><span style="color: #000000">, </span><span style="color: #09885A">23</span><span style="color: #000000">]</span><span>
    </span><span>
    </span><span style="color: #008000">// Remove the last element of the array.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> twentyThree = numbers.removeLast()</span><span>
    </span><span style="color: #008000">// `numbers` is now `[42]`</span><span>
    </span><span style="color: #008000">// `twentyThree` is `23`</span><span>
    </span><span>
    </span><span style="color: #008000">// Remove the last element of the array.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> fortyTwo = numbers.removeLast()</span><span>
    </span><span style="color: #008000">// `numbers` is now `[]`</span><span>
    </span><span style="color: #008000">// `fortyTwo` is `42`</span><span>
    </span><span>
    </span><span style="color: #008000">// Run-time error: The array is empty, the program aborts.</span><span>
    </span><span style="color: #000000">numbers.removeLast()</span><span>
    </span></pre></code>

<!--

TODO

- filter, etc. for all array types
- Document and link to array concatenation operator `&` in operators section

-->

### [](#dictionaries)Dictionaries

Dictionaries are mutable, unordered collections of key-value associations.
In a dictionary, all keys must have the same type,
and all values must have the same type.
Dictionaries may contain a key only once and
may contain a value multiple times.

Dictionary literals start with an opening brace `{`
and end with a closing brace `}`.
Keys are separated from values by a colon,
and key-value associations are separated by commas.

<code><pre><span style="color: #008000">// An empty dictionary</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">{}</span><span>
</span><span>
</span><span style="color: #008000">// A dictionary which associates integers with booleans</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">{</span><span>
</span><span style="color: #000000">    </span><span style="color: #09885A">1</span><span style="color: #000000">: </span><span style="color: #0000FF">true</span><span style="color: #000000">,</span><span>
</span><span style="color: #000000">    </span><span style="color: #09885A">2</span><span style="color: #000000">: </span><span style="color: #0000FF">false</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: mixed types</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">{</span><span>
</span><span style="color: #000000">    </span><span style="color: #09885A">1</span><span style="color: #000000">: </span><span style="color: #0000FF">true</span><span style="color: #000000">,</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">false</span><span style="color: #000000">: </span><span style="color: #09885A">2</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

#### [](#dictionary-types)Dictionary Types

Dictionaries have the form `{K: V}`,
where `K` is the type of the key,
and `V` is the type of the value.
For example, a dictionary with `Int` keys and `Bool`
values has type `{Int: Bool}`.

<code><pre><span style="color: #008000">// Declare a constant that has type `{Int: Bool}`,</span><span>
</span><span style="color: #008000">// a dictionary mapping integers to booleans.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> booleans = {</span><span>
</span><span style="color: #000000">    </span><span style="color: #09885A">1</span><span style="color: #000000">: </span><span style="color: #0000FF">true</span><span style="color: #000000">,</span><span>
</span><span style="color: #000000">    </span><span style="color: #09885A">0</span><span style="color: #000000">: </span><span style="color: #0000FF">false</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Declare a constant that has type `{Bool: Int}`,</span><span>
</span><span style="color: #008000">// a dictionary mapping booleans to integers.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> integers = {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">true</span><span style="color: #000000">: </span><span style="color: #09885A">1</span><span style="color: #000000">,</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">false</span><span style="color: #000000">: </span><span style="color: #09885A">0</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

Dictionary types are covariant in their key and value types.
For example, `[Int: String]` is a subtype of `[AnyStruct: String]`
and also a subtype of `[Int: AnyStruct]`.
This is safe because dictionaries are value types and not reference types.

#### [](#dictionary-access)Dictionary Access

To get the value for a specific key from a dictionary,
the access syntax can be used:
The dictionary is followed by an opening square bracket `[`, the key,
and ends with a closing square bracket `]`.

Accessing a key returns an [optional](#optionals):
If the key is found in the dictionary, the value for the given key is returned,
and if the key is not found, `nil` is returned.

<code><pre><span style="color: #008000">// Declare a constant that has type `{Bool: Int}`,</span><span>
</span><span style="color: #008000">// a dictionary mapping integers to booleans.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> booleans = {</span><span>
</span><span style="color: #000000">    </span><span style="color: #09885A">1</span><span style="color: #000000">: </span><span style="color: #0000FF">true</span><span style="color: #000000">,</span><span>
</span><span style="color: #000000">    </span><span style="color: #09885A">0</span><span style="color: #000000">: </span><span style="color: #0000FF">false</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// The result of accessing a key has type `Bool?`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">booleans[</span><span style="color: #09885A">1</span><span style="color: #000000">]  </span><span style="color: #008000">// is `true`</span><span>
</span><span style="color: #000000">booleans[</span><span style="color: #09885A">0</span><span style="color: #000000">]  </span><span style="color: #008000">// is `false`</span><span>
</span><span style="color: #000000">booleans[</span><span style="color: #09885A">2</span><span style="color: #000000">]  </span><span style="color: #008000">// is `nil`</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Accessing a key which does not have type `Int`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">booleans[</span><span style="color: #A31515">"1"</span><span style="color: #000000">]</span><span>
</span></pre></code>

<code><pre><span style="color: #008000">// Declare a constant that has type `{Bool: Int}`,</span><span>
</span><span style="color: #008000">// a dictionary mapping booleans to integers.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> integers = {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">true</span><span style="color: #000000">: </span><span style="color: #09885A">1</span><span style="color: #000000">,</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">false</span><span style="color: #000000">: </span><span style="color: #09885A">0</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// The result of accessing a key has type `Int?`</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">integers[</span><span style="color: #0000FF">true</span><span style="color: #000000">] </span><span style="color: #008000">// is `1`</span><span>
</span><span style="color: #000000">integers[</span><span style="color: #0000FF">false</span><span style="color: #000000">] </span><span style="color: #008000">// is `0`</span><span>
</span></pre></code>

To set the value for a key of a dictionary,
the access syntax can be used as well.

<code><pre><span style="color: #008000">// Declare a constant that has type `{Int: Bool}`,</span><span>
</span><span style="color: #008000">// a dictionary mapping booleans to integers.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> booleans = {</span><span>
</span><span style="color: #000000">    </span><span style="color: #09885A">1</span><span style="color: #000000">: </span><span style="color: #0000FF">true</span><span style="color: #000000">,</span><span>
</span><span style="color: #000000">    </span><span style="color: #09885A">0</span><span style="color: #000000">: </span><span style="color: #0000FF">false</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Assign new values for the keys `1` and `0`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">booleans[</span><span style="color: #09885A">1</span><span style="color: #000000">] = </span><span style="color: #0000FF">false</span><span>
</span><span style="color: #000000">booleans[</span><span style="color: #09885A">0</span><span style="color: #000000">] = </span><span style="color: #0000FF">true</span><span>
</span><span style="color: #008000">// `booleans` is `{1: false, 0: true}`</span><span>
</span></pre></code>

#### [](#dictionary-fields-and-functions)Dictionary Fields and Functions

-   `fun length: Int`:
    Returns the number of entries in the dictionary.

    <code><pre><span style="color: #008000">// Declare a dictionary mapping strings to integers.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> numbers = {</span><span style="color: #A31515">"fortyTwo"</span><span style="color: #000000">: </span><span style="color: #09885A">42</span><span style="color: #000000">, </span><span style="color: #A31515">"twentyThree"</span><span style="color: #000000">: </span><span style="color: #09885A">23</span><span style="color: #000000">}</span><span>
    </span><span>
    </span><span style="color: #008000">// Find the number of entries of the dictionary.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> length = numbers.length</span><span>
    </span><span>
    </span><span style="color: #008000">// `length` is `2`</span><span>
    </span></pre></code>

-   `fun remove(key: K): V?`:
    Removes the value for the given `key` of type `K` from the dictionary.

    Returns the value of type `V` as an optional
    if the dictionary contained the key,
    otherwise `nil`.

    <code><pre><span style="color: #008000">// Declare a dictionary mapping strings to integers.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> numbers = {</span><span style="color: #A31515">"fortyTwo"</span><span style="color: #000000">: </span><span style="color: #09885A">42</span><span style="color: #000000">, </span><span style="color: #A31515">"twentyThree"</span><span style="color: #000000">: </span><span style="color: #09885A">23</span><span style="color: #000000">}</span><span>
    </span><span>
    </span><span style="color: #008000">// Remove the key `"fortyTwo"` from the dictionary.</span><span>
    </span><span style="color: #008000">// The key exists in the dictionary,</span><span>
    </span><span style="color: #008000">// so the value associated with the key is returned.</span><span>
    </span><span style="color: #008000">//</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> fortyTwo = numbers.remove(key: </span><span style="color: #A31515">"fortyTwo"</span><span style="color: #000000">)</span><span>
    </span><span>
    </span><span style="color: #008000">// `fortyTwo` is `42`</span><span>
    </span><span style="color: #008000">// `numbers` is `{"twentyThree": 23}`</span><span>
    </span><span>
    </span><span style="color: #008000">// Remove the key `"oneHundred"` from the dictionary.</span><span>
    </span><span style="color: #008000">// The key does not exist in the dictionary, so `nil` is returned.</span><span>
    </span><span style="color: #008000">//</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> oneHundred = numbers.remove(key: </span><span style="color: #A31515">"oneHundred"</span><span style="color: #000000">)</span><span>
    </span><span>
    </span><span style="color: #008000">// `oneHundred` is `nil`</span><span>
    </span><span style="color: #008000">// `numbers` is `{"twentyThree": 23}`</span><span>
    </span></pre></code>

-   `let keys: [K]`:
    Returns an array of the keys of type `K` in the dictionary.  This does not
    modify the dictionary, just returns a copy of the keys as an array.
    If the dictionary is empty, this returns an empty array.

    <code><pre><span style="color: #008000">// Declare a dictionary mapping strings to integers.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> numbers = {</span><span style="color: #A31515">"fortyTwo"</span><span style="color: #000000">: </span><span style="color: #09885A">42</span><span style="color: #000000">, </span><span style="color: #A31515">"twentyThree"</span><span style="color: #000000">: </span><span style="color: #09885A">23</span><span style="color: #000000">}</span><span>
    </span><span>
    </span><span style="color: #008000">// Find the keys of the dictionary.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> keys = numbers.keys</span><span>
    </span><span>
    </span><span style="color: #008000">// `keys` has type `[String]` and is `["fortyTwo","twentyThree"]`</span><span>
    </span></pre></code>

-   `let values: [V]`:
    Returns an array of the values of type `V` in the dictionary.  This does not
    modify the dictionary, just returns a copy of the values as an array.
    If the dictionary is empty, this returns an empty array.

    This field is not available if `V` is a resource type.

    <code><pre><span style="color: #008000">// Declare a dictionary mapping strings to integers.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> numbers = {</span><span style="color: #A31515">"fortyTwo"</span><span style="color: #000000">: </span><span style="color: #09885A">42</span><span style="color: #000000">, </span><span style="color: #A31515">"twentyThree"</span><span style="color: #000000">: </span><span style="color: #09885A">23</span><span style="color: #000000">}</span><span>
    </span><span>
    </span><span style="color: #008000">// Find the values of the dictionary.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> values = numbers.values</span><span>
    </span><span>
    </span><span style="color: #008000">// `values` has type [Int] and is `[42, 23]`</span><span>
    </span></pre></code>

#### [](#dictionary-keys)Dictionary Keys

Dictionary keys must be hashable and equatable,
i.e., must implement the [`Hashable`](#hashable-interface)
and [`Equatable`](#equatable-interface) [interfaces](#interfaces).

Most of the built-in types, like booleans and integers,
are hashable and equatable, so can be used as keys in dictionaries.

## [](#operators)Operators

Operators are special symbols that perform a computation
for one or more values.
They are either unary, binary, or ternary.

-   Unary operators perform an operation for a single value.
    The unary operator symbol appears before the value.

-   Binary operators operate on two values.
      The binary operator symbol appears between the two values (infix).

-   Ternary operators operate on three values.
    The first operator symbol appears between the first and second value,
    the second operator symbol appears between the second and third value (infix).

### [](#negation)Negation

The `-` unary operator negates an integer:

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> a = </span><span style="color: #09885A">1</span><span>
</span><span style="color: #000000">-a  </span><span style="color: #008000">// is `-1`</span><span>
</span></pre></code>

The `!` unary operator logically negates a boolean:

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> a = </span><span style="color: #0000FF">true</span><span>
</span><span style="color: #000000">!a  </span><span style="color: #008000">// is `false`</span><span>
</span></pre></code>

### [](#assignment)Assignment

The binary assignment operator `=` can be used
to assign a new value to a variable.
It is only allowed in a statement and is not allowed in expressions.

<code><pre><span style="color: #0000FF">var</span><span style="color: #000000"> a = </span><span style="color: #09885A">1</span><span>
</span><span style="color: #000000">a = </span><span style="color: #09885A">2</span><span>
</span><span style="color: #008000">// `a` is `2`</span><span>
</span><span>
</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> b = </span><span style="color: #09885A">3</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> c = </span><span style="color: #09885A">4</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: The assignment operation cannot be used in an expression.</span><span>
</span><span style="color: #000000">a = b = c</span><span>
</span><span>
</span><span style="color: #008000">// Instead, the intended assignment must be written in multiple statements.</span><span>
</span><span style="color: #000000">b = c</span><span>
</span><span style="color: #000000">a = b</span><span>
</span></pre></code>

Assignments to constants are invalid.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> a = </span><span style="color: #09885A">1</span><span>
</span><span style="color: #008000">// Invalid: Assignments are only for variables, not constants.</span><span>
</span><span style="color: #000000">a = </span><span style="color: #09885A">2</span><span>
</span></pre></code>

The left-hand side of the assignment operand must be an identifier.
For arrays and dictionaries, this identifier can be followed
by one or more index or access expressions.

<code><pre><span style="color: #008000">// Declare an array of integers.</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> numbers = [</span><span style="color: #09885A">1</span><span style="color: #000000">, </span><span style="color: #09885A">2</span><span style="color: #000000">]</span><span>
</span><span>
</span><span style="color: #008000">// Change the first element of the array.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">numbers[</span><span style="color: #09885A">0</span><span style="color: #000000">] = </span><span style="color: #09885A">3</span><span>
</span><span>
</span><span style="color: #008000">// `numbers` is `[3, 2]`</span><span>
</span></pre></code>

<code><pre><span style="color: #008000">// Declare an array of arrays of integers.</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> arrays = [[</span><span style="color: #09885A">1</span><span style="color: #000000">, </span><span style="color: #09885A">2</span><span style="color: #000000">], [</span><span style="color: #09885A">3</span><span style="color: #000000">, </span><span style="color: #09885A">4</span><span style="color: #000000">]]</span><span>
</span><span>
</span><span style="color: #008000">// Change the first element in the second array</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">arrays[</span><span style="color: #09885A">1</span><span style="color: #000000">][</span><span style="color: #09885A">0</span><span style="color: #000000">] = </span><span style="color: #09885A">5</span><span>
</span><span>
</span><span style="color: #008000">// `arrays` is `[[1, 2], [5, 4]]`</span><span>
</span></pre></code>

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> dictionaries = {</span><span>
</span><span style="color: #000000">  </span><span style="color: #0000FF">true</span><span style="color: #000000">: {</span><span style="color: #09885A">1</span><span style="color: #000000">: </span><span style="color: #09885A">2</span><span style="color: #000000">},</span><span>
</span><span style="color: #000000">  </span><span style="color: #0000FF">false</span><span style="color: #000000">: {</span><span style="color: #09885A">3</span><span style="color: #000000">: </span><span style="color: #09885A">4</span><span style="color: #000000">}</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #000000">dictionaries[</span><span style="color: #0000FF">false</span><span style="color: #000000">][</span><span style="color: #09885A">3</span><span style="color: #000000">] = </span><span style="color: #09885A">0</span><span>
</span><span>
</span><span style="color: #008000">// `dictionaries` is `{</span><span>
</span><span style="color: #008000">//   true: {1: 2},</span><span>
</span><span style="color: #008000">//   false: {3: 0}</span><span>
</span><span style="color: #008000">//}`</span><span>
</span></pre></code>

### [](#swapping)Swapping

The binary swap operator `<->` can be used
to exchange the values of two variables.
It is only allowed in a statement and is not allowed in expressions.

<code><pre><span style="color: #0000FF">var</span><span style="color: #000000"> a = </span><span style="color: #09885A">1</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> b = </span><span style="color: #09885A">2</span><span>
</span><span style="color: #000000">a &#x3C;-> b</span><span>
</span><span style="color: #008000">// `a` is `2`</span><span>
</span><span style="color: #008000">// `b` is `1`</span><span>
</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> c = </span><span style="color: #09885A">3</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: The swap operation cannot be used in an expression.</span><span>
</span><span style="color: #000000">a &#x3C;-> b &#x3C;-> c</span><span>
</span><span>
</span><span style="color: #008000">// Instead, the intended swap must be written in multiple statements.</span><span>
</span><span style="color: #000000">b &#x3C;-> c</span><span>
</span><span style="color: #000000">a &#x3C;-> b</span><span>
</span></pre></code>

Both sides of the swap operation must be variable,
assignment to constants is invalid.

<code><pre><span style="color: #0000FF">var</span><span style="color: #000000"> a = </span><span style="color: #09885A">1</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> b = </span><span style="color: #09885A">2</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Swapping is only possible for variables, not constants.</span><span>
</span><span style="color: #000000">a &#x3C;-> b</span><span>
</span></pre></code>

Both sides of the swap operation must be an identifier,
followed by one or more index or access expressions.

### [](#arithmetic)Arithmetic

There are four arithmetic operators:

-   Addition: `+`
-   Subtraction: `-`
-   Multiplication: `*`
-   Division: `/`
-   Remainder: `%`

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> a = </span><span style="color: #09885A">1</span><span style="color: #000000"> + </span><span style="color: #09885A">2</span><span>
</span><span style="color: #008000">// `a` is `3`</span><span>
</span></pre></code>

The arguments for the operators need to be of the same type.
The result is always the same type as the arguments.

The division and remainder operators abort the program when the divisor is zero.

Arithmetic operations on the signed integer types
`Int8`, `Int16`, `Int32`, `Int64`, `Int128`, `Int256`,
and on the unsigned integer types
`UInt8`, `UInt16`, `UInt32`, `UInt64`, `UInt128`, `UInt256`,
do not cause values to overflow or underflow.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> a: </span><span style="color: #0000FF">UInt8</span><span style="color: #000000"> = </span><span style="color: #09885A">255</span><span>
</span><span>
</span><span style="color: #008000">// Run-time error: The result `256` does not fit in the range of `UInt8`,</span><span>
</span><span style="color: #008000">// thus a fatal overflow error is raised and the program aborts</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> b = a + </span><span style="color: #09885A">1</span><span>
</span></pre></code>

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> a: </span><span style="color: #0000FF">Int8</span><span style="color: #000000"> = </span><span style="color: #09885A">100</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> b: </span><span style="color: #0000FF">Int8</span><span style="color: #000000"> = </span><span style="color: #09885A">100</span><span>
</span><span>
</span><span style="color: #008000">// Run-time error: The result `10000` does not fit in the range of `Int8`,</span><span>
</span><span style="color: #008000">// thus a fatal overflow error is raised and the program aborts</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> c = a * b</span><span>
</span></pre></code>

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> a: </span><span style="color: #0000FF">Int8</span><span style="color: #000000"> = </span><span style="color: #09885A">-128</span><span>
</span><span>
</span><span style="color: #008000">// Run-time error: The result `128` does not fit in the range of `Int8`,</span><span>
</span><span style="color: #008000">// thus a fatal overflow error is raised and the program aborts</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> b = -a</span><span>
</span></pre></code>

Arithmetic operations on the unsigned integer types
`Word8`, `Word16`, `Word32`, `Word64`
may cause values to overflow or underflow.

For example, the maximum value of an unsigned 8-bit integer is 255 (binary 11111111).
Adding 1 results in an overflow, truncation to 8 bits, and the value 0.

<code><pre><span style="color: #008000">//    11111111 = 255</span><span>
</span><span style="color: #008000">// +         1</span><span>
</span><span style="color: #008000">// = 100000000 = 0</span><span>
</span></pre></code>

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> a: </span><span style="color: #0000FF">Word8</span><span style="color: #000000"> = </span><span style="color: #09885A">255</span><span>
</span><span style="color: #000000">a + </span><span style="color: #09885A">1</span><span style="color: #000000"> </span><span style="color: #008000">// is `0`</span><span>
</span></pre></code>

Similarly, for the minimum value 0,
subtracting 1 wraps around and results in the maximum value 255.

<code><pre><span style="color: #008000">//    00000000</span><span>
</span><span style="color: #008000">// -         1</span><span>
</span><span style="color: #008000">// =  11111111 = 255</span><span>
</span></pre></code>

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> b: </span><span style="color: #0000FF">Word8</span><span style="color: #000000"> = </span><span style="color: #09885A">0</span><span>
</span><span style="color: #000000">b - </span><span style="color: #09885A">1</span><span style="color: #000000">  </span><span style="color: #008000">// is `255`</span><span>
</span></pre></code>

### [](#logical-operators)Logical Operators

Logical operators work with the boolean values `true` and `false`.

-   Logical AND: `a && b`

    <code><pre><span style="color: #0000FF">true</span><span style="color: #000000"> &#x26;&#x26; </span><span style="color: #0000FF">true</span><span style="color: #000000">  </span><span style="color: #008000">// is `true`</span><span>
    </span><span>
    </span><span style="color: #0000FF">true</span><span style="color: #000000"> &#x26;&#x26; </span><span style="color: #0000FF">false</span><span style="color: #000000">  </span><span style="color: #008000">// is `false`</span><span>
    </span><span>
    </span><span style="color: #0000FF">false</span><span style="color: #000000"> &#x26;&#x26; </span><span style="color: #0000FF">true</span><span style="color: #000000">  </span><span style="color: #008000">// is `false`</span><span>
    </span><span>
    </span><span style="color: #0000FF">false</span><span style="color: #000000"> &#x26;&#x26; </span><span style="color: #0000FF">false</span><span style="color: #000000">  </span><span style="color: #008000">// is `false`</span><span>
    </span></pre></code>

      If the left-hand side is false, the right-hand side is not evaluated.

-   Logical OR: `a || b`

    <code><pre><span style="color: #0000FF">true</span><span style="color: #000000"> || </span><span style="color: #0000FF">true</span><span style="color: #000000">  </span><span style="color: #008000">// is `true`</span><span>
    </span><span>
    </span><span style="color: #0000FF">true</span><span style="color: #000000"> || </span><span style="color: #0000FF">false</span><span style="color: #000000">  </span><span style="color: #008000">// is `true`</span><span>
    </span><span>
    </span><span style="color: #0000FF">false</span><span style="color: #000000"> || </span><span style="color: #0000FF">true</span><span style="color: #000000">  </span><span style="color: #008000">// is `true`</span><span>
    </span><span>
    </span><span style="color: #0000FF">false</span><span style="color: #000000"> || </span><span style="color: #0000FF">false</span><span style="color: #000000"> </span><span style="color: #008000">// is `false`</span><span>
    </span></pre></code>

      If the left-hand side is true, the right-hand side is not evaluated.

### [](#comparison-operators)Comparison operators

Comparison operators work with boolean and integer values.

-   Equality: `==`, for booleans and integers

      Both sides of the equality operator may be optional, even of different levels,
      so it is for example possible to compare a non-optional with a double-optional (`??`).

    <code><pre><span style="color: #09885A">1</span><span style="color: #000000"> == </span><span style="color: #09885A">1</span><span style="color: #000000">  </span><span style="color: #008000">// is `true`</span><span>
    </span><span>
    </span><span style="color: #09885A">1</span><span style="color: #000000"> == </span><span style="color: #09885A">2</span><span style="color: #000000">  </span><span style="color: #008000">// is `false`</span><span>
    </span></pre></code>

    <code><pre><span style="color: #0000FF">true</span><span style="color: #000000"> == </span><span style="color: #0000FF">true</span><span style="color: #000000">  </span><span style="color: #008000">// is `true`</span><span>
    </span><span>
    </span><span style="color: #0000FF">true</span><span style="color: #000000"> == </span><span style="color: #0000FF">false</span><span style="color: #000000">  </span><span style="color: #008000">// is `false`</span><span>
    </span></pre></code>

    <code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> x: </span><span style="color: #0000FF">Int</span><span style="color: #000000">? = </span><span style="color: #09885A">1</span><span>
    </span><span style="color: #000000">x == </span><span style="color: #0000FF">nil</span><span style="color: #000000">  </span><span style="color: #008000">// is `false`</span><span>
    </span></pre></code>

    <code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> x: </span><span style="color: #0000FF">Int</span><span style="color: #000000"> = </span><span style="color: #09885A">1</span><span>
    </span><span style="color: #000000">x == </span><span style="color: #0000FF">nil</span><span style="color: #000000">  </span><span style="color: #008000">// is `false`</span><span>
    </span></pre></code>

    <code><pre><span style="color: #008000">// Comparisons of different levels of optionals are possible.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> x: </span><span style="color: #0000FF">Int</span><span style="color: #000000">? = </span><span style="color: #09885A">2</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> y: </span><span style="color: #0000FF">Int</span><span style="color: #000000">?? = </span><span style="color: #0000FF">nil</span><span>
    </span><span style="color: #000000">x == y  </span><span style="color: #008000">// is `false`</span><span>
    </span></pre></code>

    <code><pre><span style="color: #008000">// Comparisons of different levels of optionals are possible.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> x: </span><span style="color: #0000FF">Int</span><span style="color: #000000">? = </span><span style="color: #09885A">2</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> y: </span><span style="color: #0000FF">Int</span><span style="color: #000000">?? = </span><span style="color: #09885A">2</span><span>
    </span><span style="color: #000000">x == y  </span><span style="color: #008000">// is `true`</span><span>
    </span></pre></code>

-   Inequality: `!=`, for booleans and integers (possibly optional)

      Both sides of the inequality operator may be optional, even of different levels,
      so it is for example possible to compare a non-optional with a double-optional (`??`).

    <code><pre><span style="color: #09885A">1</span><span style="color: #000000"> != </span><span style="color: #09885A">1</span><span style="color: #000000">  </span><span style="color: #008000">// is `false`</span><span>
    </span><span>
    </span><span style="color: #09885A">1</span><span style="color: #000000"> != </span><span style="color: #09885A">2</span><span style="color: #000000">  </span><span style="color: #008000">// is `true`</span><span>
    </span></pre></code>

    <code><pre><span style="color: #0000FF">true</span><span style="color: #000000"> != </span><span style="color: #0000FF">true</span><span style="color: #000000">  </span><span style="color: #008000">// is `false`</span><span>
    </span><span>
    </span><span style="color: #0000FF">true</span><span style="color: #000000"> != </span><span style="color: #0000FF">false</span><span style="color: #000000">  </span><span style="color: #008000">// is `true`</span><span>
    </span></pre></code>

    <code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> x: </span><span style="color: #0000FF">Int</span><span style="color: #000000">? = </span><span style="color: #09885A">1</span><span>
    </span><span style="color: #000000">x != </span><span style="color: #0000FF">nil</span><span style="color: #000000">  </span><span style="color: #008000">// is `true`</span><span>
    </span></pre></code>

    <code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> x: </span><span style="color: #0000FF">Int</span><span style="color: #000000"> = </span><span style="color: #09885A">1</span><span>
    </span><span style="color: #000000">x != </span><span style="color: #0000FF">nil</span><span style="color: #000000">  </span><span style="color: #008000">// is `true`</span><span>
    </span></pre></code>

    <code><pre><span style="color: #008000">// Comparisons of different levels of optionals are possible.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> x: </span><span style="color: #0000FF">Int</span><span style="color: #000000">? = </span><span style="color: #09885A">2</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> y: </span><span style="color: #0000FF">Int</span><span style="color: #000000">?? = </span><span style="color: #0000FF">nil</span><span>
    </span><span style="color: #000000">x != y  </span><span style="color: #008000">// is `true`</span><span>
    </span></pre></code>

    <code><pre><span style="color: #008000">// Comparisons of different levels of optionals are possible.</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> x: </span><span style="color: #0000FF">Int</span><span style="color: #000000">? = </span><span style="color: #09885A">2</span><span>
    </span><span style="color: #0000FF">let</span><span style="color: #000000"> y: </span><span style="color: #0000FF">Int</span><span style="color: #000000">?? = </span><span style="color: #09885A">2</span><span>
    </span><span style="color: #000000">x != y  </span><span style="color: #008000">// is `false`</span><span>
    </span></pre></code>

-   Less than: `<`, for integers

    <code><pre><span style="color: #09885A">1</span><span style="color: #000000"> &#x3C; </span><span style="color: #09885A">1</span><span style="color: #000000">  </span><span style="color: #008000">// is `false`</span><span>
    </span><span>
    </span><span style="color: #09885A">1</span><span style="color: #000000"> &#x3C; </span><span style="color: #09885A">2</span><span style="color: #000000">  </span><span style="color: #008000">// is `true`</span><span>
    </span><span>
    </span><span style="color: #09885A">2</span><span style="color: #000000"> &#x3C; </span><span style="color: #09885A">1</span><span style="color: #000000">  </span><span style="color: #008000">// is `false`</span><span>
    </span></pre></code>

-   Less or equal than: `<=`, for integers

    <code><pre><span style="color: #09885A">1</span><span style="color: #000000"> &#x3C;= </span><span style="color: #09885A">1</span><span style="color: #000000">  </span><span style="color: #008000">// is `true`</span><span>
    </span><span>
    </span><span style="color: #09885A">1</span><span style="color: #000000"> &#x3C;= </span><span style="color: #09885A">2</span><span style="color: #000000">  </span><span style="color: #008000">// is `true`</span><span>
    </span><span>
    </span><span style="color: #09885A">2</span><span style="color: #000000"> &#x3C;= </span><span style="color: #09885A">1</span><span style="color: #000000">  </span><span style="color: #008000">// is `false`</span><span>
    </span></pre></code>

-   Greater than: `>`, for integers

    <code><pre><span style="color: #09885A">1</span><span style="color: #000000"> > </span><span style="color: #09885A">1</span><span style="color: #000000">  </span><span style="color: #008000">// is `false`</span><span>
    </span><span>
    </span><span style="color: #09885A">1</span><span style="color: #000000"> > </span><span style="color: #09885A">2</span><span style="color: #000000">  </span><span style="color: #008000">// is `false`</span><span>
    </span><span>
    </span><span style="color: #09885A">2</span><span style="color: #000000"> > </span><span style="color: #09885A">1</span><span style="color: #000000">  </span><span style="color: #008000">// is `true`</span><span>
    </span></pre></code>

-   Greater or equal than: `>=`, for integers

    <code><pre><span style="color: #09885A">1</span><span style="color: #000000"> >= </span><span style="color: #09885A">1</span><span style="color: #000000">  </span><span style="color: #008000">// is `true`</span><span>
    </span><span>
    </span><span style="color: #09885A">1</span><span style="color: #000000"> >= </span><span style="color: #09885A">2</span><span style="color: #000000">  </span><span style="color: #008000">// is `false`</span><span>
    </span><span>
    </span><span style="color: #09885A">2</span><span style="color: #000000"> >= </span><span style="color: #09885A">1</span><span style="color: #000000">  </span><span style="color: #008000">// is `true`</span><span>
    </span></pre></code>

### [](#ternary-conditional-operator)Ternary Conditional Operator

There is only one ternary conditional operator, the ternary conditional operator (`a ? b : c`).

It behaves like an if-statement, but is an expression:
If the first operator value is true, the second operator value is returned.
If the first operator value is false, the third value is returned.

The first value must be a boolean (must have the type `Bool`).
The second value and third value can be of any type.
The result type is the least common supertype of the second and third value.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> x = </span><span style="color: #09885A">1</span><span style="color: #000000"> > </span><span style="color: #09885A">2</span><span style="color: #000000"> ? </span><span style="color: #09885A">3</span><span style="color: #000000"> : </span><span style="color: #09885A">4</span><span>
</span><span style="color: #008000">// `x` is `4` and has type `Int`</span><span>
</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> y = </span><span style="color: #09885A">1</span><span style="color: #000000"> > </span><span style="color: #09885A">2</span><span style="color: #000000"> ? </span><span style="color: #0000FF">nil</span><span style="color: #000000"> : </span><span style="color: #09885A">3</span><span>
</span><span style="color: #008000">// `y` is `3` and has type `Int?`</span><span>
</span></pre></code>

### [](#precedence-and-associativity)Precedence and Associativity

Operators have the following precedences, highest to lowest:

-   Multiplication precedence: `*`, `&*`, `/`, `%`
-   Addition precedence: `+`, `&+`, `-`, `&-`
-   Relational precedence: `<`, `<=`, `>`, `>=`
-   Equality precedence: `==`, `!=`
-   Logical conjunction precedence: `&&`
-   Logical disjunction precedence: `||`
-   Ternary precedence: `? :`

All operators are left-associative, except for the ternary operator, which is right-associative.

Expressions can be wrapped in parentheses to override precedence conventions,
i.e. an alternate order should be indicated, or when the default order should be emphasized
e.g. to avoid confusion.
For example, `(2 + 3) * 4` forces addition to precede multiplication,
and `5 + (6 * 7)` reinforces the default order.

## [](#functions)Functions

Functions are sequences of statements that perform a specific task.
Functions have parameters (inputs) and an optional return value (output).
Functions are typed: the function type consists of the parameter types and the return type.

Functions are values, i.e., they can be assigned to constants and variables,
and can be passed as arguments to other functions.
This behavior is often called &quot;first-class functions&quot;.

### [](#function-declarations)Function Declarations

Functions can be declared by using the `fun` keyword, followed by the name of the declaration,
 the parameters, the optional return type,
 and the code that should be executed when the function is called.

The parameters need to be enclosed in parentheses.
The return type, if any, is separated from the parameters by a colon (`:`).
The function code needs to be enclosed in opening and closing braces.

Each parameter must have a name, which is the name that the argument value
will be available as within the function.

An additional argument label can be provided to require function calls to use the label
to provide an argument value for the parameter.

Argument labels make code more explicit and readable.
For example, they avoid confusion about the order of arguments
when there are multiple arguments that have the same type.

Argument labels should be named so they make sense from the perspective of the function call.

Argument labels precede the parameter name.
The special argument label `_` indicates
that a function call can omit the argument label.
If no argument label is declared in the function declaration,
the parameter name is the argument label of the function declaration,
and function calls must use the parameter name as the argument label.

Each parameter needs to have a type annotation,
which follows the parameter name after a colon.

Function calls may provide arguments for parameters
which are subtypes of the parameter types.

There is **no** support for optional parameters,
i.e. default values for parameters,
and variadic functions,
i.e. functions that take an arbitrary amount of arguments.

<code><pre><span style="color: #008000">// Declare a function named `double`, which multiples a number by two.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// The special argument label _ is specified for the parameter,</span><span>
</span><span style="color: #008000">// so no argument label has to be provided in a function call.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">fun</span><span style="color: #000000"> double(_ x: </span><span style="color: #0000FF">Int</span><span style="color: #000000">): </span><span style="color: #0000FF">Int</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">return</span><span style="color: #000000"> x * </span><span style="color: #09885A">2</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Call the function named `double` with the value 4 for the first parameter.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// The argument label can be omitted in the function call as the declaration</span><span>
</span><span style="color: #008000">// specifies the special argument label _ for the parameter.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">double(</span><span style="color: #09885A">2</span><span style="color: #000000">)  </span><span style="color: #008000">// is `4`</span><span>
</span></pre></code>

It is possible to require argument labels for some parameters,
and not require argument labels for other parameters.

<code><pre><span style="color: #008000">// Declare a function named `clamp`. The function takes an integer value,</span><span>
</span><span style="color: #008000">// the lower limit, and the upper limit. It returns an integer between</span><span>
</span><span style="color: #008000">// the lower and upper limit.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// For the first parameter the special argument label _ is used,</span><span>
</span><span style="color: #008000">// so no argument label has to be given for it in a function call.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// For the second and third parameter no argument label is given,</span><span>
</span><span style="color: #008000">// so the parameter names are the argument labels, i.e., the parameter names</span><span>
</span><span style="color: #008000">// have to be given as argument labels in a function call.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">fun</span><span style="color: #000000"> clamp(_ value: </span><span style="color: #0000FF">Int</span><span style="color: #000000">, min: </span><span style="color: #0000FF">Int</span><span style="color: #000000">, max: </span><span style="color: #0000FF">Int</span><span style="color: #000000">): </span><span style="color: #0000FF">Int</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">if</span><span style="color: #000000"> value > max {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">return</span><span style="color: #000000"> max</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">if</span><span style="color: #000000"> value &#x3C; min {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">return</span><span style="color: #000000"> min</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">return</span><span style="color: #000000"> value</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Declare a constant which has the result of a call to the function</span><span>
</span><span style="color: #008000">// named `clamp` as its initial value.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// For the first argument no label is given, as it is not required by</span><span>
</span><span style="color: #008000">// the function declaration (the special argument label `_` is specified).</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// For the second and this argument the labels must be provided,</span><span>
</span><span style="color: #008000">// as the function declaration does not specify the special argument label `_`</span><span>
</span><span style="color: #008000">// for these two parameters.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// As the function declaration also does not specify argument labels</span><span>
</span><span style="color: #008000">// for these parameters, the parameter names must be used as argument labels.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> clamped = clamp(</span><span style="color: #09885A">123</span><span style="color: #000000">, min: </span><span style="color: #09885A">0</span><span style="color: #000000">, max: </span><span style="color: #09885A">100</span><span style="color: #000000">)</span><span>
</span><span style="color: #008000">// `clamped` is `100`</span><span>
</span></pre></code>

<code><pre><span style="color: #008000">// Declare a function named `send`, which transfers an amount</span><span>
</span><span style="color: #008000">// from one account to another.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// The implementation is omitted for brevity.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// The first two parameters of the function have the same type, so there is</span><span>
</span><span style="color: #008000">// a potential that a function call accidentally provides arguments in</span><span>
</span><span style="color: #008000">// the wrong order.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// While the parameter names `senderAddress` and `receiverAddress`</span><span>
</span><span style="color: #008000">// are descriptive inside the function, they might be too verbose</span><span>
</span><span style="color: #008000">// to require them as argument labels in function calls.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// For this reason the shorter argument labels `from` and `to` are specified,</span><span>
</span><span style="color: #008000">// which still convey the meaning of the two parameters without being overly</span><span>
</span><span style="color: #008000">// verbose.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// The name of the third parameter, `amount`, is both meaningful inside</span><span>
</span><span style="color: #008000">// the function and also in a function call, so no argument label is given,</span><span>
</span><span style="color: #008000">// and the parameter name is required as the argument label in a function call.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">fun</span><span style="color: #000000"> send(from senderAddress: </span><span style="color: #0000FF">Address</span><span style="color: #000000">, to receivingAddress: </span><span style="color: #0000FF">Address</span><span style="color: #000000">, amount: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// The function code is omitted for brevity.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// ...</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Declare a constant which refers to the sending account's address.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// The initial value is omitted for brevity.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> sender: </span><span style="color: #0000FF">Address</span><span style="color: #000000"> = </span><span style="color: #008000">// ...</span><span>
</span><span>
</span><span style="color: #008000">// Declare a constant which refers to the receiving account's address.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// The initial value is omitted for brevity.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> receiver: </span><span style="color: #0000FF">Address</span><span style="color: #000000"> = </span><span style="color: #008000">// ...</span><span>
</span><span>
</span><span style="color: #008000">// Call the function named `send`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// The function declaration requires argument labels for all parameters,</span><span>
</span><span style="color: #008000">// so they need to be provided in the function call.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// This avoids ambiguity. For example, in some languages (like C) it is</span><span>
</span><span style="color: #008000">// a convention to order the parameters so that the receiver occurs first,</span><span>
</span><span style="color: #008000">// followed by the sender. In other languages, it is common to have</span><span>
</span><span style="color: #008000">// the sender be the first parameter, followed by the receiver.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// Here, the order is clear – send an amount from an account to another account.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">send(from: sender, to: receiver, amount: </span><span style="color: #09885A">100</span><span style="color: #000000">)</span><span>
</span></pre></code>

The order of the arguments in a function call must
match the order of the parameters in the function declaration.

<code><pre><span style="color: #008000">// Declare a function named `test`, which accepts two parameters, named `first` and `second`</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">fun</span><span style="color: #000000"> test(first: </span><span style="color: #0000FF">Int</span><span style="color: #000000">, second: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// ...</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: the arguments are provided in the wrong order,</span><span>
</span><span style="color: #008000">// even though the argument labels are provided correctly.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">test(second: </span><span style="color: #09885A">1</span><span style="color: #000000">, first: </span><span style="color: #09885A">2</span><span style="color: #000000">)</span><span>
</span></pre></code>

Functions can be nested,
i.e., the code of a function may declare further functions.

<code><pre><span style="color: #008000">// Declare a function which multiplies a number by two, and adds one.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">fun</span><span style="color: #000000"> doubleAndAddOne(_ x: </span><span style="color: #0000FF">Int</span><span style="color: #000000">): </span><span style="color: #0000FF">Int</span><span style="color: #000000"> {</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Declare a nested function which multiplies a number by two.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">fun</span><span style="color: #000000"> double(_ x: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">return</span><span style="color: #000000"> x * </span><span style="color: #09885A">2</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">return</span><span style="color: #000000"> double(x) + </span><span style="color: #09885A">1</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #000000">doubleAndAddOne(</span><span style="color: #09885A">2</span><span style="color: #000000">)  </span><span style="color: #008000">// is `5`</span><span>
</span></pre></code>

### [](#function-overloading)Function overloading

> 🚧 Status: Function overloading is not implemented.

It is possible to declare functions with the same name,
as long as they have different sets of argument labels.
This is known as function overloading.

<code><pre><span style="color: #008000">// Declare a function named "assert" which requires a test value</span><span>
</span><span style="color: #008000">// and a message argument.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">fun</span><span style="color: #000000"> assert(_ test: </span><span style="color: #0000FF">Bool</span><span style="color: #000000">, message: </span><span style="color: #0000FF">String</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// ...</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Declare a function named "assert" which only requires a test value.</span><span>
</span><span style="color: #008000">// The function calls the `assert` function declared above.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">fun</span><span style="color: #000000"> assert(_ test: </span><span style="color: #0000FF">Bool</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">    assert(test, message: </span><span style="color: #A31515">"test is false"</span><span style="color: #000000">)</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

### [](#function-expressions)Function Expressions

Functions can be also used as expressions.
The syntax is the same as for function declarations,
except that function expressions have no name, i.e., they are anonymous.

<code><pre><span style="color: #008000">// Declare a constant named `double`, which has a function as its value.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// The function multiplies a number by two when it is called.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// This function's type is `((Int): Int)`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> double =</span><span>
</span><span style="color: #000000">    fun (_ x: Int): Int {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">return</span><span style="color: #000000"> x * </span><span style="color: #09885A">2</span><span>
</span><span style="color: #000000">    }</span><span>
</span></pre></code>

### [](#function-calls)Function Calls

Functions can be called (invoked). Function calls
need to provide exactly as many argument values as the function has parameters.

<code><pre><span style="color: #0000FF">fun</span><span style="color: #000000"> double(_ x: </span><span style="color: #0000FF">Int</span><span style="color: #000000">): </span><span style="color: #0000FF">Int</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">return</span><span style="color: #000000"> x * </span><span style="color: #09885A">2</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Valid: the correct amount of arguments is provided.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">double(</span><span style="color: #09885A">2</span><span style="color: #000000">)  </span><span style="color: #008000">// is `4`</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: too many arguments are provided.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">double(</span><span style="color: #09885A">2</span><span style="color: #000000">, </span><span style="color: #09885A">3</span><span style="color: #000000">)</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: too few arguments are provided.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">double()</span><span>
</span></pre></code>

### [](#function-types)Function Types

Function types consist of the function&#x27;s parameter types
and the function&#x27;s return type.

The parameter types need to be enclosed in parentheses,
followed by a colon (`:`), and end with the return type.
The whole function type needs to be enclosed in parentheses.

<code><pre><span style="color: #008000">// Declare a function named `add`, with the function type `((Int, Int): Int)`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">fun</span><span style="color: #000000"> add(a: </span><span style="color: #0000FF">Int</span><span style="color: #000000">, b: </span><span style="color: #0000FF">Int</span><span style="color: #000000">): </span><span style="color: #0000FF">Int</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">return</span><span style="color: #000000"> a + b</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

<code><pre><span style="color: #008000">// Declare a constant named `add`, with the function type `((Int, Int): Int)`</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> add: ((</span><span style="color: #0000FF">Int</span><span style="color: #000000">, </span><span style="color: #0000FF">Int</span><span style="color: #000000">): </span><span style="color: #0000FF">Int</span><span style="color: #000000">) =</span><span>
</span><span style="color: #000000">    fun (a: Int, b: Int): Int {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">return</span><span style="color: #000000"> a + b</span><span>
</span><span style="color: #000000">    }</span><span>
</span></pre></code>

If the function has no return type, it implicitly has the return type `Void`.

<code><pre><span style="color: #008000">// Declare a constant named `doNothing`, which is a function</span><span>
</span><span style="color: #008000">// that takes no parameters and returns nothing.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> doNothing: ((): </span><span style="color: #0000FF">Void</span><span style="color: #000000">) =</span><span>
</span><span style="color: #000000">    fun () {}</span><span>
</span></pre></code>

Parentheses also control precedence.
For example, a function type `((Int): ((): Int))` is the type
for a function which accepts one argument with type `Int`,
and which returns another function,
that takes no arguments and returns an `Int`.

The type `[((Int): Int); 2]` specifies an array type of two functions,
which accept one integer and return one integer.

Argument labels are not part of the function type.
This has the advantage that functions with different argument labels,
potentially written by different authors are compatible
as long as the parameter types and the return type match.
It has the disadvantage that function calls to plain function values,
cannot accept argument labels.

<code><pre><span style="color: #008000">// Declare a function which takes one argument that has type `Int`.</span><span>
</span><span style="color: #008000">// The function has type `((Int): Void)`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">fun</span><span style="color: #000000"> foo1(x: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {}</span><span>
</span><span>
</span><span style="color: #008000">// Call function `foo1`. This requires an argument label.</span><span>
</span><span style="color: #000000">foo1(x: </span><span style="color: #09885A">1</span><span style="color: #000000">)</span><span>
</span><span>
</span><span style="color: #008000">// Declare another function which takes one argument that has type `Int`.</span><span>
</span><span style="color: #008000">// The function also has type `((Int): Void)`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">fun</span><span style="color: #000000"> foo2(y: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {}</span><span>
</span><span>
</span><span style="color: #008000">// Call function `foo2`. This requires an argument label.</span><span>
</span><span style="color: #000000">foo2(y: </span><span style="color: #09885A">2</span><span style="color: #000000">)</span><span>
</span><span>
</span><span style="color: #008000">// Declare a variable which has type `((Int): Void)` and use `foo1`</span><span>
</span><span style="color: #008000">// as its initial value.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> someFoo: ((</span><span style="color: #0000FF">Int</span><span style="color: #000000">): </span><span style="color: #0000FF">Void</span><span style="color: #000000">) = foo1</span><span>
</span><span>
</span><span style="color: #008000">// Call the function assigned to variable `someFoo`.</span><span>
</span><span style="color: #008000">// This is valid as the function types match.</span><span>
</span><span style="color: #008000">// This does neither require nor allow argument labels.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">someFoo(</span><span style="color: #09885A">3</span><span style="color: #000000">)</span><span>
</span><span>
</span><span style="color: #008000">// Assign function `foo2` to variable `someFoo`.</span><span>
</span><span style="color: #008000">// This is valid as the function types match.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">someFoo = foo2</span><span>
</span><span>
</span><span style="color: #008000">// Call the function assigned to variable `someFoo`.</span><span>
</span><span style="color: #008000">// This does neither require nor allow argument labels.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">someFoo(</span><span style="color: #09885A">4</span><span style="color: #000000">)</span><span>
</span></pre></code>

### [](#closures)Closures

A function may refer to variables and constants of its outer scopes
in which it is defined.
It is called a closure, because
it is closing over those variables and constants.
A closure can can read from the variables and constants
and assign to the variables it refers to.

<code><pre><span style="color: #008000">// Declare a function named `makeCounter` which returns a function that</span><span>
</span><span style="color: #008000">// each time when called, returns the next integer, starting at 1.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">fun</span><span style="color: #000000"> makeCounter(): ((): </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">var</span><span style="color: #000000"> count = </span><span style="color: #09885A">0</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">return</span><span style="color: #000000"> fun (): Int {</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">// NOTE: read from and assign to the non-local variable</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">// `count`, which is declared in the outer function.</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">        count = count + </span><span style="color: #09885A">1</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">return</span><span style="color: #000000"> count</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> test = makeCounter()</span><span>
</span><span style="color: #000000">test()  </span><span style="color: #008000">// is `1`</span><span>
</span><span style="color: #000000">test()  </span><span style="color: #008000">// is `2`</span><span>
</span></pre></code>

### [](#argument-passing-behavior)Argument Passing Behavior

When arguments are passed to a function, they are copied.
Therefore, values that are passed into a function
are unchanged in the caller&#x27;s scope when the function returns.
This behavior is known as
[call-by-value](https://en.wikipedia.org/w/index.php?title=Evaluation_strategy&amp;oldid=896280571#Call_by_value).

<code><pre><span style="color: #008000">// Declare a function that changes the first two elements</span><span>
</span><span style="color: #008000">// of an array of integers.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">fun</span><span style="color: #000000"> change(_ numbers: [</span><span style="color: #0000FF">Int</span><span style="color: #000000">]) {</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Change the elements of the passed in array.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// The changes are only local, as the array was copied.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    numbers[</span><span style="color: #09885A">0</span><span style="color: #000000">] = </span><span style="color: #09885A">1</span><span>
</span><span style="color: #000000">    numbers[</span><span style="color: #09885A">1</span><span style="color: #000000">] = </span><span style="color: #09885A">2</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// `numbers` is `[1, 2]`</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> numbers = [</span><span style="color: #09885A">0</span><span style="color: #000000">, </span><span style="color: #09885A">1</span><span style="color: #000000">]</span><span>
</span><span>
</span><span style="color: #000000">change(numbers)</span><span>
</span><span style="color: #008000">// `numbers` is still `[0, 1]`</span><span>
</span></pre></code>

Parameters are constant, i.e., it is not allowed to assign to them.

<code><pre><span style="color: #0000FF">fun</span><span style="color: #000000"> test(x: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Invalid: cannot assign to a parameter (constant)</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    x = </span><span style="color: #09885A">2</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

### [](#function-preconditions-and-postconditions)Function Preconditions and Postconditions

Functions may have preconditions and may have postconditions.
Preconditions and postconditions can be used to restrict the inputs (values for parameters)
and output (return value) of a function.

Preconditions must be true right before the execution of the function.
Preconditions are part of the function and introduced by the `pre` keyword,
followed by the condition block.

Postconditions must be true right after the execution of the function.
Postconditions are part of the function and introduced by the `post` keyword,
followed by the condition block.
Postconditions may only occur after preconditions, if any.

A conditions block consists of one or more conditions.
Conditions are expressions evaluating to a boolean.
They may not call functions, i.e., they cannot have side-effects and must be pure expressions.
Also, conditions may not contain function expressions.

<!--

TODO:

For now, function calls are not allowed in preconditions and postconditions.
See https://github.com/dapperlabs/flow-go/issues/70

-->

Conditions may be written on separate lines,
or multiple conditions can be written on the same line,
separated by a semicolon.
This syntax follows the syntax for [statements](#semicolons).

Following each condition, an optional description can be provided after a colon.
The condition description is used as an error message when the condition fails.

In postconditions, the special constant `result` refers to the result of the function.

<code><pre><span style="color: #0000FF">fun</span><span style="color: #000000"> factorial(_ n: </span><span style="color: #0000FF">Int</span><span style="color: #000000">): </span><span style="color: #0000FF">Int</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pre</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">// Require the parameter `n` to be greater than or equal to zero.</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">        n >= </span><span style="color: #09885A">0</span><span style="color: #000000">:</span><span>
</span><span style="color: #000000">            </span><span style="color: #A31515">"factorial is only defined for integers greater than or equal to zero"</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">post</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">// Ensure the result will be greater than or equal to 1.</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">        result >= </span><span style="color: #09885A">1</span><span style="color: #000000">:</span><span>
</span><span style="color: #000000">            </span><span style="color: #A31515">"the result must be greater than or equal to 1"</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">if</span><span style="color: #000000"> n &#x3C; </span><span style="color: #09885A">1</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">       </span><span style="color: #0000FF">return</span><span style="color: #000000"> </span><span style="color: #09885A">1</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">return</span><span style="color: #000000"> n * factorial(n - </span><span style="color: #09885A">1</span><span style="color: #000000">)</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #000000">factorial(</span><span style="color: #09885A">5</span><span style="color: #000000">)  </span><span style="color: #008000">// is `120`</span><span>
</span><span>
</span><span style="color: #008000">// Run-time error: The given argument does not satisfy</span><span>
</span><span style="color: #008000">// the precondition `n >= 0` of the function, the program aborts.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">factorial(</span><span style="color: #09885A">-2</span><span style="color: #000000">)</span><span>
</span></pre></code>

In postconditions, the special function `before` can be used
to get the value of an expression just before the function is called.

<code><pre><span style="color: #0000FF">var</span><span style="color: #000000"> n = </span><span style="color: #09885A">0</span><span>
</span><span>
</span><span style="color: #0000FF">fun</span><span style="color: #000000"> incrementN() {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">post</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">// Require the new value of `n` to be the old value of `n`, plus one.</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">        n == before(n) + </span><span style="color: #09885A">1</span><span style="color: #000000">:</span><span>
</span><span style="color: #000000">            </span><span style="color: #A31515">"n must be incremented by 1"</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    n = n + </span><span style="color: #09885A">1</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

## [](#control-flow)Control flow

Control flow statements control the flow of execution in a function.

### [](#conditional-branching-if-statement)Conditional branching: if-statement

If-statements allow a certain piece of code to be executed only when a given condition is true.

The if-statement starts with the `if` keyword, followed by the condition,
and the code that should be executed if the condition is true
inside opening and closing braces.
The condition expression must be Bool
The braces are required and not optional.
Parentheses around the condition are optional.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> a = </span><span style="color: #09885A">0</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> b = </span><span style="color: #09885A">0</span><span>
</span><span>
</span><span style="color: #0000FF">if</span><span style="color: #000000"> a == </span><span style="color: #09885A">0</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">   b = </span><span style="color: #09885A">1</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Parentheses can be used around the condition, but are not required.</span><span>
</span><span style="color: #000000">if (a != </span><span style="color: #09885A">0</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">   b = </span><span style="color: #09885A">2</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// `b` is `1`</span><span>
</span></pre></code>

An additional, optional else-clause can be added to execute another piece of code
when the condition is false.
The else-clause is introduced by the `else` keyword followed by braces
that contain the code that should be executed.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> a = </span><span style="color: #09885A">0</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> b = </span><span style="color: #09885A">0</span><span>
</span><span>
</span><span style="color: #0000FF">if</span><span style="color: #000000"> a == </span><span style="color: #09885A">1</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">   b = </span><span style="color: #09885A">1</span><span>
</span><span style="color: #000000">} </span><span style="color: #0000FF">else</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">   b = </span><span style="color: #09885A">2</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// `b` is `2`</span><span>
</span></pre></code>

The else-clause can contain another if-statement, i.e., if-statements can be chained together.
In this case the braces can be omitted.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> a = </span><span style="color: #09885A">0</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> b = </span><span style="color: #09885A">0</span><span>
</span><span>
</span><span style="color: #0000FF">if</span><span style="color: #000000"> a == </span><span style="color: #09885A">1</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">   b = </span><span style="color: #09885A">1</span><span>
</span><span style="color: #000000">} </span><span style="color: #0000FF">else</span><span style="color: #000000"> </span><span style="color: #0000FF">if</span><span style="color: #000000"> a == </span><span style="color: #09885A">2</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">   b = </span><span style="color: #09885A">2</span><span>
</span><span style="color: #000000">} </span><span style="color: #0000FF">else</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">   b = </span><span style="color: #09885A">3</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// `b` is `3`</span><span>
</span><span>
</span><span style="color: #0000FF">if</span><span style="color: #000000"> a == </span><span style="color: #09885A">1</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">   b = </span><span style="color: #09885A">1</span><span>
</span><span style="color: #000000">} </span><span style="color: #0000FF">else</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">if</span><span style="color: #000000"> a == </span><span style="color: #09885A">0</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">        b = </span><span style="color: #09885A">2</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// `b` is `2`</span><span>
</span></pre></code>

### [](#optional-binding)Optional Binding

Optional binding allows getting the value inside an optional.
It is a variant of the if-statement.

If the optional contains a value, the first branch is executed
and a temporary constant or variable is declared and set to the value contained in the optional;
otherwise, the else branch (if any) is executed.

Optional bindings are declared using the `if` keyword like an if-statement,
but instead of the boolean test value, it is followed by the `let` or `var` keywords,
to either introduce a constant or variable, followed by a name,
the equal sign (`=`), and the optional value.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> maybeNumber: </span><span style="color: #0000FF">Int</span><span style="color: #000000">? = </span><span style="color: #09885A">1</span><span>
</span><span>
</span><span style="color: #0000FF">if</span><span style="color: #000000"> </span><span style="color: #0000FF">let</span><span style="color: #000000"> number = maybeNumber {</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// This branch is executed as `maybeNumber` is not `nil`.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// The constant `number` is `1` and has type `Int`.</span><span>
</span><span style="color: #000000">} </span><span style="color: #0000FF">else</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// This branch is *not* executed as `maybeNumber` is not `nil`</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> noNumber: </span><span style="color: #0000FF">Int</span><span style="color: #000000">? = </span><span style="color: #0000FF">nil</span><span>
</span><span>
</span><span style="color: #0000FF">if</span><span style="color: #000000"> </span><span style="color: #0000FF">let</span><span style="color: #000000"> number = noNumber {</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// This branch is *not* executed as `noNumber` is `nil`.</span><span>
</span><span style="color: #000000">} </span><span style="color: #0000FF">else</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// This branch is executed as `noNumber` is `nil`.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// The constant `number` is *not* available.</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

### [](#looping)Looping

#### [](#while-statement)while-statement

While-statements allow a certain piece of code to be executed repeatedly,
as long as a condition remains true.

The while-statement starts with the `while` keyword, followed by the condition,
and the code that should be repeatedly
executed if the condition is true inside opening and closing braces.
The condition must be boolean and the braces are required.

The while-statement will first evaluate the condition.
If the condition is false, the execution is done.
If it is true, the piece of code is executed and the evaluation of the condition is repeated.
Thus, the piece of code is executed zero or more times.

<code><pre><span style="color: #0000FF">var</span><span style="color: #000000"> a = </span><span style="color: #09885A">0</span><span>
</span><span style="color: #0000FF">while</span><span style="color: #000000"> a &#x3C; </span><span style="color: #09885A">5</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">    a = a + </span><span style="color: #09885A">1</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// `a` is `5`</span><span>
</span></pre></code>

#### [](#for-in-statement)For-in statement

For-in statements allow a certain piece of code to be executed repeatedly for
each element in an array.

The for-in statement starts with the `for` keyword, followed by the name of
the element that is used in each iteration of the loop,
followed by the `in` keyword, and then followed by the array
that is being iterated through in the loop.

Then, the code that should be repeatedly executed in each iteration of the loop
is enclosed in curly braces.

If there are no elements in the data structure, the code in the loop will not
be executed at all. Otherwise, the code will execute as many times
as there are elements in the array.

<code><pre><span style="color: #0000FF">var</span><span style="color: #000000"> array = [</span><span style="color: #A31515">"Hello"</span><span style="color: #000000">, </span><span style="color: #A31515">"World"</span><span style="color: #000000">, </span><span style="color: #A31515">"Foo"</span><span style="color: #000000">, </span><span style="color: #A31515">"Bar"</span><span style="color: #000000">]</span><span>
</span><span style="color: #000000">for element in array {</span><span>
</span><span style="color: #000000">    log(element)</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// The loop would log:</span><span>
</span><span style="color: #008000">// "Hello"</span><span>
</span><span style="color: #008000">// "World"</span><span>
</span><span style="color: #008000">// "Foo"</span><span>
</span><span style="color: #008000">// "Bar"</span><span>
</span><span>
</span></pre></code>

#### [](#continue-and-break)continue and break

In for-loops and while-loops, the `continue` statement can be used to stop
the current iteration of a loop and start the next iteration.

<code><pre><span style="color: #0000FF">var</span><span style="color: #000000"> i = </span><span style="color: #09885A">0</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> x = </span><span style="color: #09885A">0</span><span>
</span><span style="color: #0000FF">while</span><span style="color: #000000"> i &#x3C; </span><span style="color: #09885A">10</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">    i = i + </span><span style="color: #09885A">1</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">if</span><span style="color: #000000"> i &#x3C; </span><span style="color: #09885A">3</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">continue</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">    x = x + </span><span style="color: #09885A">1</span><span>
</span><span style="color: #000000">}</span><span>
</span><span style="color: #008000">// `x` is `8`</span><span>
</span><span>
</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> array = [</span><span style="color: #09885A">2</span><span style="color: #000000">, </span><span style="color: #09885A">2</span><span style="color: #000000">, </span><span style="color: #09885A">3</span><span style="color: #000000">]</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> sum = </span><span style="color: #09885A">0</span><span>
</span><span style="color: #000000">for element in array {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">if</span><span style="color: #000000"> element == </span><span style="color: #09885A">2</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">continue</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">    sum = sum + element</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// `sum` is `3`</span><span>
</span><span>
</span></pre></code>

The `break` statement can be used to stop the execution
of a for-loop or a while-loop.

<code><pre><span style="color: #0000FF">var</span><span style="color: #000000"> x = </span><span style="color: #09885A">0</span><span>
</span><span style="color: #0000FF">while</span><span style="color: #000000"> x &#x3C; </span><span style="color: #09885A">10</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">    x = x + </span><span style="color: #09885A">1</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">if</span><span style="color: #000000"> x == </span><span style="color: #09885A">5</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">break</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span><span style="color: #008000">// `x` is `5`</span><span>
</span><span>
</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> array = [</span><span style="color: #09885A">1</span><span style="color: #000000">, </span><span style="color: #09885A">2</span><span style="color: #000000">, </span><span style="color: #09885A">3</span><span style="color: #000000">]</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> sum = </span><span style="color: #09885A">0</span><span>
</span><span style="color: #000000">for element in array {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">if</span><span style="color: #000000"> element == </span><span style="color: #09885A">2</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">break</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">    sum = sum + element</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// `sum` is `1`</span><span>
</span></pre></code>

### [](#immediate-function-return-return-statement)Immediate function return: return-statement

The return-statement causes a function to return immediately,
i.e., any code after the return-statement is not executed.
The return-statement starts with the `return` keyword
and is followed by an optional expression that should be the return value of the function call.

<!--
TODO: examples

- in function
- in while
- in if
-->

## [](#scope)Scope

Every function and block (`{` ... `}`) introduces a new scope for declarations.
Each function and block can refer to declarations in its scope or any of the outer scopes.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> x = </span><span style="color: #09885A">10</span><span>
</span><span>
</span><span style="color: #0000FF">fun</span><span style="color: #000000"> f(): </span><span style="color: #0000FF">Int</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">let</span><span style="color: #000000"> y = </span><span style="color: #09885A">10</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">return</span><span style="color: #000000"> x + y</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #000000">f()  </span><span style="color: #008000">// is `20`</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: the identifier `y` is not in scope.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">y</span><span>
</span></pre></code>

<code><pre><span style="color: #0000FF">fun</span><span style="color: #000000"> doubleAndAddOne(_ n: </span><span style="color: #0000FF">Int</span><span style="color: #000000">): </span><span style="color: #0000FF">Int</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">fun</span><span style="color: #000000"> double(_ x: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">return</span><span style="color: #000000"> x * </span><span style="color: #09885A">2</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">return</span><span style="color: #000000"> double(n) + </span><span style="color: #09885A">1</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: the identifier `double` is not in scope.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">double(</span><span style="color: #09885A">1</span><span style="color: #000000">)</span><span>
</span></pre></code>

Each scope can introduce new declarations, i.e., the outer declaration is shadowed.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> x = </span><span style="color: #09885A">2</span><span>
</span><span>
</span><span style="color: #0000FF">fun</span><span style="color: #000000"> test(): </span><span style="color: #0000FF">Int</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">let</span><span style="color: #000000"> x = </span><span style="color: #09885A">3</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">return</span><span style="color: #000000"> x</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #000000">test()  </span><span style="color: #008000">// is `3`</span><span>
</span></pre></code>

Scope is lexical, not dynamic.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> x = </span><span style="color: #09885A">10</span><span>
</span><span>
</span><span style="color: #0000FF">fun</span><span style="color: #000000"> f(): </span><span style="color: #0000FF">Int</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">   </span><span style="color: #0000FF">return</span><span style="color: #000000"> x</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #0000FF">fun</span><span style="color: #000000"> g(): </span><span style="color: #0000FF">Int</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">   </span><span style="color: #0000FF">let</span><span style="color: #000000"> x = </span><span style="color: #09885A">20</span><span>
</span><span style="color: #000000">   </span><span style="color: #0000FF">return</span><span style="color: #000000"> f()</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #000000">g()  </span><span style="color: #008000">// is `10`, not `20`</span><span>
</span></pre></code>

Declarations are **not** moved to the top of the enclosing function (hoisted).

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> x = </span><span style="color: #09885A">2</span><span>
</span><span>
</span><span style="color: #0000FF">fun</span><span style="color: #000000"> f(): </span><span style="color: #0000FF">Int</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">if</span><span style="color: #000000"> x == </span><span style="color: #09885A">0</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">let</span><span style="color: #000000"> x = </span><span style="color: #09885A">3</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">return</span><span style="color: #000000"> x</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">return</span><span style="color: #000000"> x</span><span>
</span><span style="color: #000000">}</span><span>
</span><span style="color: #000000">f()  </span><span style="color: #008000">// is `2`</span><span>
</span></pre></code>

## [](#type-safety)Type Safety

The Cadence programming language is a _type-safe_ language.

When assigning a new value to a variable, the value must be the same type as the variable.
For example, if a variable has type `Bool`,
it can _only_ be assigned a value that has type `Bool`,
and not for example a value that has type `Int`.

<code><pre><span style="color: #008000">// Declare a variable that has type `Bool`.</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> a = </span><span style="color: #0000FF">true</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: cannot assign a value that has type `Int` to a variable which has type `Bool`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">a = </span><span style="color: #09885A">0</span><span>
</span></pre></code>

When passing arguments to a function,
the types of the values must match the function parameters&#x27; types.
For example, if a function expects an argument that has type `Bool`,
_only_ a value that has type `Bool` can be provided,
and not for example a value which has type `Int`.

<code><pre><span style="color: #0000FF">fun</span><span style="color: #000000"> nand(_ a: </span><span style="color: #0000FF">Bool</span><span style="color: #000000">, _ b: </span><span style="color: #0000FF">Bool</span><span style="color: #000000">): </span><span style="color: #0000FF">Bool</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">return</span><span style="color: #000000"> !(a &#x26;&#x26; b)</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #000000">nand(</span><span style="color: #0000FF">false</span><span style="color: #000000">, </span><span style="color: #0000FF">false</span><span style="color: #000000">)  </span><span style="color: #008000">// is `true`</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: The arguments of the function calls are integers and have type `Int`,</span><span>
</span><span style="color: #008000">// but the function expects parameters booleans (type `Bool`).</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">nand(</span><span style="color: #09885A">0</span><span style="color: #000000">, </span><span style="color: #09885A">0</span><span style="color: #000000">)</span><span>
</span></pre></code>

Types are **not** automatically converted.
For example, an integer is not automatically converted to a boolean,
nor is an `Int32` automatically converted to an `Int8`,
nor is an optional integer `Int?`
automatically converted to a non-optional integer `Int`,
or vice-versa.

<code><pre><span style="color: #0000FF">fun</span><span style="color: #000000"> add(_ a: </span><span style="color: #0000FF">Int8</span><span style="color: #000000">, _ b: </span><span style="color: #0000FF">Int8</span><span style="color: #000000">): </span><span style="color: #0000FF">Int8</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">return</span><span style="color: #000000"> a + b</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// The arguments are not declared with a specific type, but they are inferred</span><span>
</span><span style="color: #008000">// to be `Int8` since the parameter types of the function `add` are `Int8`.</span><span>
</span><span style="color: #000000">add(</span><span style="color: #09885A">1</span><span style="color: #000000">, </span><span style="color: #09885A">2</span><span style="color: #000000">)  </span><span style="color: #008000">// is `3`</span><span>
</span><span>
</span><span style="color: #008000">// Declare two constants which have type `Int32`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> a: </span><span style="color: #0000FF">Int32</span><span style="color: #000000"> = </span><span style="color: #09885A">3_000_000_000</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> b: </span><span style="color: #0000FF">Int32</span><span style="color: #000000"> = </span><span style="color: #09885A">3_000_000_000</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: cannot pass arguments which have type `Int32` to parameters which have type `Int8`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">add(a, b)</span><span>
</span></pre></code>

## [](#type-inference)Type Inference

> 🚧 Status: Only basic type inference is implemented.

If a variable or constant declaration is not annotated explicitly with a type,
the declaration&#x27;s type is inferred from the initial value.

Integer literals are inferred to type `Int`.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> a = </span><span style="color: #09885A">1</span><span>
</span><span>
</span><span style="color: #008000">// `a` has type `Int`</span><span>
</span></pre></code>

Array literals are inferred based on the elements of the literal, and to be variable-size.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> integers = [</span><span style="color: #09885A">1</span><span style="color: #000000">, </span><span style="color: #09885A">2</span><span style="color: #000000">]</span><span>
</span><span style="color: #008000">// `integers` has type `[Int]`</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: mixed types</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> invalidMixed = [</span><span style="color: #09885A">1</span><span style="color: #000000">, </span><span style="color: #0000FF">true</span><span style="color: #000000">, </span><span style="color: #09885A">2</span><span style="color: #000000">, </span><span style="color: #0000FF">false</span><span style="color: #000000">]</span><span>
</span></pre></code>

Dictionary literals are inferred based on the keys and values of the literal.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> booleans = {</span><span>
</span><span style="color: #000000">    </span><span style="color: #09885A">1</span><span style="color: #000000">: </span><span style="color: #0000FF">true</span><span style="color: #000000">,</span><span>
</span><span style="color: #000000">    </span><span style="color: #09885A">2</span><span style="color: #000000">: </span><span style="color: #0000FF">false</span><span>
</span><span style="color: #000000">}</span><span>
</span><span style="color: #008000">// `booleans` has type `{Int: Bool}`</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: mixed types</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> invalidMixed = {</span><span>
</span><span style="color: #000000">    </span><span style="color: #09885A">1</span><span style="color: #000000">: </span><span style="color: #0000FF">true</span><span style="color: #000000">,</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">false</span><span style="color: #000000">: </span><span style="color: #09885A">2</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

Functions are inferred based on the parameter types and the return type.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> add = (a: Int8, b: Int8): Int {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">return</span><span style="color: #000000"> a + b</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// `add` has type `((Int8, Int8): Int)`</span><span>
</span></pre></code>

Type inference is performed for each expression / statement, and not across statements.

There are cases where types cannot be inferred.
In these cases explicit type annotations are required.

<code><pre><span style="color: #008000">// Invalid: not possible to infer type based on array literal's elements.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> array = []</span><span>
</span><span>
</span><span style="color: #008000">// Instead, specify the array type and the concrete element type, e.g. `Int`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> arrary: [</span><span style="color: #0000FF">Int</span><span style="color: #000000">] = []</span><span>
</span></pre></code>

<code><pre><span style="color: #008000">// Invalid: not possible to infer type based on dictionary literal's keys and values.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> dictionary = {}</span><span>
</span><span>
</span><span style="color: #008000">// Instead, specify the dictionary type and the concrete key</span><span>
</span><span style="color: #008000">// and value types, e.g. `String` and `Int`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> dictionary: {</span><span style="color: #0000FF">String</span><span style="color: #000000">: </span><span style="color: #0000FF">Int</span><span style="color: #000000">} = {}</span><span>
</span></pre></code>

<code><pre><span style="color: #008000">// Invalid: not possible to infer type based on nil literal.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> maybeSomething = </span><span style="color: #0000FF">nil</span><span>
</span><span>
</span><span style="color: #008000">// Instead, specify the optional type and the concrete element type, e.g. `Int`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> maybeSomething: </span><span style="color: #0000FF">Int</span><span style="color: #000000">? = </span><span style="color: #0000FF">nil</span><span>
</span></pre></code>

## [](#composite-types)Composite Types

Composite types allow composing simpler types into more complex types,
i.e., they allow the composition of multiple values into one.
Composite types have a name and consist of zero or more named fields,
and zero or more functions that operate on the data.
Each field may have a different type.

Composite types can only be declared within a [contract](#contracts) and nowhere else.

There are two kinds of composite types.
The kinds differ in their usage and the behaviour
when a value is used as the initial value for a constant or variable,
when the value is assigned to a variable,
when the value is passed as an argument to a function,
and when the value is returned from a function:

-   [**Structures**](#structures) are **copied**, they are value types.

      Structures are useful when copies with independent state are desired.

-   [**Resources**](#resources) are **moved**, they are linear types and **must** be used **exactly once**.

      Resources are useful when it is desired to model ownership
      (a value exists exactly in one location and it should not be lost).

      Certain constructs in a blockchain represent assets of real, tangible value,
      as much as a house or car or bank account.
      We have to worry about literal loss and theft,
      perhaps even on the scale of millions of dollars.

      Structures are not an ideal way to represent this ownership because they are copied.
      This would mean that there could be a risk of having multiple copies
      of certain assets floating around,
      which breaks the scarcity requirements needed for these assets to have real value.

      A structure is much more useful for representing information
      that can be grouped together in a logical way,
      but doesn&#x27;t have value or a need to be able to be owned or transferred.

      A structure could for example be used to contain the information associated
      with a division of a company,
      but a resource would be used to represent the assets that have been allocated
      to that organization for spending.

Nesting of resources is only allowed within other resource types,
or in data structures like arrays and dictionaries,
but not in structures, as that would allow resources to be copied.

### [](#composite-type-declaration-and-creation)Composite Type Declaration and Creation

Structures are declared using the `struct` keyword
and resources are declared using the `resource` keyword.
The keyword is followed by the name.

<code><pre><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">struct</span><span style="color: #000000"> SomeStruct {</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// ...</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">resource</span><span style="color: #000000"> SomeResource {</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// ...</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

Structures and resources are types.

Structures are created (instantiated) by calling the type like a function.

<code><pre><span style="color: #008000">// instantiate a new struct object and assign it to a constant</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> a = SomeStruct()</span><span>
</span></pre></code>

The constructor function may require parameters if the [initializer](#composite-type-fields)
of the composite type requires them.

Composite types can only be declared within [contracts](#contracts)
and not locally in functions.
They can also not be nested.

Resource must be created (instantiated) by using the `create` keyword
and calling the type like a function.

Resources can only be created in functions and types
that are declared in the same contract in which the resource is declared.

<code><pre><span style="color: #008000">// instantiate a new resource object and assign it to a constant</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> b &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> SomeResource()</span><span>
</span></pre></code>

### [](#composite-type-fields)Composite Type Fields

Fields are declared like variables and constants.
However, the initial values for fields are set in the initializer,
**not** in the field declaration.
All fields **must** be initialized in the initializer, exactly once.

Having to provide initial values in the initializer might seem restrictive,
but this ensures that all fields are always initialized in one location, the initializer,
and the initialization order is clear.

The initialization of all fields is checked statically
and it is invalid to not initialize all fields in the initializer.
Also, it is statically checked that a field is definitely initialized before it is used.

The initializer&#x27;s main purpose is to initialize fields, though it may also contain other code.
Just like a function, it may declare parameters and may contain arbitrary code.
However, it has no return type, i.e., it is always `Void`.

The initializer is declared using the `init` keyword.

The initializer always follows any fields.

There are three kinds of fields:

-   **Constant fields** are also stored in the composite value,
      but after they have been initialized with a value
      they **cannot** have new values assigned to them afterwards.
      A constant field must be initialized exactly once.

      Constant fields are declared using the `let` keyword.

-   **Variable fields** are stored in the composite value
      and can have new values assigned to them.

      Variable fields are declared using the `var` keyword.

-   **Synthetic fields** are **not stored** in the composite value,
      i.e. they are derived/computed from other values.
      They can have new values assigned to them.

      Synthetic fields are declared using the `synthetic` keyword.

      Synthetic fields must have a getter and a setter.
      Getters and setters are explained in the
      [next section](#composite-type-field-getters-and-setters).
      Synthetic fields are explained in a [separate section](#synthetic-composite-type-fields).

| Field Kind          | Stored in memory | Assignable | Keyword     |
| ------------------- | ---------------- | ---------- | ----------- |
| **Variable field**  | Yes              | Yes        | `var`       |
| **Constant field**  | Yes              | **No**     | `let`       |
| **Synthetic field** | **No**           | Yes        | `synthetic` |

In initializers, the special constant `self` refers to the composite value
that is to be initialized.

Fields can be read (if they are constant or variable) and set (if they are variable),
using the access syntax: the composite value is followed by a dot (`.`)
and the name of the field.

<code><pre><span style="color: #008000">// Declare a structure named `Token`, which has a constant field</span><span>
</span><span style="color: #008000">// named `id` and a variable field named `balance`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// Both fields are initialized through the initializer.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// The public access modifier `pub` is used in this example to allow</span><span>
</span><span style="color: #008000">// the fields to be read in outer scopes. Fields can also be declared</span><span>
</span><span style="color: #008000">// private so they cannot be accessed in outer scopes.</span><span>
</span><span style="color: #008000">// Access control will be explained in a later section.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">struct</span><span style="color: #000000"> Token {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">let</span><span style="color: #000000"> id: </span><span style="color: #0000FF">Int</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">var</span><span style="color: #000000"> balance: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">init</span><span style="color: #000000">(id: </span><span style="color: #0000FF">Int</span><span style="color: #000000">, balance: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.id = id</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.balance = balance</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

Note that it is invalid to provide the initial value for a field in the field declaration.

<code><pre><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">struct</span><span style="color: #000000"> StructureWithConstantField {</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Invalid: It is invalid to provide an initial value in the field declaration.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// The field must be initialized by setting the initial value in the initializer.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">let</span><span style="color: #000000"> id: </span><span style="color: #0000FF">Int</span><span style="color: #000000"> = </span><span style="color: #09885A">1</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

The field access syntax must be used to access fields –  fields are not available as variables.

<code><pre><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">struct</span><span style="color: #000000"> Token {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">let</span><span style="color: #000000"> id: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">init</span><span style="color: #000000">(initialID: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">// Invalid: There is no variable with the name `id` available.</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">// The field `id` must be initialized by setting `self.id`.</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">        id = initialID</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

The initializer is **not** automatically derived from the fields, it must be explicitly declared.

<code><pre><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">struct</span><span style="color: #000000"> Token {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">let</span><span style="color: #000000"> id: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Invalid: Missing initializer initializing field `id`.</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

A composite value can be created by calling the constructor and providing
the field values as arguments.

The value&#x27;s fields can be accessed on the object after it is created.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> token = Token(id: </span><span style="color: #09885A">42</span><span style="color: #000000">, balance: </span><span style="color: #09885A">1_000_00</span><span style="color: #000000">)</span><span>
</span><span>
</span><span style="color: #000000">token.id  </span><span style="color: #008000">// is `42`</span><span>
</span><span style="color: #000000">token.balance  </span><span style="color: #008000">// is `1_000_000`</span><span>
</span><span>
</span><span style="color: #000000">token.balance = </span><span style="color: #09885A">1</span><span>
</span><span style="color: #008000">// `token.balance` is `1`</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: assignment to constant field</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">token.id = </span><span style="color: #09885A">23</span><span>
</span></pre></code>

Resources have the implicit field `let owner: PublicAccount?`.
If the resource is currently [stored in an account](#account-storage),
then the field contains the publicly accessible portion of the account.
Otherwise the field is `nil`.

The field&#x27;s value changes when the resource is moved from outside account storage
into account storage, when it is moved from the storage of one account
to the storage of another account, and when it is moved out of account storage.

### [](#composite-data-initializer-overloading)Composite Data Initializer Overloading

> 🚧 Status: Initializer overloading is not implemented yet.

Initializers support overloading.
This allows for example providing default values for certain parameters.

<code><pre><span style="color: #008000">// Declare a structure named `Token`, which has a constant field</span><span>
</span><span style="color: #008000">// named `id` and a variable field named `balance`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// The first initializer allows initializing both fields with a given value.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// A second initializer is provided for convenience to initialize the `id` field</span><span>
</span><span style="color: #008000">// with a given value, and the `balance` field with the default value `0`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">struct</span><span style="color: #000000"> Token {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">let</span><span style="color: #000000"> id: </span><span style="color: #0000FF">Int</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">var</span><span style="color: #000000"> balance: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">init</span><span style="color: #000000">(id: </span><span style="color: #0000FF">Int</span><span style="color: #000000">, balance: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.id = id</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.balance = balance</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">init</span><span style="color: #000000">(id: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.id = id</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.balance = </span><span style="color: #09885A">0</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

### [](#composite-type-field-getters-and-setters)Composite Type Field Getters and Setters

Fields may have an optional getter and an optional setter.
Getters are functions that are called when a field is read,
and setters are functions that are called when a field is written.
Only certain assignments are allowed in getters and setters.

Getters and setters are enclosed in opening and closing braces, after the field&#x27;s type.

Getters are declared using the `get` keyword.
Getters have no parameters and their return type is implicitly the type of the field.

<code><pre><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">struct</span><span style="color: #000000"> GetterExample {</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Declare a variable field named `balance` with a getter</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// which ensures the read value is always non-negative.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">var</span><span style="color: #000000"> balance: </span><span style="color: #0000FF">Int</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">get</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">           </span><span style="color: #0000FF">if</span><span style="color: #000000"> </span><span style="color: #0000FF">self</span><span style="color: #000000">.balance &#x3C; </span><span style="color: #09885A">0</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">               </span><span style="color: #0000FF">return</span><span style="color: #000000"> </span><span style="color: #09885A">0</span><span>
</span><span style="color: #000000">           }</span><span>
</span><span>
</span><span style="color: #000000">           </span><span style="color: #0000FF">return</span><span style="color: #000000"> </span><span style="color: #0000FF">self</span><span style="color: #000000">.balance</span><span>
</span><span style="color: #000000">        }</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">init</span><span style="color: #000000">(balance: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.balance = balance</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> example = GetterExample(balance: </span><span style="color: #09885A">10</span><span style="color: #000000">)</span><span>
</span><span style="color: #008000">// `example.balance` is `10`</span><span>
</span><span>
</span><span style="color: #000000">example.balance = </span><span style="color: #09885A">-50</span><span>
</span><span style="color: #008000">// The stored value of the field `example` is `-50` internally,</span><span>
</span><span style="color: #008000">// though `example.balance` is `0` because the getter for `balance` returns `0` instead.</span><span>
</span></pre></code>

Setters are declared using the `set` keyword,
followed by the name for the new value enclosed in parentheses.
The parameter has implicitly the type of the field.
Another type cannot be specified. Setters have no return type.

The types of values assigned to setters must always match the field&#x27;s type.

<code><pre><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">struct</span><span style="color: #000000"> SetterExample {</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Declare a variable field named `balance` with a setter</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// which requires written values to be positive.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">var</span><span style="color: #000000"> balance: </span><span style="color: #0000FF">Int</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">set</span><span style="color: #000000">(newBalance) {</span><span>
</span><span style="color: #000000">            </span><span style="color: #0000FF">pre</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">                newBalance >= </span><span style="color: #09885A">0</span><span>
</span><span style="color: #000000">            }</span><span>
</span><span style="color: #000000">            </span><span style="color: #0000FF">self</span><span style="color: #000000">.balance = newBalance</span><span>
</span><span style="color: #000000">        }</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">init</span><span style="color: #000000">(balance: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.balance = balance</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> example = SetterExample(balance: </span><span style="color: #09885A">10</span><span style="color: #000000">)</span><span>
</span><span style="color: #008000">// `example.balance` is `10`</span><span>
</span><span>
</span><span style="color: #008000">// Run-time error: The precondition of the setter for the field `balance` fails,</span><span>
</span><span style="color: #008000">// the program aborts.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">example.balance = </span><span style="color: #09885A">-50</span><span>
</span></pre></code>

### [](#synthetic-composite-type-fields)Synthetic Composite Type Fields

> 🚧 Status: Synthetic fields are not implemented yet.

Fields which are not stored in the composite value are _synthetic_,
i.e., the field value is computed.
Synthetic can be either read-only, or readable and writable.

Synthetic fields are declared using the `synthetic` keyword.

Synthetic fields are read-only when only a getter is provided.

<code><pre><span style="color: #0000FF">struct</span><span style="color: #000000"> Rectangle {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">var</span><span style="color: #000000"> width: </span><span style="color: #0000FF">Int</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">var</span><span style="color: #000000"> height: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Declare a synthetic field named `area`,</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// which computes the area based on the `width` and `height` fields.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> synthetic area: Int {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">get</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">            </span><span style="color: #0000FF">return</span><span style="color: #000000"> width * height</span><span>
</span><span style="color: #000000">        }</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Declare an initializer which accepts width and height.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// As `area` is synthetic and there is only a getter provided for it,</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// the `area` field cannot be assigned a value.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">init</span><span style="color: #000000">(width: </span><span style="color: #0000FF">Int</span><span style="color: #000000">, height: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.width = width</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.height = height</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

Synthetic fields are readable and writable when both a getter and a setter is declared.

<code><pre><span style="color: #008000">// Declare a struct named `GoalTracker` which stores a number</span><span>
</span><span style="color: #008000">// of target goals, a number of completed goals,</span><span>
</span><span style="color: #008000">// and has a synthetic field to provide the left number of goals.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// NOTE: the tracker only implements some functionality to demonstrate</span><span>
</span><span style="color: #008000">// synthetic fields, it is incomplete (e.g. assignments to `goal` are not handled properly).</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">struct</span><span style="color: #000000"> GoalTracker {</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">var</span><span style="color: #000000"> goal: </span><span style="color: #0000FF">Int</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">var</span><span style="color: #000000"> completed: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Declare a synthetic field which is both readable and writable.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// When the field is read from (in the getter), the number</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// of left goals is computed from the target number of goals</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// and the completed number of goals.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// When the field is written to (in the setter), the number</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// of completed goals is updated, based on the number</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// of target goals and the new remaining number of goals.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> synthetic left: Int {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">get</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">            </span><span style="color: #0000FF">return</span><span style="color: #000000"> </span><span style="color: #0000FF">self</span><span style="color: #000000">.goal - </span><span style="color: #0000FF">self</span><span style="color: #000000">.completed</span><span>
</span><span style="color: #000000">        }</span><span>
</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">set</span><span style="color: #000000">(newLeft) {</span><span>
</span><span style="color: #000000">            </span><span style="color: #0000FF">self</span><span style="color: #000000">.completed = </span><span style="color: #0000FF">self</span><span style="color: #000000">.goal - newLeft</span><span>
</span><span style="color: #000000">        }</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">init</span><span style="color: #000000">(goal: </span><span style="color: #0000FF">Int</span><span style="color: #000000">, completed: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.goal = goal</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.completed = completed</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> tracker = GoalTracker(goal: </span><span style="color: #09885A">10</span><span style="color: #000000">, completed: </span><span style="color: #09885A">0</span><span style="color: #000000">)</span><span>
</span><span style="color: #008000">// `tracker.goal` is `10`</span><span>
</span><span style="color: #008000">// `tracker.completed` is `0`</span><span>
</span><span style="color: #008000">// `tracker.left` is `10`</span><span>
</span><span>
</span><span style="color: #000000">tracker.completed = </span><span style="color: #09885A">1</span><span>
</span><span style="color: #008000">// `tracker.left` is `9`</span><span>
</span><span>
</span><span style="color: #000000">tracker.left = </span><span style="color: #09885A">8</span><span>
</span><span style="color: #008000">// `tracker.completed` is `2`</span><span>
</span></pre></code>

It is invalid to declare a synthetic field with only a setter.

### [](#composite-type-functions)Composite Type Functions

> 🚧 Status: Function overloading is not implemented yet.

Composite types may contain functions.
Just like in the initializer, the special constant `self` refers to the composite value that the function is called on.

<code><pre><span style="color: #008000">// Declare a structure named "Rectangle", which represents a rectangle</span><span>
</span><span style="color: #008000">// and has variable fields for the width and height.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">struct</span><span style="color: #000000"> Rectangle {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">var</span><span style="color: #000000"> width: </span><span style="color: #0000FF">Int</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">var</span><span style="color: #000000"> height: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">init</span><span style="color: #000000">(width: </span><span style="color: #0000FF">Int</span><span style="color: #000000">, height: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.width = width</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.height = height</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Declare a function named "scale", which scales</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// the rectangle by the given factor.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> scale(factor: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.width = </span><span style="color: #0000FF">self</span><span style="color: #000000">.width * factor</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.height = </span><span style="color: #0000FF">self</span><span style="color: #000000">.height * factor</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> rectangle = Rectangle(width: </span><span style="color: #09885A">2</span><span style="color: #000000">, height: </span><span style="color: #09885A">3</span><span style="color: #000000">)</span><span>
</span><span style="color: #000000">rectangle.scale(factor: </span><span style="color: #09885A">4</span><span style="color: #000000">)</span><span>
</span><span style="color: #008000">// `rectangle.width` is `8`</span><span>
</span><span style="color: #008000">// `rectangle.height` is `12`</span><span>
</span></pre></code>

Functions support overloading.

<code><pre><span style="color: #008000">// Declare a structure named "Rectangle", which represents a rectangle</span><span>
</span><span style="color: #008000">// and has variable fields for the width and height.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">struct</span><span style="color: #000000"> Rectangle {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">var</span><span style="color: #000000"> width: </span><span style="color: #0000FF">Int</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">var</span><span style="color: #000000"> height: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">init</span><span style="color: #000000">(width: </span><span style="color: #0000FF">Int</span><span style="color: #000000">, height: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.width = width</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.height = height</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Declare a function named "scale", which independently scales</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// the width by a given factor and the height by a given factor.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> scale(widthFactor: </span><span style="color: #0000FF">Int</span><span style="color: #000000">, heightFactor: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.width = </span><span style="color: #0000FF">self</span><span style="color: #000000">.width * widthFactor</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.height = </span><span style="color: #0000FF">self</span><span style="color: #000000">.height * heightFactor</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Declare a another function also named "scale", which scales</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// both width and height by a given factor.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// The function calls the `scale` function declared above.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> scale(factor: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.scale(</span><span>
</span><span style="color: #000000">            widthFactor: factor,</span><span>
</span><span style="color: #000000">            heightFactor: factor</span><span>
</span><span style="color: #000000">        )</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

### [](#composite-type-subtyping)Composite Type Subtyping

Two composite types are compatible if and only if they refer to the same declaration by name,
i.e., nominal typing applies instead of structural typing.

Even if two composite types declare the same fields and functions,
the types are only compatible if their names match.

<code><pre><span style="color: #008000">// Declare a structure named `A` which has a function `test`</span><span>
</span><span style="color: #008000">// which has type `((): Void)`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">struct</span><span style="color: #000000"> A {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">fun</span><span style="color: #000000"> test() {}</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Declare a structure named `B` which has a function `test`</span><span>
</span><span style="color: #008000">// which has type `((): Void)`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">struct</span><span style="color: #000000"> B {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">fun</span><span style="color: #000000"> test() {}</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Declare a variable named which accepts values of type `A`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> something: </span><span style="color: #0000FF">A</span><span style="color: #000000"> = A()</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Assign a value of type `B` to the variable.</span><span>
</span><span style="color: #008000">// Even though types `A` and `B` have the same declarations,</span><span>
</span><span style="color: #008000">// a function with the same name and type, the types' names differ,</span><span>
</span><span style="color: #008000">// so they are not compatible.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">something = B()</span><span>
</span><span>
</span><span style="color: #008000">// Valid: Reassign a new value of type `A`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">something = A()</span><span>
</span></pre></code>

### [](#composite-type-behaviour)Composite Type Behaviour

#### [](#structures)Structures

Structures are **copied** when
used as an initial value for constant or variable,
when assigned to a different variable,
when passed as an argument to a function,
and when returned from a function.

Accessing a field or calling a function of a structure does not copy it.

<code><pre><span style="color: #008000">// Declare a structure named `SomeStruct`, with a variable integer field.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">struct</span><span style="color: #000000"> SomeStruct {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">var</span><span style="color: #000000"> value: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">init</span><span style="color: #000000">(value: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.value = value</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">fun</span><span style="color: #000000"> increment() {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.value = </span><span style="color: #0000FF">self</span><span style="color: #000000">.value + </span><span style="color: #09885A">1</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Declare a constant with value of structure type `SomeStruct`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> a = SomeStruct(value: </span><span style="color: #09885A">0</span><span style="color: #000000">)</span><span>
</span><span>
</span><span style="color: #008000">// *Copy* the structure value into a new constant.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> b = a</span><span>
</span><span>
</span><span style="color: #000000">b.value = </span><span style="color: #09885A">1</span><span>
</span><span style="color: #008000">// NOTE: `b.value` is 1, `a.value` is *`0`*</span><span>
</span><span>
</span><span style="color: #000000">b.increment()</span><span>
</span><span style="color: #008000">// `b.value` is 2, `a.value` is `0`</span><span>
</span></pre></code>

#### [](#accessing-fields-and-functions-of-composite-types-using-optional-chaining)Accessing Fields and Functions of Composite Types Using Optional Chaining

If a composite type with fields and functions is wrapped in an optional,
optional chaining can be used to get those values or call the function without
having to get the value of the optional first.

Optional chaining is used by adding a `?`
before the `.` access operator for fields or
functions of an optional composite type.

When getting a field value or
calling a function with a return value, the access returns
the value as an optional.
If the object doesn&#x27;t exist, the value will always be `nil`

When calling a function on an optional like this, if the object doesn&#x27;t exist,
nothing will happen and the execution will continue.

It is still invalid to access an undeclared field
of an optional composite type.

<code><pre><span style="color: #008000">// Declare a struct with a field and method.</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">struct</span><span style="color: #000000"> Value {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">var</span><span style="color: #000000"> number: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">init</span><span style="color: #000000">() {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.number = </span><span style="color: #09885A">2</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> set(new: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.number = new</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> setAndReturn(new: </span><span style="color: #0000FF">Int</span><span style="color: #000000">): </span><span style="color: #0000FF">Int</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.number = new</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">return</span><span style="color: #000000"> new</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Create a new instance of the struct as an optional</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> value: </span><span style="color: #0000FF">Value</span><span style="color: #000000">? = Value()</span><span>
</span><span style="color: #008000">// Create another optional with the same type, but nil</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> noValue: </span><span style="color: #0000FF">Value</span><span style="color: #000000">? = </span><span style="color: #0000FF">nil</span><span>
</span><span>
</span><span style="color: #008000">// Access the `number` field using optional chaining</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> twoOpt = value?.number</span><span>
</span><span style="color: #008000">// Because `value` is an optional, `twoOpt` has type `Int?`</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> two = zeroOpt ?? </span><span style="color: #09885A">0</span><span>
</span><span style="color: #008000">// `two` is `2`</span><span>
</span><span>
</span><span style="color: #008000">// Try to access the `number` field of `noValue`, which has type `Value?`</span><span>
</span><span style="color: #008000">// This still returns an `Int?`</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> nilValue = noValue?.number</span><span>
</span><span style="color: #008000">// This time, since `noValue` is `nil`, `nilValue` will also be `nil`</span><span>
</span><span>
</span><span style="color: #008000">// Call the `set` function of the struct</span><span>
</span><span style="color: #008000">// whether or not the object exists, this will not fail</span><span>
</span><span style="color: #000000">value?.set(new: </span><span style="color: #09885A">4</span><span style="color: #000000">)</span><span>
</span><span style="color: #000000">noValue?.set(new: </span><span style="color: #09885A">4</span><span style="color: #000000">)</span><span>
</span><span>
</span><span style="color: #008000">// Call the `setAndReturn` function, which returns an `Int`</span><span>
</span><span style="color: #008000">// Because `value` is an optional, the return value is type `Int?`</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> sixOpt = value?.setAndReturn(new: </span><span style="color: #09885A">6</span><span style="color: #000000">)</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> six = sixOpt ?? </span><span style="color: #09885A">0</span><span>
</span><span style="color: #008000">// `six` is `6`</span><span>
</span></pre></code>

This is also possible by using the force-unwrap operator (`!`).

Forced-Optional chaining is used by adding a `!`
before the `.` access operator for fields or
functions of an optional composite type.

When getting a field value or calling a function with a return value,
the access returns the value.
If the object doesn&#x27;t exist, the execution will panic and revert.

It is still invalid to access an undeclared field
of an optional composite type.

<code><pre><span style="color: #008000">// Declare a struct with a field and method.</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">struct</span><span style="color: #000000"> Value {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">var</span><span style="color: #000000"> number: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">init</span><span style="color: #000000">() {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.number = </span><span style="color: #09885A">2</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> set(new: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.number = new</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> setAndReturn(new: </span><span style="color: #0000FF">Int</span><span style="color: #000000">): </span><span style="color: #0000FF">Int</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.number = new</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">return</span><span style="color: #000000"> new</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Create a new instance of the struct as an optional</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> value: </span><span style="color: #0000FF">Value</span><span style="color: #000000">? = Value()</span><span>
</span><span style="color: #008000">// Create another optional with the same type, but nil</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> noValue: </span><span style="color: #0000FF">Value</span><span style="color: #000000">? = </span><span style="color: #0000FF">nil</span><span>
</span><span>
</span><span style="color: #008000">// Access the `number` field using force-optional chaining</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> two = value!.number</span><span>
</span><span style="color: #008000">// `two` is `2`</span><span>
</span><span>
</span><span style="color: #008000">// Try to access the `number` field of `noValue`, which has type `Value?`</span><span>
</span><span style="color: #008000">// Run-time error: This time, since `noValue` is `nil`,</span><span>
</span><span style="color: #008000">// the program execution will revert</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> number = noValue!.number</span><span>
</span><span>
</span><span style="color: #008000">// Call the `set` function of the struct</span><span>
</span><span>
</span><span style="color: #008000">// This succeeds and sets the value to 4</span><span>
</span><span style="color: #000000">value!.set(new: </span><span style="color: #09885A">4</span><span style="color: #000000">)</span><span>
</span><span>
</span><span style="color: #008000">// Run-time error: Since `noValue` is nil, the value is not set</span><span>
</span><span style="color: #008000">// and the program execution reverts.</span><span>
</span><span style="color: #000000">noValue!.set(new: </span><span style="color: #09885A">4</span><span style="color: #000000">)</span><span>
</span><span>
</span><span style="color: #008000">// Call the `setAndReturn` function, which returns an `Int`</span><span>
</span><span style="color: #008000">// Because we use force-unwrap before calling the function,</span><span>
</span><span style="color: #008000">// the return value is type `Int`</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> six = value!.setAndReturn(new: </span><span style="color: #09885A">6</span><span style="color: #000000">)</span><span>
</span><span style="color: #008000">// `six` is `6`</span><span>
</span></pre></code>

#### [](#resources)Resources

Resources are types that can only exist in **one** location at a time
and **must** be used **exactly once**.

Resources **must** be created (instantiated) by using the `create` keyword.

At the end of a function which has resources (variables, constants, parameters) in scope,
the resources **must** be either **moved** or **destroyed**.

They are **moved** when used as an initial value for a constant or variable,
when assigned to a different variable,
when passed as an argument to a function,
and when returned from a function.

Resources can be explicitly **destroyed** using the `destroy` keyword.

Accessing a field or calling a function of a resource does not move or destroy it.

When the resource is moved, the constant or variable
that referred to the resource before the move becomes **invalid**.
An **invalid** resource cannot be used again.

To make the usage and behaviour of resource types explicit,
the prefix `@` must be used in type annotations
of variable or constant declarations, parameters, and return types.

To make moves of resources explicit, the move operator `<-` must be used
when the resource is the initial value of a constant or variable,
when it is moved to a different variable,
when it is moved to a function as an argument,
and when it is returned from a function.

<code><pre><span style="color: #008000">// Declare a resource named `SomeResource`, with a variable integer field.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">resource</span><span style="color: #000000"> SomeResource {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">var</span><span style="color: #000000"> value: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">init</span><span style="color: #000000">(value: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.value = value</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Declare a constant with value of resource type `SomeResource`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> a: @</span><span style="color: #0000FF">SomeResource</span><span style="color: #000000"> &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> SomeResource(value: </span><span style="color: #09885A">0</span><span style="color: #000000">)</span><span>
</span><span>
</span><span style="color: #008000">// *Move* the resource value to a new constant.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> b &#x3C;- a</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Cannot use constant `a` anymore as the resource that it referred to</span><span>
</span><span style="color: #008000">// was moved to constant `b`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">a.value</span><span>
</span><span>
</span><span style="color: #008000">// Constant `b` owns the resource.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">b.value </span><span style="color: #008000">// equals 0</span><span>
</span><span>
</span><span style="color: #008000">// Declare a function which accepts a resource.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// The parameter has a resource type, so the type annotation must be prefixed with `@`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> use(resource: @</span><span style="color: #0000FF">SomeResource</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// ...</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Call function `use` and move the resource into it.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">use(resource: &#x3C;-b)</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Cannot use constant `b` anymore as the resource</span><span>
</span><span style="color: #008000">// it referred to was moved into function `use`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">b.value</span><span>
</span></pre></code>

A resource object cannot go out of scope and be dynamically lost.
The program must either explicitly destroy it or move it to another context.

<code><pre><span style="color: #000000">{</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Declare another, unrelated value of resource type `SomeResource`.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">let</span><span style="color: #000000"> c &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> SomeResource(value: </span><span style="color: #09885A">10</span><span style="color: #000000">)</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Invalid: `c` is not used before the end of the scope, but must be.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// It cannot be lost.</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

<code><pre><span style="color: #008000">// Declare another, unrelated value of resource type `SomeResource`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> d &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> SomeResource(value: </span><span style="color: #09885A">20</span><span style="color: #000000">)</span><span>
</span><span>
</span><span style="color: #008000">// Destroy the resource referred to by constant `d`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">destroy</span><span style="color: #000000"> d</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Cannot use constant `d` anymore as the resource</span><span>
</span><span style="color: #008000">// it referred to was destroyed.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">d.value</span><span>
</span></pre></code>

To make it explicit that the type is a resource type
and must follow the rules associated with resources,
it must be prefixed with `@` in all type annotations,
e.g. for variable declarations, parameters, or return types.

<code><pre><span style="color: #008000">// Declare a constant with an explicit type annotation.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// The constant has a resource type, so the type annotation must be prefixed with `@`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> someResource: @</span><span style="color: #0000FF">SomeResource</span><span style="color: #000000"> &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> SomeResource(value: </span><span style="color: #09885A">5</span><span style="color: #000000">)</span><span>
</span><span>
</span><span style="color: #008000">// Declare a function which consumes a resource and destroys it.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// The parameter has a resource type, so the type annotation must be prefixed with `@`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> use(resource: @</span><span style="color: #0000FF">SomeResource</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">destroy</span><span style="color: #000000"> resource</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Declare a function which returns a resource.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// The return type is a resource type, so the type annotation must be prefixed with `@`.</span><span>
</span><span style="color: #008000">// The return statement must also use the `&#x3C;-` operator to make it explicit the resource is moved.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> get(): @</span><span style="color: #0000FF">SomeResource</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">let</span><span style="color: #000000"> newResource &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> SomeResource()</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">return</span><span style="color: #000000"> &#x3C;-newResource</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

Resources **must** be used exactly once.

<code><pre><span style="color: #008000">// Declare a function which consumes a resource but does not use it.</span><span>
</span><span style="color: #008000">// This function is invalid, because it would cause a loss of the resource.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> forgetToUse(resource: @</span><span style="color: #0000FF">SomeResource</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Invalid: The resource parameter `resource` is not used, but must be.</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

<code><pre><span style="color: #008000">// Declare a constant named `res` which has the resource type `SomeResource`.</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> res &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> SomeResource()</span><span>
</span><span>
</span><span style="color: #008000">// Call the function `use` and move the resource `res` into it.</span><span>
</span><span style="color: #000000">use(resource: &#x3C;-res)</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: The resource constant `res` cannot be used again,</span><span>
</span><span style="color: #008000">// as it was moved in the previous function call.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">use(resource: &#x3C;-res)</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: The resource constant `res` cannot be used again,</span><span>
</span><span style="color: #008000">// as it was moved in the previous function call.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">res.value</span><span>
</span></pre></code>

<code><pre><span style="color: #008000">// Declare a function which has a resource parameter but does not use it.</span><span>
</span><span style="color: #008000">// This function is invalid, because it would cause a loss of the resource.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> forgetToUse(resource: @</span><span style="color: #0000FF">SomeResource</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Invalid: The resource parameter `resource` is not used, but must be.</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

<code><pre><span style="color: #008000">// Declare a function which has a resource parameter.</span><span>
</span><span style="color: #008000">// This function is invalid, because it does not always use the resource parameter,</span><span>
</span><span style="color: #008000">// which would cause a loss of the resource.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> sometimesDestroy(resource: @</span><span style="color: #0000FF">SomeResource</span><span style="color: #000000">, destroy: </span><span style="color: #0000FF">Bool</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">if</span><span style="color: #000000"> destroyResource {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">destroy</span><span style="color: #000000"> resource</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Invalid: The resource parameter `resource` is not always used, but must be.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// The destroy statement is not always executed, so at the end of this function</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// it might have been destroyed or not.</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

<code><pre><span style="color: #008000">// Declare a function which has a resource parameter.</span><span>
</span><span style="color: #008000">// This function is valid, as it always uses the resource parameter,</span><span>
</span><span style="color: #008000">// and does not cause a loss of the resource.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> alwaysUse(resource: @</span><span style="color: #0000FF">SomeResource</span><span style="color: #000000">, destroyResource: </span><span style="color: #0000FF">Bool</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">if</span><span style="color: #000000"> destroyResource {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">destroy</span><span style="color: #000000"> resource</span><span>
</span><span style="color: #000000">    } </span><span style="color: #0000FF">else</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">        use(resource: &#x3C;-resource)</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// At the end of the function the resource parameter was definitely used:</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// It was either destroyed or moved in the call of function `use`.</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

<code><pre><span style="color: #008000">// Declare a function which has a resource parameter.</span><span>
</span><span style="color: #008000">// This function is invalid, because it does not always use the resource parameter,</span><span>
</span><span style="color: #008000">// which would cause a loss of the resource.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> returnBeforeDestroy(: </span><span style="color: #0000FF">Bool</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">let</span><span style="color: #000000"> res &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> SomeResource(value: </span><span style="color: #09885A">1</span><span style="color: #000000">)</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">if</span><span style="color: #000000"> move {</span><span>
</span><span style="color: #000000">        use(resource: &#x3C;-res)</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">return</span><span>
</span><span style="color: #000000">    } </span><span style="color: #0000FF">else</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">// Invalid: When this function returns here, the resource variable</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">// `res` was not used, but must be.</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">return</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Invalid: the resource variable `res` was potentially moved in the</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// previous if-statement, and both branches definitely return,</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// so this statement is unreachable.</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">destroy</span><span style="color: #000000"> res</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

#### [](#resource-variables)Resource Variables

Resource variables cannot be assigned to,
as that would lead to the loss of the variable&#x27;s current resource value.

Instead, use a swap statement (`<->`) or shift statement (`<- target <-`)
to replace the resource variable with another resource.

<code><pre><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">resource</span><span style="color: #000000"> R {}</span><span>
</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> x &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> R()</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> y &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> R()</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Cannot assign to resource variable `x`,</span><span>
</span><span style="color: #008000">// as its current resource would be lost</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">x &#x3C;- y</span><span>
</span><span>
</span><span style="color: #008000">// Instead, use a swap statement.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> replacement &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> R()</span><span>
</span><span style="color: #000000">x &#x3C;-> replacement</span><span>
</span><span style="color: #008000">// `x` is the new resource.</span><span>
</span><span style="color: #008000">// `replacement` is the old resource.</span><span>
</span><span>
</span><span style="color: #008000">// Or use the shift statement (`&#x3C;- target &#x3C;-`)</span><span>
</span><span style="color: #008000">// This statement moves the resource out of `x` and into `oldX`,</span><span>
</span><span style="color: #008000">// and at the same time assigns `x` with the new value on the right-hand side.</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> oldX &#x3C;- x &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> R()</span><span>
</span><span style="color: #008000">// oldX still needs to be explicitly handled after this statement</span><span>
</span><span style="color: #0000FF">destroy</span><span style="color: #000000"> oldX</span><span>
</span></pre></code>

#### [](#resource-destructors)Resource Destructors

Resource may have a destructor, which is executed when the resource is destroyed.
Destructors have no parameters and no return value and are declared using the `destroy` name.
A resource may have only one destructor.

<code><pre><span style="color: #0000FF">var</span><span style="color: #000000"> destructorCalled = </span><span style="color: #0000FF">false</span><span>
</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">resource</span><span style="color: #000000"> Resource {</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Declare a destructor for the resource, which is executed</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// when the resource is destroyed.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    destroy() {</span><span>
</span><span style="color: #000000">        destructorCalled = </span><span style="color: #0000FF">true</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> res &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> Resource()</span><span>
</span><span style="color: #0000FF">destroy</span><span style="color: #000000"> res</span><span>
</span><span style="color: #008000">// `destructorCalled` is `true`</span><span>
</span></pre></code>

#### [](#nested-resources)Nested Resources

Fields in composite types behave differently when they have a resource type.

If a resource type has fields that have a resource type,
it **must** declare a destructor,
which **must** invalidate all resource fields, i.e. move or destroy them.

<code><pre><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">resource</span><span style="color: #000000"> Child {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">let</span><span style="color: #000000"> name: </span><span style="color: #0000FF">String</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">init</span><span style="color: #000000">(name: </span><span style="color: #0000FF">String</span><span style="color: #000000">)</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.name = name</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Declare a resource with a resource field named `child`.</span><span>
</span><span style="color: #008000">// The resource *must* declare a destructor</span><span>
</span><span style="color: #008000">// and the destructor *must* invalidate the resource field.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">resource</span><span style="color: #000000"> Parent {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">let</span><span style="color: #000000"> name: </span><span style="color: #0000FF">String</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">var</span><span style="color: #000000"> child: @</span><span style="color: #0000FF">Child</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">init</span><span style="color: #000000">(name: </span><span style="color: #0000FF">String</span><span style="color: #000000">, child: @</span><span style="color: #0000FF">Child</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.name = name</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.child &#x3C;- child</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Declare a destructor which invalidates the resource field</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// `child` by destroying it.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    destroy() {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">destroy</span><span style="color: #000000"> </span><span style="color: #0000FF">self</span><span style="color: #000000">.child</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

Accessing a field or calling function on a resource field is valid,
however moving a resource out of a variable resource field is **not** allowed.
Instead, use a swap statement to replace the resource with another resource.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> child &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> Child(name: </span><span style="color: #A31515">"Child 1"</span><span style="color: #000000">)</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> parent &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> Parent(name: </span><span style="color: #A31515">"Parent"</span><span style="color: #000000">, child: &#x3C;-child)</span><span>
</span><span>
</span><span style="color: #000000">child.name  </span><span style="color: #008000">// is "Child"</span><span>
</span><span style="color: #000000">parent.child.name  </span><span style="color: #008000">// is "Child"</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Cannot move resource out of variable resource field.</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> childAgain &#x3C;- parent.child</span><span>
</span><span>
</span><span style="color: #008000">// Instead, use a swap statement.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> otherChild &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> Child(name: </span><span style="color: #A31515">"Child 2"</span><span style="color: #000000">)</span><span>
</span><span style="color: #000000">parent.child &#x3C;-> otherChild</span><span>
</span><span style="color: #008000">// `parent.child` is the second child, Child 2.</span><span>
</span><span style="color: #008000">// `otherChild` is the first child, Child 1.</span><span>
</span></pre></code>

#### [](#resources-in-closures)Resources in Closures

Resources can not be captured in closures, as that could potentially result in duplications.

<code><pre><span style="color: #0000FF">resource</span><span style="color: #000000"> R {}</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Declare a function which returns a closure which refers to</span><span>
</span><span style="color: #008000">// the resource parameter `resource`. Each call to the returned function</span><span>
</span><span style="color: #008000">// would return the resource, which should not be possible.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">fun</span><span style="color: #000000"> makeCloner(resource: @</span><span style="color: #0000FF">R</span><span style="color: #000000">): ((): @</span><span style="color: #0000FF">R</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">return</span><span style="color: #000000"> fun (): @R {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">return</span><span style="color: #000000"> &#x3C;-resource</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> test = makeCloner(resource: &#x3C;-create R())</span><span>
</span></pre></code>

#### [](#resources-in-arrays-and-dictionaries)Resources in Arrays and Dictionaries

Arrays and dictionaries behave differently when they contain resources:
It is **not** allowed to index into an array to read an element at a certain index or assign to it,
or index into a dictionary to read a value for a certain key or set a value for the key.

Instead, use a swap statement (`<->`) or shift statement (`<- target <-`)
to replace the accessed resource with another resource.

<code><pre><span style="color: #0000FF">resource</span><span style="color: #000000"> R {}</span><span>
</span><span>
</span><span style="color: #008000">// Declare a constant for an array of resources.</span><span>
</span><span style="color: #008000">// Create two resources and move them into the array.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> resources &#x3C;- [</span><span>
</span><span style="color: #000000">    &#x3C;-</span><span style="color: #0000FF">create</span><span style="color: #000000"> R(),</span><span>
</span><span style="color: #000000">    &#x3C;-</span><span style="color: #0000FF">create</span><span style="color: #000000"> R()</span><span>
</span><span style="color: #000000">]</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Reading an element from a resource array is not allowed.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> firstResource &#x3C;- resources[</span><span style="color: #09885A">0</span><span style="color: #000000">]</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Setting an element in a resource array is not allowed,</span><span>
</span><span style="color: #008000">// as it would result in the loss of the current value.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">resources[</span><span style="color: #09885A">0</span><span style="color: #000000">] &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> R()</span><span>
</span><span>
</span><span style="color: #008000">// Instead, when attempting to either read an element or update an element</span><span>
</span><span style="color: #008000">// in a resource array, use a swap statement with a variable to replace</span><span>
</span><span style="color: #008000">// the accessed element.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> res &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> R()</span><span>
</span><span style="color: #000000">resources[</span><span style="color: #09885A">0</span><span style="color: #000000">] &#x3C;-> res</span><span>
</span><span style="color: #008000">// `resources[0]` now contains the new resource.</span><span>
</span><span style="color: #008000">// `res` now contains the old resource.</span><span>
</span><span>
</span><span style="color: #008000">// Use the shift statement to move the new resource into</span><span>
</span><span style="color: #008000">// the array at the same time that the old resource is being moved out</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> oldRes &#x3C;- resources[</span><span style="color: #09885A">0</span><span style="color: #000000">] &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> R()</span><span>
</span><span style="color: #008000">// The old object still needs to be handled</span><span>
</span><span style="color: #0000FF">destroy</span><span style="color: #000000"> oldRes</span><span>
</span></pre></code>

The same applies to dictionaries.

<code><pre><span style="color: #008000">// Declare a constant for a dictionary of resources.</span><span>
</span><span style="color: #008000">// Create two resources and move them into the dictionary.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> resources &#x3C;- {</span><span>
</span><span style="color: #000000">    </span><span style="color: #A31515">"r1"</span><span style="color: #000000">: &#x3C;-</span><span style="color: #0000FF">create</span><span style="color: #000000"> R(),</span><span>
</span><span style="color: #000000">    </span><span style="color: #A31515">"r2"</span><span style="color: #000000">: &#x3C;-</span><span style="color: #0000FF">create</span><span style="color: #000000"> R()</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Reading an element from a resource dictionary is not allowed.</span><span>
</span><span style="color: #008000">// It's not obvious that an access like this would have to remove</span><span>
</span><span style="color: #008000">// the key from the dictionary.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> firstResource &#x3C;- resources[</span><span style="color: #A31515">"r1"</span><span style="color: #000000">]</span><span>
</span><span>
</span><span style="color: #008000">// Instead, make the removal explicit by using the `remove` function.</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> firstResource &#x3C;- resources.remove(key: </span><span style="color: #A31515">"r1"</span><span style="color: #000000">)</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Setting an element in a resource dictionary is not allowed,</span><span>
</span><span style="color: #008000">// as it would result in the loss of the current value.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">resources[</span><span style="color: #A31515">"r1"</span><span style="color: #000000">] &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> R()</span><span>
</span><span>
</span><span style="color: #008000">// Instead, when attempting to either read an element or update an element</span><span>
</span><span style="color: #008000">// in a resource dictionary, use a swap statement with a variable to replace</span><span>
</span><span style="color: #008000">// the accessed element.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> res &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> R()</span><span>
</span><span style="color: #000000">resources[</span><span style="color: #A31515">"r1"</span><span style="color: #000000">] &#x3C;-> res</span><span>
</span><span style="color: #008000">// `resources["r1"]` now contains the new resource.</span><span>
</span><span style="color: #008000">// `res` now contains the old resource.</span><span>
</span><span>
</span><span style="color: #008000">// Use the shift statement to move the new resource into</span><span>
</span><span style="color: #008000">// the dictionary at the same time that the old resource is being moved out</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> oldRes &#x3C;- resources[</span><span style="color: #A31515">"r2"</span><span style="color: #000000">] &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> R()</span><span>
</span><span style="color: #008000">// The old object still needs to be handled</span><span>
</span><span style="color: #0000FF">destroy</span><span style="color: #000000"> oldRes</span><span>
</span></pre></code>

Resources cannot be moved into arrays and dictionaries multiple times,
as that would cause a duplication.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> resource &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> R()</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: The resource variable `resource` can only be moved into the array once.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> resources &#x3C;- [</span><span>
</span><span style="color: #000000">    &#x3C;-resource,</span><span>
</span><span style="color: #000000">    &#x3C;-resource</span><span>
</span><span style="color: #000000">]</span><span>
</span></pre></code>

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> resource &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> R()</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: The resource variable `resource` can only be moved into the dictionary once.</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> resources &#x3C;- {</span><span>
</span><span style="color: #000000">    </span><span style="color: #A31515">"res1"</span><span style="color: #000000">: &#x3C;-resource,</span><span>
</span><span style="color: #000000">    </span><span style="color: #A31515">"res2"</span><span style="color: #000000">: &#x3C;-resource</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

Resource arrays and dictionaries can be destroyed.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> resources &#x3C;- [</span><span>
</span><span style="color: #000000">    &#x3C;-</span><span style="color: #0000FF">create</span><span style="color: #000000"> R(),</span><span>
</span><span style="color: #000000">    &#x3C;-</span><span style="color: #0000FF">create</span><span style="color: #000000"> R()</span><span>
</span><span style="color: #000000">]</span><span>
</span><span style="color: #0000FF">destroy</span><span style="color: #000000"> resources</span><span>
</span></pre></code>

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> resources &#x3C;- {</span><span>
</span><span style="color: #000000">    </span><span style="color: #A31515">"r1"</span><span style="color: #000000">: &#x3C;-</span><span style="color: #0000FF">create</span><span style="color: #000000"> R(),</span><span>
</span><span style="color: #000000">    </span><span style="color: #A31515">"r2"</span><span style="color: #000000">: &#x3C;-</span><span style="color: #0000FF">create</span><span style="color: #000000"> R()</span><span>
</span><span style="color: #000000">}</span><span>
</span><span style="color: #0000FF">destroy</span><span style="color: #000000"> resources</span><span>
</span></pre></code>

The variable array functions like `append`, `insert`, and `remove`
behave like for non-resource arrays.
Note however, that the result of the `remove` functions must be used.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> resources &#x3C;- [&#x3C;-</span><span style="color: #0000FF">create</span><span style="color: #000000"> R()]</span><span>
</span><span style="color: #008000">// `resources.length` is `1`</span><span>
</span><span>
</span><span style="color: #000000">resources.append(&#x3C;-create R())</span><span>
</span><span style="color: #008000">// `resources.length` is `2`</span><span>
</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> first &#x3C;- resource.remove(at: </span><span style="color: #09885A">0</span><span style="color: #000000">)</span><span>
</span><span style="color: #008000">// `resources.length` is `1`</span><span>
</span><span style="color: #0000FF">destroy</span><span style="color: #000000"> first</span><span>
</span><span>
</span><span style="color: #000000">resources.insert(at: </span><span style="color: #09885A">0</span><span style="color: #000000">, &#x3C;-create R())</span><span>
</span><span style="color: #008000">// `resources.length` is `2`</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: The statement ignores the result of the call to `remove`,</span><span>
</span><span style="color: #008000">// which would result in a loss.</span><span>
</span><span style="color: #000000">resource.remove(at: </span><span style="color: #09885A">0</span><span style="color: #000000">)</span><span>
</span><span>
</span><span style="color: #0000FF">destroy</span><span style="color: #000000"> resources</span><span>
</span></pre></code>

The variable array function `contains` is not available, as it is impossible:
If the resource can be passed to the `contains` function,
it is by definition not in the array.

The variable array function `concat` is not available,
as it would result in the duplication of resources.

The dictionary functions like `insert` and `remove`
behave like for non-resource dictionaries.
Note however, that the result of these functions must be used.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> resources &#x3C;- {</span><span style="color: #A31515">"r1"</span><span style="color: #000000">: &#x3C;-</span><span style="color: #0000FF">create</span><span style="color: #000000"> R()}</span><span>
</span><span style="color: #008000">// `resources.length` is `1`</span><span>
</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> first &#x3C;- resource.remove(key: </span><span style="color: #A31515">"r1"</span><span style="color: #000000">)</span><span>
</span><span style="color: #008000">// `resources.length` is `0`</span><span>
</span><span style="color: #0000FF">destroy</span><span style="color: #000000"> first</span><span>
</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> old &#x3C;- resources.insert(key: </span><span style="color: #A31515">"r1"</span><span style="color: #000000">, &#x3C;-create R())</span><span>
</span><span style="color: #008000">// `old` is nil, as there was no value for the key "r1"</span><span>
</span><span style="color: #008000">// `resources.length` is `1`</span><span>
</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> old2 &#x3C;- resources.insert(key: </span><span style="color: #A31515">"r1"</span><span style="color: #000000">, &#x3C;-create R())</span><span>
</span><span style="color: #008000">// `old2` is the old value for the key "r1"</span><span>
</span><span style="color: #008000">// `resources.length` is `2`</span><span>
</span><span>
</span><span style="color: #0000FF">destroy</span><span style="color: #000000"> old</span><span>
</span><span style="color: #0000FF">destroy</span><span style="color: #000000"> old2</span><span>
</span><span style="color: #0000FF">destroy</span><span style="color: #000000"> resources</span><span>
</span></pre></code>

### [](#unbound-references--nulls)Unbound References / Nulls

There is **no** support for `null`.

### [](#inheritance-and-abstract-types)Inheritance and Abstract Types

There is **no** support for inheritance.
Inheritance is a feature common in other programming languages,
that allows including the fields and functions of one type in another type.

Instead, follow the &quot;composition over inheritance&quot; principle,
the idea of composing functionality from multiple individual parts,
rather than building an inheritance tree.

Furthermore, there is also **no** support for abstract types.
An abstract type is a feature common in other programming languages,
that prevents creating values of the type and only
allows the creation of values of a subtype.
In addition, abstract types may declare functions,
but omit the implementation of them
and instead require subtypes to implement them.

Instead, consider using [interfaces](#interfaces).

## [](#access-control)Access control

Access control allows making certain parts of the program accessible/visible
and making other parts inaccessible/invisible.

In Flow and Cadence, there are two types of access control:

1.  Access control on objects in account storage using capability security.

    Within Flow, a caller is not able to access an object
    unless it owns the object or has a specific reference to that object.
    This means that nothing is truly public by default.
    Other accounts can not read or write the objects in an account
    unless the owner of the account has granted them access
    by providing references to the objects.

2.  Access control within contracts and objects
    using `pub` and `access` keywords.

    For the explanations of the following keywords, we assume that
    the defining type is either a contract, where capability security
    doesn&#x27;t apply, or that the caller would have valid access to the object
    governed by capability security.

The high-level reference-based security (point 1 above)
will be covered in a later section.

Top-level declarations
(variables, constants, functions, structures, resources, interfaces)
and fields (in structures, and resources) are always only able to be written
to in the scope where it is defined (self).

There are four levels of access control defined in the code that specify where
a declaration can be accessed or called.

-   **Public** or **access(all)** means the declaration
    is accessible/visible in all scopes.

    This includes the current scope, inner scopes, and the outer scopes.

    For example, a public field in a type can be accessed using the access syntax
    on an instance of the type in an outer scope.
    This does not allow the declaration to be publicly writable though.

    An element is made public by using the `pub` or `access(all)` keywords.

-   **access(account)** means the declaration is only accessible/visible in the
    scope of the entire account where it is defined. This means that
    other contracts in the account are able to access it,

    An element is specified with account access
    by using the `access(account)` keyword.

-   **access(contract)** means the declaration is only accessible/visible in the
    scope of the contract that defined it. This means that other types
    and functions that are defined in the same contract can access it,
    but not other contracts in the same account.

    An element is specified with contract access
    by using the `access(contract)` keyword.

-   Private or **access(self)** means the declaration is only accessible/visible
    in the current and inner scopes.

    For example, an `access(self)` field can only be
    accessed by functions of the type is part of,
    not by code in an outer scope.

    This level is specified by using the `access(self)` keyword.

**Access level must be specified for each declaration**

The `(set)` suffix can be used to make variables also publicly writable.

To summarize the behavior for variable declarations, constant declarations, and fields:

| Declaration kind | Access modifier     | Read scope                            | Write scope       |
| :--------------- | :------------------ | :------------------------------------ | :---------------- |
| `let`            | `access(self)`      | Current and inner                     | _None_            |
| `let`            | `access(contract)`  | Current, inner, and its contract      | _None_            |
| `let`            | `access(account)`   | Current, inner, and account contracts | _None_            |
| `let`            | `pub`,`access(all)` | **All**                               | _None_            |
| `var`            | `access(self)`      | Current and inner                     | Current and inner |
| `var`            | `access(contract)`  | Current, inner, and its contract      | Current and inner |
| `var`            | `access(account)`   | Current, inner, and account contracts | Current and inner |
| `var`            | `pub`,`access(all)` | **All**                               | Current and inner |
| `var`            | `pub(set)`          | **All**                               | **All**           |

To summarize the behavior for functions, structures, resources, and interfaces:

| Declaration kind                                                  | Access modifier     | Access scope                          |
| :---------------------------------------------------------------- | :------------------ | :------------------------------------ |
| `fun`,`struct`,`resource`,`struct interface`,`resource interface` | `access(self)`      | Current and inner                     |
| `fun`,`struct`,`resource`,`struct interface`,`resource interface` | `access(contract)`  | Current, inner, and its contract      |
| `fun`,`struct`,`resource`,`struct interface`,`resource interface` | `access(account)`   | Current, inner, and account contracts |
| `fun`,`struct`,`resource`,`struct interface`,`resource interface` | `pub`,`access(all)` | **All**                               |

Currently, all contract defined types must have an access declaration, but
only code within the [contract](#contracts) in which the type is declared
is allowed to create instances of the type.
See the linked contracts section for more information.

<code><pre><span style="color: #008000">// Declare a private constant, inaccessible/invisible in outer scope.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">access(</span><span style="color: #0000FF">self</span><span style="color: #000000">) </span><span style="color: #0000FF">let</span><span style="color: #000000"> a = </span><span style="color: #09885A">1</span><span>
</span><span>
</span><span style="color: #008000">// Declare a public constant, accessible/visible in all scopes.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">let</span><span style="color: #000000"> b = </span><span style="color: #09885A">2</span><span>
</span></pre></code>

<code><pre><span style="color: #008000">// Declare a public struct, accessible/visible in all scopes.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">struct</span><span style="color: #000000"> SomeStruct {</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Declare a private constant field which is only readable</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// in the current and inner scopes.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    access(</span><span style="color: #0000FF">self</span><span style="color: #000000">) </span><span style="color: #0000FF">let</span><span style="color: #000000"> a: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Declare a public constant field which is readable in all scopes.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">let</span><span style="color: #000000"> b: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Declare a private variable field which is only readable</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// and writable in the current and inner scopes.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    access(</span><span style="color: #0000FF">self</span><span style="color: #000000">) </span><span style="color: #0000FF">var</span><span style="color: #000000"> c: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Declare a public variable field which is not settable,</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// so it is only writable in the current and inner scopes,</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// and readable in all scopes.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">var</span><span style="color: #000000"> d: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Declare a public variable field which is settable,</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// so it is readable and writable in all scopes.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    pub(set) </span><span style="color: #0000FF">var</span><span style="color: #000000"> e: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// The initializer is omitted for brevity.</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Declare a private function which is only callable</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// in the current and inner scopes.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    access(</span><span style="color: #0000FF">self</span><span style="color: #000000">) </span><span style="color: #0000FF">fun</span><span style="color: #000000"> privateTest() {</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">// ...</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Declare a public function which is callable in all scopes.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> privateTest() {</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">// ...</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// The initializer is omitted for brevity.</span><span>
</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> some = SomeStruct()</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: cannot read private constant field in outer scope.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">some.a</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: cannot set private constant field in outer scope.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">some.a = </span><span style="color: #09885A">1</span><span>
</span><span>
</span><span style="color: #008000">// Valid: can read public constant field in outer scope.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">some.b</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: cannot set public constant field in outer scope.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">some.b = </span><span style="color: #09885A">2</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: cannot read private variable field in outer scope.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">some.c</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: cannot set private variable field in outer scope.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">some.c = </span><span style="color: #09885A">3</span><span>
</span><span>
</span><span style="color: #008000">// Valid: can read public variable field in outer scope.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">some.d</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: cannot set public variable field in outer scope.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">some.d = </span><span style="color: #09885A">4</span><span>
</span><span>
</span><span style="color: #008000">// Valid: can read publicly settable variable field in outer scope.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">some.e</span><span>
</span><span>
</span><span style="color: #008000">// Valid: can set publicly settable variable field in outer scope.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">some.e = </span><span style="color: #09885A">5</span><span>
</span></pre></code>

## [](#interfaces)Interfaces

An interface is an abstract type that specifies the behavior of types
that _implement_ the interface.
Interfaces declare the required functions and fields,
the access control for those declarations,
and preconditions and postconditions that implementing types need to provide.

There are three kinds of interfaces:

-   **Structure interfaces**: implemented by [structures](#structures)
-   **Resource interfaces**: implemented by [resources](#resources)
-   **Contract interfaces**: implemented by [contracts](#contracts)

Structure, resource, and contract types may implement multiple interfaces.

Nominal typing applies to composite types that implement interfaces.
This means that a type only implements an interface
if it has explicitly declared it.

Interfaces consist of the function and field requirements
that a type implementing the interface must provide implementations for.
Interface requirements, and therefore also their implementations,
must always be at least public.

Variable field requirements may be annotated
to require them to be publicly settable.

Function requirements consist of the name of the function,
parameter types, an optional return type,
and optional preconditions and postconditions.

Field requirements consist of the name and the type of the field.
Field requirements may optionally declare a getter requirement and a setter requirement,
each with preconditions and postconditions.

Calling functions with preconditions and postconditions on interfaces
instead of concrete implementations can improve the security of a program,
as it ensures that even if implementations change,
some aspects of them will always hold.

### [](#interface-declaration)Interface Declaration

Interfaces are declared using the `struct`, `resource`, or `contract` keyword,
followed by the `interface` keyword,
the name of the interface,
and the requirements, which must be enclosed in opening and closing braces.

Field requirements can be annotated to
require the implementation to be a variable field, by using the `var` keyword;
require the implementation to be a constant field, by using the `let` keyword;
or the field requirement may specify nothing,
in which case the implementation may either be
a variable field, a constant field, or a synthetic field.

Field requirements and function requirements must specify the required level of access.
The access must be at least be public, so the `pub` keyword must be provided.
Variable field requirements can be specified to also be publicly settable
by using the `pub(set)` keyword.

Interfaces can be used in types.
This is explained in detail in the section [Interfaces in Types](#interfaces-in-types).
For now, the syntax `{I}` can be read as the type of any value that implements the interface `I`.

<code><pre><span style="color: #008000">// Declare a resource interface for a fungible token.</span><span>
</span><span style="color: #008000">// Only resources can implement this resource interface.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">resource interface</span><span style="color: #000000"> FungibleToken {</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Require the implementing type to provide a field for the balance</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// that is readable in all scopes (`pub`).</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Neither the `var` keyword, nor the `let` keyword is used,</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// so the field may be implemented as either a variable field,</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// a constant field, or a synthetic field.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// The read balance must always be positive.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// NOTE: no requirement is made for the kind of field,</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// it can be either variable or constant in the implementation.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> balance: Int {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">set</span><span style="color: #000000">(newBalance) {</span><span>
</span><span style="color: #000000">            </span><span style="color: #0000FF">pre</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">                newBalance >= </span><span style="color: #09885A">0</span><span style="color: #000000">:</span><span>
</span><span style="color: #000000">                    </span><span style="color: #A31515">"Balances are always set as non-negative numbers"</span><span>
</span><span style="color: #000000">            }</span><span>
</span><span style="color: #000000">        }</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Require the implementing type to provide an initializer that</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// given the initial balance, must initialize the balance field.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">init</span><span style="color: #000000">(balance: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">pre</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">            balance >= </span><span style="color: #09885A">0</span><span style="color: #000000">:</span><span>
</span><span style="color: #000000">                </span><span style="color: #A31515">"Balances are always non-negative"</span><span>
</span><span style="color: #000000">        }</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">post</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">            </span><span style="color: #0000FF">self</span><span style="color: #000000">.balance == balance:</span><span>
</span><span style="color: #000000">                </span><span style="color: #A31515">"the balance must be initialized to the initial balance"</span><span>
</span><span style="color: #000000">        }</span><span>
</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">// NOTE: The declaration contains no implementation code.</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Require the implementing type to provide a function that is</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// callable in all scopes, which withdraws an amount from</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// this fungible token and returns the withdrawn amount as</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// a new fungible token.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// The given amount must be positive and the function implementation</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// must add the amount to the balance.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// The function must return a new fungible token.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// The type `{FungibleToken}` is the type of any resource</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// that implements the resource interface `FungibleToken`.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> withdraw(amount: </span><span style="color: #0000FF">Int</span><span style="color: #000000">): @{FungibleToken} {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">pre</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">            amount > </span><span style="color: #09885A">0</span><span style="color: #000000">:</span><span>
</span><span style="color: #000000">                </span><span style="color: #A31515">"the amount must be positive"</span><span>
</span><span style="color: #000000">            amount &#x3C;= </span><span style="color: #0000FF">self</span><span style="color: #000000">.balance:</span><span>
</span><span style="color: #000000">                </span><span style="color: #A31515">"insufficient funds: the amount must be smaller or equal to the balance"</span><span>
</span><span style="color: #000000">        }</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">post</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">            </span><span style="color: #0000FF">self</span><span style="color: #000000">.balance == before(</span><span style="color: #0000FF">self</span><span style="color: #000000">.balance) - amount:</span><span>
</span><span style="color: #000000">                </span><span style="color: #A31515">"the amount must be deducted from the balance"</span><span>
</span><span style="color: #000000">        }</span><span>
</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">// NOTE: The declaration contains no implementation code.</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Require the implementing type to provide a function that is</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// callable in all scopes, which deposits a fungible token</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// into this fungible token.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// No precondition is required to check the given token's balance</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// is positive, as this condition is already ensured by</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// the field requirement.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// The parameter type `{FungibleToken}` is the type of any resource</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// that implements the resource interface `FungibleToken`.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> deposit(_ token: @{</span><span style="color: #0000FF">FungibleToken</span><span style="color: #000000">}) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">post</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">            </span><span style="color: #0000FF">self</span><span style="color: #000000">.balance == before(</span><span style="color: #0000FF">self</span><span style="color: #000000">.balance) + token.balance:</span><span>
</span><span style="color: #000000">                </span><span style="color: #A31515">"the amount must be added to the balance"</span><span>
</span><span style="color: #000000">        }</span><span>
</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">// NOTE: The declaration contains no implementation code.</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

Note that the required initializer and functions do not have any executable code.

Struct and resource Interfaces can only be declared directly inside contracts,
i.e. not inside of functions.
Contract interfaces can only be declared globally and not inside contracts.

### [](#interface-implementation)Interface Implementation

Declaring that a type implements (conforms) to an interface
is done in the type declaration of the composite type (e.g., structure, resource):
The kind and the name of the composite type is followed by a colon (`:`)
and the name of one or more interfaces that the composite type implements.

This will tell the checker to enforce any requirements from the specified interfaces
onto the declared type.

A type implements (conforms to) an interface if it declares
the implementation in its signature, provides field declarations
for all fields required by the interface,
and provides implementations for all functions required by the interface.

The field declarations in the implementing type must match the field requirements
in the interface in terms of name, type, and declaration kind (e.g. constant, variable)
if given. For example, an interface may require a field with a certain name and type,
but leaves it to the implementation what kind the field is.

The function implementations must match the function requirements in the interface
in terms of name, parameter argument labels, parameter types, and the return type.

<code><pre><span style="color: #008000">// Declare a resource named `ExampleToken` that has to implement</span><span>
</span><span style="color: #008000">// the `FungibleToken` interface.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// It has a variable field named `balance`, that can be written</span><span>
</span><span style="color: #008000">// by functions of the type, but outer scopes can only read it.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">resource</span><span style="color: #000000"> ExampleToken: FungibleToken {</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Implement the required field `balance` for the `FungibleToken` interface.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// The interface does not specify if the field must be variable, constant,</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// so in order for this type (`ExampleToken`) to be able to write to the field,</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// but limit outer scopes to only read from the field, it is declared variable,</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// and only has public access (non-settable).</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">var</span><span style="color: #000000"> balance: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Implement the required initializer for the `FungibleToken` interface:</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// accept an initial balance and initialize the `balance` field.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// This implementation satisfies the required postcondition.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// NOTE: the postcondition declared in the interface</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// does not have to be repeated here in the implementation.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">init</span><span style="color: #000000">(balance: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.balance = balance</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Implement the required function named `withdraw` of the interface</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// `FungibleToken`, that withdraws an amount from the token's balance.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// The function must be public.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// This implementation satisfies the required postcondition.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// NOTE: neither the precondition nor the postcondition declared</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// in the interface have to be repeated here in the implementation.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> withdraw(amount: </span><span style="color: #0000FF">Int</span><span style="color: #000000">): @</span><span style="color: #0000FF">ExampleToken</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.balance = </span><span style="color: #0000FF">self</span><span style="color: #000000">.balance - amount</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">return</span><span style="color: #000000"> </span><span style="color: #0000FF">create</span><span style="color: #000000"> ExampleToken(balance: amount)</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Implement the required function named `deposit` of the interface</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// `FungibleToken`, that deposits the amount from the given token</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// to this token.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// The function must be public.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// NOTE: the type of the parameter is `{FungibleToken}`,</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// i.e., any resource that implements the resource interface `FungibleToken`,</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// so any other token – however, we want to ensure that only tokens</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// of the same type can be deposited.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// This implementation satisfies the required postconditions.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// NOTE: neither the precondition nor the postcondition declared</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// in the interface have to be repeated here in the implementation.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> deposit(_ token: @{</span><span style="color: #0000FF">FungibleToken</span><span style="color: #000000">}) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">if</span><span style="color: #000000"> </span><span style="color: #0000FF">let</span><span style="color: #000000"> exampleToken = token as? ExampleToken {</span><span>
</span><span style="color: #000000">            </span><span style="color: #0000FF">self</span><span style="color: #000000">.balance = </span><span style="color: #0000FF">self</span><span style="color: #000000">.balance + exampleToken.balance</span><span>
</span><span style="color: #000000">            </span><span style="color: #0000FF">destroy</span><span style="color: #000000"> exampleToken</span><span>
</span><span style="color: #000000">        } </span><span style="color: #0000FF">else</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">            panic(</span><span style="color: #A31515">"cannot deposit token which is not an example token"</span><span style="color: #000000">)</span><span>
</span><span style="color: #000000">        }</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Declare a constant which has type `ExampleToken`,</span><span>
</span><span style="color: #008000">// and is initialized with such an example token.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> token &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> ExampleToken(balance: </span><span style="color: #09885A">100</span><span style="color: #000000">)</span><span>
</span><span>
</span><span style="color: #008000">// Withdraw 10 units from the token.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// The amount satisfies the precondition of the `withdraw` function</span><span>
</span><span style="color: #008000">// in the `FungibleToken` interface.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// Invoking a function of a resource does not destroy the resource,</span><span>
</span><span style="color: #008000">// so the resource `token` is still valid after the call of `withdraw`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> withdrawn &#x3C;- token.withdraw(amount: </span><span style="color: #09885A">10</span><span style="color: #000000">)</span><span>
</span><span>
</span><span style="color: #008000">// The postcondition of the `withdraw` function in the `FungibleToken`</span><span>
</span><span style="color: #008000">// interface ensured the balance field of the token was updated properly.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// `token.balance` is `90`</span><span>
</span><span style="color: #008000">// `withdrawn.balance` is `10`</span><span>
</span><span>
</span><span style="color: #008000">// Deposit the withdrawn token into another one.</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> receiver: @</span><span style="color: #0000FF">ExampleToken</span><span style="color: #000000"> &#x3C;- </span><span style="color: #008000">// ...</span><span>
</span><span style="color: #000000">receiver.deposit(&#x3C;-withdrawn)</span><span>
</span><span>
</span><span style="color: #008000">// Run-time error: The precondition of function `withdraw` in interface</span><span>
</span><span style="color: #008000">// `FungibleToken` fails, the program aborts: the parameter `amount`</span><span>
</span><span style="color: #008000">// is larger than the field `balance` (100 > 90).</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">token.withdraw(amount: </span><span style="color: #09885A">100</span><span style="color: #000000">)</span><span>
</span><span>
</span><span style="color: #008000">// Withdrawing tokens so that the balance is zero does not destroy the resource.</span><span>
</span><span style="color: #008000">// The resource has to be destroyed explicitly.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">token.withdraw(amount: </span><span style="color: #09885A">90</span><span style="color: #000000">)</span><span>
</span></pre></code>

The access level for variable fields in an implementation
may be less restrictive than the interface requires.
For example, an interface may require a field to be
at least public (i.e. the `pub` keyword is specified),
and an implementation may provide a variable field which is public,
but also publicly settable (the `pub(set)` keyword is specified).

<code><pre><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">struct interface</span><span style="color: #000000"> AnInterface {</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Require the implementing type to provide a publicly readable</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// field named `a` that has type `Int`. It may be a constant field,</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// a variable field, or a synthetic field.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> a: Int</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">struct</span><span style="color: #000000"> AnImplementation: AnInterface {</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Declare a publicly settable variable field named `a` that has type `Int`.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// This implementation satisfies the requirement for interface `AnInterface`:</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// The field is at least publicly readable, but this implementation also</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// allows the field to be written to in all scopes.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    pub(set) </span><span style="color: #0000FF">var</span><span style="color: #000000"> a: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">init</span><span style="color: #000000">(a: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.a = a</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

### [](#interfaces-in-types)Interfaces in Types

Interfaces can be used in types: The type `{I}` is the type of all objects
that implement the interfaace `I`.

This is called a [restricted type](#restricted-types):
Only the functionality (members and functions) of the interface can be used
when accessing a value of such a type.

<code><pre><span style="color: #008000">// Declare an interface named `Shape`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// Require implementing types to provide a field which returns the area,</span><span>
</span><span style="color: #008000">// and a function which scales the shape by a given factor.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">struct interface</span><span style="color: #000000"> Shape {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> getArea(): </span><span style="color: #0000FF">Int</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> scale(factor: </span><span style="color: #0000FF">Int</span><span style="color: #000000">)</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Declare a structure named `Square` the implements the `Shape` interface.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">struct</span><span style="color: #000000"> Square: Shape {</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// In addition to the required fields from the interface,</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// the type can also declare additional fields.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">var</span><span style="color: #000000"> length: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Provided the field `area`  which is required to conform</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// to the interface `Shape`.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Since `area` was not declared as a constant, variable,</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// field in the interface, it can be declared.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> getArea(): </span><span style="color: #0000FF">Int</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">return</span><span style="color: #000000"> </span><span style="color: #0000FF">self</span><span style="color: #000000">.length * </span><span style="color: #0000FF">self</span><span style="color: #000000">.length</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">init</span><span style="color: #000000">(length: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.length = length</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Provided the implementation of the function `scale`</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// which is required to conform to the interface `Shape`.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> scale(factor: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.length = </span><span style="color: #0000FF">self</span><span style="color: #000000">.length * factor</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Declare a structure named `Rectangle` that also implements the `Shape` interface.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">struct</span><span style="color: #000000"> Rectangle: Shape {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">var</span><span style="color: #000000"> width: </span><span style="color: #0000FF">Int</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">var</span><span style="color: #000000"> height: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Provided the field `area  which is required to conform</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// to the interface `Shape`.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> getArea(): </span><span style="color: #0000FF">Int</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">return</span><span style="color: #000000"> </span><span style="color: #0000FF">self</span><span style="color: #000000">.width * </span><span style="color: #0000FF">self</span><span style="color: #000000">.height</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">init</span><span style="color: #000000">(width: </span><span style="color: #0000FF">Int</span><span style="color: #000000">, height: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.width = width</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.height = height</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Provided the implementation of the function `scale`</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// which is required to conform to the interface `Shape`.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> scale(factor: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.width = </span><span style="color: #0000FF">self</span><span style="color: #000000">.width * factor</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.height = </span><span style="color: #0000FF">self</span><span style="color: #000000">.height * factor</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Declare a constant that has type `Shape`, which has a value that has type `Rectangle`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">var</span><span style="color: #000000"> shape: {</span><span style="color: #0000FF">Shape</span><span style="color: #000000">} = Rectangle(width: </span><span style="color: #09885A">10</span><span style="color: #000000">, height: </span><span style="color: #09885A">20</span><span style="color: #000000">)</span><span>
</span></pre></code>

Values implementing an interface are assignable to variables that have the interface as their type.

<code><pre><span style="color: #008000">// Assign a value of type `Square` to the variable `shape` that has type `Shape`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">shape = Square(length: </span><span style="color: #09885A">30</span><span style="color: #000000">)</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: cannot initialize a constant that has type `Rectangle`.</span><span>
</span><span style="color: #008000">// with a value that has type `Square`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> rectangle: </span><span style="color: #0000FF">Rectangle</span><span style="color: #000000"> = Square(length: </span><span style="color: #09885A">10</span><span style="color: #000000">)</span><span>
</span></pre></code>

Fields declared in an interface can be accessed
and functions declared in an interface
can be called on values of a type that implements the interface.

<code><pre><span style="color: #008000">// Declare a constant which has the type `Shape`.</span><span>
</span><span style="color: #008000">// and is initialized with a value that has type `Rectangle`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> shape: {</span><span style="color: #0000FF">Shape</span><span style="color: #000000">} = Rectangle(width: </span><span style="color: #09885A">2</span><span style="color: #000000">, height: </span><span style="color: #09885A">3</span><span style="color: #000000">)</span><span>
</span><span>
</span><span style="color: #008000">// Access the field `area` declared in the interface `Shape`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">shape.area  </span><span style="color: #008000">// is `6`</span><span>
</span><span>
</span><span style="color: #008000">// Call the function `scale` declared in the interface `Shape`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">shape.scale(factor: </span><span style="color: #09885A">3</span><span style="color: #000000">)</span><span>
</span><span>
</span><span style="color: #000000">shape.area  </span><span style="color: #008000">// is `54`</span><span>
</span></pre></code>

### [](#interface-implementation-requirements)Interface Implementation Requirements

Interfaces can require implementing types
to also implement other interfaces of the same kind.
Interface implementation requirements can be declared
by following the interface name with a colon (`:`)
and one or more names of interfaces of the same kind, separated by commas.

<code><pre><span style="color: #008000">// Declare a structure interface named `Shape`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">struct interface</span><span style="color: #000000"> Shape {}</span><span>
</span><span>
</span><span style="color: #008000">// Declare a structure interface named `Polygon`.</span><span>
</span><span style="color: #008000">// Require implementing types to also implement the structure interface `Shape`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">struct interface</span><span style="color: #000000"> Polygon: Shape {}</span><span>
</span><span>
</span><span style="color: #008000">// Declare a structure named `Hexagon` that implements the `Polygon` interface.</span><span>
</span><span style="color: #008000">// This also is required to implement the `Shape` interface,</span><span>
</span><span style="color: #008000">// because the `Polygon` interface requires it.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">struct</span><span style="color: #000000"> Hexagon: Polygon {}</span><span>
</span></pre></code>

### [](#interface-nesting)Interface Nesting

> 🚧 Status: Currently only contracts and contract interfaces support nested interfaces.

Interfaces can be arbitrarily nested.
Declaring an interface inside another does not require implementing types
of the outer interface to provide an implementation of the inner interfaces.

<code><pre><span style="color: #008000">// Declare a resource interface `OuterInterface`, which declares</span><span>
</span><span style="color: #008000">// a nested structure interface named `InnerInterface`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// Resources implementing `OuterInterface` do not need to provide</span><span>
</span><span style="color: #008000">// an implementation of `InnerInterface`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// Structures may just implement `InnerInterface`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">resource interface</span><span style="color: #000000"> OuterInterface {</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">struct interface</span><span style="color: #000000"> InnerInterface {}</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Declare a resource named `SomeOuter` that implements the interface `OuterInterface`</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// The resource is not required to implement `OuterInterface.InnerInterface`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">resource</span><span style="color: #000000"> SomeOuter: OuterInterface {}</span><span>
</span><span>
</span><span style="color: #008000">// Declare a structure named `SomeInner` that implements `InnerInterface`,</span><span>
</span><span style="color: #008000">// which is nested in interface `OuterInterface`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">struct</span><span style="color: #000000"> SomeInner: OuterInterface.InnerInterface {}</span><span>
</span><span>
</span></pre></code>

### [](#nested-type-requirements)Nested Type Requirements

> 🚧 Status: Currently only contracts and contract interfaces support nested type requirements.

Interfaces can require implementing types to provide concrete nested types.
For example, a resource interface may require an implementing type to provide a resource type.

<code><pre><span style="color: #008000">// Declare a resource interface named `FungibleToken`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// Require implementing types to provide a resource type named `Vault`</span><span>
</span><span style="color: #008000">// which must have a field named `balance`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">resource interface</span><span style="color: #000000"> FungibleToken {</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">resource</span><span style="color: #000000"> Vault {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">pub</span><span style="color: #000000"> balance: Int</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Declare a resource named `ExampleToken` that implements the `FungibleToken` interface.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// The nested type `Vault` must be provided to conform to the interface.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">resource</span><span style="color: #000000"> ExampleToken: FungibleToken {</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">resource</span><span style="color: #000000"> Vault {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">var</span><span style="color: #000000"> balance: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">init</span><span style="color: #000000">(balance: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">            </span><span style="color: #0000FF">self</span><span style="color: #000000">.balance = balance</span><span>
</span><span style="color: #000000">        }</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

### [](#equatable-interface)`Equatable` Interface

> 🚧 Status: The `Equatable` interface is not implemented yet.

An equatable type is a type that can be compared for equality.
Types are equatable when they  implement the `Equatable` interface.

Equatable types can be compared for equality using the equals operator (`==`)
or inequality using the unequals operator (`!=`).

Most of the built-in types are equatable, like booleans and integers.
Arrays are equatable when their elements are equatable.
Dictionaries are equatable when their values are equatable.

To make a type equatable the `Equatable` interface must be implemented,
which requires the implementation of the function `equals`,
which accepts another value that the given value should be compared for equality.

<code><pre><span style="color: #0000FF">struct interface</span><span style="color: #000000"> Equatable {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> equals(_ other: {</span><span style="color: #0000FF">Equatable</span><span style="color: #000000">}): </span><span style="color: #0000FF">Bool</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

<code><pre><span style="color: #008000">// Declare a struct named `Cat`, which has one field named `id`</span><span>
</span><span style="color: #008000">// that has type `Int`, i.e., the identifier of the cat.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// `Cat` also will implement the interface `Equatable`</span><span>
</span><span style="color: #008000">// to allow cats to be compared for equality.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">struct</span><span style="color: #000000"> Cat: Equatable {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">let</span><span style="color: #000000"> id: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">init</span><span style="color: #000000">(id: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.id = id</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> equals(_ other: {</span><span style="color: #0000FF">Equatable</span><span style="color: #000000">}): </span><span style="color: #0000FF">Bool</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">if</span><span style="color: #000000"> </span><span style="color: #0000FF">let</span><span style="color: #000000"> otherCat = other as? Cat {</span><span>
</span><span style="color: #000000">            </span><span style="color: #008000">// Cats are equal if their identifier matches.</span><span>
</span><span style="color: #000000">            </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">            </span><span style="color: #0000FF">return</span><span style="color: #000000"> otherCat.id == </span><span style="color: #0000FF">self</span><span style="color: #000000">.id</span><span>
</span><span style="color: #000000">        } </span><span style="color: #0000FF">else</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">            </span><span style="color: #0000FF">return</span><span style="color: #000000"> </span><span style="color: #0000FF">false</span><span>
</span><span style="color: #000000">        }</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #000000">Cat(</span><span style="color: #09885A">1</span><span style="color: #000000">) == Cat(</span><span style="color: #09885A">2</span><span style="color: #000000">)  </span><span style="color: #008000">// is `false`</span><span>
</span><span style="color: #000000">Cat(</span><span style="color: #09885A">3</span><span style="color: #000000">) == Cat(</span><span style="color: #09885A">3</span><span style="color: #000000">)  </span><span style="color: #008000">// is `true`</span><span>
</span></pre></code>

### [](#hashable-interface)`Hashable` Interface

> 🚧 Status: The `Hashable` interface is not implemented yet.

A hashable type is a type that can be hashed to an integer hash value,
i.e., it is distilled into a value that is used as evidence of inequality.
Types are hashable when they implement the `Hashable` interface.

Hashable types can be used as keys in dictionaries.

Hashable types must also be equatable,
i.e., they must also implement the `Equatable` interface.
This is because the hash value is only evidence for inequality:
two values that have different hash values are guaranteed to be unequal.
However, if the hash values of two values are the same,
then the two values could still be unequal
and just happen to hash to the same hash value.
In that case equality still needs to be determined through an equality check.
Without `Equatable`, values could be added to a dictionary,
but it would not be possible to retrieve them.

Most of the built-in types are hashable, like booleans and integers.
Arrays are hashable when their elements are hashable.
Dictionaries are hashable when their values are equatable.

Hashing a value means passing its essential components into a hash function.
Essential components are those that are used in the type&#x27;s implementation of `Equatable`.

If two values are equal because their `equals` function returns true,
then the implementation must return the same integer hash value for each of the two values.

The implementation must also consistently return the same integer hash value during the execution
of the program when the essential components have not changed.
The integer hash value must not necessarily be the same across multiple executions.

<code><pre><span style="color: #0000FF">struct interface</span><span style="color: #000000"> Hashable: Equatable {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> hashValue: Int</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

<code><pre><span style="color: #008000">// Declare a structure named `Point` with two fields</span><span>
</span><span style="color: #008000">// named `x` and `y` that have type `Int`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// `Point` is declared to implement the `Hashable` interface,</span><span>
</span><span style="color: #008000">// which also means it needs to implement the `Equatable` interface.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">struct</span><span style="color: #000000"> Point: Hashable {</span><span>
</span><span>
</span><span style="color: #000000">    pub(set) </span><span style="color: #0000FF">var</span><span style="color: #000000"> x: </span><span style="color: #0000FF">Int</span><span>
</span><span style="color: #000000">    pub(set) </span><span style="color: #0000FF">var</span><span style="color: #000000"> y: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">init</span><span style="color: #000000">(x: </span><span style="color: #0000FF">Int</span><span style="color: #000000">, y: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.x = x</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.y = y</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Implementing the function `equals` will allow points to be compared</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// for equality and satisfies the `Equatable` interface.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> equals(_ other: {</span><span style="color: #0000FF">Equatable</span><span style="color: #000000">}): </span><span style="color: #0000FF">Bool</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">if</span><span style="color: #000000"> </span><span style="color: #0000FF">let</span><span style="color: #000000"> otherPoint = other as? Point {</span><span>
</span><span style="color: #000000">            </span><span style="color: #008000">// Points are equal if their coordinates match.</span><span>
</span><span style="color: #000000">            </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">            </span><span style="color: #008000">// The essential components are therefore the fields `x` and `y`,</span><span>
</span><span style="color: #000000">            </span><span style="color: #008000">// which must be used in the implementation of the field requirement</span><span>
</span><span style="color: #000000">            </span><span style="color: #008000">// `hashValue` of the `Hashable` interface.</span><span>
</span><span style="color: #000000">            </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">            </span><span style="color: #0000FF">return</span><span style="color: #000000"> otherPoint.x == </span><span style="color: #0000FF">self</span><span style="color: #000000">.x</span><span>
</span><span style="color: #000000">                &#x26;&#x26; otherPoint.y == </span><span style="color: #0000FF">self</span><span style="color: #000000">.y</span><span>
</span><span style="color: #000000">        } </span><span style="color: #0000FF">else</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">            </span><span style="color: #0000FF">return</span><span style="color: #000000"> </span><span style="color: #0000FF">false</span><span>
</span><span style="color: #000000">        }</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Providing an implementation for the hash value field</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// satisfies the `Hashable` interface.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> synthetic hashValue: Int {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">get</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">            </span><span style="color: #008000">// Calculate a hash value based on the essential components,</span><span>
</span><span style="color: #000000">            </span><span style="color: #008000">// the fields `x` and `y`.</span><span>
</span><span style="color: #000000">            </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">            </span><span style="color: #0000FF">var</span><span style="color: #000000"> hash = </span><span style="color: #09885A">7</span><span>
</span><span style="color: #000000">            hash = </span><span style="color: #09885A">31</span><span style="color: #000000"> * hash + </span><span style="color: #0000FF">self</span><span style="color: #000000">.x</span><span>
</span><span style="color: #000000">            hash = </span><span style="color: #09885A">31</span><span style="color: #000000"> * hash + </span><span style="color: #0000FF">self</span><span style="color: #000000">.y</span><span>
</span><span style="color: #000000">            </span><span style="color: #0000FF">return</span><span style="color: #000000"> hash</span><span>
</span><span style="color: #000000">        }</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

## [](#restricted-types)Restricted Types

Structure and resource types can be **restricted**. Restrictions are interfaces.
Restricted types only allow access to a subset of the members and functions
of the type that is restricted, indicated by the restrictions.

The syntax of a restriced type is `T{U1, U2, ... Un}`,
where `T` is the restricted type, a concrete resource or strucure type,
and the types `U1` to `Un` are the restrictions, interfaces that `T` conforms to.

Only the members and functions of the union of the set of restrictions are available.

Restricted types are useful for increasing the safety in functions
that are suposed to only work on a subset of the type.
For example, by using a restricted type for a parameter&#x27;s type,
the function may only access the functionality of the restriction:
If the function accidentally attempts to access other functionality,
this is prevented by the static checker.

<code><pre><span style="color: #008000">// Declare a resource interface named `HasCount`,</span><span>
</span><span style="color: #008000">// which has a read-only `count` field</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">resource interface</span><span style="color: #000000"> HasCount {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">let</span><span style="color: #000000"> count: </span><span style="color: #0000FF">Int</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Declare a resource named `Counter`, which has a writeable `count` field,</span><span>
</span><span style="color: #008000">// and conforms to the resource interface `HasCount`</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">resource</span><span style="color: #000000"> Counter: HasCount {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">var</span><span style="color: #000000"> count: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">init</span><span style="color: #000000">(count: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.count = count</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> increment() {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.count = </span><span style="color: #0000FF">self</span><span style="color: #000000">.count + </span><span style="color: #09885A">1</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Create an instance of the resource `Counter`</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> counter: @</span><span style="color: #0000FF">Counter</span><span style="color: #000000"> &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> Counter(count: </span><span style="color: #09885A">42</span><span style="color: #000000">)</span><span>
</span><span>
</span><span style="color: #000000">counterRef.count  </span><span style="color: #008000">// is `42`</span><span>
</span><span>
</span><span style="color: #000000">counterRef.increment()</span><span>
</span><span>
</span><span style="color: #000000">counterRef.count  </span><span style="color: #008000">// is `43`</span><span>
</span><span>
</span><span style="color: #008000">// Move the resource in variable `counter` to a new variable `restrictedCounter`,</span><span>
</span><span style="color: #008000">// but typed with the restricted type `Counter{HasCount}`:</span><span>
</span><span style="color: #008000">// The variable may hold any `Counter`, but only the functionality</span><span>
</span><span style="color: #008000">// defined in the given restriction, the interface `HasCount`, may be accessed</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> restrictedCounter: @</span><span style="color: #0000FF">Counter</span><span style="color: #000000">{</span><span style="color: #0000FF">Count</span><span style="color: #000000">} &#x3C;- counter</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Only functionality of restriction `Count` is available,</span><span>
</span><span style="color: #008000">// i.e. the read-only field `count`, but not the function `increment` of `Counter`</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">restrictedCounter.increment()</span><span>
</span><span>
</span><span style="color: #008000">// Move the resource in variable `restrictedCounter` to a new variable `unrestrictedCounter`,</span><span>
</span><span style="color: #008000">// again typed as `Counter`, i.e. all functionality of the counter is available</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> unrestrictedCounter: @</span><span style="color: #0000FF">Counter</span><span style="color: #000000"> &#x3C;- restrictedCounter</span><span>
</span><span>
</span><span style="color: #008000">// Valid: The variable `unrestrictedCounter` has type `Counter`,</span><span>
</span><span style="color: #008000">// so all its functionality is available, including the function `increment`</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">unrestrictedCounter.increment()</span><span>
</span><span>
</span><span style="color: #008000">// Declare another resource type named `Strings`</span><span>
</span><span style="color: #008000">// which implements the resource interface `HasCount`</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">resource</span><span style="color: #000000"> Strings: HasCount {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">var</span><span style="color: #000000"> count: </span><span style="color: #0000FF">Int</span><span>
</span><span style="color: #000000">    access(</span><span style="color: #0000FF">self</span><span style="color: #000000">) </span><span style="color: #0000FF">var</span><span style="color: #000000"> strings: [</span><span style="color: #0000FF">String</span><span style="color: #000000">]</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">init</span><span style="color: #000000">() {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.count = </span><span style="color: #09885A">0</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.strings = []</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> append(_ string: </span><span style="color: #0000FF">String</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.strings.append(string)</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.count = </span><span style="color: #0000FF">self</span><span style="color: #000000">.count + </span><span style="color: #09885A">1</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: The resource type `Strings` is not compatible</span><span>
</span><span style="color: #008000">// with the restricted type `Counter{HasCount}`.</span><span>
</span><span style="color: #008000">// Even though the resource `Strings` implements the resource interface `HasCount`,</span><span>
</span><span style="color: #008000">// it is not compatible with `Counter`</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> counter2: @</span><span style="color: #0000FF">Counter</span><span style="color: #000000">{</span><span style="color: #0000FF">HasCount</span><span style="color: #000000">} &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> Strings()</span><span>
</span></pre></code>

In addition to restricting concrete types is also possible
to restrict the built-in types `AnyStruct`, the supertype of all structures,
and `AnyResource`, the supertype of all resources.
For example, restricted type `AnyResource{HasCount}` is any resource type
for which only the functionality of the `HasCount` resource interface can be used.

The restricted types `AnyStruct` and `AnyResource` can be ommited.
For example, the type `{HasCount}` is any resource that implements
the resource interface `HasCount`.

<code><pre><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">struct interface</span><span style="color: #000000"> HasID {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">let</span><span style="color: #000000"> id: </span><span style="color: #0000FF">String</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">struct</span><span style="color: #000000"> A: HasID {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">let</span><span style="color: #000000"> name: </span><span style="color: #0000FF">String</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">init</span><span style="color: #000000">(name: </span><span style="color: #0000FF">String</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.name = name</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">struct</span><span style="color: #000000"> B: HasID {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">let</span><span style="color: #000000"> name: </span><span style="color: #0000FF">String</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">init</span><span style="color: #000000">(name: </span><span style="color: #0000FF">String</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.name = name</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Create two instances, one of type `A`, and one of type `B`.</span><span>
</span><span style="color: #008000">// Both types conform to interface `HasID`, so the structs can be assigned</span><span>
</span><span style="color: #008000">// to variables with type `AnyResource{HasID}`: Some resource type which only allows</span><span>
</span><span style="color: #008000">// access to the functionality of resource interface `HasID`</span><span>
</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> hasID1: {</span><span style="color: #0000FF">HasID</span><span style="color: #000000">} = A(name: </span><span style="color: #A31515">"1"</span><span style="color: #000000">)</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> hasID2: {</span><span style="color: #0000FF">HasID</span><span style="color: #000000">} = B(name: </span><span style="color: #A31515">"2"</span><span style="color: #000000">)</span><span>
</span><span>
</span><span style="color: #008000">// Declare a function named `getID` which has one parameter with type `{HasID}`.</span><span>
</span><span style="color: #008000">// The type `{HasID}` is a short-hand for `AnyStruct{HasID}`:</span><span>
</span><span style="color: #008000">// Some structure which only allows access to the functionality of interface `HasID`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> getID(_ value: {</span><span style="color: #0000FF">HasID</span><span style="color: #000000">}): </span><span style="color: #0000FF">String</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">return</span><span style="color: #000000"> value.id</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> id1 = getID(hasID1)</span><span>
</span><span style="color: #008000">// `id1` is "1"</span><span>
</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> id2 = getID(hasID2)</span><span>
</span><span style="color: #008000">// `id2` is "2"</span><span>
</span></pre></code>

Only concrete types may be restriced, e.g., the restricted type may not be an array,
the type `[T]{U}` is invalid.

Restricted types are also useful when giving access to resources and structures
to potentially untrusted third-party programs through [references](#references),
which are discussed in the next section.

## [](#references)References

It is possible to create references to objects, i.e. resources or structures.
A reference can be used to access fields and call functions on the referenced object.

References are **copied**, i.e. they are value types.

References are created by using the `&` operator, followed by the object,
the `as` keyword, and the type through which they should be accessed.
The given type must be a supertype of the referenced object&#x27;s type.

References have the type `&T`, where `T` is the type of the referenced object.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> hello = </span><span style="color: #A31515">"Hello"</span><span>
</span><span>
</span><span style="color: #008000">// Create a reference to the "Hello" string, typed as a `String`</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> helloRef: &#x26;</span><span style="color: #0000FF">String</span><span style="color: #000000"> = &#x26;hello as &#x26;String</span><span>
</span><span>
</span><span style="color: #000000">helloRef.length </span><span style="color: #008000">// is `5`</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Cannot create a reference to `hello`</span><span>
</span><span style="color: #008000">// typed as `&#x26;Int`, as it has type `String`</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> intRef: &#x26;</span><span style="color: #0000FF">Int</span><span style="color: #000000"> = &#x26;hello as &#x26;Int</span><span>
</span></pre></code>

References are covariant in their base types.
For example, `&T` is a subtype of `&U`, if `T` is a subtype of `U`.

<code><pre><span>
</span><span style="color: #008000">// Declare a resource interface named `HasCount`,</span><span>
</span><span style="color: #008000">// that has a field `count`</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">resource interface</span><span style="color: #000000"> HasCount {</span><span>
</span><span style="color: #000000">    count: Int</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Declare a resource named `Counter` that conforms to `HasCount`</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">resource</span><span style="color: #000000"> Counter: HasCount {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">var</span><span style="color: #000000"> count: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">init</span><span style="color: #000000">(count: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.count = count</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> increment() {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.count = </span><span style="color: #0000FF">self</span><span style="color: #000000">.count + </span><span style="color: #09885A">1</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Create a new instance of the resource type `Counter`</span><span>
</span><span style="color: #008000">// and create a reference to it, typed as `&#x26;Counter`,</span><span>
</span><span style="color: #008000">// so the reference allows access to all fields and functions</span><span>
</span><span style="color: #008000">// of the counter</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> counter &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> Counter(count: </span><span style="color: #09885A">42</span><span style="color: #000000">)</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> counterRef: &#x26;</span><span style="color: #0000FF">Counter</span><span style="color: #000000"> = &#x26;counter as &#x26;Counter</span><span>
</span><span>
</span><span style="color: #000000">counterRef.count  </span><span style="color: #008000">// is `42`</span><span>
</span><span>
</span><span style="color: #000000">counterRef.increment()</span><span>
</span><span>
</span><span style="color: #000000">counterRef.count  </span><span style="color: #008000">// is `43`</span><span>
</span></pre></code>

References may be **authorized** or **unauthorized**.

Authorized references have the `auth` modifier, i.e. the full syntax is `auth &T`,
whereas unauthorized references do not have a modifier.

Authorized references can be freely upcasted and downcasted,
whereas unauthorized references can only be upcasted.
Also, authorized references are subtypes of unauthorized references.

<code><pre><span>
</span><span style="color: #008000">// Create an unauthorized reference to the counter,</span><span>
</span><span style="color: #008000">// typed with the restricted type `&#x26;{HasCount}`,</span><span>
</span><span style="color: #008000">// i.e. some resource that conforms to the `HasCount` interface</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> countRef: &#x26;{</span><span style="color: #0000FF">HasCount</span><span style="color: #000000">} = &#x26;counter as &#x26;{HasCount}</span><span>
</span><span>
</span><span style="color: #000000">countRef.count  </span><span style="color: #008000">// is `43`</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: The function `increment` is not available</span><span>
</span><span style="color: #008000">// for the type `&#x26;{HasCount}`</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">countRef.increment()</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Cannot failably downcast to reference type `&#x26;Counter`,</span><span>
</span><span style="color: #008000">// as the reference `countRef` is unauthorized.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// The counter value has type `Counter`, which is a subtype of `{HasCount}`,</span><span>
</span><span style="color: #008000">// but as the reference is unauthorized, the cast is not allowed.</span><span>
</span><span style="color: #008000">// It is not possible to "look under the covers"</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> counterRef2: &#x26;</span><span style="color: #0000FF">Counter</span><span style="color: #000000"> = countRef as? &#x26;Counter</span><span>
</span><span>
</span><span style="color: #008000">// Create an authorized reference to the counter,</span><span>
</span><span style="color: #008000">// again with the restricted type `{HasCount}`, i.e. some resource</span><span>
</span><span style="color: #008000">// that conforms to the `HasCount` interface</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> authCountRef: </span><span style="color: #0000FF">auth</span><span style="color: #000000"> &#x26;{</span><span style="color: #0000FF">HasCount</span><span style="color: #000000">} = &#x26;counter as auth &#x26;{HasCount}</span><span>
</span><span>
</span><span style="color: #008000">// Failably downcast to reference type `&#x26;Counter`.</span><span>
</span><span style="color: #008000">// This is valid, because the reference `authCountRef` is authorized</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> counterRef3: &#x26;</span><span style="color: #0000FF">Counter</span><span style="color: #000000"> = authCountRef as? &#x26;Counter</span><span>
</span><span>
</span><span style="color: #000000">counterRef3.count  </span><span style="color: #008000">// is `43`</span><span>
</span><span>
</span><span style="color: #000000">counterRef3.increment()</span><span>
</span><span>
</span><span style="color: #000000">counterRef3.count  </span><span style="color: #008000">// is `44`</span><span>
</span></pre></code>

## [](#imports)Imports

Programs can import declarations (types, functions, variables, etc.) from other programs.

Imports are declared using the `import` keyword.

It can either be followed by a location, which imports all declarations;
or it can be followed by the names of the declarations that should be imported,
followed by the `from` keyword, and then followed by the location.

If importing a local file, the location is a string literal, and the path to the file.

> 🚧 Status: Imports from local files are not currently implemented.

If importing an external type in a different account,
the location is an address literal, and the address
of the account where the declarations are deployed to and published.

<code><pre><span style="color: #008000">// Import the type `Counter` from a local file.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">import</span><span style="color: #000000"> Counter </span><span style="color: #0000FF">from</span><span style="color: #000000"> </span><span style="color: #A31515">"examples/counter.cdc"</span><span>
</span><span>
</span><span style="color: #008000">// Import the type `Counter` from an external account.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">import</span><span style="color: #000000"> Counter </span><span style="color: #0000FF">from</span><span style="color: #000000"> </span><span style="color: #09885A">0x299F20A29311B9248F12</span><span>
</span></pre></code>

## [](#accounts)Accounts

Every account can be accessed through two types:

-   As a **Public Account** with the type `PublicAccount`,
    which represents the publicly available portion of an account.

    <code><pre><span style="color: #0000FF">struct</span><span style="color: #000000"> PublicAccount {</span><span>
    </span><span>
    </span><span style="color: #000000">    </span><span style="color: #0000FF">let</span><span style="color: #000000"> address: </span><span style="color: #0000FF">Address</span><span>
    </span><span>
    </span><span style="color: #000000">    </span><span style="color: #008000">// Storage operations</span><span>
    </span><span>
    </span><span style="color: #000000">    </span><span style="color: #0000FF">fun</span><span style="color: #000000"> getCapability(at: </span><span style="color: #0000FF">Path</span><span style="color: #000000">): </span><span style="color: #0000FF">Capability</span><span style="color: #000000">?</span><span>
    </span><span style="color: #000000">    </span><span style="color: #0000FF">fun</span><span style="color: #000000"> getLinkTarget(_ path: </span><span style="color: #0000FF">Path</span><span style="color: #000000">): </span><span style="color: #0000FF">Path</span><span style="color: #000000">?</span><span>
    </span><span style="color: #000000">}</span><span>
    </span></pre></code>

    Any code can get the `PublicAccount` for an account address
    using the built-in `getAccount` function:

    <code><pre><span style="color: #0000FF">fun</span><span style="color: #000000"> getAccount(_ address: </span><span style="color: #0000FF">Address</span><span style="color: #000000">): </span><span style="color: #0000FF">PublicAccount</span><span>
    </span></pre></code>

-   As an **Authorized Account** with type `AuthAccount`,
    which represents the authorized portion of an account.

    Access to an `AuthAccount` means having full access to its [storage](#account-storage),
    public keys, and code.

    Only [signed transactions](#transactions) can get the `AuthAccount` for an account.
    For each script signer of the transaction, the corresponding `AuthAccount` is passed
    to the `prepare` phase of the transaction.

    <code><pre><span style="color: #0000FF">struct</span><span style="color: #000000"> AuthAccount {</span><span>
    </span><span>
    </span><span style="color: #000000">    </span><span style="color: #0000FF">let</span><span style="color: #000000"> address: </span><span style="color: #0000FF">Address</span><span>
    </span><span>
    </span><span style="color: #000000">    </span><span style="color: #008000">// Contract code</span><span>
    </span><span>
    </span><span style="color: #000000">    </span><span style="color: #0000FF">fun</span><span style="color: #000000"> setCode(_ code: [</span><span style="color: #0000FF">Int</span><span style="color: #000000">])</span><span>
    </span><span>
    </span><span style="color: #000000">    </span><span style="color: #008000">// Key management</span><span>
    </span><span>
    </span><span style="color: #000000">    </span><span style="color: #0000FF">fun</span><span style="color: #000000"> addPublicKey(_ publicKey: [</span><span style="color: #0000FF">Int</span><span style="color: #000000">])</span><span>
    </span><span style="color: #000000">    </span><span style="color: #0000FF">fun</span><span style="color: #000000"> removePublicKey(_ index: </span><span style="color: #0000FF">Int</span><span style="color: #000000">)</span><span>
    </span><span>
    </span><span style="color: #000000">    </span><span style="color: #008000">// Storage operations</span><span>
    </span><span>
    </span><span style="color: #000000">    </span><span style="color: #0000FF">fun</span><span style="color: #000000"> save&#x3C;T>(_ value: </span><span style="color: #0000FF">T</span><span style="color: #000000">, to: </span><span style="color: #0000FF">Path</span><span style="color: #000000">)</span><span>
    </span><span style="color: #000000">    </span><span style="color: #0000FF">fun</span><span style="color: #000000"> load&#x3C;T>(from: </span><span style="color: #0000FF">Path</span><span style="color: #000000">): </span><span style="color: #0000FF">T</span><span style="color: #000000">?</span><span>
    </span><span style="color: #000000">    </span><span style="color: #0000FF">fun</span><span style="color: #000000"> copy&#x3C;T: </span><span style="color: #0000FF">AnyStruct</span><span style="color: #000000">>(</span><span style="color: #0000FF">from</span><span style="color: #000000">: </span><span style="color: #0000FF">Path</span><span style="color: #000000">): </span><span style="color: #0000FF">T</span><span style="color: #000000">?</span><span>
    </span><span>
    </span><span style="color: #000000">    </span><span style="color: #0000FF">fun</span><span style="color: #000000"> borrow&#x3C;T: &#x26;</span><span style="color: #0000FF">Any</span><span style="color: #000000">>(</span><span style="color: #0000FF">from</span><span style="color: #000000">: </span><span style="color: #0000FF">Path</span><span style="color: #000000">): </span><span style="color: #0000FF">T</span><span style="color: #000000">?</span><span>
    </span><span>
    </span><span style="color: #000000">    </span><span style="color: #0000FF">fun</span><span style="color: #000000"> link&#x3C;T: &#x26;</span><span style="color: #0000FF">Any</span><span style="color: #000000">>(</span><span style="color: #0000FF">_</span><span style="color: #000000"> </span><span style="color: #0000FF">newCapabilityPath</span><span style="color: #000000">: </span><span style="color: #0000FF">Path</span><span style="color: #000000">, </span><span style="color: #0000FF">target</span><span style="color: #000000">: </span><span style="color: #0000FF">Path</span><span style="color: #000000">): </span><span style="color: #0000FF">Capability</span><span style="color: #000000">?</span><span>
    </span><span style="color: #000000">    </span><span style="color: #0000FF">fun</span><span style="color: #000000"> getLinkTarget(_ path: </span><span style="color: #0000FF">Path</span><span style="color: #000000">): </span><span style="color: #0000FF">Path</span><span style="color: #000000">?</span><span>
    </span><span style="color: #000000">    </span><span style="color: #0000FF">fun</span><span style="color: #000000"> unlink(_ path: </span><span style="color: #0000FF">Path</span><span style="color: #000000">)</span><span>
    </span><span>
    </span><span style="color: #000000">    </span><span style="color: #0000FF">fun</span><span style="color: #000000"> getCapability(at: </span><span style="color: #0000FF">Path</span><span style="color: #000000">): </span><span style="color: #0000FF">Capability</span><span style="color: #000000">?</span><span>
    </span><span style="color: #000000">  }</span><span>
    </span></pre></code>

## [](#account-storage)Account Storage

All accounts have storage.

Objects are stored under paths in storage.
Paths consist of a domain and an identifier.

Paths start with the character `/`, followed by the domain, the path separator `/`,
and finally the identifier.
For example, the path `/storage/test` has the domain `storage` and the identifier `test`.

There are only three valid domains: `storage`, `private`, and `public`.

Objects in storage are always stored in the `storage` domain.

Both resources and structures can be stored in account storage.

Account storage is accessed through the following functions of `AuthAccount`.
This means that any code that has access to the authorized account has access
to all its stored objects.

-   `fun save<T>(_ value: T, to: Path)`:

    Saves an object to account storage.
    Resources are moved into storage, and structures are copied.

    `T` is the type parameter for the object type.
    It can be inferred from the argument&#x27;s type.

    If there is already an object stored under the given path, the program aborts.

    The path must be a storage path, i.e., only the domain `storage` is allowed.

-   `fun load<T>(from: Path): T?`:

     Loads an object from account storage.
     If no object is stored under the given path, the function returns `nil`.
     If there is an object stored, the stored resource or structure is moved
     out of storage and returned as an optional.
     When the function returns, the storage no longer contains an object
     under the given path.

     `T` is the type parameter for the object type.
     A type argument for the parameter must be provided explicitly.

     The type `T` must be a supertype of the type of the loaded object.
     If it is not, the function returns `nil`.
     The given type must not necessarily be exactly the same as the type of the loaded object.

     The path must be a storage path, i.e., only the domain `storage` is allowed.

-   `fun copy<T>(from: Path): T?`, where `T` is the type parameter for the value type:

       Returns a copy of a structure stored in account storage, without removing it from storage.

       If no strucure is stored under the given path, the function returns `nil`.
       If there is a structure stored, it is copied.
       The structure stays stored in storage after the function returns.

       `T` is the type parameter for the structure type.
       A type argument for the parameter must be provided explicitly.

       The type `T` must be a supertype of the type of the copied structure.
       If it is not, the function returns `nil`.
       The given type must not necessarily be exactly the same as the type of the copied structure
    structure.

       The path must be a storage path, i.e., only the domain `storage` is allowed.

<code><pre><span style="color: #008000">// Declare a resource named `Counter`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">resource</span><span style="color: #000000"> Counter {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">var</span><span style="color: #000000"> count: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">init</span><span style="color: #000000">(count: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.count = count</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// In this example an authorized account is available through the constant `authAccount`.</span><span>
</span><span>
</span><span style="color: #008000">// Create a new instance of the resource type `Counter`</span><span>
</span><span style="color: #008000">// and save it in the storage of the account.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// The path `/storage/counter` is used to refer to the stored value.</span><span>
</span><span style="color: #008000">// Its identifier `counter` was chosen freely and could be something else.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">authAccount.save(&#x3C;-create Counter(count: </span><span style="color: #09885A">42</span><span style="color: #000000">), to: /storage/counter)</span><span>
</span><span>
</span><span style="color: #008000">// Run-time error: Storage already contains an object under path `/storage/counter`</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">authAccount.save(&#x3C;-create Counter(count: </span><span style="color: #09885A">123</span><span style="color: #000000">), to: /storage/counter)</span><span>
</span><span>
</span><span style="color: #008000">// Load the `Counter` resource from storage path `/storage/counter`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// The new constant `counter` has the type `Counter?`, i.e., it is an optional,</span><span>
</span><span style="color: #008000">// and its value is the counter resource, that was saved at the beginning</span><span>
</span><span style="color: #008000">// of the example.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> counter &#x3C;- authAccount.load&#x3C;@Counter>(</span><span style="color: #0000FF">from</span><span style="color: #000000">: /storage/counter)</span><span>
</span><span>
</span><span style="color: #008000">// The storage is now empty, there is no longer an object stored</span><span>
</span><span style="color: #008000">// under the path `/storage/counter`.</span><span>
</span><span>
</span><span style="color: #008000">// Load the `Counter` resource again from storage path `/storage/counter`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// The new constant `counter2` has the type `Counter?` and is `nil`,</span><span>
</span><span style="color: #008000">// as nothing is stored under the path `/storage/counter` anymore,</span><span>
</span><span style="color: #008000">// because the previous load moved the counter out of storage.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> counter2 &#x3C;- authAccount.load&#x3C;@Counter>(</span><span style="color: #0000FF">from</span><span style="color: #000000">: /storage/counter)</span><span>
</span><span>
</span><span style="color: #008000">// Create another new instance of the resource type `Counter`</span><span>
</span><span style="color: #008000">// and save it in the storage of the account.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// The path `/storage/otherCounter` is used to refer to the stored value.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">authAccount.save(&#x3C;-create Counter(count: </span><span style="color: #09885A">123</span><span style="color: #000000">), to: /storage/otherCounter)</span><span>
</span><span>
</span><span style="color: #008000">// Load the `Vault` resource from storage path `/storage/otherCounter`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// The new constant `vault` has the type `Vault?` and its value is `nil`,</span><span>
</span><span style="color: #008000">// as there is a resource with type `Counter` stored under the path,</span><span>
</span><span style="color: #008000">// which is not a subtype of the requested type `Vault`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> vault &#x3C;- authAccount.load&#x3C;@Vault>(</span><span style="color: #0000FF">from</span><span style="color: #000000">: /storage/otherCounter)</span><span>
</span><span>
</span><span style="color: #008000">// The storage still stores a `Counter` resource under the path `/storage/otherCounter`.</span><span>
</span><span>
</span><span style="color: #008000">// Save the string "Hello, World" in storage</span><span>
</span><span style="color: #008000">// under the path `/storage/helloWorldMessage`.</span><span>
</span><span>
</span><span style="color: #000000">authAccount.save(</span><span style="color: #A31515">"Hello, world!"</span><span style="color: #000000">, to: /storage/helloWorldMessage)</span><span>
</span><span>
</span><span style="color: #008000">// Copy the stored message from storage.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// After the copy, the storage still stores the string under the path.</span><span>
</span><span style="color: #008000">// Unlike `load`, `copy` does not remove the object from storage.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> message = authAccount.copy&#x3C;String>(</span><span style="color: #0000FF">from</span><span style="color: #000000">: /storage/helloWorldMessage)</span><span>
</span><span>
</span><span style="color: #008000">// Create a new instance of the resource type `Vault`</span><span>
</span><span style="color: #008000">// and save it in the storage of the account.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">authAccount.save(&#x3C;-createEmptyVault(), to: /storage/vault)</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Cannot copy a resource, as this would allow arbitrary duplication.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> vault &#x3C;- authAccount.copy&#x3C;@Vault>(</span><span style="color: #0000FF">from</span><span style="color: #000000">: /storage/vault)</span><span>
</span></pre></code>

As it is convenient to work with objects in storage
without having to move them out of storage,
as it is necessary for resources,
it is also possible to create references to objects in storage:
This is possible using the `borrow` function of an `AuthAccount`:

-   `fun borrow<T: &Any>(from: Path): T?`

     Returns a reference to an object in storage without removing it from storage.
     If no object is stored under the given path, the function returns `nil`.
     If there is an object stored, a reference is returned as an optional.

     `T` is the type parameter for the object type.
     A type argument for the parameter must be provided explicitly.
     The type argument must be a reference to any type (`&Any`; `Any` is the supertype of all types).
     It must be possible top create the given reference type `T` for the stored /  borrowed object.
     If it is not, the function returns `nil`.
     The given type must not necessarily be exactly the same as the type of the borrowed object.

     The path must be a storage path, i.e., only the domain `storage` is allowed.

<code><pre><span style="color: #008000">// Declare a resource interface named `HasCount`, that has a field `count`</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">resource interface</span><span style="color: #000000"> HasCount {</span><span>
</span><span style="color: #000000">    count: Int</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Declare a resource named `Counter` that conforms to `HasCount`</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">resource</span><span style="color: #000000"> Counter: HasCount {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">var</span><span style="color: #000000"> count: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">init</span><span style="color: #000000">(count: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.count = count</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// In this example an authorized account is available through the constant `authAccount`.</span><span>
</span><span>
</span><span style="color: #008000">// Create a new instance of the resource type `Counter`</span><span>
</span><span style="color: #008000">// and save it in the storage of the account.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// The path `/storage/counter` is used to refer to the stored value.</span><span>
</span><span style="color: #008000">// Its identifier `counter` was chosen freely and could be something else.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">authAccount.save(&#x3C;-create Counter(count: </span><span style="color: #09885A">42</span><span style="color: #000000">), to: /storage/counter)</span><span>
</span><span>
</span><span style="color: #008000">// Create a reference to the object stored under path `/storage/counter`,</span><span>
</span><span style="color: #008000">// typed as `&#x26;Counter`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// `counterRef` has type `&#x26;Counter?` and is a valid reference, i.e. non-`nil`,</span><span>
</span><span style="color: #008000">// because the borrow succeeded:</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// There is an object stored under path `/storage/counter`</span><span>
</span><span style="color: #008000">// and it has type `Counter`, so it can be borrowed as `&#x26;Counter`</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> counterRef = authAccount.borrow&#x3C;&#x26;Counter>(</span><span style="color: #0000FF">from</span><span style="color: #000000">: /storage/counter)</span><span>
</span><span>
</span><span style="color: #000000">counterRef?.count </span><span style="color: #008000">// is `42`</span><span>
</span><span>
</span><span style="color: #008000">// Create a reference to the object stored under path `/storage/counter`,</span><span>
</span><span style="color: #008000">// typed as `&#x26;{HasCount}`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// `hasCountRef` is non-`nil`, as there is an object stored under path `/storage/counter`,</span><span>
</span><span style="color: #008000">// and the stored value of type `Counter` conforms to the requested type `{HasCount}`:</span><span>
</span><span style="color: #008000">// the type `Counter` implements the restricted type's restriction `HasCount`</span><span>
</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> hasCountRef = authAccount.borrow&#x3C;&#x26;{HasCount}>(</span><span style="color: #0000FF">from</span><span style="color: #000000">: /storage/counter)</span><span>
</span><span>
</span><span style="color: #008000">// Create a reference to the object stored under path `/storage/counter`,</span><span>
</span><span style="color: #008000">// typed as `&#x26;{SomethingElse}`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// `otherRef` is `nil`, as there is an object stored under path `/storage/counter`,</span><span>
</span><span style="color: #008000">// but the stored value of type `Counter` does not conform to to the requested type `{Other}`:</span><span>
</span><span style="color: #008000">// the type `Counter` does not implement the restricted type's restriction `Other`</span><span>
</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> otherRef = authAccount.borrow&#x3C;&#x26;{Other}>(</span><span style="color: #0000FF">from</span><span style="color: #000000">: /storage/counter)</span><span>
</span><span>
</span><span style="color: #008000">// Create a reference to the object stored under path `/storage/nonExistent`,</span><span>
</span><span style="color: #008000">// typed as `&#x26;{HasCount}`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// `nonExistentRef` is `nil`, as there is nothing stored under path `/storage/nonExistent`</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> nonExistentRef = authAccount.borrow&#x3C;&#x26;{HasCount}>(</span><span style="color: #0000FF">from</span><span style="color: #000000">: /storage/nonExistent)</span><span>
</span></pre></code>

## [](#capability-based-access-control)Capability-based Access Control

Users will often want to make it so that specific other users or even anyone else
can access certain fields and functions of a stored object.
This can be done by creating a capability.

As was mentioned before, access to stored objects is governed by the
tenets of [Capability Security](https://en.wikipedia.org/wiki/Capability-based_security).
This means that if an account wants to be able to access another account&#x27;s
stored objects, it must have a valid capability to that object.

Capabilities are identified by a path and link to a target path, not directly to an object.
Capabilities are either public (any user can get access),
or private (access to/from the authorized user is necessary).

Public capabilities are created using public paths, i.e. they have the domain `public`.
After creation they can be obtained from both authorized accounts (`AuthAccount`)
and public accounts (`PublicAccount`).

Private capabilities are created using private paths, i.e. they have the domain `private`.
After creation they can be obtained from authorized accounts (`AuthAccount`),
but not from public accounts (`PublicAccount`).

Once a capability is created and obtained, it can be borrowed to get a reference
to the stored object.
When a capability is created, a type is specified that determines as what type
the capability can be borrowed.
This allows exposing and hiding certain functionality of a stored object.

Capabilities are created using the `link` function of an authorized account (`AuthAccount`):

-   `fun link<T: &Any>(_ newCapabilityPath: Path, target: Path): Capability?`

    `newCapabilityPath` is the public or private path identifiying the new capability.

    `target` is any public, private, or storage path that leads to the object
    that will provide the functionality defined by this capability.

    `T` is the type parameter for the capability type.
    A type argument for the parameter must be provided explicitly.

    The type parameter defines how the capability can be borrowed,
    i.e., how the stored value can be accessed.

    The link function returns `nil` if a link for the given capability path already exists,
    or the newly created capability if not.

    It is not necessary for the target path to lead to a valid object;
    the target path could be empty, or could lead to an object
    which does not provide the necessary type interface:

    The link function does **not** check if the target path is valid/exists at the time
    the capability is created and does **not** check if the target value conforms to the given type.

    The link is latent.
    The target value might be stored after the link is created,
    and the target value might be moved out after the link has been created.

Capabilities can be removed using the `unlink` function of an authorized account (`AuthAccount`):

-   `fun unlink(_ path: Path)`:

    `path` is the public or private path identifying the capability that should be removed.

To get the target path for a capability, the `getLinkTarget` function
of an authorized account (`AuthAccount`) can be used:

-   `fun getLinkTarget(_ path: Path): Path?`

    `path` is the public or private path identifying the capability.
    The function returns the link target path,
    if a capability exists at the given path,
    or `nil` if it does not.

Existing capabilities can be obtained by using the `getCapability` function
of authorized accounts (`AuthAccount`) and public accounts (`PublicAccount`):

-   `fun getCapability(_ at: Path): Capability?`

    For public accounts, the function returns a capability
    if the given path is public.
    It is not possible to obtain private capabilities from public accounts.
    If the path is private or a storage path, the function returns `nil`.

    For authorized accounts, the function returns a capability
    if the given path is public or private.
    If the path is a storage path, the function returns `nil`.

The `getCapability` function does **not** check if the target exists.
The link is latent.
To check if the target exists currently and could be borrowed,
the `check` function of the capability can be used:

-   `fun check<T: &Any>(): Bool`

    `T` is the type parameter for the reference type.
     A type argument for the parameter must be provided explicitly.

     The function returns true if the capability currently targets an object
     that satisfies the given type, i.e. could be borrowed using the given type.

Finally, the capability can be borrowed to get a reference to the stored object.
This can be done using the `borrow` function of the capability:

-   `fun borrow<T: &Any>(): T?`

    The function returns a reference to the object targeted by the capability,
    provided it can be borrowed using the given type.

    `T` is the type parameter for the reference type.
     A type argument for the parameter must be provided explicitly.

    The function returns `nil` if the targeted path is empty, i.e. nothing is stored under it,
    if  the requested type exceeds what is allowed by the capability (or any interim capabilities)

<code><pre><span style="color: #008000">// Declare a resource interface named `HasCount`, that has a field `count`</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">resource interface</span><span style="color: #000000"> HasCount {</span><span>
</span><span style="color: #000000">    count: Int</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// Declare a resource named `Counter` that conforms to `HasCount`</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">resource</span><span style="color: #000000"> Counter: HasCount {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">var</span><span style="color: #000000"> count: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">init</span><span style="color: #000000">(count: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.count = count</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #008000">// In this example an authorized account is available through the constant `authAccount`.</span><span>
</span><span>
</span><span style="color: #008000">// Create a new instance of the resource type `Counter`</span><span>
</span><span style="color: #008000">// and save it in the storage of the account.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// The path `/storage/counter` is used to refer to the stored value.</span><span>
</span><span style="color: #008000">// Its identifier `counter` was chosen freely and could be something else.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">authAccount.save(&#x3C;-create Counter(count: </span><span style="color: #09885A">42</span><span style="color: #000000">), to: /storage/counter)</span><span>
</span><span>
</span><span style="color: #008000">// Create a public capability that allows access to the stored counter object</span><span>
</span><span style="color: #008000">// as the type `{HasCount}`, i.e. only the functionality of reading the field</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">authAccount.link&#x3C;&#x26;{HasCount}>(/public/hasCount, target: /storage/counter)</span><span>
</span></pre></code>

To get the published portion of an account, the `getAccount` function can be used.

Imagine that the next example is from a different account as before.

<code><pre><span>
</span><span style="color: #008000">// Get the public account for the address that stores the counter</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> publicAccount = getAccount(</span><span style="color: #09885A">0x42</span><span style="color: #000000">)</span><span>
</span><span>
</span><span style="color: #008000">// Get a capability for the counter that is made publicly accessible</span><span>
</span><span style="color: #008000">// through the path `/public/hasCount`</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> countCap = publicAccount.getCapability(/public/hasCount)!</span><span>
</span><span>
</span><span style="color: #008000">// Borrow the capability to get a reference to the stored counter.</span><span>
</span><span style="color: #008000">// Use the type `&#x26;{HasCount}`, as this is the type that the capability can be borrowed as.</span><span>
</span><span style="color: #008000">// See the example below for borrowing using the type `&#x26;Counter`</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// This borrow succeeds, i.e. the result is not `nil`,</span><span>
</span><span style="color: #008000">// it is a valid reference, because:</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// 1. Dereferencing the path chain results in a stored object</span><span>
</span><span style="color: #008000">//    (`/public/hasCount` links to `/storage/counter`,</span><span>
</span><span style="color: #008000">//    and there is an object stored under `/storage/counter`)</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// 2. The stored value is a subtype of the requested type `{HasCount}`</span><span>
</span><span style="color: #008000">//    (the stored object has type `Counter` which conforms to interface `HasCount`)</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> countRef = countCap.borrow&#x3C;&#x26;{HasCount}>()!</span><span>
</span><span>
</span><span style="color: #000000">countRef.count  </span><span style="color: #008000">// is `43`</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: The `increment` function is not accessible for the reference,</span><span>
</span><span style="color: #008000">// because it has the type `&#x26;{HasCount}`</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">countRef.increment()</span><span>
</span><span>
</span><span style="color: #008000">// Attempt to borrow the capability with the type `&#x26;Counter`.</span><span>
</span><span style="color: #008000">// This results in `nil`, i.e. the borrow fails,</span><span>
</span><span style="color: #008000">// because the capability was created/linked using the type `&#x26;{HasCount}`.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// The resource type `Counter` implements the resource interface `HasCount`,</span><span>
</span><span style="color: #008000">// so `Counter` is a subtype of `{HasCount}`, but the capability only allows</span><span>
</span><span style="color: #008000">// borrowing using unauthorized references of `{HasCount}` (`&#x26;{HasCount}`)</span><span>
</span><span style="color: #008000">// instead of authorized references (`auth &#x26;{HasCount}`),</span><span>
</span><span style="color: #008000">// so users of the capability are not allowed to borrow using subtypes,</span><span>
</span><span style="color: #008000">// and they can't escalate the type by casting the reference either.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #008000">// This shows how parts of the functionality of stored objects</span><span>
</span><span style="color: #008000">// can be safely exposed to other code</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> counterRef = countCap.borrow&#x3C;&#x26;Counter>()</span><span>
</span><span>
</span><span style="color: #008000">// `counterRef` is `nil`</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Cannot access the counter object in storage directly,</span><span>
</span><span style="color: #008000">// the `borrow` function is not available for public accounts</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> counterRef2 = publicAccount.borrow&#x3C;&#x26;Counter>(/storage/counter)</span><span>
</span></pre></code>

## [](#contracts)Contracts

A contract in Cadence is a collection of type definitions
of interfaces, structs, resources, data (its state), and code (its functions)
that lives in the contract storage area of an account in Flow.
Contracts are where all composite types like structs, resources,
events, and interfaces for these types in Cadence have to be defined.
Therefore, an object of one of these types cannot exist
without having been defined in a deployed Cadence contract.

Contracts can be created, updated, and deleted using the `setCode`
function of [accounts](#accounts).
Contract creation is also possible when creating accounts,
i.e. when using the `Account` constructor.
This functionality is covered in the [next section](#deploying-and-updating-contracts)

Contracts are types.
They are similar to composite types, but are stored differently than
structs or resources and cannot be used as values, copied, or moved
like resources or structs.

Contract stay in an account&#x27;s contract storage
area and can only be updated or deleted by the account owner
with special commands.

Contracts are declared using the `contract` keyword. The keyword is followed
by the name of the contract.

<code><pre><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">contract</span><span style="color: #000000"> SomeContract {</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// ...</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

Contracts cannot be nested in each other.

<code><pre><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">contract</span><span style="color: #000000"> Invalid {</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Invalid: Contracts cannot be nested in any other type.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">contract</span><span style="color: #000000"> Nested {</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">// ...</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #000000">One of the simplest forms of a </span><span style="color: #0000FF">contract</span><span style="color: #000000"> would just be one with a state field,</span><span>
</span><span style="color: #000000">a function, and an `init` function that initializes the field:</span><span>
</span><span>
</span><span style="color: #000000">```cadence,file=contract-hello.cdc</span><span>
</span><span style="color: #008000">// HelloWorldResource.cdc</span><span>
</span><span>
</span><span style="color: #000000">pub contract HelloWorld {</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Declare a stored state field in HelloWorld</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">let</span><span style="color: #000000"> greeting: </span><span style="color: #0000FF">String</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Declare a function that can be called by anyone</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// who imports the contract</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> hello(): </span><span style="color: #0000FF">String</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">return</span><span style="color: #000000"> </span><span style="color: #0000FF">self</span><span style="color: #000000">.greeting</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">init</span><span style="color: #000000">() {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.greeting = </span><span style="color: #A31515">"Hello World!"</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

This contract could be deployed to an account and live permanently
in the contract storage.  Transactions and other contracts
can interact with contracts by importing them at the beginning
of a transaction or contract definition.

Anyone could call the above contract&#x27;s `hello` function by importing
the contract from the account it was deployed to and using the imported
object to call the hello function.

<code><pre><span style="color: #0000FF">import</span><span style="color: #000000"> HelloWorld </span><span style="color: #0000FF">from</span><span style="color: #000000"> </span><span style="color: #09885A">0x42</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: The contract does not know where hello comes from</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">log(hello())        </span><span style="color: #008000">// Error</span><span>
</span><span>
</span><span style="color: #008000">// Valid: Using the imported contract object to call the hello</span><span>
</span><span style="color: #008000">// function</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">log(HelloWorld.hello())    </span><span style="color: #008000">// prints "Hello World!"</span><span>
</span><span>
</span><span style="color: #008000">// Valid: Using the imported contract object to read the greeting</span><span>
</span><span style="color: #008000">// field.</span><span>
</span><span style="color: #000000">log(HelloWorld.greeting)   </span><span style="color: #008000">// prints "Hello World!"</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Cannot call the init function after the contract has been created.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">HelloWorld.</span><span style="color: #0000FF">init</span><span style="color: #000000">()    </span><span style="color: #008000">// Error</span><span>
</span></pre></code>

There can be any number of contracts per account
and they can include an arbitrary amount of data.
This means that a contract can have any number of fields, functions, and type definitions,
but they have to be in the contract and not another top-level definition.

<code><pre><span style="color: #008000">// Invalid: Top-level declarations are restricted to only be contracts</span><span>
</span><span style="color: #008000">//          or contract interfaces. Therefore, all of these would be invalid</span><span>
</span><span style="color: #008000">//          if they were deployed to the account contract storage and</span><span>
</span><span style="color: #008000">//          the deployment would be rejected.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">resource</span><span style="color: #000000"> Vault {}</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">struct</span><span style="color: #000000"> Hat {}</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> helloWorld(): </span><span style="color: #0000FF">String</span><span style="color: #000000"> {}</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> num: </span><span style="color: #0000FF">Int</span><span>
</span></pre></code>

Another important feature of contracts is that instances of resources and events
that are declared in contracts can only be created/emitted within functions or types
that are declared in the same contract.

It is not possible create instances of resources and events outside the contract.

The contract below defines a resource interface `Receiver` and a resource `Vault`
that implements that interface.  The way this example is written,
there is no way to create this resource, so it would not be usable.

<code><pre><span style="color: #008000">// Valid</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">contract</span><span style="color: #000000"> FungibleToken {</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">resource interface</span><span style="color: #000000"> Receiver {</span><span>
</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">pub</span><span style="color: #000000"> balance: Int</span><span>
</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> deposit(from: @</span><span style="color: #0000FF">Receiver</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">            </span><span style="color: #0000FF">pre</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">                </span><span style="color: #0000FF">from</span><span style="color: #000000">.balance > </span><span style="color: #09885A">0</span><span style="color: #000000">:</span><span>
</span><span style="color: #000000">                    </span><span style="color: #A31515">"Deposit balance needs to be positive!"</span><span>
</span><span style="color: #000000">            }</span><span>
</span><span style="color: #000000">            </span><span style="color: #0000FF">post</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">                </span><span style="color: #0000FF">self</span><span style="color: #000000">.balance == before(</span><span style="color: #0000FF">self</span><span style="color: #000000">.balance) + before(from.balance):</span><span>
</span><span style="color: #000000">                    </span><span style="color: #A31515">"Incorrect amount removed"</span><span>
</span><span style="color: #000000">            }</span><span>
</span><span style="color: #000000">        }</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">resource</span><span style="color: #000000"> Vault: Receiver {</span><span>
</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">// keeps track of the total balance of the accounts tokens</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">var</span><span style="color: #000000"> balance: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">init</span><span style="color: #000000">(balance: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">            </span><span style="color: #0000FF">self</span><span style="color: #000000">.balance = balance</span><span>
</span><span style="color: #000000">        }</span><span>
</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">// withdraw subtracts amount from the vaults balance and</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">// returns a vault object with the subtracted balance</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> withdraw(amount: </span><span style="color: #0000FF">Int</span><span style="color: #000000">): @</span><span style="color: #0000FF">Vault</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">            </span><span style="color: #0000FF">self</span><span style="color: #000000">.balance = </span><span style="color: #0000FF">self</span><span style="color: #000000">.balance - amount</span><span>
</span><span style="color: #000000">            </span><span style="color: #0000FF">return</span><span style="color: #000000"> &#x3C;-</span><span style="color: #0000FF">create</span><span style="color: #000000"> Vault(balance: amount)</span><span>
</span><span style="color: #000000">        }</span><span>
</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">// deposit takes a vault object as a parameter and adds</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">// its balance to the balance of the Account's vault, then</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">// destroys the sent vault because its balance has been consumed</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> deposit(from: @</span><span style="color: #0000FF">Receiver</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">            </span><span style="color: #0000FF">self</span><span style="color: #000000">.balance = </span><span style="color: #0000FF">self</span><span style="color: #000000">.balance + </span><span style="color: #0000FF">from</span><span style="color: #000000">.balance</span><span>
</span><span style="color: #000000">            </span><span style="color: #0000FF">destroy</span><span style="color: #000000"> </span><span style="color: #0000FF">from</span><span>
</span><span style="color: #000000">        }</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

If a user tried to run a transaction that created an instance of the `Vault` type,
the type checker would not allow it because only code in the `FungibleToken`
contract can create new `Vault`s.

<code><pre><span style="color: #0000FF">import</span><span style="color: #000000"> FungibleToken </span><span style="color: #0000FF">from</span><span style="color: #000000"> </span><span style="color: #09885A">0x42</span><span>
</span><span>
</span><span style="color: #008000">// Invalid: Cannot create an instance of the `Vault` type outside</span><span>
</span><span style="color: #008000">// of the contract that defines `Vault`</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> newVault &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> FungibleToken.Vault(balance: </span><span style="color: #09885A">10</span><span style="color: #000000">)</span><span>
</span></pre></code>

The contract would have to either define a function that creates new
`Vault` instances or use its `init` function to create an instance and
store it in the owner&#x27;s account storage.

This brings up another key feature of contracts in Cadence.  Contracts
can interact with its account&#x27;s `storage` and `published` objects to store
resources, structs, and references.
They do so by using the special `self.account` object that is only accessible within the contract.

Imagine that these were declared in the above `FungibleToken` contract.

<code><pre><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">fun</span><span style="color: #000000"> createVault(initialBalance: </span><span style="color: #0000FF">Int</span><span style="color: #000000">): @</span><span style="color: #0000FF">Vault</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">return</span><span style="color: #000000"> &#x3C;-</span><span style="color: #0000FF">create</span><span style="color: #000000"> Vault(balance: initialBalance)</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">init</span><span style="color: #000000">(balance: </span><span style="color: #0000FF">Int</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">let</span><span style="color: #000000"> vault &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> Vault(balance: </span><span style="color: #09885A">1000</span><span style="color: #000000">)</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.account.save(&#x3C;-vault, to: /storage/initialVault)</span><span>
</span><span style="color: #000000">    }</span><span>
</span></pre></code>

Now, any account could call the `createVault` function declared in the contract
to create a `Vault` object.
Or the owner could call the `withdraw` function on their own `Vault` to send new vaults to others.

<code><pre><span style="color: #0000FF">import</span><span style="color: #000000"> FungibleToken </span><span style="color: #0000FF">from</span><span style="color: #000000"> </span><span style="color: #09885A">0x42</span><span>
</span><span>
</span><span style="color: #008000">// Valid: Create an instance of the `Vault` type by calling the contract's</span><span>
</span><span style="color: #008000">// `createVault` function.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> newVault &#x3C;- </span><span style="color: #0000FF">create</span><span style="color: #000000"> FungibleToken.createVault(initialBalance: </span><span style="color: #09885A">10</span><span style="color: #000000">)</span><span>
</span></pre></code>

Contracts have the implicit field `let account: Account`,
which is the account in which the contract is deployed too.
This gives the contract the ability to e.g. read and write to the account&#x27;s storage.

<code><pre><span>
</span></pre></code>

### [](#deploying-and-updating-contracts)Deploying and Updating Contracts

In order for a contract to be used in Cadence, it needs
to be deployed to an account.

Contract can be deployed to an account using the `setCode` function of the `Account` type:
`setCode(_ code: [UInt8], ...)`.
The function&#x27;s `code` parameter is the byte representation of the source code.
Additional arguments are passed to the initializer of the contract.

For example, assuming the following contract code should be deployed:

<code><pre><span style="color: #0000FF">contract</span><span style="color: #000000"> Test {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">let</span><span style="color: #000000"> message: </span><span style="color: #0000FF">String</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">init</span><span style="color: #000000">(message: </span><span style="color: #0000FF">String</span><span style="color: #000000">) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">self</span><span style="color: #000000">.message = message</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

The contract can be deployed as follows:

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> signer: </span><span style="color: #0000FF">Account</span><span style="color: #000000"> = ...</span><span>
</span><span style="color: #000000">signer.setCode(</span><span>
</span><span style="color: #000000">    [</span><span style="color: #09885A">0x63</span><span style="color: #000000">, </span><span style="color: #09885A">0x6f</span><span style="color: #000000">, </span><span style="color: #09885A">0x6e</span><span style="color: #000000">, </span><span style="color: #09885A">0x74</span><span style="color: #000000">, </span><span style="color: #09885A">0x72</span><span style="color: #000000">, </span><span style="color: #09885A">0x61</span><span style="color: #008000">/*, ... */</span><span style="color: #000000">],</span><span>
</span><span style="color: #000000">    message: </span><span style="color: #A31515">"I'm a new contract in an existing account"</span><span>
</span><span style="color: #000000">)</span><span>
</span></pre></code>

The contract can also be deployed when creating an account by using the `Account` constructor.

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> newAccount = Account(</span><span>
</span><span style="color: #000000">    publicKeys: [],</span><span>
</span><span style="color: #000000">    code: [</span><span style="color: #09885A">0x63</span><span style="color: #000000">, </span><span style="color: #09885A">0x6f</span><span style="color: #000000">, </span><span style="color: #09885A">0x6e</span><span style="color: #000000">, </span><span style="color: #09885A">0x74</span><span style="color: #000000">, </span><span style="color: #09885A">0x72</span><span style="color: #000000">, </span><span style="color: #09885A">0x61</span><span style="color: #008000">/*, ... */</span><span style="color: #000000">],</span><span>
</span><span style="color: #000000">    message: </span><span style="color: #A31515">"I'm a new contract in a new account"</span><span>
</span><span style="color: #000000">)</span><span>
</span></pre></code>

### [](#contract-interfaces)Contract Interfaces

Like composite types, contracts can have interfaces that specify rules
about their behavior, their types, and the behavior of their types.

Contract interfaces have to be declared globally.  Declarations
cannot be nested in other types.

If a contract interface declares a concrete type, implementations of it
must also declare the same concrete type conforming to the type requirement.

If a contract interface declares an interface type, the implementing contract
does not have to also define that interface.  They can refer to that nested
interface by saying `{ContractInterfaceName}.{NestedInterfaceName}`

<code><pre><span style="color: #008000">// Declare a contract interface that declares an interface and a resource</span><span>
</span><span style="color: #008000">// that needs to implement that interface in the contract implementation.</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">contract interface</span><span style="color: #000000"> InterfaceExample {</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Implementations do not need to declare this</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// They refer to it as InterfaceExample.NestedInterface</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">resource interface</span><span style="color: #000000"> NestedInterface {}</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Implementations must declare this type</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">resource</span><span style="color: #000000"> Composite: NestedInterface {}</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">contract</span><span style="color: #000000"> ExampleContract: InterfaceExample {</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// The contract doesn't need to redeclare the `NestedInterface` interface</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// because it is already declared in the contract interface</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// The resource has to refer to the resrouce interface using the name</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// of the contract interface to access it</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">resource</span><span style="color: #000000"> Composite: InterfaceExample.NestedInterface {</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

## [](#events)Events

Events are special values that can be emitted during the execution of a program.

An event type can be declared with the `event` keyword.

<code><pre><span style="color: #0000FF">event</span><span style="color: #000000"> FooEvent(x: Int, y: Int)</span><span>
</span></pre></code>

The syntax of an event declaration is similar to that of
a [function declaration](#function-declarations);
events contain named parameters, each of which has an optional argument label.
Types that can be in event definitions are restricted
to booleans, strings, integer, and arrays or dictionaries of these types.

Events can only be declared within a [contract](#contracts) body.
Events cannot be declared globally or within resource or struct types.

Resource argument types are not allowed because when a resource is used as
an argument, it is moved.  A piece of code would not want to move a resource
to emit an event, so it is not allowed as a parameter.

<code><pre><span style="color: #008000">// Invalid: An event cannot be declared globally</span><span>
</span><span style="color: #008000">//</span><span>
</span><span style="color: #0000FF">event</span><span style="color: #000000"> GlobalEvent(field: Int)</span><span>
</span><span>
</span><span style="color: #000000">pub contract Events {</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Event with explicit argument labels</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">event</span><span style="color: #000000"> BarEvent(labelA fieldA: Int, labelB fieldB: Int)</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Invalid: A resource type is not allowed to be used</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// because it would be moved and lost</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    event ResourceEvent(resourceField: @Vault)</span><span>
</span><span style="color: #000000">}</span><span>
</span><span>
</span></pre></code>

### [](#emitting-events)Emitting events

To emit an event from a program, use the `emit` statement:

<code><pre><span style="color: #0000FF">pub</span><span style="color: #000000"> </span><span style="color: #0000FF">contract</span><span style="color: #000000"> Events {</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">event</span><span style="color: #000000"> FooEvent(x: Int, y: Int)</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Event with argument labels</span><span>
</span><span style="color: #000000">    event BarEvent(labelA fieldA: Int, labelB fieldB: Int)</span><span>
</span><span>
</span><span style="color: #000000">    fun events() {</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">emit</span><span style="color: #000000"> FooEvent(x: </span><span style="color: #09885A">1</span><span style="color: #000000">, y: </span><span style="color: #09885A">2</span><span style="color: #000000">)</span><span>
</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">// Emit event with explicit argument labels</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">// Note that the emitted event will only contain the field names,</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">// not the argument labels used at the invocation site.</span><span>
</span><span style="color: #000000">        </span><span style="color: #0000FF">emit</span><span style="color: #000000"> FooEvent(labelA: </span><span style="color: #09885A">1</span><span style="color: #000000">, labelB: </span><span style="color: #09885A">2</span><span style="color: #000000">)</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

Emitting events has the following restrictions:

-   Events can only be invoked in an `emit` statement.

    This means events cannot be assigned to variables or used as function parameters.

-   Events can only be emitted from the location in which they are declared.

## [](#transactions)Transactions

Transactions are objects that are signed by one or more [accounts](#accounts)
and are sent to the chain to interact with it.

Transactions are structured as such:

First, the transaction can import any number of types from external accounts
using the import syntax.

<code><pre><span style="color: #0000FF">import</span><span style="color: #000000"> FungibleToken </span><span style="color: #0000FF">from</span><span style="color: #000000"> </span><span style="color: #09885A">0x01</span><span>
</span></pre></code>

The body is declared using the `transaction` keyword and its contents
are contained in curly braces.

Next is the body of the transaction,
which first contains local variable declarations that are valid
throughout the whole of the transaction.

<code><pre><span style="color: #000000">transaction {</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// transaction contents</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">let</span><span style="color: #000000"> localVar: </span><span style="color: #0000FF">Int</span><span>
</span><span>
</span><span style="color: #000000">    ...</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

then three optional main phases:
Preparation, execution, and postconditions, only in that order.
Each phase is a block of code that executes sequentially.

-   The **prepare phase** (declared using the `prepare` keyword)
    acts like the initializer in a composite type,
    i.e., it has to initialize the local fields of the transaction
    that can then be used in the execution phase.

    The prepare phase also has access to the authorized account objects
    (`AuthAccount`) of the accounts that signed it.
    These authorized account objects have to be declared as parameters
    to the prepare phase, one for each signer of the transaction:

    <code><pre><span style="color: #008000">// There needs to be exactly as many `AuthAccount`-typed parameters</span><span>
    </span><span style="color: #008000">// as there are signers for the transaction.</span><span>
    </span><span style="color: #008000">// In this case, there would be two signers</span><span>
    </span><span>
    </span><span style="color: #000000">prepare(signer1: AuthAccount, signer2: AuthAccount) {</span><span>
    </span><span style="color: #000000">    </span><span style="color: #008000">// ...</span><span>
    </span><span style="color: #000000">}</span><span>
    </span></pre></code>

    `AuthAccount` objects have the permissions
    to read from and write to the private storage
    of the account, which cannot be directly accessed anywhere else.

-   The **execute phase** (declared using the `execute` keyword)
    is where interaction with other accounts
    and contracts should usually happen.

    This usually involves interacting with contracts with public types
    and functions, calling functions using references to other accounts&#x27;
    objects, and performing specific computation on these values.

    This phase does not have access to any signer&#x27;s authorized account object
    and can only access public contract fields and functions,
    public account objects (`PublicAccount`) using the built-in `getAccount`
    function, and any local transaction variables
    that were initialized in the `prepare` block.

    <code><pre><span style="color: #000000">  </span><span style="color: #0000FF">execute</span><span style="color: #000000"> {</span><span>
    </span><span style="color: #000000">      </span><span style="color: #008000">// Invalid: Cannot access the authorized account object,</span><span>
    </span><span style="color: #000000">      </span><span style="color: #008000">// as `account1` is not in scope</span><span>
    </span><span>
    </span><span style="color: #000000">      </span><span style="color: #0000FF">let</span><span style="color: #000000"> resource &#x3C;- account1.load&#x3C;@Resource>(</span><span style="color: #0000FF">from</span><span style="color: #000000">: /storage/resource)</span><span>
    </span><span style="color: #000000">      </span><span style="color: #0000FF">destroy</span><span style="color: #000000"> resource</span><span>
    </span><span>
    </span><span style="color: #000000">      </span><span style="color: #008000">// Valid: Can access any account's public Account object</span><span>
    </span><span>
    </span><span style="color: #000000">      </span><span style="color: #0000FF">let</span><span style="color: #000000"> publicAccount = getAccount(</span><span style="color: #09885A">0x03</span><span style="color: #000000">)</span><span>
    </span><span style="color: #000000">}</span><span>
    </span><span>
    </span></pre></code>


-   The **postcondition phase** (declared using the `post` keyword)
    is where the transaction can check
    that its functionality was executed correctly with specific condition checks.

    If any of the condition checks result in `false`, the transaction will fail
    and be completely reverted.

    Only condition checks are allowed in this section. No actual computation
    or modification of values is allowed.

    <code><pre><span style="color: #000000">  </span><span style="color: #0000FF">post</span><span style="color: #000000"> {</span><span>
    </span><span style="color: #000000">      result.balance == </span><span style="color: #09885A">30</span><span style="color: #000000">: </span><span style="color: #A31515">"Balance after transaction is incorrect!"</span><span>
    </span><span style="color: #000000">  }</span><span>
    </span><span>
    </span></pre></code>

<code><pre><span style="color: #008000">// Optional: Importing external types from other accounts using `import`.</span><span>
</span><span style="color: #0000FF">import</span><span style="color: #000000"> HelloWorld </span><span style="color: #0000FF">from</span><span style="color: #000000"> </span><span style="color: #09885A">0x01</span><span>
</span><span>
</span><span style="color: #000000">transaction {</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// Optional: type declarations and fields, which must be initialized in `prepare`.</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// The prepare phase needs to have as many account parameters</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">// as there are signers for the transaction.</span><span>
</span><span style="color: #000000">    </span><span style="color: #008000">//</span><span>
</span><span style="color: #000000">    prepare(signer1: AuthAccount) {</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">// ...</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">execute</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">// ...</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span>
</span><span style="color: #000000">    </span><span style="color: #0000FF">post</span><span style="color: #000000"> {</span><span>
</span><span style="color: #000000">        </span><span style="color: #008000">// ...</span><span>
</span><span style="color: #000000">    }</span><span>
</span><span style="color: #000000">}</span><span>
</span></pre></code>

### [](#importing-and-using-deployed-contract-code)Importing and using Deployed Contract Code

Deploying contract code to an account was covered
in the [Deploying and Updating Contracts](#Deploying-and-Updating-Contracts) section of the spec.

Once a contract or contract interface has been deployed to an account,
anybody can import the type from the account where it was deployed to and use it in their
contracts or transactions.

<!--

TODO

#### Document how contract code is imported and used in another contract with simpler examples than FungibleToken

-->

<code><pre><span>
</span></pre></code>

Again, the type must be deployed to the account where it is being imported from.

Once code is deployed, it can be used in other code and in transactions.

<!--

TODO

#### Document how to create objects of types defined in other contracts

-->

## [](#built-in-functions)Built-in Functions

### [](#block-and-transaction-information)Block and Transaction Information

To get the addresses of the signers of a transaction,
use the `address` field of each signing `AuthAccount`
that is passed to the transaction&#x27;s `prepare` block.

There is currently no built-in function that allows getting the current block number,
or timestamp.
These are being worked on.

### [](#panic)`panic`

<code><pre><span style="color: #0000FF">fun</span><span style="color: #000000"> panic(_ message: </span><span style="color: #0000FF">String</span><span style="color: #000000">): </span><span style="color: #0000FF">Never</span><span>
</span></pre></code>

Terminates the program unconditionally
and reports a message which explains why the unrecoverable error occurred.

#### [](#example)Example

<code><pre><span style="color: #0000FF">let</span><span style="color: #000000"> optionalAccount: </span><span style="color: #0000FF">Account</span><span style="color: #000000">? = </span><span style="color: #008000">// ...</span><span>
</span><span style="color: #0000FF">let</span><span style="color: #000000"> account = optionalAccount ?? panic(</span><span style="color: #A31515">"missing account"</span><span style="color: #000000">)</span><span>
</span></pre></code>

### [](#assert)`assert`

<code><pre><span style="color: #0000FF">fun</span><span style="color: #000000"> assert(_ condition: </span><span style="color: #0000FF">Bool</span><span style="color: #000000">, message: </span><span style="color: #0000FF">String</span><span style="color: #000000">)</span><span>
</span></pre></code>

Terminates the program if the given condition is false,
and reports a message which explains how the condition is false.
Use this function for internal sanity checks.

The message argument is optional.
