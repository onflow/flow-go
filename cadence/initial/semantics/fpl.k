/**
 K Semantics of the flow programming language

This definition contains execution rules for the language,
a definition of the execution state (configuration)
which combines the code to
execute with other necessary state such as call stacks,
and a grammar of the language, augmented with extra productions
as needed to represent intermediate states in execution.

Organization
------------

The execution semantics is assembled in the FPL module,
which imports the configuration from FPL-CONFIGURATION,
and rules and productions from various other modules.
The syntax for parsing programs is assembled in
FPL-SYNTAX, which imports only the productions allowed
in program text, more or less.
For testing, the FPL-TESTING and FPL-TESTING-SYNTAX modules
add an `assert` statement, and some extra rules to empty
parts of the configuration after execution finishes, to
make unit testing easier by giving a simpler post-test
execution state.

By default the K tools find the module defining program
syntax by adding the suffix -SYNTAX to the name of the
module defining the overall semantics. It's also possible
to provide another module name with an option to the
compiler 'kompile', or completely avoid using K's
concrete parser by writing your own program which parses
input text into the 'kore' AST format, and directing
'krun' to use your own parser.

The module FPL-COMMON-SYNTAX defines parts of the AST
which are allowed both in programs and rules.
The concrete syntax of identifiers and integer literals
would conflict with K's own lexical syntax in rules.
So, concrete syntax for these tokens is defined in the module
FPL-TOKENS, which is imported into FPL-SYNTAX but not
into the main semantics, and rules use K's explicit
token syntax like `#token("_","Ident")` if a rule needs
to mention a literal identifier.

List productions like `syntax TypeIds ::= List{TypeId,","}`
have special handling so that the empty string
can be parsed as an empty list in the program parser, while
an empty list must explicitly be written like `.TypeIds` in
rules, which avoids many possible ambiguities.

A more invasive mechanism for separating program syntax
from the syntax allowed in rules is that when generating
the program parser, any module FOO reachable from
the syntax module will be replaced by the suffixed module
FOO-PROGRAM-PARSING, if that exists.
We use this with the FPL-TYPE-ANNOTATION module so
that type annotations in programs must be actual types,
but type annotation positions in execution rules can
be a distinct `noAnnot()` value.
This allows us to use `[macro]` rules like

```rule (let Ident  = Exp)::VarDef => let Ident : noAnnot()  = Exp [macro]```

to expand the unannotated productions like `"let" Ident "=" Exp`
into a use of the annotated production, rather than
needing duplicate AST nodes, duplicate execution rules to
handle the common aspects of annotated and unannotated forms.
Macros are similarly used to normalize language constructs
that have extra punctuation when a nonempty list is given,
such as the declaration forms
```
   | "struct" Ident ":" TypeIds "{" CompositeItems "}"
   | "struct" Ident             "{" CompositeItems "}"
```
Macro expansion is applied to both sides of rules before they are compiled,
and to programs before execution, making the replacement completely
transparent.
 */

module FPL-TYPE-ANNOTATION
  syntax Type
  syntax Annot ::= Type | noAnnot()

endmodule

module FPL-TYPE-ANNOTATION-PROGRAM-PARSING
  syntax Type
  syntax Annot ::= Type
endmodule

module FPL-COMMON-SYNTAX
  /* This module defines most of the Flow AST,
     containing all productions whose actual syntax can be
     used in K rules without ambiguity.

     The remaining constructs are given their actual syntax for
     parsing programs in FPL-SYNTAX and a syntax that can be used
     in semantic rules in FPL-RULE-SYNTAX
   */
  imports FPL-TYPE-ANNOTATION
  imports UNSIGNED-INT-SYNTAX
  imports BOOL-SYNTAX

  syntax Ident
  syntax FPLString
  syntax IntLit

  syntax Definition ::=
     FunDef
   | VarDef
   | "struct" Ident ":" TypeIds "{" CompositeItems "}"
   | "struct" Ident             "{" CompositeItems "}"
   | "resource" Ident ":" TypeIds "{" CompositeItems "}"
   | "resource" Ident             "{" CompositeItems "}"
   | "interface" Ident "{" InterfaceItems "}"
   // TODO: there should be separate "struct interface" and "resource interface"
   | "impl" TypeId "for" TypeId "{" ImplBody "}"
  syntax TopDefinition ::=
     Definition
   | "import" ImportSource
   | "import" TypeIds "from" ImportSource

  rule struct Ident { Items } => struct Ident : .TypeIds { Items } [macro]
  rule resource Ident { Items } => resource Ident : .TypeIds { Items } [macro]

  syntax Transaction ::=
     "transaction" "{"
        Fields
        "prepare" "(" Formals ")" "{" Block "}"
        "execute" "{" Block "}"
        "post" "{" Conditions "}"
     "}"
  syntax Fields ::= List{FieldDecl,""}

  syntax FileName ::= FPLString
  syntax ImportSource ::= IntLit | FPLString

  syntax FunDef ::= "fun" Ident "(" Formals ")"           FunBody
                  | "fun" Ident "(" Formals ")" ":" Annot FunBody

  syntax Ident ::= "Void" [token]

  rule fun Ident ( Formals ) Body::FunBody => fun Ident ( Formals ) : Void Body [macro]

  syntax CompositeItems ::= List{CompositeItemAccess,""}
  syntax CompositeItemAccess ::= Access CompositeItem
                               | CompositeItem
  syntax CompositeItem ::= Definition
                         | FieldDecl
                         | "init" "(" Formals ")" FunBody
  syntax Access ::= "pub" "(" "set" ")"
                  | "pub"

  syntax InterfaceItems ::= List{InterfaceItemAccess,""}
  syntax InterfaceItemAccess ::= Access InterfaceItem
                               | InterfaceItem
  syntax InterfaceItem ::= Definition
                         | InterfaceFieldDecl
                         | InterfaceFunDecl
                         | "init" "(" Formals ")"
  syntax InterfaceFieldDecl ::= "var" Ident ":" Type
                              | "var" Ident ":" Type "{" AccessorGuards "}"
                              | "let" Ident ":" Type
                              | "let" Ident ":" Type "{" AccessorGuards "}"
                              |       Ident ":" Type
                              |       Ident ":" Type "{" AccessorGuards "}"

  rule (var  Ident : Type)::InterfaceFieldDecl => var Ident : Type { .AccessorGuards } [macro]
  rule (let  Ident : Type)::InterfaceFieldDecl => let Ident : Type { .AccessorGuards } [macro]
  rule      (Ident : Type)::InterfaceFieldDecl =>     Ident : Type { .AccessorGuards } [macro]

  syntax InterfaceFunDecl ::= "fun" Ident "(" Formals ")"
                            | "fun" Ident "(" Formals ")"           GuardBlock
                            | "fun" Ident "(" Formals ")" ":" Annot
                            | "fun" Ident "(" Formals ")" ":" Annot GuardBlock

  rule fun Ident ( Formals )                   => fun Ident ( Formals ) : Void {} [macro]
  rule fun Ident ( Formals ) Block::GuardBlock => fun Ident ( Formals ) : Void Block [macro]
  rule fun Ident ( Formals ) : Annot           => fun Ident ( Formals ) : Annot {} [macro]

  syntax AccessorGuards ::= List{AccessorGuard,""}
  syntax AccessorGuard ::=
     "get"               GuardBlock
   | "set" "(" Ident ")" GuardBlock

  syntax ImplBody ::= List{ImplItemAccess,";"}
  syntax ImplItemAccess ::= Access ImplItem
                          | ImplItem
  syntax ImplItem ::= Definition
                    | FunDef
                    | FieldDecl // must be a synthetic field

  syntax FieldDecl ::=
     "let" Ident ":" Type
   | "var" Ident ":" Type
   | "synthetic" Ident "{" AccessorDefs "}"
  syntax AccessorDefs ::= List{AccessorDef,""}
  syntax AccessorDef ::= "get"               FunBody
                       | "set" "(" Ident ")" FunBody
  // Nesting left out for now

  syntax VarDef ::=
     "let" Ident            "=" Exp
   | "let" Ident ":" Annot  "=" Exp
   | "let" Ident           "<-" Exp
   | "let" Ident ":" Annot "<-" Exp
   | "var" Ident            "=" Exp
   | "var" Ident ":" Annot  "=" Exp
   | "var" Ident           "<-" Exp
   | "var" Ident ":" Annot "<-" Exp

  rule (let Ident  = Exp)::VarDef => let Ident : noAnnot()  = Exp [macro]
  rule  let Ident <- Exp          => let Ident : noAnnot() <- Exp [macro]
  rule (var Ident  = Exp)::VarDef => var Ident : noAnnot()  = Exp [macro]
  rule  var Ident <- Exp          => var Ident : noAnnot() <- Exp [macro]

  syntax Formal ::= Ident Ident ":" Type
                  |       Ident ":" Type
  rule (I : T)::Formal => I I : T [macro]
  syntax Formals ::= List{Formal,","} [klabel(formals)]

  syntax FunBody ::= "{" Block "}"
                   | "{" Guards Block "}"
  syntax Guards ::= "pre" "{" Conditions "}"
                  | "post" "{" Conditions "}"
                  | "pre" "{" Conditions "}" "post" "{" Conditions "}"
  // a block that may only contain pre- and post-conditions, as in interface declarations
  syntax GuardBlock ::= "{" Guards "}"
                      | "{" "}"

  // any place in the AST where guards are allowed is normalized to
  // have both pre and post conditions, each possibly an empty list
  rule { B } ::FunBody => { pre { .Conditions } post { .Conditions } B } ::FunBody [macro]

  rule {} => { pre { .Conditions } post { .Conditions } } [macro]
  rule pre { Cond } => pre { Cond } post { .Conditions } [macro]
  rule post { Cond } => pre { .Conditions } post { Cond } [macro]

  syntax Conditions ::= NeList{Condition,""}
  syntax Condition ::= Exp
                     | Exp ":" FPLString

  syntax Stmt ::= VarDef
                | FunDef
                | Path "=" Exp
                | Path "<-" Exp
                | Exp
                // Deploy?
                // Publish?
                | IfStmt
                | "while" Exp "{" Block "}"
                | "continue"
                | "break"
                | "return"
                | "return" Exp [strict]

  syntax IfStmt ::= "if" IfCond "{" Block "}"
                  | "if" IfCond "{" Block "}" "else" "{" Block "}"
                  | "if" IfCond "{" Block "}" "else" IfStmt

  rule if Cond { Block } => if Cond { Block } else { .Block } [macro]
  rule if Cond { Block } else If => if Cond { Block } else { If } [macro]

  syntax IfCond ::= Exp
                  | "let" Ident "=" Exp
                  | "var" Ident "=" Exp
  syntax Block ::= List{Stmt,";"}

  syntax Path ::= Ident
                | "self"
                | Path "[" Exp "]" [klabel(index)]
                | Path "." Ident [klabel(field)]
                | "(" Path ")" [bracket]

  syntax left lval

  syntax Exp ::= IntLit [klabel(intLit), avoid, symbol, function]
               | FPLString
               | Path
               | "nil"
               | Bool
               | "(" Exp ")" [bracket]
               | "[" Exps "]"        [strict(1)]
               | "[" Type "]"               // type-indexed collections
               | "{" DictEntries "}" [strict(1)]
               | "fun" "(" Formals ")" FunBody
               | "fun" "(" Formals ")" ":" Annot FunBody
               | Exp "[" Exp "]" [lval, seqstrict, klabel(index)]
               | Exp "." Ident    [lval, klabel(field)] // field access
               | Ident "(" Actuals ")" [strict(2), klabel(call)]
               | Exp "." Ident "(" Actuals ")"
               | TypeId "(" Actuals ")" [klabel(call)]          // struct creation
               | "create" TypeId "(" Actuals ")" // resource creation
               > non-assoc:
                 "-" Exp       [strict]
               | "!" Exp       [strict]
               > left:
                 Exp "*" Exp   [seqstrict]
               | Exp "&*" Exp  [seqstrict]
               | Exp "/" Exp   [seqstrict]
               | Exp "&/" Exp  [seqstrict]
               | Exp "%" Exp   [seqstrict]
               > left: Exp "+" Exp   [seqstrict]
               | Exp "&+" Exp  [seqstrict]
               | Exp "-" Exp   [seqstrict]
               | Exp "&-" Exp  [seqstrict]
               > left:
                 Exp "?" "?" Exp  [strict(1)]
               | Exp "as?" Exp [seqstrict]
               > left:
                 Exp "==" Exp  [seqstrict]
               | Exp "!=" Exp  [seqstrict]
               | Exp "<" Exp   [seqstrict]
               | Exp "<=" Exp  [seqstrict]
               | Exp ">" Exp   [seqstrict]
               | Exp ">=" Exp  [seqstrict]
               > left:
                 Exp "&&" Exp  [strict(1)]
               | Exp "||" Exp  [strict(1)]
               > Exp "?" Exp ":" Exp [right,strict(1)]
  syntax Exps ::= List{Exp,","}      [seqstrict, hybrid, klabel(exps)]
  rule isKResult(.Exps) => true

  rule fun ( Formals ) { Body } => fun ( Formals ) : Void { Body } [macro]

  syntax DictEntries ::= List{DictEntry,","} [seqstrict]
  syntax DictEntry ::= Exp ":" Exp [seqstrict]

  syntax KResult
  syntax Actual ::= Ident ":" Exp [hybrid, strict(2)]
                  | Exp
  syntax Actuals ::= List{Actual,","} [seqstrict, klabel(actuals), hybrid]
  rule isKResult(.Actuals) => true

  syntax TypeId ::= Ident // plus fully qualified names
  syntax TypeIds ::= List{TypeId,","}

  syntax Type ::= Ident
                | "Self"
                | "(" "(" Types ")" ":" Type ")"
                | Type "?"
                | "[" Type "]"
                | "[" Type ";" IntLit "]"
                | Type "[" Type "]"
                > "@" Type
  syntax Types ::= List{Type,","}

  syntax Pgm ::= List{TopDefinition,";"}
endmodule

module FPL-TOKENS
  /* This modules defines the lexical syntax of
     tokens that we need for parsing programs, but
     but must not be parsed in K rules, to
     avoid ambiguity with K syntax
   */
  imports FPL-COMMON-SYNTAX
  syntax IntLit ::=
     r"-?0x[0-9a-fA-F]([_0-9a-fA-F]*[0-9a-fA-F])?" [token]
   | r"-?0o[0-7]([_0-7]*[0-7])?" [token]
   | r"-?0b[01]([_01]*[01])?" [token]
   | r"-?[0-9]+_[_0-9]*[0-9]" [token]
   | r"-?[0-9]+" [token]
   | Int [token]
  syntax Ident ::=
     r"[_a-zA-Z][_a-zA-Z0-9]*" [token,autoReject]
  syntax FPLString ::=
     r"[\\\"](([^\"\\n\\r\\\\])|([\\\\][0\\\\tnr\"'])|([\\\\]u\\{[0-9a-fA-F]+\\}))*[\\\"]" [token]
endmodule

module FPL-SYNTAX
  /* This module defines the concrete syntax available in programs */
  imports FPL-COMMON-SYNTAX
  imports FPL-TOKENS
endmodule

module FPL-SCOPES
  /** These productions are for tracking the context in which
      declarations are being processed or code executed.

      Without struct support, a `Scope` can only be the
      global scope, or inside of a function.

      The wrappers `declare` and `initialize` added to KItem
      allow code to be processed separately for gathering
      declarations and for executing initializers
   */
    syntax Val
    syntax Scope ::= "global" | local(Val)

    syntax Pgm
    syntax KItem ::= declare(Pgm) | initialize(Pgm)
endmodule

module FPL-VALUES
  /** This module defines the runtime representation
      of values. These will appear in environments
      of the execution configuration, and also replacing
      already-evaluated subterms during expression evaluation.
   */
  imports BOOL-SYNTAX
  imports INT-SYNTAX
  imports STRING-SYNTAX
  imports LIST

  syntax Type ::= "[" Type ";" Int "]"
  syntax Array ::= array(List)
  syntax Val ::= Int | Bool | unit() | Array
  syntax TypedVal ::= v(Val, Type)
  syntax Exp ::= TypedVal
  syntax KResult ::= TypedVal | Int | Array // for constant expressions
  syntax FPLString ::= String
  syntax Exp ::= Val

  syntax Vals ::= List{Val,","} [klabel(exps)]
  syntax Exps ::= Vals

endmodule

module FPL-ENV
  /**
     The configuration cell `<env>` records a type and
     mutability along with a value for each current identifier.
     To more easily handle shadowing without losing identity,
     the actual values are indexed by integers in the `<store>`.
   */
  imports INT-SYNTAX

syntax Type
  syntax Mutability ::= "let" | "var"
  syntax EnvEntry ::= env(mut: Mutability, t: Type, loc: Int)
endmodule

module FPL-STACK
  /**
   FPL-STACK defines the structure of a frame on the
   function call stack.
   Frames record the current code at the call, the
   local variables, and the record of surrounding loops.
   */
  imports MAP

  syntax Scope
  syntax Frame ::= frame(code:K, env: Map, scope: Scope, loop: List)
endmodule

module FPL-FUN-ENV
  /**
   FPL-FUN-ENV defines another kind of Val to record functions.
   */
  imports FPL-COMMON-SYNTAX
  imports MAP

  syntax Val ::= funDef(args: Formals, result: Type, guards: Guards, body: Block, env: Map)
endmodule

module FPL-PANIC
  /**
   The panic form aborts the rest of execution and leaves the panic,
   with its message, as the only contents of the K cell.
   */
  imports FPL-CONFIGURATION

  syntax Exp ::= panic(String)

  rule <k> panic(_) ~> (_:KItem ~> _ => .K) </k>
endmodule

module FPL-TYPING
  /**
   FPL-TYPING defines the type checking and inference rules.
   */
  imports FPL-CONFIGURATION
  imports FPL-ENV
  imports FPL-FUN-ENV
  imports FPL-PANIC

  syntax Type ::= type(Exp) [function]
                | #type(Exp) [function]
                | infer(Type) [function]
  /**
     This productions make these specific names parse as an Ident in
     execution rules, while leaving other names available for K
     variables. The `[token]` attribute means these parse to the
     same AST representations like `#token("Bool","Ident")` as would
     be produced by the regular expressions in the FPL-TOKENS module.
   */
  syntax Ident ::= "Bool" [token]
                 | "Int8" [token]
                 | "Int16" [token]
                 | "Int32" [token]
                 | "Int64" [token]
                 | "UInt8" [token]
                 | "UInt16" [token]
                 | "UInt32" [token]
                 | "UInt64" [token]
                 | "Int" [token]

  syntax Ident ::= "IntLit" [token]

  syntax Ident ::= "length" [token] | "x" [token] | "y" [token] | "at" [token]
                 | "concat" [token] | "contains" [token] | "append" [token]
                 | "insert" [token] | "remove" [token] | "removeFirst" [token]
                 | "removeLast" [token]

  /* #type() calculates the result type of an expression, but may
     use an extra "type" IntLit when there are integer literals
     that are not forced to a specific type.
     type() produces a legal type by defaulting any unresolved
     integer literals to Int
   */
  rule type(E) => infer(#type(E))
  rule infer(IntLit) => Int
  rule infer([T]) => [infer(T)::Type]
  rule infer([T;N::Int]) => [infer(T);N]
  rule infer(T) => T [owise]

  rule #type(I:Int) => IntLit
  rule #type(B:Bool) => Bool
  rule [[ #type(X:Ident) => T ]]
       <env>... X |-> env(... t: T) ...</env>
  rule [[ #type(X:Ident(_)) => R ]]
       <env>... X |-> env(... t: ((_): R)) ...</env>
  rule #type([ E, .Exps ]) => [ #type(E) ]
  rule #type([ E1, E2, Es ]) => type([ E1 ], [ E2, Es ])
  rule #type(E1 [ E2 ]) => elementType(#type(E1))
  rule #type(E . F) => fieldType(#type(E), F)
  rule #type(E . F(_)) => returnType(fieldType(#type(E), F))
  rule #type(- E => E)
  rule #type(! _) => Bool
  rule #type(E1 * E2) => type(E1, E2)
  rule #type(E1 &* E2) => type(E1, E2)
  rule #type(E1 / E2) => type(E1, E2)
  rule #type(E1 % E2) => type(E1, E2)
  rule #type(E1 + E2) => type(E1, E2)
  rule #type(E1 &+ E2) => type(E1, E2)
  rule #type(E1 - E2) => type(E1, E2)
  rule #type(E1 &- E2) => type(E1, E2)
  rule #type(_ && _) => Bool
  rule #type(_ || _) => Bool
  rule #type(_ == _) => Bool
  rule #type(_ != _) => Bool
  rule #type(_ < _) => Bool
  rule #type(_ <= _) => Bool
  rule #type(_ > _) => Bool
  rule #type(_ >= _) => Bool
  rule #type(_ ? E1 : E2) => type(E1, E2)

  /**
   fieldType() gives the type of fields or methods.
   Currently it just gives hardcoded results for
   the built-in methods of arrays
   */
  syntax Type ::= fieldType(Type, Ident) [function]
  rule fieldType([_] #Or [_;_::Int], length) => Int
  rule fieldType(([_] #Or [_;_::Int]) #as T, concat) => ((T): T)
  rule fieldType([T] #Or [T;_::Int], contains) => ((T): Bool)
  rule fieldType([T] #Or [T;_::Int], append) => ((T): Void)
  rule fieldType([T] #Or [T;_::Int], insert) => ((Int, T): Void)
  rule fieldType([T] #Or [T;_::Int], remove) => ((Int): T)
  rule fieldType([T] #Or [T;_::Int], removeFirst) => ((.Types): T)
  rule fieldType([T] #Or [T;_::Int], removeLast) => ((.Types): T)

  syntax Type ::= type(Exp, Exp) [function]
                | #type(Type, Type) [function]
                | elementType(Type) [function]
                | returnType(Type) [function]

  rule type(E1, E2) => #type(#type(E1), #type(E2))
  rule #type(IntLit, T) => T
  rule #type(T, IntLit) => T
  rule #type(T, T) => T

  rule elementType([T]) => T
  rule elementType([T;_::Int]) => T

  rule returnType(((_): T)) => T

  syntax Int ::= "pow8" | "pow16" | "pow32" | "pow64"
  rule pow8 => 256 [macro]
  rule pow16 => 65536 [macro]
  rule pow32 => 4294967296 [macro]
  rule pow64 => 18446744073709551616 [macro]

  syntax Int ::= "min8" | "min16" | "min32" | "min64"
  rule min8 => -128 [macro]
  rule min16 => -32768 [macro]
  rule min32 => -2147483648 [macro]
  rule min64 => -9223372036854775808 [macro]

  syntax Exp ::= check(TypedVal) [function]
  rule check(v(V, T)) => #fun(V' => #if V' ==K v(V, T) #then v(V, T) #else panic("Integer overflow") #fi)(chop(v(V, T)))

  syntax Exp ::= chop(TypedVal) [function]
  rule chop(v(I, Int)) => v(I, Int)
  rule chop(v(I, UInt8)) => v(I modInt pow8, UInt8)
  rule chop(v(I, UInt16)) => v(I modInt pow16, UInt16)
  rule chop(v(I, UInt32)) => v(I modInt pow32, UInt32)
  rule chop(v(I, UInt64)) => v(I modInt pow64, UInt64)
  rule chop(v(I, Int8)) => v((I -Int min8) modInt pow8 +Int min8, Int8)
  rule chop(v(I, Int16)) => v((I -Int min16) modInt pow16 +Int min16, Int16)
  rule chop(v(I, Int32)) => v((I -Int min32) modInt pow32 +Int min32, Int32)
  rule chop(v(I, Int64)) => v((I -Int min64) modInt pow64 +Int min64, Int64)
endmodule

module FPL-CONFIGURATION
  imports DOMAINS
  imports FPL-COMMON-SYNTAX
  imports FPL-SCOPES
  imports FPL-VALUES
  imports FPL-STACK

  configuration
    <k> declare($PGM:Pgm) ~> initialize($PGM:Pgm) </k>
    <scope> global </scope>
      // scope tracks the context of the current code
    <env> .Map </env>
      // env records the variable name in scope
    <store> .Map </store>
      // values of variables are kept in the store so shadowing
      // can be handled by modifying and restoring the <env>
    <fstack> .List </fstack>
      // function call stack
    <lstack> .List </lstack>
      // records surrounding loops to support break/continue
endmodule

module FPL-VARIABLE-DECLARATION
  /**
   FPL-VARIABLE-DECLARATION handles variable declarations and also
   assignment statements.
   */
  imports FPL-CONFIGURATION
  imports FPL-TYPING

  /* These `context` declarations allow the subterm at HOLE to
     be evaluated when the side condition is true.
     This is similar to putting a `strict(3)` declaration on
     the `let _ : _ = _` production, except it only applies when
     the statement is inside the `initialize()` wrapper
   */
  context initialize(let _ : _ = HOLE)
    requires notBool isVal(HOLE)
  context initialize(var _ : _ = HOLE)
    requires notBool isVal(HOLE)

  /* These rules add the type information from the environment
     onto a computed Val during initialization
   */
  rule <k> initialize(let X : _ = (V:Val => v(V, T))) ... </k>
       <env>... X |-> env(... t: T) ...</env>
  rule <k> initialize(var X : _ = (V:Val => v(V, T))) ... </k>
       <env>... X |-> env(... t: T) ...</env>

  /* When processing declarations the type and mutability is
     recorded into `<env>`, and a fresh store index is selected.
     (!L evaluates to a fresh integer each time the rule applies)
   */
  rule <k> declare(let X : T:Type = E) => . ... </k>
       <env> Rho => Rho [ X <- env(let, T, !L:Int) ] </env>
  rule <k> declare(var X : T:Type = E) => . ... </k>
       <env> Rho => Rho [ X <- env(var, T, !L:Int) ] </env>

  /* When the initializer has become a typed value,
     the Val part is written into the previously-selected store index
     (the type is expected to match)
   */
  rule <k> initialize(let X : _ = v(V, _)) => . ... </k>
       <env>... X |-> env(_, _, L:Int) ...</env>
       <store>... .Map => L |-> V ...</store>
  rule <k> initialize(var X : _ = v(V, _)) => . ... </k>
       <env>... X |-> env(_, _, L:Int) ...</env>
       <store>... .Map => L |-> V ...</store>

  /* A declaration with no type annotation has the expected type
     filled in based on the initializer
   */
  rule <k> declare(let _ : (noAnnot() => type(E)) = E) ... </k>
  rule <k> declare(var _ : (noAnnot() => type(E)) = E) ... </k>

  /* Simple variable lookups retrieve a value from the store
   */
  rule <k> X:Ident => v(V, T) ... </k>
       <env>... X |-> env(... t: T, loc: L) ...</env>
       <store>... L |-> V ...</store>

  rule <k> X:Ident => panic("Declaration not initialized yet") ... </k>
       <env>... X |-> env(... t: T, loc: L) ...</env>
       <store> S </store>
    requires notBool L in_keys(S)

  rule <k> declare(X ; Y ; Z) => declare(X) ~> declare(Y ; Z) ... </k>
  rule <k> declare(.Pgm) => . ... </k>
  rule <k> initialize(X ; Y ; Z) => initialize(X) ~> initialize(Y ; Z) ... </k>
  rule <k> initialize(.Pgm) => . ... </k>

  rule <k> (let X : T = E) #as D::Definition => declare(D) ~> initialize(D) ... </k>
  rule <k> (var X : T = E) #as D::Definition => declare(D) ~> initialize(D) ... </k>

  /**
   For an assigment the LHS may contain array indexing.
   It is evaluted to a `Loc`, which contains a store index along with
   and optional list of (nested) array indices.
   */
  syntax Path ::= loc(Path)
  syntax Path ::= Loc
  syntax Loc ::= loc(Int, List, Type)
  syntax KResult ::= Loc

  /* This form of context puts the subterm to be evaluated inside the loc(Path) wrapper,
     and expects to find the result inside loc as well. It's equivalent to rules
```
  rule (H::Path = R) => loc(H) ~> []= R
     requires notBool isKResult(H)
  rule loc(H) ~> []= R => H = R
     requires isKResult(H)
```
   where `"[]=" Exp` stands for the compiler-generated production that is
   places in the <k> cell to record the rest of the expression after the
   LHS is promoted into its own item in the <k> cell
   */
  context (HOLE::Path => loc(HOLE)) = _

  /* when the path is reduced to a plain identifier, the store index
     and expected type are found in the environment
   */
  rule <k> loc(X:Ident) => loc(loc(L, .List, T)) ... </k>
       <env>... X |-> env(... t: T, loc: L) ...</env>
  /* When a path is an indexing expression, the index value and the
     location of the target array need to be calculated first,
     which is allowed by these `context` declarations
   */
  context loc((HOLE::Path => loc(HOLE)) [ _ ])
  context loc(loc(_, _, _) [ HOLE ] )
  rule loc(loc(_, _, _) [ v(I:Int, _) => I ] )
  // When the target loc and index are availabe, the index is added to the list of indices
  rule loc(loc(L, Idx, T) [ I:Int ]) => loc(loc(L, Idx ListItem(I), elementType(T)))

  context loc(_, _, _) = HOLE

  /* Onces the LHS is reduced to a location and the RHS has been evaluated,
     the updateLoc function is used to update the at the given indices within
     the target store slot.
   */
  rule <k> loc(L, Idx, _) = (V':Val #Or v(V':Val, _)) => . ... </k>
       <store>... L |-> (V => updateLoc(Idx, V, V')) ...</store>
    requires isValid(V, Idx)
  rule <k> loc(L, Idx, _) = (_:Val #Or v(_, _)) => panic("Array index out of bounds") ... </k>
       <store>... L |-> V ...</store>
    requires notBool isValid(V, Idx)

  syntax Bool ::= isValid(Val, List) [function]
  rule isValid(_, .List) => true
  rule isValid(array(L::List), ListItem(I:Int) Idx) => I >=Int 0 andBool I <Int size(L) andBool isValid({L [ I ]}:>Val, Idx)

  syntax Val ::= updateLoc(List, Val, Val) [function]
  rule updateLoc(.List, _, V) => V
  rule updateLoc(ListItem(I:Int) L, array(V:List), V') => array(V [ I <- updateLoc(L, {V [ I ]}:>Val, V') ])

  syntax Val ::= getLoc(List, Val) [function]
  rule getLoc(.List, V) => V
  rule getLoc(ListItem(I:Int) L, array(V:List)) => getLoc(L, {V [ I ]}:>Val)

endmodule

module FPL-STMTS
  /**
   FPL-STMTS defines the execution of statements.
   break and continue are handled by maintaining a stack
   of surrounding loops.
   */
  imports FPL-CONFIGURATION

  rule .Block => .K
  rule E:Exp ; B::Block => expressionStmt(E) ~> B
  rule S; B::Block => S ~> B requires notBool isExp(S)

  syntax Stmt ::= expressionStmt(Exp) [strict]
  rule expressionStmt(_:Int) => .
  rule expressionStmt(v(...)) => .

  context if HOLE:Exp { _ } else { _ }
  rule if v(true, _) { B } else { _ } => B
  rule if v(false, _) { _ } else { B } => B

  /* while statements use loop() to record the context, and
     handle the condition by exapnding to an if, break, and continue
   */
  rule while E { B } => loop(if E { loopBody(B) } else { break })
  syntax Stmt ::= loopBody(Block)
  rule loopBody(B) => B ~> continue

  /* The loop() command pushes an entry onto the loop stack.
     It moves the code following the loop, so the body must finish
     with `continue` or `break`, and records a copy of the loop
     body to support `continue` and a copy of the in-scope variables
     from `env` to support `break`.
     The function call stack is also recorded, which allows a
     break/continue inide of a local closure.
   */
  syntax Stmt ::= loop(Stmt)
  rule <k> loop(Stmt) ~> K => Stmt </k>
       <env> Rho </env>
       <fstack> FS </fstack>
       <lstack> .List => ListItem(loopState(Rho, K, Stmt, FS)) ... </lstack>

  syntax LoopFrame ::= loopState(Map, K, Stmt, List)

  rule <k> break ~> _ => K </k>
       <env> _ => Rho </env>
       <fstack> _ => FS </fstack>
       <lstack> ListItem(loopState(Rho, K, _, FS)) => .List ...</lstack>
  rule <k> continue ~> _ => K </k>
       <env> _ => Rho </env>
       <fstack> _ => FS </fstack>
       <lstack> ListItem(loopState(Rho, _, K, FS)) ...</lstack>
endmodule

module FPL-FUNCTIONS
  /**
    Function calls check that the argument types and labels match
    the declaration, then push the current code and environment
    onto the function call stack `<fstack>`, and set up the
    function body to run with the arguments available.
   */
  imports FPL-CONFIGURATION
  imports FPL-TYPING
  imports FPL-VARIABLE-DECLARATION

  rule <k> declare(fun F:Ident ( Fs ) : T { _::Guards _ }) => . ... </k>
       <env> Rho => Rho [ F <- env(let, ((types(Fs)): T), !L:Int) ] </env>

  rule <k> initialize(fun F:Ident ( Fs ) : T { C B }) => . ... </k>
       <env> (_::Map F |-> env(... loc: L)) #as Rho </env>
       <store>... .Map => L |-> funDef(Fs, T, C, B, Rho) ...</store>

  /* types drops labels to calculate the argument-type part of a function type
   */
  syntax Types ::= types(Formals) [function]
  rule types(.Formals) => .Types
  rule types(_ _ : T, Fs) => T, types(Fs)

  /* checkLabels checks that the argument list in a call is written
     with the correct labels */
  syntax Bool ::= checkLabels(Formals,Actuals) [function]
  rule checkLabels(.Formals, .Actuals) => true
  rule checkLabels(((Lbl _ : _), FS), ((ArgLbl : _), AS)) => checkLabels(FS,AS)
    requires Lbl ==K ArgLbl
  rule checkLabels(((Lbl _ : _), FS), (_::Exp, AS)) => checkLabels(FS,AS)
    requires Lbl ==K #token("_","Ident")
  rule checkLabels(_,_) => false [owise]

  /* The initArgs function contructs the initial variable environment
     from the list of formals and actuals.
     Labels have been checked, so this just enforces that some label is present
     where expected
   */
  syntax Block ::= initArgs(Formals,Actuals) [function]
  syntax Stmt ::= initArg(Formal,Actual) [function]
  rule initArgs(.Formals, .Actuals) => .Block
  rule initArgs((F, FS), (A, AS)) => initArg(F,A) ; initArgs(FS,AS)

  rule initArg((Lbl Name : T), (ArgLbl : X)) => let Name : T = X
  rule initArg((NoLbl Name : T), X) => let Name : T = X
    requires NoLbl ==K #token("_","Ident")

  rule <k> (F:Ident)(_::Actuals) => panic("Declaration not initialized yet") ... </k>
       <env>... F |-> env(... loc: L) ...</env>
       <store> S </store>
    requires notBool L in_keys(S)

  rule <k> (F:Ident)(Args) => call v(V, T) (Args) ... </k>
       <env>... F |-> env(... t: T, loc: L) ...</env>
       <store>... L |-> V ...</store>
    requires isKResult(Args)

  /* These declarations evaluate the target of field lookups and method calls
     on paths or non-path expressions
   */
  context (HOLE::Path => loc(HOLE)) . _::Ident
  context HOLE . _::Ident requires notBool isPath(HOLE)

  context (HOLE::Path => loc(HOLE)) . _::Ident ( _::Actuals )
  context E . _::Ident ( HOLE::Actuals ) requires isKResult(E)
  context HOLE . _::Ident ( _::Actuals ) requires notBool isPath(HOLE)

  rule <k> V . F:Ident (Args) => call V . F (Args) ... </k>
    requires isKResult(Args) andBool isKResult(V)

  syntax Exp ::= "call" Exp "(" Actuals ")" [strict(1)]

  rule <k> (call v(funDef(... args: Params, body: B, env: Rho') #as V, _) (Args) ~> K) => initArgs(Params, Args) ~> B:Block</k>
       <env> Rho => Rho' </env>
       <scope> Scope => local(V) </scope>
       <fstack> .List => ListItem(frame(K, Rho, Scope, LS)) ...</fstack>
       <lstack> LS => .List </lstack>
  rule <k> return => return v(unit(), Void) ... </k>
  rule <k> return X:TypedVal ~> _ => X ~> K </k>
       <env> _ => Rho </env>
       <scope> _ => Scope </scope>
       <fstack> ListItem(frame(K, Rho, Scope, LS)) => .List ...</fstack>
       <lstack> _ => LS </lstack>
  rule <k> return V:Val => return v(V, T) ... </k>
       <scope> local(funDef(... result: T)) </scope>

endmodule

module FPL-ARITHMETIC
  /**
   Arithmetic operations use K's primitive Int operations,
   then either check for overflow with check() or enforce wrapping with chop(),
   according to the type of the values.
   */
  imports FPL-CONFIGURATION
  imports FPL-TYPING

  /* Integer literals are handled by rewriting the string contents of
     the literal into a form that can be parsed by K's standard library
     functions String2Int and String2Base
   */
  rule intLit(I::IntLit) => processIntLit(replaceAll(IntLit2String(I), "_", ""))
  /* The attribute `hook(STRING.token2string)` implements this function with
     a primitive of the K runtime that returns the string contents from an argument
     of any sort that was defined with `[token]`.
   */
  syntax String ::= IntLit2String(IntLit) [function, hook(STRING.token2string)]
  syntax Int ::= processIntLit(String) [function]
  rule processIntLit(Neg::String) => 0 -Int processIntLit(substrString(Neg, 1, lengthString(Neg)))
    requires substrString(Neg, 0, 1) ==String "-"
  rule processIntLit(Hex::String) => String2Base(substrString(Hex, 2, lengthString(Hex)), 16)
    requires lengthString(Hex) >=Int 2 andBool substrString(Hex, 0, 2) ==String "0x"
  rule processIntLit(Oct::String) => String2Base(substrString(Oct, 2, lengthString(Oct)), 8)
    requires lengthString(Oct) >=Int 2 andBool substrString(Oct, 0, 2) ==String "0o"
  rule processIntLit(Bin::String) => String2Base(substrString(Bin, 2, lengthString(Bin)), 2)
    requires lengthString(Bin) >=Int 2 andBool substrString(Bin, 0, 2) ==String "0b"
  rule processIntLit(Dec::String) => String2Int(Dec) [owise]

  rule [ T::Type ; I::IntLit ] => [ T ; {intLit(I)}:>Int ] [macro]

  /** This uses the pattern union #Or to extract the left and right
      operands from any arithmetic expression
   */
  syntax Exp ::= intExp(Exp, Exp)
  rule intExp(L, R)
    => ((L * R)
   #Or ((L &* R)
   #Or ((L / R)
   #Or ((L &/ R)
   #Or ((L % R)
   #Or ((L + R)
   #Or ((L &+ R)
   #Or ((L - R)
   #Or ((L &- R)
   #Or ((L == R)
   #Or ((L != R)
   #Or ((L < R)
   #Or ((L <= R)
   #Or ((L > R)
   #Or (L >= R)))))))))))))))::Exp [macro]

  /* These rules use the `Pat #as Var` pattern construct to name the entire
     original expression as O, while they extract the operands and propagate a type from
     one side to the integer literal from the other.
     Then fillExp uses the original expression as a template to plug in the now-fully-typed
     operands v(I1,T) and v(I2,T) */
  rule <k> intExp(I1:Int, v(I2, T)) #as O::Exp => fillExp(v(I1, T), v(I2, T), O) ... </k>
  rule <k> intExp(v(I1, T), I2:Int) #as O::Exp => fillExp(v(I1, T), v(I2, T), O) ... </k>

  syntax Exp ::= fillExp(TypedVal, TypedVal, Exp) [function]
  rule fillExp(L, R, _ * _) => L * R
  rule fillExp(L, R, _ &* _) => L &* R
  rule fillExp(L, R, _ / _) => L / R
  rule fillExp(L, R, _ &/ _) => L &/ R
  rule fillExp(L, R, _ % _) => L % R
  rule fillExp(L, R, _ + _) => L + R
  rule fillExp(L, R, _ &+ _) => L &+ R
  rule fillExp(L, R, _ - _) => L - R
  rule fillExp(L, R, _ &- _) => L &- R
  rule fillExp(L, R, _ == _) => L == R
  rule fillExp(L, R, _ != _) => L != R
  rule fillExp(L, R, _ < _) => L < R
  rule fillExp(L, R, _ <= _) => L <= R
  rule fillExp(L, R, _ > _) => L > R
  rule fillExp(L, R, _ >= _) => L >= R

  /*
   All the operations are implemented with K's operations
   on Int, checking for overlow with `check` or enforcing
   wrapping with `chop` as needed.
   */
  rule <k> B:Bool => v(B, Bool) ... </k>

  rule <k> - I:Int => 0 -Int I ... </k>
  rule <k> - v(I, T) => v(0 -Int I, T) ... </k>

  rule <k> ! v(B, T) => v(notBool B, T) ... </k>

  rule <k> I1  * I2 => I1 *Int I2 ... </k>
  rule <k> I1 &* I2 => I1 *Int I2 ... </k>
  rule <k> I1  / I2 => I1 /Int I2 ... </k> requires I2 =/=Int 0
  rule <k> I1 &/ I2 => I1 /Int I2 ... </k> requires I2 =/=Int 0
  rule <k> I1  % I2 => I1 %Int I2 ... </k> requires I2 =/=Int 0
  rule <k> I1  + I2 => I1 +Int I2 ... </k>
  rule <k> I1 &+ I2 => I1 +Int I2 ... </k>
  rule <k> (I1  - I2)::Exp => I1 -Int I2 ... </k>
  rule <k> I1 &- I2 => I1 -Int I2 ... </k>
  rule <k> I1 == I2 => v(I1 ==Int I2, Bool) ... </k>
  rule <k> I1 != I2 => v(I1 =/=Int I2, Bool) ... </k>
  rule <k> I1 < I2 => v(I1 <Int I2, Bool) ... </k>
  rule <k> I1 <= I2 => v(I1 <=Int I2, Bool) ... </k>
  rule <k> I1 > I2 => v(I1 >Int I2, Bool) ... </k>
  rule <k> I1 >= I2 => v(I1 >=Int I2, Bool) ... </k>

  rule <k> v(I1, T)  * v(I2, T) => check(v(I1 *Int I2, T)) ... </k>
  rule <k> v(I1, T) &* v(I2, T) => chop (v(I1 *Int I2, T)) ... </k>
  rule <k> v(I1, T)  / v(I2, T) => check(v(I1 /Int I2, T)) ... </k> requires I2 =/=Int 0
  rule <k> v(I1, T) &/ v(I2, T) => chop (v(I1 /Int I2, T)) ... </k> requires I2 =/=Int 0
  rule <k> v(I1, T)  % v(I2, T) => check(v(I1 %Int I2, T)) ... </k> requires I2 =/=Int 0
  rule <k> v(I1, T)  + v(I2, T) => check(v(I1 +Int I2, T)) ... </k>
  rule <k> v(I1, T) &+ v(I2, T) => chop (v(I1 +Int I2, T)) ... </k>
  rule <k> (v(I1, T)  - v(I2, T))::Exp => check(v(I1 -Int I2, T)) ... </k>
  rule <k> v(I1, T) &- v(I2, T) => chop (v(I1 -Int I2, T)) ... </k>
  rule <k> v(I1, _) == v(I2, _) => v(I1 ==Int I2, Bool) ... </k>
  rule <k> v(I1, _) != v(I2, _) => v(I1 =/=Int I2, Bool) ... </k>
  rule <k> v(I1, T) < v(I2, T) => v(I1 <Int I2, Bool) ... </k>
  rule <k> v(I1, T) <= v(I2, T) => v(I1 <=Int I2, Bool) ... </k>
  rule <k> v(I1, T) > v(I2, T) => v(I1 >Int I2, Bool) ... </k>
  rule <k> v(I1, T) >= v(I2, T) => v(I1 >=Int I2, Bool) ... </k>

  rule <k> v(B1, _) == v(B2, _) => v(B1 ==Bool B2, Bool) ... </k>
  rule <k> v(B1, _) != v(B2, _) => v(B1 =/=Bool B2, Bool) ... </k>

  rule <k> v(true, T) && E => E ... </k>
  rule <k> v(false, T) && _ => v(false, Bool) ... </k>
  rule <k> v(true, T) || E => v(true, Bool) ... </k>
  rule <k> v(false, T) || E => E ... </k>

  rule v(true, T) ? E1 : E2 => giveType(E1, type(E1, E2))
  rule v(false, T) ? E1 : E2 => giveType(E2, type(E1, E2))

  syntax Exp ::= giveType(Exp, Type) [strict(1)]
  rule giveType(v(...) #as V, _) => V
  rule giveType(I:Int, IntLit) => I
  rule giveType(I:Int, T) => v(I, T) requires T =/=K IntLit

endmodule

module FPL-ARRAYS
  /**
    FPL-Arrays defines the evaluation of array literals,
    array indexing expressions, and built-in methods of arrays.
    Assigments with array indexing on the LHS are handled in FPL-VARIABLE-DECLARATION
   */
  imports FPL-CONFIGURATION
  imports FPL-TYPING
  imports FPL-VARIABLE-DECLARATION

  rule [ Vs ] => array(arrayLit(Vs))
    requires isKResult(Vs)
  syntax List ::= arrayLit(Exps) [function]
  rule arrayLit(.Vals) => .List
  rule arrayLit(V:Val, Vs) => ListItem(V) arrayLit(Vs)
  rule arrayLit(v(V, _), Vs) => ListItem(V) arrayLit(Vs)

  /* These rule's use K's library function `KItem = List "[" Int "]"` to implement indexing
   */
  rule (_::Exp [ v(I:Int, _) => I ])::Exp
  rule (v(array(L::List), [T]) [ I:Int ])::Exp => v({L [ I ]}:>Val, T)
    requires I >=Int 0 andBool I <Int size(L)
  rule (v(array(L::List), [T ; _:Int ]) [ I:Int ])::Exp => v({L [ I ]}:>Val, T)
    requires I >=Int 0 andBool I <Int size(L)
  rule (v(array(L::List), _) [ I:Int ])::Exp => panic("Array indexing out of bounds")
    requires I <Int 0 orBool I >=Int size(L)

  /* Method calls on arrays are handled by having the `arr . method` projection evaluate to a function value,
     which will perform the requested operation with the special `arrayBuiltin()` KItem.
   */
  rule <k> loc(Loc, Idx, [_] #Or [_;_::Int]) . length => #fun(array(L) => v(size(L), Int))(getLoc(Idx, V)) ... </k>
       <store>... Loc |-> V </store>
  rule loc(L, Idx, ([_] #Or [_;_::Int]) #as T) . concat => v(funDef(#token("_", "Ident") x: T, T, pre { .Conditions }, return arrayBuiltin(L, Idx, concat, x), .Map), ((T): T))
  rule loc(L, Idx, [T] #Or [T;_::Int]) . contains => v(funDef(#token("_", "Ident") x: T, Bool, pre { .Conditions }, return arrayBuiltin(L, Idx, contains, x), .Map), ((T): Bool))
  rule v(array(L::List), _) . length => v(size(L), Int)
  rule v(array(L::List), T) . concat => v(funDef(#token("_", "Ident") x: T, T, pre { .Conditions }, return arrayBuiltin(L, concat, x), .Map), ((T): T))
  rule v(array(L::List), [T]) . contains => v(funDef(#token("_", "Ident") x: T, Bool, pre { .Conditions }, return arrayBuiltin(L, contains, x), .Map), ((T): Bool))
  rule loc(L, Idx, [T]) . append => v(funDef(#token("_", "Ident") x: T, Void, pre { .Conditions }, arrayBuiltin(L, Idx, append, x), .Map), ((T): Void))
  rule loc(L, Idx, [T]) . insert => v(funDef(at x: Int, #token("_", "Ident") y: T, Void, pre { .Conditions }, arrayBuiltin(L, Idx, insert, x, y), .Map), ((Int, T): Void))
  rule loc(L, Idx, [T]) . remove => v(funDef(at x: Int, T, pre { .Conditions }, return arrayBuiltin(L, Idx, remove, x), .Map), ((Int): T))
  rule loc(L, Idx, [T]) . removeFirst => v(funDef(.Formals, T, pre { .Conditions }, return arrayBuiltin(L, Idx, removeFirst, .Exps), .Map), ((.Types): T))
  rule loc(L, Idx, [T]) . removeLast => v(funDef(.Formals, T, pre { .Conditions }, return arrayBuiltin(L, Idx, removeLast, .Exps), .Map), ((.Types): T))

  syntax Exp ::= arrayBuiltin(List, Ident, Exps) [strict(3)]
  rule arrayBuiltin(L, concat, v(array(L2), _)) => array(L L2)
  rule arrayBuiltin(L, contains, v(V, _)) => V in L

  /* arrayBuiltin() does the work for most of the methods.
     K's `#fun(LHS => RHS)(Term)` contruct applies an anonymous rewrite rule to a given term,
     otherwise we would need to define several `[function]` productions
   */
  syntax Exp ::= arrayBuiltin(Int, List, Ident, Exps) [strict(4)]
  rule <k> arrayBuiltin(Loc, Idx, concat, v(array(L2), _)) => #fun(array(L) => array(L L2))(getLoc(Idx, V)) ... </k>
       <store>... Loc |-> V ... </store>
  rule <k> arrayBuiltin(Loc, Idx, contains, v(V', _)) => #fun(array(L) => V' in L)(getLoc(Idx, V)) ... </k>
       <store>... Loc |-> V ... </store>
  rule <k> arrayBuiltin(Loc, Idx, append, v(V', _)) => v(unit(), Void) ... </k>
       <store>... Loc |-> (V => updateLoc(Idx, V, #fun(array(L) => array(L ListItem(V')))(getLoc(Idx, V)))) ...</store>
  rule <k> arrayBuiltin(Loc, Idx, insert, v(At, _), v(V', _)) => v(unit(), Void) ... </k>
       <store>... Loc |-> (V => updateLoc(Idx, V, #fun(array(L) => array(range(L, 0, size(L) -Int At) ListItem(V') range(L, At, 0)))(getLoc(Idx, V)))) ...</store>
    requires At >=Int 0 andBool At <Int #fun(array(L) => size(L))(getLoc(Idx, V))
  rule <k> arrayBuiltin(Loc, Idx, insert, v(At, _), _) => panic("Array indexing out of bounds") ... </k>
       <store>... Loc |-> V ... </store>
    requires At <Int 0 orBool At >=Int #fun(array(L) => size(L))(getLoc(Idx, V))
  rule <k> arrayBuiltin(Loc, Idx, remove, v(At, _)) => getLoc(Idx ListItem(At), V) ... </k>
       <store>... Loc |-> (V => updateLoc(Idx, V, #fun(array(L) => array(range(L, 0, size(L) -Int At) range(L, At +Int 1, 0)))(getLoc(Idx, V)))) ...</store>
    requires At >=Int 0 andBool At <Int #fun(array(L) => size(L))(getLoc(Idx, V))
  rule <k> arrayBuiltin(Loc, Idx, remove, v(At, _)) => panic("Array indexing out of bounds") ... </k>
       <store>... Loc |-> V ... </store>
    requires At <Int 0 orBool At >=Int #fun(array(L) => size(L))(getLoc(Idx, V))
  rule <k> arrayBuiltin(Loc, Idx::List, removeFirst, .Exps) => arrayBuiltin(Loc, Idx, remove, v(0, Int)) ... </k>
  rule <k> arrayBuiltin(Loc, Idx, removeLast, .Exps) => arrayBuiltin(Loc, Idx, remove, v(#fun(array(L) => size(L) -Int 1)(getLoc(Idx, V)), Int)) ... </k>
       <store>... Loc |-> V ...</store>
endmodule

module FPL
  /**
    FPL gathers the execution semantics of the flow programming language
   */
  imports FPL-CONFIGURATION
  imports FPL-VARIABLE-DECLARATION
  imports FPL-STMTS
  imports FPL-FUNCTIONS
  imports FPL-ARITHMETIC
  imports FPL-ARRAYS

  rule T::TopDefinition ; Ts::Pgm => T ~> Ts
endmodule

module FPL-TESTING-COMMON-SYNTAX
  imports FPL-COMMON-SYNTAX
  syntax Definition ::= "assert" Exp [strict]
endmodule

module FPL-TESTING-SYNTAX
  /**
    FPL-TESTING-SYNTAX adds the assert form to
    the program grammar from FPL-SYNTAX
   */
  imports FPL-SYNTAX
  imports FPL-TESTING-COMMON-SYNTAX
endmodule

module FPL-TESTING
  /**
    FPL-TESTING adds assert as a top-level declaration,
    so tests do not need to depend on function support
    It also adds extra rules that clear parts of the configuration
    after execution finishes, so we can more easily write an
    expected final state to be checked after tests.
   */
  imports FPL
  imports FPL-TESTING-COMMON-SYNTAX

  rule <k> initialize(assert E) => assert E ... </k>
  rule <k> assert v(true, Bool) => . ... </k>
  rule <k> declare(assert _) => . ... </k>

  rule <k> .K #Or panic(_) </k>
       <scope> Scope => global </scope>
       <env> E => .Map </env>
       <store> State => .Map </store>
       <fstack> FS => .List </fstack>
       <lstack> LS => .List </lstack>
    requires E =/=K .Map orBool State =/=K .Map orBool Scope =/=K global orBool FS =/=K .List orBool LS =/=K .List

  rule <k> panic(S::String => "") </k>
    requires S =/=String ""
endmodule
